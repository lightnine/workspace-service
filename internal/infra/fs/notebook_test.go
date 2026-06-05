package fs

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	domainfs "git.woa.com/leondli/workspace-service/internal/domain/fs"
	"git.woa.com/leondli/workspace-service/internal/infra/persistence/memory"
)

func TestCreateNotebook(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mount := t.TempDir()
	store := memory.NewFileNodeStore()
	client := NewWorkspaceFSClient(store, mount)
	actor := domainfs.Actor{
		OwnerUIN: "100001", UIN: "200001",
		AppID: "app1", WorkspaceID: "ws1",
	}

	dir := filepath.Join(mount, actor.UserPathPrefix(), "nb")
	if _, err := client.CreateFolder(ctx, domainfs.CreateFolderReq{Actor: actor, Path: dir}); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	nbPath := filepath.Join(dir, "demo.ipynb")
	info, err := client.CreateNotebook(ctx, domainfs.CreateNotebookReq{
		Actor: actor, Path: nbPath, KernelName: "python3",
	})
	if err != nil {
		t.Fatalf("create notebook: %v", err)
	}
	if info.NodeType != domainfile.NodeTypeNotebook {
		t.Fatalf("node_type = %q", info.NodeType)
	}

	read, err := client.ReadFile(ctx, domainfs.ReadFileReq{Actor: actor, Path: nbPath})
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(read.Content, &doc); err != nil {
		t.Fatalf("invalid ipynb json: %v", err)
	}
	if doc["nbformat"].(float64) != 4 {
		t.Fatalf("nbformat = %v", doc["nbformat"])
	}

	list, err := client.ListFiles(ctx, domainfs.ListFilesReq{Actor: actor, Path: dir})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Files) != 1 || list.Files[0].NodeType != domainfile.NodeTypeNotebook {
		t.Fatalf("listed node_type = %+v", list.Files)
	}
}
