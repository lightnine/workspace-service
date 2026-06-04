package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	domaingit "git.woa.com/leondli/workspace-service/internal/domain/git"
	"github.com/go-git/go-billy/v5/osfs"
)

type fakeFileNodeStore struct {
	upserts []domainfile.NodeMutation
	deletes []domainfile.NodeMutation
}

func (s *fakeFileNodeStore) UpsertCreatedOrUpdated(ctx context.Context, mutation domainfile.NodeMutation) error {
	s.upserts = append(s.upserts, mutation)
	return nil
}

func (s *fakeFileNodeStore) MarkDeleted(ctx context.Context, mutation domainfile.NodeMutation) error {
	s.deletes = append(s.deletes, mutation)
	return nil
}

func TestIdentityFSRecordsCreatedFileNode(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := &fakeFileNodeStore{}
	fs := newIdentityFS(context.Background(), osfs.New(root), domaingit.Actor{
		OwnerUIN: "100001",
		UIN:      "200001",
	}, store)

	f, err := fs.Create("repo.txt")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := f.Write([]byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if len(store.upserts) == 0 {
		t.Fatal("expected file node upsert")
	}
	got := store.upserts[len(store.upserts)-1]
	if got.InodeID == 0 {
		t.Fatal("inode id is zero")
	}
	if got.Actor.OwnerUIN != "100001" || got.Actor.UIN != "200001" {
		t.Fatalf("actor = %+v", got.Actor)
	}
}

func TestIdentityFSMarksDeletedFileNode(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "old.txt"), []byte("old"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	store := &fakeFileNodeStore{}
	fs := newIdentityFS(context.Background(), osfs.New(root), domaingit.Actor{
		OwnerUIN: "100001",
		UIN:      "200001",
	}, store)

	if err := fs.Remove("old.txt"); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if len(store.deletes) != 1 {
		t.Fatalf("delete records = %d, want 1", len(store.deletes))
	}
	if store.deletes[0].InodeID == 0 {
		t.Fatal("deleted inode id is zero")
	}
}
