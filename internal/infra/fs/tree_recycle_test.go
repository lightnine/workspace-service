package fs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	domainfs "git.woa.com/leondli/workspace-service/internal/domain/fs"
)

func TestValidatePathAndBreadcrumb(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mount := t.TempDir()
	client := NewWorkspaceFSClient(nil, mount)
	actor := domainfs.Actor{
		OwnerUIN: "100001", UIN: "200001", AppID: "app1", WorkspaceID: "ws1",
	}

	root := client.userRootAbs(actor)
	dir := filepath.Join(root, "docs")
	if _, err := client.CreateFolder(ctx, domainfs.CreateFolderReq{Actor: actor, Path: dir}); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	file := filepath.Join(dir, "a.txt")
	if _, err := client.CreateFile(ctx, domainfs.CreateFileReq{Actor: actor, Path: file, Content: []byte("x")}); err != nil {
		t.Fatalf("create: %v", err)
	}

	exists, err := client.ValidatePath(ctx, domainfs.ValidatePathReq{Actor: actor, ParentPath: dir, Name: "a.txt"})
	if err != nil || !exists.Exists {
		t.Fatalf("validate existing: exists=%v err=%v", exists.Exists, err)
	}
	missing, err := client.ValidatePath(ctx, domainfs.ValidatePathReq{Actor: actor, ParentPath: dir, Name: "missing.txt"})
	if err != nil || missing.Exists {
		t.Fatalf("validate missing: exists=%v err=%v", missing.Exists, err)
	}

	crumbs, err := client.GetFolderNodePath(ctx, domainfs.GetFolderNodePathReq{Actor: actor, Path: dir})
	if err != nil {
		t.Fatalf("breadcrumb: %v", err)
	}
	if len(crumbs.Nodes) < 2 {
		t.Fatalf("nodes = %d, want >= 2", len(crumbs.Nodes))
	}
}

func TestSoftDeleteRestoreRecycleBin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mount := t.TempDir()
	client := NewWorkspaceFSClient(nil, mount)
	actor := domainfs.Actor{
		OwnerUIN: "100001", UIN: "200001", AppID: "app1", WorkspaceID: "ws1",
	}

	file := filepath.Join(client.userRootAbs(actor), "note.txt")
	if _, err := client.CreateFile(ctx, domainfs.CreateFileReq{Actor: actor, Path: file, Content: []byte("x")}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := client.DeletePath(ctx, domainfs.DeletePathReq{Actor: actor, Path: file, SoftDelete: true}); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("original should be gone")
	}

	bin, err := client.ListRecycleBin(ctx, domainfs.ListRecycleBinReq{Actor: actor})
	if err != nil || len(bin.Files) == 0 {
		t.Fatalf("list recycle: files=%d err=%v", len(bin.Files), err)
	}

	trashItem := bin.Files[0]
	restored, err := client.RestorePath(ctx, domainfs.RestorePathReq{Actor: actor, TrashPath: trashItem.Path})
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if _, err := os.Stat(restored.Path); err != nil {
		t.Fatalf("restored missing: %v", err)
	}
}
