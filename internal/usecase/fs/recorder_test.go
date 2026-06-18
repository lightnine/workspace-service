package fs

import (
	"context"
	"errors"
	"testing"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	"git.woa.com/leondli/workspace-service/internal/domain/identity"
	infrafs "git.woa.com/leondli/workspace-service/internal/infra/fs"
	"git.woa.com/leondli/workspace-service/internal/infra/persistence/memory"
)

func testRequestContext() identity.RequestContext {
	return identity.RequestContext{
		OwnerUIN: "100001", UIN: "200001", AppID: "app1", WorkspaceID: "ws1",
	}
}

// Happy path: the usecase writes storage, records ws_file_node, and the
// write-ahead intent ends up resolved (no pending rows remain).
func TestServiceCreateFileOrchestratesStorageAndDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mount := t.TempDir()
	nodes := memory.NewFileNodeStore()
	intents := memory.NewFileIntentStore()
	fsClient := infrafs.NewWorkspaceFSClient(nodes, mount)
	svc := NewServiceWithStores(fsClient, mount, nil, nodes, intents)

	resp, err := svc.CreateFile(ctx, CreateFileReq{
		Context: testRequestContext(), Path: "note.txt", Content: []byte("hi"),
	})
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if resp.InodeID == 0 {
		t.Fatal("expected inode in response")
	}

	recs, _ := nodes.LookupByInodeIDs(ctx, []uint64{resp.InodeID})
	if rec, ok := recs[resp.InodeID]; !ok || rec.OwnerUIN != "100001" {
		t.Fatalf("node not recorded: %+v", recs)
	}

	if pending, _ := intents.ListPending(ctx, 10); len(pending) != 0 {
		t.Fatalf("pending intents = %d, want 0", len(pending))
	}
}

// Crash simulation: the storage write succeeds but the DB upsert fails. The
// user operation still succeeds, the intent stays pending, and a later recovery
// pass converges storage and ws_file_node.
func TestServiceCreateFileLeavesPendingThenRecovers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mount := t.TempDir()
	fsNodes := memory.NewFileNodeStore()
	fsClient := infrafs.NewWorkspaceFSClient(fsNodes, mount)
	intents := memory.NewFileIntentStore()

	// The recorder's node store fails, simulating a crash/DB outage between the
	// storage write and the metadata write.
	svc := NewServiceWithStores(fsClient, mount, nil, failingNodeStore{err: errors.New("db down")}, intents)

	resp, err := svc.CreateFile(ctx, CreateFileReq{
		Context: testRequestContext(), Path: "note.txt", Content: []byte("hi"),
	})
	if err != nil {
		t.Fatalf("create file should still succeed: %v", err)
	}

	pending, _ := intents.ListPending(ctx, 10)
	if len(pending) != 1 {
		t.Fatalf("pending intents = %d, want 1 (crash-equivalent state)", len(pending))
	}
	if recs, _ := fsNodes.LookupByInodeIDs(ctx, []uint64{resp.InodeID}); len(recs) != 0 {
		t.Fatalf("node should not be recorded yet: %+v", recs)
	}

	// Recovery with a healthy DB converges the state.
	stats, err := NewRecoverer(intents, fsNodes, infrafs.NewInodeInspector(), nil).Recover(ctx)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if stats.Completed != 1 {
		t.Fatalf("stats = %+v, want completed=1", stats)
	}
	if pending2, _ := intents.ListPending(ctx, 10); len(pending2) != 0 {
		t.Fatalf("pending after recovery = %d, want 0", len(pending2))
	}
	if recs, _ := fsNodes.LookupByInodeIDs(ctx, []uint64{resp.InodeID}); len(recs) != 1 {
		t.Fatalf("recovery did not record node: %+v", recs)
	}
}

// When the storage write itself fails, the intent must be aborted (no file =>
// no node row), not left pending.
func TestServiceCreateFileAbortsIntentOnStorageError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mount := t.TempDir()
	nodes := memory.NewFileNodeStore()
	intents := memory.NewFileIntentStore()
	fsClient := infrafs.NewWorkspaceFSClient(nodes, mount)
	svc := NewServiceWithStores(fsClient, mount, nil, nodes, intents)

	req := CreateFileReq{Context: testRequestContext(), Path: "note.txt", Content: []byte("hi")}
	if _, err := svc.CreateFile(ctx, req); err != nil {
		t.Fatalf("first create: %v", err)
	}
	// Second create without overwrite must fail at the storage layer.
	if _, err := svc.CreateFile(ctx, req); err == nil {
		t.Fatal("expected already-exists error on second create")
	}

	if pending, _ := intents.ListPending(ctx, 10); len(pending) != 0 {
		t.Fatalf("pending intents = %d, want 0 (failed op aborted)", len(pending))
	}
}

// When the journal cannot durably record the intent, the user write still
// proceeds (best-effort), it just loses the write-ahead guarantee.
func TestServiceCreateFileDegradesWhenEnqueueFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mount := t.TempDir()
	nodes := memory.NewFileNodeStore()
	intents := memory.NewFileIntentStore()
	intents.EnqueueErr = errors.New("journal unavailable")
	fsClient := infrafs.NewWorkspaceFSClient(nodes, mount)
	svc := NewServiceWithStores(fsClient, mount, nil, nodes, intents)

	resp, err := svc.CreateFile(ctx, CreateFileReq{
		Context: testRequestContext(), Path: "note.txt", Content: []byte("hi"),
	})
	if err != nil {
		t.Fatalf("create file should still succeed: %v", err)
	}
	// Node is still recorded directly even without a journal entry.
	if recs, _ := nodes.LookupByInodeIDs(ctx, []uint64{resp.InodeID}); len(recs) != 1 {
		t.Fatalf("node should be recorded best-effort: %+v", recs)
	}
}

// Notebooks must record node_type=notebook through the same orchestration.
func TestServiceCreateNotebookRecordsNotebookType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mount := t.TempDir()
	nodes := memory.NewFileNodeStore()
	intents := memory.NewFileIntentStore()
	fsClient := infrafs.NewWorkspaceFSClient(nodes, mount)
	svc := NewServiceWithStores(fsClient, mount, nil, nodes, intents)

	resp, err := svc.CreateNotebook(ctx, CreateNotebookReq{
		Context: testRequestContext(), Path: "demo", KernelName: "python3",
	})
	if err != nil {
		t.Fatalf("create notebook: %v", err)
	}
	recs, _ := nodes.LookupByInodeIDs(ctx, []uint64{resp.InodeID})
	if rec, ok := recs[resp.InodeID]; !ok || rec.NodeType != domainfile.NodeTypeNotebook {
		t.Fatalf("notebook node_type not recorded: %+v", recs)
	}
	if pending, _ := intents.ListPending(ctx, 10); len(pending) != 0 {
		t.Fatalf("pending intents = %d, want 0", len(pending))
	}
}
