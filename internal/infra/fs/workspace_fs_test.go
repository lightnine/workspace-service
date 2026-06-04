package fs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	domainfs "git.woa.com/leondli/workspace-service/internal/domain/fs"
)

func TestWorkspaceFSFileLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mount := t.TempDir()
	client := NewWorkspaceFSClient(nil, mount)
	actor := domainfs.Actor{
		OwnerUIN: "100001", UIN: "200001",
		AppID: "app1", WorkspaceID: "ws1",
	}

	dir := filepath.Join(mount, actor.UserPathPrefix(), "docs")
	file := filepath.Join(dir, "note.txt")

	_, err := client.CreateFolder(ctx, domainfs.CreateFolderReq{Actor: actor, Path: dir})
	if err != nil {
		t.Fatalf("create folder: %v", err)
	}

	_, err = client.CreateFile(ctx, domainfs.CreateFileReq{
		Actor: actor, Path: file, Content: []byte("hello"), Overwrite: false,
	})
	if err != nil {
		t.Fatalf("create file: %v", err)
	}

	status, err := client.ListFiles(ctx, domainfs.ListFilesReq{Actor: actor, Path: dir})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(status.Files) != 1 {
		t.Fatalf("files = %d, want 1", len(status.Files))
	}

	read, err := client.ReadFile(ctx, domainfs.ReadFileReq{Actor: actor, Path: file})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(read.Content) != "hello" {
		t.Fatalf("content = %q", read.Content)
	}

	moved := filepath.Join(mount, actor.UserPathPrefix(), "moved.txt")
	_, err = client.MovePath(ctx, domainfs.MovePathReq{
		Actor: actor, SrcPath: file, DestPath: moved, Overwrite: false,
	})
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("old file should be gone")
	}

	if err := client.DeletePath(ctx, domainfs.DeletePathReq{Actor: actor, Path: moved}); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
