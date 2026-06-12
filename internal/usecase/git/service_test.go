package git

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	domaingit "git.woa.com/leondli/workspace-service/internal/domain/git"
	"git.woa.com/leondli/workspace-service/internal/testutil"
)

type fakeGitClient struct {
	req domaingit.CloneReq
}

func (f *fakeGitClient) Clone(ctx context.Context, req domaingit.CloneReq) (domaingit.CloneResult, error) {
	f.req = req
	return domaingit.CloneResult{
		RepoURL: req.RepoURL,
		Path:    req.TargetPath,
		Branch:  req.Branch,
	}, nil
}

func (f *fakeGitClient) Pull(context.Context, domaingit.PullReq) (domaingit.PullResult, error) {
	return domaingit.PullResult{}, nil
}
func (f *fakeGitClient) Stage(context.Context, domaingit.StageReq) error { return nil }
func (f *fakeGitClient) Unstage(context.Context, domaingit.UnstageReq) error { return nil }
func (f *fakeGitClient) Commit(context.Context, domaingit.CommitReq) (domaingit.CommitResult, error) {
	return domaingit.CommitResult{}, nil
}
func (f *fakeGitClient) Push(context.Context, domaingit.PushReq) (domaingit.PushResult, error) {
	return domaingit.PushResult{}, nil
}
func (f *fakeGitClient) CommitAndPush(context.Context, domaingit.CommitAndPushReq) (domaingit.CommitAndPushResult, error) {
	return domaingit.CommitAndPushResult{}, nil
}
func (f *fakeGitClient) CreateBranch(context.Context, domaingit.CreateBranchReq) (domaingit.BranchResult, error) {
	return domaingit.BranchResult{}, nil
}
func (f *fakeGitClient) CheckoutBranch(context.Context, domaingit.CheckoutBranchReq) (domaingit.BranchResult, error) {
	return domaingit.BranchResult{}, nil
}
func (f *fakeGitClient) ListBranches(context.Context, domaingit.ListBranchesReq) (domaingit.ListBranchesResult, error) {
	return domaingit.ListBranchesResult{}, nil
}
func (f *fakeGitClient) Status(context.Context, domaingit.StatusReq) (domaingit.StatusResult, error) {
	return domaingit.StatusResult{}, nil
}
func (f *fakeGitClient) FileDiff(context.Context, domaingit.FileDiffReq) (domaingit.FileDiffResult, error) {
	return domaingit.FileDiffResult{}, nil
}
func (f *fakeGitClient) RepoInfo(context.Context, domaingit.RepoInfoReq) (domaingit.RepoInfoResult, error) {
	return domaingit.RepoInfoResult{}, nil
}
func (f *fakeGitClient) CommitHistory(context.Context, domaingit.CommitHistoryReq) (domaingit.CommitHistoryResult, error) {
	return domaingit.CommitHistoryResult{}, nil
}
func (f *fakeGitClient) DiscardChanges(context.Context, domaingit.DiscardChangesReq) error {
	return nil
}
func (f *fakeGitClient) DeleteRepo(context.Context, domaingit.DeleteRepoReq) error {
	return nil
}

func TestServiceClonePassesActorAndRepoToGitClient(t *testing.T) {
	t.Parallel()

	gitClient := &fakeGitClient{}
	mountRoot := filepath.Join(t.TempDir(), "studio")
	service := NewService(gitClient, mountRoot, nil)
	ctx := testutil.RequestContext()

	output, err := service.CloneRepository(context.Background(), CloneRepositoryReq{
		Context:    ctx,
		RepoURL:    "https://example.com/repo.git",
		TargetPath: "repo",
		Branch:     "main",
	})
	if err != nil {
		t.Fatalf("clone: %v", err)
	}

	wantPath := filepath.Join(mountRoot, ctx.UserPathPrefix(), "repo")
	if output.Path != wantPath {
		t.Fatalf("output path = %q, want %q", output.Path, wantPath)
	}
	if gitClient.req.Actor.OwnerUIN != ctx.OwnerUIN {
		t.Fatalf("owner uin = %q, want %q", gitClient.req.Actor.OwnerUIN, ctx.OwnerUIN)
	}
	if gitClient.req.Actor.AppID != ctx.AppID {
		t.Fatalf("app id = %q, want %q", gitClient.req.Actor.AppID, ctx.AppID)
	}
	if gitClient.req.RepoURL != "https://example.com/repo.git" {
		t.Fatalf("repo url = %q, want %q", gitClient.req.RepoURL, "https://example.com/repo.git")
	}
	if gitClient.req.TargetPath != wantPath {
		t.Fatalf("git client target path = %q, want %q", gitClient.req.TargetPath, wantPath)
	}
}

func TestServiceCloneRejectsMissingBranch(t *testing.T) {
	service := NewService(&fakeGitClient{}, t.TempDir(), nil)

	_, err := service.CloneRepository(context.Background(), CloneRepositoryReq{
		Context:    testutil.RequestContext(),
		RepoURL:    "https://example.com/repo.git",
		TargetPath: "repo",
	})
	if !errors.Is(err, ErrInvalidCloneReq) {
		t.Fatalf("expected ErrInvalidCloneReq, got %v", err)
	}
}

func TestServiceCloneRejectsMissingActor(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeGitClient{}, t.TempDir(), nil)
	_, err := service.CloneRepository(context.Background(), CloneRepositoryReq{
		RepoURL:    "https://example.com/repo.git",
		TargetPath: "repo",
		Branch:     "main",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServiceCloneRejectsPathOutsideMountRoot(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeGitClient{}, t.TempDir(), nil)
	_, err := service.CloneRepository(context.Background(), CloneRepositoryReq{
		Context:    testutil.RequestContext(),
		RepoURL:    "https://example.com/repo.git",
		TargetPath: "../escape",
		Branch:     "main",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
