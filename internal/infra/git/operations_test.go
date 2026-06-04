package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	domaingit "git.woa.com/leondli/workspace-service/internal/domain/git"
	"git.woa.com/leondli/workspace-service/internal/testutil"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// initSourceRepo creates a local repository with one commit to act as a clone
// source ("remote").
func initSourceRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init source repo: %v", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := worktree.Add("README.md"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := worktree.Commit("init", &gogit.CommitOptions{
		Author: &object.Signature{Name: "seed", Email: "seed@example.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return dir
}

func cloneInto(t *testing.T, client *WorkspaceGitClient, sourceDir string) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "clone")
	_, err := client.Clone(context.Background(), domaingit.CloneReq{
		Actor:      testutil.RequestContext(),
		RepoURL:    sourceDir,
		TargetPath: target,
		Branch:     "master",
	})
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	return target
}

func TestBranchStatusCommitAndHistoryFlow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := NewWorkspaceGitClient(nil, "", "")
	actor := testutil.RequestContext()

	source := initSourceRepo(t)
	repoPath := cloneInto(t, client, source)

	// Freshly cloned repo is clean.
	status, err := client.Status(ctx, domaingit.StatusReq{Actor: actor, Path: repoPath})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Clean {
		t.Fatalf("expected clean status, got %+v", status.Files)
	}

	// Create and switch to a new branch.
	branchResult, err := client.CreateBranch(ctx, domaingit.CreateBranchReq{
		Actor: actor, Path: repoPath, Branch: "feature", Checkout: true,
	})
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}
	if branchResult.CurrentBranch != "feature" {
		t.Fatalf("current branch = %q, want feature", branchResult.CurrentBranch)
	}

	branches, err := client.ListBranches(ctx, domaingit.ListBranchesReq{Actor: actor, Path: repoPath})
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if !containsString(branches.Branches, "feature") {
		t.Fatalf("branches = %v, want to contain feature", branches.Branches)
	}

	// Modify a file -> status reports it.
	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("modify file: %v", err)
	}
	status, err = client.Status(ctx, domaingit.StatusReq{Actor: actor, Path: repoPath})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Clean {
		t.Fatalf("expected dirty status after edit")
	}

	// Commit without pushing (no remote credentials).
	commit, err := client.CommitAndPush(ctx, domaingit.CommitAndPushReq{
		Actor:       actor,
		Path:        repoPath,
		Message:     "update readme",
		AuthorName:  "tester",
		AuthorEmail: "tester@example.com",
		Push:        false,
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if commit.CommitHash == "" || commit.NothingToCommit {
		t.Fatalf("unexpected commit result %+v", commit)
	}

	// History now has the init commit plus our commit.
	history, err := client.CommitHistory(ctx, domaingit.CommitHistoryReq{Actor: actor, Path: repoPath})
	if err != nil {
		t.Fatalf("commit history: %v", err)
	}
	if len(history.Commits) < 2 {
		t.Fatalf("history len = %d, want >= 2", len(history.Commits))
	}
	if history.Commits[0].Message != "update readme" {
		t.Fatalf("latest commit message = %q", history.Commits[0].Message)
	}
}

func TestPullReturnsAlreadyUpToDate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := NewWorkspaceGitClient(nil, "", "")
	actor := testutil.RequestContext()

	source := initSourceRepo(t)
	repoPath := cloneInto(t, client, source)

	result, err := client.Pull(ctx, domaingit.PullReq{Actor: actor, Path: repoPath})
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if !result.AlreadyUpToDate {
		t.Fatalf("expected already up to date right after clone")
	}
}

func TestDiscardChangesRevertsWorktree(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := NewWorkspaceGitClient(nil, "", "")
	actor := testutil.RequestContext()

	source := initSourceRepo(t)
	repoPath := cloneInto(t, client, source)

	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("modify: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoPath, "untracked.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write untracked: %v", err)
	}

	if err := client.DiscardChanges(ctx, domaingit.DiscardChangesReq{Actor: actor, Path: repoPath}); err != nil {
		t.Fatalf("discard: %v", err)
	}

	status, err := client.Status(ctx, domaingit.StatusReq{Actor: actor, Path: repoPath})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Clean {
		t.Fatalf("expected clean worktree after discard, got %+v", status.Files)
	}
	content, err := os.ReadFile(filepath.Join(repoPath, "README.md"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "hello\n" {
		t.Fatalf("README content = %q, want original", string(content))
	}
}

func TestCloneStoresGitMetadataOutsideMountWhenConfigured(t *testing.T) {
	t.Parallel()
	mountRoot := t.TempDir()
	metaRoot := t.TempDir()
	sourceDir := initSourceRepo(t)
	ctx := testutil.RequestContext()
	target := filepath.Join(mountRoot, ctx.UserPathPrefix(), "repo")

	client := NewWorkspaceGitClient(nil, mountRoot, metaRoot)
	_, err := client.Clone(context.Background(), domaingit.CloneReq{
		Actor:      testutil.RequestContext(),
		RepoURL:    sourceDir,
		TargetPath: target,
		Branch:     "master",
	})
	if err != nil {
		t.Fatalf("clone: %v", err)
	}

	metaGit := filepath.Join(metaRoot, ctx.AppID, ctx.WorkspaceID, "users", ctx.UIN, "repo", ".git")
	if _, err := os.Stat(filepath.Join(metaGit, "objects")); err != nil {
		t.Fatalf("expected git objects under %s: %v", metaGit, err)
	}

	worktreeGit := filepath.Join(target, ".git")
	info, err := os.Stat(worktreeGit)
	if err != nil {
		t.Fatalf("worktree .git: %v", err)
	}
	if info.IsDir() {
		if _, err := os.Stat(filepath.Join(worktreeGit, "objects")); err == nil {
			t.Fatalf("expected objects not stored under worktree .git directory")
		}
	}
}

func TestDeleteRepoRemovesDirectory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	client := NewWorkspaceGitClient(nil, "", "")
	actor := testutil.RequestContext()

	source := initSourceRepo(t)
	repoPath := cloneInto(t, client, source)

	if err := client.DeleteRepo(ctx, domaingit.DeleteRepoReq{Actor: actor, Path: repoPath}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(repoPath); !os.IsNotExist(err) {
		t.Fatalf("repo dir still exists, err = %v", err)
	}
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
