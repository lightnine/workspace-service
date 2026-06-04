package git

import (
	"path/filepath"
	"testing"
)

func TestResolveGitDirInsideWorktreeWhenMetaRootEmpty(t *testing.T) {
	t.Parallel()
	mount := "/mnt/studio"
	worktree := filepath.Join(mount, "users/200001/repo")

	got, err := resolveGitDir(mount, "", worktree)
	if err != nil {
		t.Fatalf("resolveGitDir: %v", err)
	}
	want := filepath.Join(worktree, ".git")
	if got != want {
		t.Fatalf("git dir = %q, want %q", got, want)
	}
}

func TestResolveGitDirOutsideJuiceFSWhenMetaRootSet(t *testing.T) {
	t.Parallel()
	mount := "/mnt/studio"
	meta := "/var/wedata/git-meta"
	worktree := filepath.Join(mount, "users/200001/repo")

	got, err := resolveGitDir(mount, meta, worktree)
	if err != nil {
		t.Fatalf("resolveGitDir: %v", err)
	}
	want := filepath.Join(meta, "users", "200001", "repo", ".git")
	if got != want {
		t.Fatalf("git dir = %q, want %q", got, want)
	}
}

func TestResolveGitDirRejectsEscape(t *testing.T) {
	t.Parallel()
	_, err := resolveGitDir("/mnt/studio", "/var/meta", "/etc/passwd")
	if err == nil {
		t.Fatal("expected error for path outside mount root")
	}
}
