package depsaccept

import (
	"context"
	"fmt"
	"strings"
)

// Release creates a GitHub release for the current VERSION without touching
// any pull request. It determines the version, waits for CI, creates the
// GitHub release with auto-generated notes, and verifies the install.
func Release(ctx context.Context, cfg Config) error {
	cfg = normalizeConfig(cfg)
	w := workflow{cfg: cfg}
	return w.release(ctx)
}

func (w workflow) release(ctx context.Context) error {
	w.banner("🚀 release")

	root, err := w.repoRoot(ctx)
	if err != nil {
		return err
	}

	if !w.cfg.DryRun {
		if err := w.requireCleanMaster(ctx, root); err != nil {
			return err
		}
		if err := w.syncMaster(ctx, root); err != nil {
			return err
		}
	}

	if err := w.cfg.runner.run(ctx, gitCommand(root, "fetch", "--tags", "origin")); err != nil {
		return fmt.Errorf("fetch tags: %w", err)
	}

	versionFile := w.versionFile(root)
	currentVersion, err := readVersion(versionFile)
	if err != nil {
		return fmt.Errorf("read VERSION: %w", err)
	}

	releaseVersion, versionChanged, err := w.firstFreeVersion(ctx, root, currentVersion)
	if err != nil {
		return err
	}

	w.step("📦", "VERSION=%s, release tag=%s", releaseVersion, tagName(releaseVersion))
	if versionChanged {
		w.step("📦", "tag %s already exists, bumping VERSION %s → %s", tagName(currentVersion), currentVersion, releaseVersion)
	}

	if w.cfg.DryRun {
		w.step("🏃", "dry-run: no commits, pushes, tags, or releases were created")
		if versionChanged {
			w.step("☑️", "would write VERSION=%s, commit it, and push master", releaseVersion)
		}
		w.step("☑️", "would wait for CI on HEAD")
		w.step("☑️", "would create GitHub release %s with auto-generated notes", tagName(releaseVersion))
		w.step("☑️", "would wait for release workflow to build and upload binaries")
		w.step("☑️", "would verify GOPROXY=direct go install and run smoke")
		return nil
	}

	if versionChanged {
		w.step("📦", "bumping VERSION %s → %s", currentVersion, releaseVersion)
		if err := writeVersion(versionFile, releaseVersion); err != nil {
			return fmt.Errorf("write VERSION: %w", err)
		}
		if err := w.commitAndPushVersion(ctx, root, releaseVersion); err != nil {
			return err
		}
	}

	headSHA, err := w.gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	headSHA = strings.TrimSpace(headSHA)

	if err := w.waitForWorkflow(ctx, root, "ci.yml", headSHA, "master CI"); err != nil {
		return err
	}

	w.step("🏷️", "creating GitHub release %s", tagName(releaseVersion))
	if err := w.createGitHubRelease(ctx, root, releaseVersion, headSHA); err != nil {
		return err
	}

	if err := w.waitForWorkflow(ctx, root, "release.yml", headSHA, "release workflow"); err != nil {
		return err
	}

	if err := w.verifyRemoteInstall(ctx, root, releaseVersion); err != nil {
		return err
	}

	w.step("✅", "released %s", tagName(releaseVersion))
	return nil
}

func (w workflow) createGitHubRelease(ctx context.Context, root string, version string, targetSHA string) error {
	tag := tagName(version)
	args := []string{"release", "create", tag, "--target", targetSHA, "--title", tag, "--generate-notes"}
	if err := w.cfg.runner.run(ctx, ghCommand(root, args...)); err != nil {
		return fmt.Errorf("create GitHub release %s: %w", tag, err)
	}
	_ = w.cfg.runner.run(ctx, gitCommand(root, "tag", tag))
	return nil
}
