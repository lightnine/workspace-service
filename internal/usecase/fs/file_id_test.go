package fs

import (
	"testing"

	domainfs "git.woa.com/leondli/workspace-service/internal/domain/fs"
)

func TestResolveFileIDUsesInode(t *testing.T) {
	t.Parallel()
	id := resolveFileID(domainfs.FileInfo{
		InodeID: 160778517,
		Path:    "/mount/260073493/ws-test/users/200001/demo",
	})
	if id != "160778517" {
		t.Fatalf("file_id = %q, want inode string", id)
	}
}

func TestResolveFileIDEmptyWithoutInode(t *testing.T) {
	t.Parallel()
	if id := resolveFileID(domainfs.FileInfo{Path: "app/ws/users/1/a.ipynb"}); id != "" {
		t.Fatalf("file_id = %q, want empty when inode is unknown", id)
	}
}
