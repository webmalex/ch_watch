package depsaccept

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	calls   []command
	outputs map[string]string
	errors  map[string]error
}

func (r *fakeRunner) run(_ context.Context, cmd command) error {
	r.calls = append(r.calls, cmd)
	if err := r.errors[commandKey(cmd)]; err != nil {
		return err
	}
	return nil
}

func (r *fakeRunner) output(_ context.Context, cmd command) ([]byte, error) {
	r.calls = append(r.calls, cmd)
	if err := r.errors[commandKey(cmd)]; err != nil {
		return nil, err
	}
	return []byte(r.outputs[commandKey(cmd)]), nil
}

func TestRunDryRunDiscoversLatestPR(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	versionFile := filepath.Join(root, "VERSION")
	if err := os.WriteFile(versionFile, []byte("0.7.5\n"), 0o644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}
	fake := newFake(root)
	fake.outputs["git rev-parse --show-toplevel"] = root + "\n"
	fake.outputs["gh pr list --state open --search author:app/dependabot --limit 1 --json number,title,state,url,isDraft,headRefName,baseRefName,headRefOid,author,files"] = `[{
		"number": 7,
		"title": "deps: bump github.com/fsnotify/fsnotify from 1.8.0 to 1.10.1",
		"state": "OPEN",
		"url": "https://github.com/webmalex/ch_watch/pull/7",
		"isDraft": false,
		"headRefName": "dependabot/go_modules/github.com/fsnotify/fsnotify-1.10.1",
		"baseRefName": "master",
		"headRefOid": "1234567890abcdef",
		"author": {"login": "app/dependabot"},
		"files": [{"path":"go.mod","additions":1,"deletions":1}]
	}]`

	var stdout strings.Builder
	err := Run(context.Background(), Config{DryRun: true, VersionFile: versionFile, Stdout: &stdout, runner: fake})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "dry-run") || !strings.Contains(stdout.String(), "PR #7") {
		t.Fatalf("stdout does not describe dry-run PR: %s", stdout.String())
	}
	if containsCommand(fake.calls, "gh", "pr", "merge") {
		t.Fatalf("dry-run executed merge: %#v", fake.calls)
	}
}

func TestRunSpecificPRUsesView(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	versionFile := filepath.Join(root, "VERSION")
	if err := os.WriteFile(versionFile, []byte("0.7.5\n"), 0o644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}
	fake := newFake(root)
	fake.outputs["git rev-parse --show-toplevel"] = root + "\n"
	fake.outputs["gh pr view 42 --json number,title,state,url,isDraft,headRefName,baseRefName,headRefOid,author,files"] = `{
		"number": 42,
		"title": "deps: bump action",
		"state": "OPEN",
		"url": "https://github.com/webmalex/ch_watch/pull/42",
		"isDraft": false,
		"headRefName": "dependabot/github_actions/actions/checkout-6",
		"baseRefName": "master",
		"headRefOid": "abcdef1234567890",
		"author": {"login": "dependabot[bot]"},
		"files": [{"path":".github/workflows/ci.yml","additions":1,"deletions":1}]
	}`

	if err := Run(context.Background(), Config{PR: 42, DryRun: true, VersionFile: versionFile, runner: fake}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !containsExactCommand(fake.calls, "gh", "pr", "view", "42", "--json", "number,title,state,url,isDraft,headRefName,baseRefName,headRefOid,author,files") {
		t.Fatalf("did not call gh pr view for specific PR: %#v", fake.calls)
	}
}

func TestFirstFreeVersionBumpsUntilTagIsFree(t *testing.T) {
	t.Parallel()

	fake := &fakeRunner{outputs: map[string]string{
		"git tag --list v0.7.5": "v0.7.5\n",
		"git tag --list v0.7.6": "v0.7.6\n",
		"git tag --list v0.7.7": "",
	}, errors: map[string]error{}}
	w := workflow{cfg: normalizeConfig(Config{runner: fake})}

	version, changed, err := w.firstFreeVersion(context.Background(), "/repo", "0.7.5")
	if err != nil {
		t.Fatalf("firstFreeVersion returned error: %v", err)
	}
	if version != "0.7.7" || !changed {
		t.Fatalf("firstFreeVersion() = %q, %v; want %q, true", version, changed, "0.7.7")
	}
}

func TestRunFullPipelineKeepsUnreleasedVersion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	versionFile := filepath.Join(root, "VERSION")
	if err := os.WriteFile(versionFile, []byte("0.7.5\n"), 0o644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}
	fake := newFake(root)
	fake.outputs["git rev-parse --show-toplevel"] = root + "\n"
	fake.outputs["git branch --show-current"] = "master\n"
	fake.outputs["git status --porcelain"] = ""
	fake.outputs["git rev-parse HEAD"] = "feedface\n"
	fake.outputs["gh pr view 1 --json number,title,state,url,isDraft,headRefName,baseRefName,headRefOid,author,files"] = `{
		"number": 1,
		"title": "deps: bump github.com/fsnotify/fsnotify from 1.8.0 to 1.10.1",
		"state": "OPEN",
		"url": "https://github.com/webmalex/ch_watch/pull/1",
		"isDraft": false,
		"headRefName": "dependabot/go_modules/github.com/fsnotify/fsnotify-1.10.1",
		"baseRefName": "master",
		"headRefOid": "1234567890abcdef",
		"author": {"login": "dependabot[bot]"},
		"files": [{"path":"go.mod","additions":1,"deletions":1}]
	}`
	fake.outputs["gh run list --workflow ci.yml --limit 10 --json databaseId,headSha,headBranch,status,conclusion"] = `[{
		"databaseId": 100,
		"headSha": "feedface",
		"headBranch": "master",
		"status": "completed",
		"conclusion": "success"
	}]`
	fake.outputs["gh run list --workflow release.yml --limit 10 --json databaseId,headSha,headBranch,status,conclusion"] = `[{
		"databaseId": 101,
		"headSha": "feedface",
		"headBranch": "v0.7.5",
		"status": "completed",
		"conclusion": "success"
	}]`
	worktreeParent := t.TempDir()

	err := Run(context.Background(), Config{
		PR:             1,
		VersionFile:    versionFile,
		WorktreeParent: worktreeParent,
		runner:         fake,
		watchSmoke: func(context.Context, string, string, io.Writer, io.Writer) error {
			return nil
		},
		sleep:     func(time.Duration) {},
		pollLimit: 1,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if containsCommand(fake.calls, "git", "commit") {
		t.Fatalf("pipeline committed VERSION even though v0.7.5 was free: %#v", fake.calls)
	}
	if !containsExactCommand(fake.calls, "git", "tag", "v0.7.5") {
		t.Fatalf("pipeline did not tag v0.7.5: %#v", fake.calls)
	}
	if !containsExactCommand(fake.calls, "gh", "pr", "merge", "1", "--squash", "--delete-branch", "--subject", "deps: bump github.com/fsnotify/fsnotify from 1.8.0 to 1.10.1", "--body", "", "--match-head-commit", "1234567890abcdef") {
		t.Fatalf("pipeline did not merge with expected args: %#v", fake.calls)
	}
}

func TestMergeFailureStopsPipeline(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	versionFile := filepath.Join(root, "VERSION")
	if err := os.WriteFile(versionFile, []byte("0.7.5\n"), 0o644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}
	fake := newFake(root)
	fake.outputs["git rev-parse --show-toplevel"] = root + "\n"
	fake.outputs["git branch --show-current"] = "master\n"
	fake.outputs["git status --porcelain"] = ""
	fake.outputs["gh pr view 1 --json number,title,state,url,isDraft,headRefName,baseRefName,headRefOid,author,files"] = `{
		"number": 1,
		"title": "deps: bump",
		"state": "OPEN",
		"url": "https://github.com/webmalex/ch_watch/pull/1",
		"isDraft": false,
		"headRefName": "dependabot/x",
		"baseRefName": "master",
		"headRefOid": "1234567890abcdef",
		"author": {"login": "dependabot[bot]"},
		"files": [{"path":"go.mod","additions":1,"deletions":1}]
	}`
	fake.errors["gh pr merge 1 --squash --delete-branch --subject deps: bump --body  --match-head-commit 1234567890abcdef"] = errors.New("merge failed")

	err := Run(context.Background(), Config{
		PR:             1,
		VersionFile:    versionFile,
		WorktreeParent: t.TempDir(),
		runner:         fake,
		watchSmoke: func(context.Context, string, string, io.Writer, io.Writer) error {
			return nil
		},
	})
	if err == nil {
		t.Fatal("Run returned nil error for merge failure")
	}
	if containsExactCommand(fake.calls, "git", "tag", "v0.7.5") {
		t.Fatalf("pipeline tagged after merge failure: %#v", fake.calls)
	}
}

func newFake(root string) *fakeRunner {
	fake := &fakeRunner{outputs: map[string]string{}, errors: map[string]error{}}
	fake.outputs["git tag --list v0.7.5"] = ""
	fake.outputs["git tag --list v0.7.6"] = ""
	fake.outputs["git tag --list v0.7.7"] = ""
	_ = root
	return fake
}

func commandKey(cmd command) string {
	return strings.TrimSpace(cmd.name + " " + strings.Join(cmd.args, " "))
}

func containsCommand(calls []command, name string, args ...string) bool {
	for _, call := range calls {
		if call.name != name {
			continue
		}
		if len(args) == 0 {
			return true
		}
		if len(call.args) >= len(args) && reflect.DeepEqual(call.args[:len(args)], args) {
			return true
		}
	}
	return false
}

func containsExactCommand(calls []command, name string, args ...string) bool {
	for _, call := range calls {
		if call.name == name && reflect.DeepEqual(call.args, args) {
			return true
		}
	}
	return false
}

func TestCommandKeyIncludesArguments(t *testing.T) {
	t.Parallel()

	got := commandKey(command{name: "gh", args: []string{"pr", "view", "1"}})
	if got != "gh pr view 1" {
		t.Fatalf("commandKey() = %q", got)
	}
}

func TestReleaseDryRun(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	versionFile := filepath.Join(root, "VERSION")
	if err := os.WriteFile(versionFile, []byte("0.8.0\n"), 0o644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}
	fake := newFake(root)
	fake.outputs["git rev-parse --show-toplevel"] = root + "\n"

	var stdout strings.Builder
	err := Release(context.Background(), Config{DryRun: true, VersionFile: versionFile, Stdout: &stdout, runner: fake})
	if err != nil {
		t.Fatalf("Release returned error: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "dry-run") || !strings.Contains(out, "v0.8.0") {
		t.Fatalf("stdout does not describe dry-run release: %s", out)
	}
	if containsCommand(fake.calls, "gh", "release", "create") {
		t.Fatalf("dry-run created a release: %#v", fake.calls)
	}
}

func TestReleaseCreatesGitHubRelease(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	versionFile := filepath.Join(root, "VERSION")
	if err := os.WriteFile(versionFile, []byte("0.8.0\n"), 0o644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}
	fake := newFake(root)
	fake.outputs["git rev-parse --show-toplevel"] = root + "\n"
	fake.outputs["git branch --show-current"] = "master\n"
	fake.outputs["git status --porcelain"] = ""
	fake.outputs["git rev-parse HEAD"] = "cafebabe\n"
	fake.outputs["gh run list --workflow ci.yml --limit 10 --json databaseId,headSha,headBranch,status,conclusion"] = `[{
		"databaseId": 200,
		"headSha": "cafebabe",
		"headBranch": "master",
		"status": "completed",
		"conclusion": "success"
	}]`
	fake.outputs["gh run list --workflow release.yml --limit 10 --json databaseId,headSha,headBranch,status,conclusion"] = `[{
		"databaseId": 201,
		"headSha": "cafebabe",
		"headBranch": "v0.8.0",
		"status": "completed",
		"conclusion": "success"
	}]`

	err := Release(context.Background(), Config{
		VersionFile:    versionFile,
		WorktreeParent: t.TempDir(),
		runner:         fake,
		watchSmoke:     func(context.Context, string, string, io.Writer, io.Writer) error { return nil },
		sleep:          func(time.Duration) {},
		pollLimit:      1,
	})
	if err != nil {
		t.Fatalf("Release returned error: %v", err)
	}
	if !containsExactCommand(fake.calls, "gh", "release", "create", "v0.8.0", "--target", "cafebabe", "--title", "v0.8.0", "--generate-release-notes") {
		t.Fatalf("Release did not create GitHub release with generate-release-notes: %#v", fake.calls)
	}
}

func TestSyncMasterRebasesWhenLocalAhead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	versionFile := filepath.Join(root, "VERSION")
	if err := os.WriteFile(versionFile, []byte("0.7.5\n"), 0o644); err != nil {
		t.Fatalf("write VERSION: %v", err)
	}
	fake := newFake(root)
	fake.outputs["git rev-parse --show-toplevel"] = root + "\n"
	fake.outputs["git branch --show-current"] = "master\n"
	fake.outputs["git status --porcelain"] = ""
	fake.outputs["git rev-parse HEAD"] = "feedface\n"
	fake.outputs["gh pr view 1 --json number,title,state,url,isDraft,headRefName,baseRefName,headRefOid,author,files"] = `{
		"number": 1,
		"title": "deps: bump something",
		"state": "OPEN",
		"url": "https://github.com/webmalex/ch_watch/pull/1",
		"isDraft": false,
		"headRefName": "dependabot/x",
		"baseRefName": "master",
		"headRefOid": "abc123",
		"author": {"login": "dependabot[bot]"},
		"files": [{"path":"go.mod","additions":1,"deletions":1}]
	}`
	fake.outputs["gh run list --workflow ci.yml --limit 10 --json databaseId,headSha,headBranch,status,conclusion"] = `[{
		"databaseId": 100,
		"headSha": "feedface",
		"headBranch": "master",
		"status": "completed",
		"conclusion": "success"
	}]`
	fake.outputs["gh run list --workflow release.yml --limit 10 --json databaseId,headSha,headBranch,status,conclusion"] = `[{
		"databaseId": 101,
		"headSha": "feedface",
		"headBranch": "v0.7.5",
		"status": "completed",
		"conclusion": "success"
	}]`
	fake.errors["git merge --ff-only origin/master"] = errors.New("cannot fast-forward")

	err := Run(context.Background(), Config{
		PR:             1,
		VersionFile:    versionFile,
		WorktreeParent: t.TempDir(),
		runner:         fake,
		watchSmoke:     func(context.Context, string, string, io.Writer, io.Writer) error { return nil },
		sleep:          func(time.Duration) {},
		pollLimit:      1,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !containsCommand(fake.calls, "git", "rebase", "origin/master") {
		t.Fatalf("syncMaster did not fall back to rebase: %#v", fake.calls)
	}
	if !containsExactCommand(fake.calls, "git", "tag", "v0.7.5") {
		t.Fatalf("pipeline did not tag v0.7.5 after rebase: %#v", fake.calls)
	}
}

func Example_tagName() {
	fmt.Println(tagName("0.7.5"))
	fmt.Println(tagName("v0.7.5"))
	// Output:
	// v0.7.5
	// v0.7.5
}
