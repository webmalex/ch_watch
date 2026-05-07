package depsaccept

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultVersionFile = "VERSION"
	defaultPollLimit   = 60
	defaultPollEvery   = 5 * time.Second
	modulePath         = "github.com/webmalex/ch_watch"
)

type Config struct {
	PR             int
	DryRun         bool
	VersionFile    string
	WorktreeParent string
	Stdout         io.Writer
	Stderr         io.Writer
	runner         runner
	mkdirTemp      func(string, string) (string, error)
	removeAll      func(string) error
	sleep          func(time.Duration)
	watchSmoke     func(context.Context, string, string, io.Writer, io.Writer) error
	pollLimit      int
	pollEvery      time.Duration
}

type pullRequest struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	State       string `json:"state"`
	URL         string `json:"url"`
	IsDraft     bool   `json:"isDraft"`
	HeadRefName string `json:"headRefName"`
	BaseRefName string `json:"baseRefName"`
	HeadRefOID  string `json:"headRefOid"`
	Author      author `json:"author"`
	Files       []file `json:"files"`
}

type author struct {
	Login string `json:"login"`
}

type file struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type workflowRun struct {
	DatabaseID int64  `json:"databaseId"`
	HeadSHA    string `json:"headSha"`
	HeadBranch string `json:"headBranch"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

type workflow struct {
	cfg Config
}

func Run(ctx context.Context, cfg Config) error {
	cfg = normalizeConfig(cfg)
	w := workflow{cfg: cfg}
	return w.run(ctx)
}

func normalizeConfig(cfg Config) Config {
	if cfg.VersionFile == "" {
		cfg.VersionFile = defaultVersionFile
	}
	if cfg.Stdout == nil {
		cfg.Stdout = io.Discard
	}
	if cfg.Stderr == nil {
		cfg.Stderr = io.Discard
	}
	if cfg.runner == nil {
		cfg.runner = realRunner{stdout: cfg.Stdout, stderr: cfg.Stderr}
	}
	if cfg.mkdirTemp == nil {
		cfg.mkdirTemp = os.MkdirTemp
	}
	if cfg.removeAll == nil {
		cfg.removeAll = os.RemoveAll
	}
	if cfg.sleep == nil {
		cfg.sleep = time.Sleep
	}
	if cfg.watchSmoke == nil {
		cfg.watchSmoke = defaultWatchSmoke
	}
	if cfg.pollLimit == 0 {
		cfg.pollLimit = defaultPollLimit
	}
	if cfg.pollEvery == 0 {
		cfg.pollEvery = defaultPollEvery
	}
	return cfg
}

func (w workflow) run(ctx context.Context) error {
	w.banner("🧰 deps_accept")

	root, err := w.repoRoot(ctx)
	if err != nil {
		return err
	}
	versionFile := w.versionFile(root)
	currentVersion, err := readVersion(versionFile)
	if err != nil {
		return fmt.Errorf("read VERSION: %w", err)
	}

	pr, err := w.findPullRequest(ctx, root)
	if err != nil {
		return err
	}
	if err := validatePullRequest(pr); err != nil {
		return err
	}

	releaseVersion, versionWouldChange, err := w.previewReleaseVersion(ctx, root, currentVersion)
	if err != nil {
		return err
	}

	w.step("🔎", "Dependabot PR #%d: %s", pr.Number, pr.Title)
	if pr.URL != "" {
		w.step("🔗", "%s", pr.URL)
	}
	w.step("📦", "VERSION=%s, release tag=%s", currentVersion, tagName(releaseVersion))
	if versionWouldChange {
		w.step("📦", "VERSION tag already exists; would bump patch to %s", releaseVersion)
	}

	w.showDiffStat(pr)

	if w.cfg.DryRun {
		w.step("🏃", "dry-run: no merge, commits, pushes, tags, or worktrees were created")
		w.printPlan(pr, releaseVersion, versionWouldChange)
		return nil
	}

	if err := w.requireCleanMaster(ctx, root); err != nil {
		return err
	}
	if err := w.localGate(ctx, root, pr.Number); err != nil {
		return err
	}
	if err := w.remotePRGate(ctx, root, pr.Number); err != nil {
		return err
	}
	if err := w.mergePullRequest(ctx, root, pr); err != nil {
		return err
	}
	if err := w.syncMaster(ctx, root); err != nil {
		return err
	}

	releaseVersion, versionChanged, err := w.ensureReleaseVersion(ctx, root, versionFile)
	if err != nil {
		return err
	}
	if versionChanged {
		if err := w.commitAndPushVersion(ctx, root, releaseVersion); err != nil {
			return err
		}
	}

	headSHA, err := w.gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if err := w.waitForWorkflow(ctx, root, "ci.yml", strings.TrimSpace(headSHA), "master CI"); err != nil {
		return err
	}
	if err := w.tagAndPush(ctx, root, releaseVersion); err != nil {
		return err
	}
	if err := w.waitForWorkflow(ctx, root, "release.yml", strings.TrimSpace(headSHA), "release workflow"); err != nil {
		return err
	}
	if err := w.verifyRemoteInstall(ctx, root, releaseVersion); err != nil {
		return err
	}

	w.step("✅", "accepted PR #%d and released %s", pr.Number, tagName(releaseVersion))
	return nil
}

func (w workflow) repoRoot(ctx context.Context) (string, error) {
	out, err := w.gitOutput(ctx, "", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("detect repository root: %w", err)
	}
	root := strings.TrimSpace(out)
	if root == "" {
		return "", errors.New("detect repository root: empty git output")
	}
	w.step("🧭", "repo=%s", root)
	return root, nil
}

func (w workflow) versionFile(root string) string {
	if filepath.IsAbs(w.cfg.VersionFile) {
		return w.cfg.VersionFile
	}
	return filepath.Join(root, w.cfg.VersionFile)
}

func (w workflow) findPullRequest(ctx context.Context, root string) (pullRequest, error) {
	fields := "number,title,state,url,isDraft,headRefName,baseRefName,headRefOid,author,files"
	if w.cfg.PR > 0 {
		out, err := w.cfg.runner.output(ctx, ghCommand(root, "pr", "view", fmt.Sprintf("%d", w.cfg.PR), "--json", fields))
		if err != nil {
			return pullRequest{}, fmt.Errorf("load PR #%d: %w", w.cfg.PR, err)
		}
		var pr pullRequest
		if err := json.Unmarshal(out, &pr); err != nil {
			return pullRequest{}, fmt.Errorf("parse PR #%d JSON: %w", w.cfg.PR, err)
		}
		return pr, nil
	}

	out, err := w.cfg.runner.output(ctx, ghCommand(root, "pr", "list", "--state", "open", "--search", "author:app/dependabot", "--limit", "1", "--json", fields))
	if err != nil {
		return pullRequest{}, fmt.Errorf("discover latest Dependabot PR: %w", err)
	}
	var prs []pullRequest
	if err := json.Unmarshal(out, &prs); err != nil {
		return pullRequest{}, fmt.Errorf("parse Dependabot PR list: %w", err)
	}
	if len(prs) == 0 {
		return pullRequest{}, errors.New("no open Dependabot PRs found")
	}
	return prs[0], nil
}

func validatePullRequest(pr pullRequest) error {
	if pr.Number == 0 {
		return errors.New("GitHub returned PR without a number")
	}
	if pr.State != "OPEN" {
		return fmt.Errorf("PR #%d is %s, not OPEN", pr.Number, pr.State)
	}
	if pr.IsDraft {
		return fmt.Errorf("PR #%d is draft", pr.Number)
	}
	if pr.BaseRefName != "master" {
		return fmt.Errorf("PR #%d targets %q, expected master", pr.Number, pr.BaseRefName)
	}
	if pr.Author.Login != "dependabot[bot]" && pr.Author.Login != "app/dependabot" {
		return fmt.Errorf("PR #%d author is %q, expected Dependabot", pr.Number, pr.Author.Login)
	}
	if pr.HeadRefOID == "" {
		return fmt.Errorf("PR #%d has empty head SHA", pr.Number)
	}
	return nil
}

func (w workflow) showDiffStat(pr pullRequest) {
	w.step("📄", "PR files")
	if len(pr.Files) == 0 {
		w.step("📄", "GitHub returned no file summary")
		return
	}
	for _, file := range pr.Files {
		_, _ = fmt.Fprintf(w.cfg.Stdout, "   %s (+%d -%d)\n", file.Path, file.Additions, file.Deletions)
	}
}

func (w workflow) previewReleaseVersion(ctx context.Context, root string, current string) (string, bool, error) {
	if err := w.cfg.runner.run(ctx, gitCommand(root, "fetch", "--tags", "origin")); err != nil {
		return "", false, fmt.Errorf("fetch tags: %w", err)
	}
	return w.firstFreeVersion(ctx, root, current)
}

func (w workflow) requireCleanMaster(ctx context.Context, root string) error {
	branch, err := w.gitOutput(ctx, root, "branch", "--show-current")
	if err != nil {
		return fmt.Errorf("check current branch: %w", err)
	}
	if strings.TrimSpace(branch) != "master" {
		return fmt.Errorf("current branch is %q, expected master", strings.TrimSpace(branch))
	}
	status, err := w.gitOutput(ctx, root, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("check worktree status: %w", err)
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("worktree is dirty; commit or stash changes before running deps_accept")
	}
	w.step("✅", "clean master worktree")
	return nil
}

func (w workflow) localGate(ctx context.Context, root string, number int) error {
	w.step("🧪", "local gate in temporary PR worktree")
	parent := w.cfg.WorktreeParent
	if parent == "" {
		parent = os.TempDir()
	}
	worktree, err := w.cfg.mkdirTemp(parent, "ch_watch_deps_accept_*")
	if err != nil {
		return fmt.Errorf("create temporary worktree directory: %w", err)
	}
	added := false
	defer w.cleanupWorktree(context.Background(), root, worktree, &added)

	ref := fmt.Sprintf("refs/remotes/origin/pr/%d", number)
	if err := w.cfg.runner.run(ctx, gitCommand(root, "fetch", "origin", fmt.Sprintf("+pull/%d/head:%s", number, ref))); err != nil {
		return fmt.Errorf("fetch PR #%d: %w", number, err)
	}
	if err := w.cfg.runner.run(ctx, gitCommand(root, "worktree", "add", "--detach", worktree, ref)); err != nil {
		return fmt.Errorf("create PR worktree: %w", err)
	}
	added = true

	if err := w.cfg.runner.run(ctx, command{dir: worktree, name: "make", args: []string{"check-full"}}); err != nil {
		return fmt.Errorf("local make check-full failed: %w", err)
	}
	installDir := filepath.Join(worktree, ".deps_accept_gobin")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("create temporary GOBIN: %w", err)
	}
	if err := w.cfg.runner.run(ctx, command{dir: worktree, name: "make", args: []string{"install"}, env: []string{"GOBIN=" + installDir}}); err != nil {
		return fmt.Errorf("local make install failed: %w", err)
	}
	binPath := filepath.Join(installDir, "ch_watch")
	if err := w.cfg.runner.run(ctx, command{dir: worktree, name: binPath, args: []string{"version"}}); err != nil {
		return fmt.Errorf("installed binary version check failed: %w", err)
	}
	if err := w.cfg.runner.run(ctx, command{dir: worktree, name: binPath, args: []string{"run", "./demo/ch/dev/tmp.sql", "--dry-run"}}); err != nil {
		return fmt.Errorf("installed binary run smoke failed: %w", err)
	}
	if err := w.cfg.watchSmoke(ctx, worktree, binPath, w.cfg.Stdout, w.cfg.Stderr); err != nil {
		return fmt.Errorf("watch smoke failed: %w", err)
	}
	w.step("✅", "local gate passed")
	return nil
}

func (w workflow) cleanupWorktree(ctx context.Context, root string, worktree string, added *bool) {
	if *added {
		if err := w.cfg.runner.run(ctx, gitCommand(root, "worktree", "remove", "--force", worktree)); err != nil {
			_, _ = fmt.Fprintf(w.cfg.Stderr, "⚠️ cleanup worktree %s: %v\n", worktree, err)
		}
		return
	}
	if err := w.cfg.removeAll(worktree); err != nil {
		_, _ = fmt.Fprintf(w.cfg.Stderr, "⚠️ cleanup temp dir %s: %v\n", worktree, err)
	}
}

func (w workflow) remotePRGate(ctx context.Context, root string, number int) error {
	w.step("🌐", "waiting for GitHub PR checks")
	if err := w.cfg.runner.run(ctx, ghCommand(root, "pr", "checks", fmt.Sprintf("%d", number), "--watch", "--fail-fast")); err != nil {
		return fmt.Errorf("PR checks failed: %w", err)
	}
	return nil
}

func (w workflow) mergePullRequest(ctx context.Context, root string, pr pullRequest) error {
	w.step("🔀", "merging PR #%d", pr.Number)
	args := []string{"pr", "merge", fmt.Sprintf("%d", pr.Number), "--squash", "--delete-branch", "--subject", pr.Title, "--body", "", "--match-head-commit", pr.HeadRefOID}
	if err := w.cfg.runner.run(ctx, ghCommand(root, args...)); err != nil {
		return fmt.Errorf("merge PR #%d: %w", pr.Number, err)
	}
	return nil
}

func (w workflow) syncMaster(ctx context.Context, root string) error {
	w.step("⬇️", "syncing local master")
	if err := w.cfg.runner.run(ctx, gitCommand(root, "checkout", "master")); err != nil {
		return fmt.Errorf("checkout master: %w", err)
	}
	if err := w.cfg.runner.run(ctx, gitCommand(root, "pull", "--ff-only", "origin", "master")); err != nil {
		return fmt.Errorf("pull master: %w", err)
	}
	return nil
}

func (w workflow) ensureReleaseVersion(ctx context.Context, root string, versionFile string) (string, bool, error) {
	if err := w.cfg.runner.run(ctx, gitCommand(root, "fetch", "--tags", "origin")); err != nil {
		return "", false, fmt.Errorf("fetch tags: %w", err)
	}
	current, err := readVersion(versionFile)
	if err != nil {
		return "", false, fmt.Errorf("read VERSION after merge: %w", err)
	}
	releaseVersion, changed, err := w.firstFreeVersion(ctx, root, current)
	if err != nil {
		return "", false, err
	}
	if !changed {
		w.step("📦", "using existing VERSION %s", releaseVersion)
		return releaseVersion, false, nil
	}
	w.step("📦", "bumping VERSION %s → %s", current, releaseVersion)
	if err := writeVersion(versionFile, releaseVersion); err != nil {
		return "", false, fmt.Errorf("write VERSION: %w", err)
	}
	return releaseVersion, true, nil
}

func (w workflow) firstFreeVersion(ctx context.Context, root string, current string) (string, bool, error) {
	version := current
	changed := false
	for {
		exists, err := w.tagExists(ctx, root, tagName(version))
		if err != nil {
			return "", false, err
		}
		if !exists {
			return version, changed, nil
		}
		next, err := bumpPatch(version)
		if err != nil {
			return "", false, err
		}
		version = next
		changed = true
	}
}

func (w workflow) tagExists(ctx context.Context, root string, tag string) (bool, error) {
	out, err := w.gitOutput(ctx, root, "tag", "--list", tag)
	if err != nil {
		return false, fmt.Errorf("list tag %s: %w", tag, err)
	}
	return strings.TrimSpace(out) != "", nil
}

func (w workflow) commitAndPushVersion(ctx context.Context, root string, version string) error {
	w.step("📝", "committing VERSION bump")
	if err := w.cfg.runner.run(ctx, gitCommand(root, "add", "VERSION")); err != nil {
		return fmt.Errorf("git add VERSION: %w", err)
	}
	if err := w.cfg.runner.run(ctx, gitCommand(root, "commit", "-m", fmt.Sprintf("chore: bump VERSION to %s", version))); err != nil {
		return fmt.Errorf("git commit VERSION bump: %w", err)
	}
	if err := w.cfg.runner.run(ctx, gitCommand(root, "push", "origin", "master")); err != nil {
		return fmt.Errorf("git push master: %w", err)
	}
	return nil
}

func (w workflow) tagAndPush(ctx context.Context, root string, version string) error {
	tag := tagName(version)
	w.step("🏷️", "creating tag %s", tag)
	if err := w.cfg.runner.run(ctx, gitCommand(root, "tag", tag)); err != nil {
		return fmt.Errorf("git tag %s: %w", tag, err)
	}
	w.step("🚀", "pushing tag %s", tag)
	if err := w.cfg.runner.run(ctx, gitCommand(root, "push", "origin", tag)); err != nil {
		return fmt.Errorf("git push %s: %w", tag, err)
	}
	return nil
}

func (w workflow) waitForWorkflow(ctx context.Context, root string, workflowFile string, headSHA string, label string) error {
	w.step("🚦", "waiting for %s (%s)", label, workflowFile)
	var runID int64
	for range w.cfg.pollLimit {
		runs, err := w.listWorkflowRuns(ctx, root, workflowFile)
		if err != nil {
			return err
		}
		for _, run := range runs {
			if run.HeadSHA == headSHA {
				runID = run.DatabaseID
				break
			}
		}
		if runID != 0 {
			break
		}
		w.cfg.sleep(w.cfg.pollEvery)
	}
	if runID == 0 {
		return fmt.Errorf("could not find %s run for commit %s", workflowFile, headSHA)
	}
	if err := w.cfg.runner.run(ctx, ghCommand(root, "run", "watch", fmt.Sprintf("%d", runID), "--exit-status")); err != nil {
		return fmt.Errorf("%s failed: %w", label, err)
	}
	return nil
}

func (w workflow) listWorkflowRuns(ctx context.Context, root string, workflowFile string) ([]workflowRun, error) {
	out, err := w.cfg.runner.output(ctx, ghCommand(root, "run", "list", "--workflow", workflowFile, "--limit", "10", "--json", "databaseId,headSha,headBranch,status,conclusion"))
	if err != nil {
		return nil, fmt.Errorf("list workflow runs for %s: %w", workflowFile, err)
	}
	var runs []workflowRun
	if err := json.Unmarshal(out, &runs); err != nil {
		return nil, fmt.Errorf("parse workflow runs for %s: %w", workflowFile, err)
	}
	return runs, nil
}

func (w workflow) verifyRemoteInstall(ctx context.Context, root string, version string) error {
	tag := tagName(version)
	w.step("📥", "verifying go install %s@%s", modulePath, tag)
	installDir, err := w.cfg.mkdirTemp("", "ch_watch_install_*")
	if err != nil {
		return fmt.Errorf("create temporary install dir: %w", err)
	}
	defer func() {
		if err := w.cfg.removeAll(installDir); err != nil {
			_, _ = fmt.Fprintf(w.cfg.Stderr, "⚠️ cleanup install dir %s: %v\n", installDir, err)
		}
	}()
	binDir := filepath.Join(installDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create temporary GOBIN: %w", err)
	}
	installArg := fmt.Sprintf("%s/cmd/ch_watch@%s", modulePath, tag)
	if err := w.cfg.runner.run(ctx, command{dir: root, name: "go", args: []string{"install", installArg}, env: []string{"GOPROXY=direct", "GOBIN=" + binDir}}); err != nil {
		return fmt.Errorf("go install %s: %w", installArg, err)
	}
	binPath := filepath.Join(binDir, "ch_watch")
	if err := w.cfg.runner.run(ctx, command{dir: root, name: binPath, args: []string{"version"}}); err != nil {
		return fmt.Errorf("installed release version check failed: %w", err)
	}
	if err := w.cfg.runner.run(ctx, command{dir: root, name: binPath, args: []string{"run", "./demo/ch/dev/tmp.sql", "--dry-run"}}); err != nil {
		return fmt.Errorf("installed release run smoke failed: %w", err)
	}
	return nil
}

func (w workflow) gitOutput(ctx context.Context, root string, args ...string) (string, error) {
	out, err := w.cfg.runner.output(ctx, gitCommand(root, args...))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (w workflow) printPlan(pr pullRequest, version string, versionWouldChange bool) {
	w.step("☑️", "would run local PR gate in a temporary worktree")
	w.step("☑️", "would wait for GitHub PR checks")
	w.step("☑️", "would squash-merge PR #%d with head guard %s", pr.Number, shortSHA(pr.HeadRefOID))
	if versionWouldChange {
		w.step("☑️", "would write VERSION=%s, commit it, and push master", version)
	} else {
		w.step("☑️", "would keep current VERSION and tag the merge commit")
	}
	w.step("☑️", "would wait for master CI, tag %s, wait for release workflow", tagName(version))
	w.step("☑️", "would verify GOPROXY=direct go install and run smoke")
}

func (w workflow) banner(label string) {
	_, _ = fmt.Fprintf(w.cfg.Stdout, "=== %s %s\n", label, strings.Repeat("=", max(1, 70-len(label))))
}

func (w workflow) step(icon string, format string, args ...any) {
	_, _ = fmt.Fprintf(w.cfg.Stdout, "%s %s\n", icon, fmt.Sprintf(format, args...))
}

func tagName(version string) string {
	return "v" + strings.TrimPrefix(version, "v")
}

func shortSHA(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}

func defaultWatchSmoke(ctx context.Context, worktree string, binPath string, stdout io.Writer, stderr io.Writer) error {
	watchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var output bytes.Buffer
	process := realWatchCommand(watchCtx, worktree, binPath, &output)
	if err := process.Start(); err != nil {
		return err
	}

	time.Sleep(700 * time.Millisecond)
	sqlPath := filepath.Join(worktree, "demo/ch/dev/tmp.sql")
	now := time.Now()
	if err := os.Chtimes(sqlPath, now, now); err != nil {
		_ = process.Process.Kill()
		return err
	}
	time.Sleep(900 * time.Millisecond)
	if process.Process != nil {
		_ = process.Process.Signal(os.Interrupt)
	}
	err := process.Wait()
	text := output.String()
	if watchCtx.Err() != nil {
		return fmt.Errorf("watch smoke timed out: %s", strings.TrimSpace(text))
	}
	if err != nil && !isExitError(err) {
		return err
	}
	if !strings.Contains(text, "tmp.sql") || !strings.Contains(text, "RUN") || !strings.Contains(text, "OK") {
		_, _ = fmt.Fprint(stderr, text)
		return errors.New("watch smoke did not observe tmp.sql RUN/OK")
	}
	_, _ = fmt.Fprintln(stdout, "✅ watch smoke observed tmp.sql rerun")
	return nil
}

func realWatchCommand(ctx context.Context, worktree string, binPath string, output io.Writer) *exec.Cmd {
	cmd := exec.CommandContext(ctx, binPath, "watch", "--root", "./demo/ch", "--dry-run")
	cmd.Dir = worktree
	cmd.Stdout = output
	cmd.Stderr = output
	return cmd
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
