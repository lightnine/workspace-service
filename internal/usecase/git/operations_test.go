package git

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	domaingit "git.woa.com/leondli/workspace-service/internal/domain/git"
	"git.woa.com/leondli/workspace-service/internal/testutil"
)

type capturingGitClient struct {
	fakeGitClient
	pullReq   domaingit.PullReq
	commitReq domaingit.CommitAndPushReq
}

func (c *capturingGitClient) Pull(_ context.Context, req domaingit.PullReq) (domaingit.PullResult, error) {
	c.pullReq = req
	return domaingit.PullResult{AlreadyUpToDate: true, CurrentBranch: "main"}, nil
}

func (c *capturingGitClient) CommitAndPush(_ context.Context, req domaingit.CommitAndPushReq) (domaingit.CommitAndPushResult, error) {
	c.commitReq = req
	return domaingit.CommitAndPushResult{CommitHash: "abc123", Pushed: req.Push}, nil
}

func TestPullRepositoryResolvesPathAndForwardsCredentials(t *testing.T) {
	t.Parallel()

	client := &capturingGitClient{}
	mountRoot := filepath.Join(t.TempDir(), "studio")
	service := NewService(client, mountRoot, nil)
	ctx := testutil.RequestContext()

	resp, err := service.PullRepository(context.Background(), PullRepositoryReq{
		Context:     ctx,
		Path:        "repo",
		Credentials: Credentials{Username: "u", Token: "tok"},
	})
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if !resp.AlreadyUpToDate {
		t.Fatalf("expected already up to date")
	}
	want := filepath.Join(mountRoot, ctx.UserPathPrefix(), "repo")
	if client.pullReq.Path != want {
		t.Fatalf("path = %q, want %q", client.pullReq.Path, want)
	}
	if client.pullReq.Credentials.Token != "tok" {
		t.Fatalf("token = %q", client.pullReq.Credentials.Token)
	}
}

func TestCommitAndPushRequiresMessage(t *testing.T) {
	t.Parallel()

	service := NewService(&capturingGitClient{}, t.TempDir(), nil)
	_, err := service.CommitAndPush(context.Background(), CommitAndPushReq{
		Context: testutil.RequestContext(),
		Path:    "repo",
		Message: "   ",
	})
	if !errors.Is(err, ErrInvalidGitRequest) {
		t.Fatalf("err = %v, want ErrInvalidGitRequest", err)
	}
}

func TestCommitAndPushDefaultsAuthorFromActor(t *testing.T) {
	t.Parallel()

	client := &capturingGitClient{}
	service := NewService(client, t.TempDir(), nil)
	ctx := testutil.RequestContext()

	_, err := service.CommitAndPush(context.Background(), CommitAndPushReq{
		Context: ctx,
		Path:    "repo",
		Message: "feat: x",
		Push:    true,
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if client.commitReq.AuthorName != ctx.UIN {
		t.Fatalf("author name = %q, want %s", client.commitReq.AuthorName, ctx.UIN)
	}
	if client.commitReq.AuthorEmail != ctx.UIN+"@wedata" {
		t.Fatalf("author email = %q", client.commitReq.AuthorEmail)
	}
}

func TestCreateBranchRequiresBranch(t *testing.T) {
	t.Parallel()

	service := NewService(&capturingGitClient{}, t.TempDir(), nil)
	_, err := service.CreateBranch(context.Background(), CreateBranchReq{
		Context: testutil.RequestContext(),
		Path:    "repo",
	})
	if !errors.Is(err, ErrInvalidGitRequest) {
		t.Fatalf("err = %v, want ErrInvalidGitRequest", err)
	}
}

func TestGitOperationsRejectMissingActor(t *testing.T) {
	t.Parallel()

	service := NewService(&capturingGitClient{}, t.TempDir(), nil)
	_, err := service.GetStatus(context.Background(), StatusReq{Path: "repo"})
	if !errors.Is(err, ErrInvalidGitRequest) {
		t.Fatalf("err = %v, want ErrInvalidGitRequest", err)
	}
}

func TestGitOperationsRejectPathOutsideMountRoot(t *testing.T) {
	t.Parallel()

	service := NewService(&capturingGitClient{}, t.TempDir(), nil)
	_, err := service.ListBranches(context.Background(), ListBranchesReq{
		Context: testutil.RequestContext(),
		Path:    "../escape",
	})
	if !errors.Is(err, ErrInvalidGitRequest) {
		t.Fatalf("err = %v, want ErrInvalidGitRequest", err)
	}
}
