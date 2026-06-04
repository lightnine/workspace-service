package git

import (
	"context"
	"path/filepath"
	"testing"

	domaingit "git.woa.com/leondli/workspace-service/internal/domain/git"
)

type fakeGitClient struct {
	req domaingit.CloneReq
}

func (f *fakeGitClient) Clone(ctx context.Context, req domaingit.CloneReq) (domaingit.CloneResult, error) {
	f.req = req
	return domaingit.CloneResult{
		RepoURL: req.RepoURL,
		Path:    req.TargetPath,
	}, nil
}

func TestServiceClonePassesActorAndRepoToGitClient(t *testing.T) {
	t.Parallel()

	gitClient := &fakeGitClient{}
	mountRoot := filepath.Join(t.TempDir(), "studio")
	service := NewService(gitClient, mountRoot)

	output, err := service.CloneRepository(context.Background(), CloneRepositoryReq{
		OwnerUIN:   "100001",
		UIN:        "200001",
		RepoURL:    "https://example.com/repo.git",
		TargetPath: "users/200001/repo",
	})
	if err != nil {
		t.Fatalf("clone: %v", err)
	}

	wantPath := filepath.Join(mountRoot, "users/200001/repo")
	if output.Path != wantPath {
		t.Fatalf("output path = %q, want %q", output.Path, wantPath)
	}
	if gitClient.req.Actor.OwnerUIN != "100001" {
		t.Fatalf("owner uin = %q, want %q", gitClient.req.Actor.OwnerUIN, "100001")
	}
	if gitClient.req.Actor.UIN != "200001" {
		t.Fatalf("uin = %q, want %q", gitClient.req.Actor.UIN, "200001")
	}
	if gitClient.req.RepoURL != "https://example.com/repo.git" {
		t.Fatalf("repo url = %q, want %q", gitClient.req.RepoURL, "https://example.com/repo.git")
	}
	if gitClient.req.TargetPath != wantPath {
		t.Fatalf("git client target path = %q, want %q", gitClient.req.TargetPath, wantPath)
	}
}

func TestServiceCloneRejectsMissingActor(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeGitClient{}, t.TempDir())
	_, err := service.CloneRepository(context.Background(), CloneRepositoryReq{
		RepoURL:    "https://example.com/repo.git",
		TargetPath: "repo",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestServiceCloneRejectsPathOutsideMountRoot(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeGitClient{}, t.TempDir())
	_, err := service.CloneRepository(context.Background(), CloneRepositoryReq{
		OwnerUIN:   "100001",
		UIN:        "200001",
		RepoURL:    "https://example.com/repo.git",
		TargetPath: "../escape",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
