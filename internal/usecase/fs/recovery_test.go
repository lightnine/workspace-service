package fs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	infrafs "git.woa.com/leondli/workspace-service/internal/infra/fs"
	"git.woa.com/leondli/workspace-service/internal/infra/persistence/memory"
)

func testActor() domainfile.NodeActor {
	return domainfile.NewNodeActor("100001", "200001", "app1", "ws1")
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// Crash after the storage write but before ws_file_node: the file exists, so
// recovery must restore the node row and mark the intent done.
func TestRecovererCompletesWhenFileExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mount := t.TempDir()
	abs := filepath.Join(mount, "app1", "ws1", "users", "200001", "note.txt")
	writeFile(t, abs, "hi")

	intents := memory.NewFileIntentStore()
	nodes := memory.NewFileNodeStore()
	id := domainfile.NewIntentID()
	if err := intents.Enqueue(ctx, domainfile.Intent{
		ID: id, Op: domainfile.IntentOpCreateFile, AbsPath: abs,
		Actor: testActor(), NodeType: domainfile.NodeTypeFile,
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	stats, err := NewRecoverer(intents, nodes, infrafs.NewInodeInspector(), nil).Recover(ctx)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if stats.Completed != 1 || stats.Aborted != 0 {
		t.Fatalf("stats = %+v, want completed=1", stats)
	}

	got, ok := intents.Get(id)
	if !ok || got.Status != domainfile.IntentDone {
		t.Fatalf("intent status = %v, want done", got.Status)
	}

	inode, exists, _ := infrafs.NewInodeInspector().StatInode(abs)
	if !exists {
		t.Fatal("file should exist")
	}
	recs, _ := nodes.LookupByInodeIDs(ctx, []uint64{inode})
	if rec, ok := recs[inode]; !ok || rec.OwnerUIN != "100001" {
		t.Fatalf("node not recorded: %+v", recs)
	}
}

// Crash before the storage write landed: the file does not exist, which is a
// consistent state (no file, no node row), so the intent must be aborted.
func TestRecovererAbortsWhenFileMissing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mount := t.TempDir()
	abs := filepath.Join(mount, "app1", "ws1", "users", "200001", "ghost.txt")

	intents := memory.NewFileIntentStore()
	nodes := memory.NewFileNodeStore()
	id := domainfile.NewIntentID()
	if err := intents.Enqueue(ctx, domainfile.Intent{
		ID: id, Op: domainfile.IntentOpCreateFile, AbsPath: abs,
		Actor: testActor(), NodeType: domainfile.NodeTypeFile,
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	stats, err := NewRecoverer(intents, nodes, infrafs.NewInodeInspector(), nil).Recover(ctx)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if stats.Aborted != 1 || stats.Completed != 0 {
		t.Fatalf("stats = %+v, want aborted=1", stats)
	}

	got, _ := intents.Get(id)
	if got.Status != domainfile.IntentAborted {
		t.Fatalf("intent status = %v, want aborted", got.Status)
	}
	if pending, _ := intents.ListPending(ctx, 10); len(pending) != 0 {
		t.Fatalf("pending = %d, want 0", len(pending))
	}
}

type failingNodeStore struct{ err error }

func (f failingNodeStore) UpsertCreatedOrUpdated(context.Context, domainfile.NodeMutation) error {
	return f.err
}
func (failingNodeStore) MarkDeleted(context.Context, domainfile.NodeMutation) error { return nil }
func (failingNodeStore) LookupByInodeIDs(context.Context, []uint64) (map[uint64]domainfile.NodeRecord, error) {
	return nil, nil
}

// A persistently failing DB keeps the intent pending across passes, but it must
// not stay pending forever: after maxTries the intent is aborted.
func TestRecovererRetriesThenAbortsOnUpsertError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mount := t.TempDir()
	abs := filepath.Join(mount, "app1", "ws1", "users", "200001", "retry.txt")
	writeFile(t, abs, "hi")

	intents := memory.NewFileIntentStore()
	nodes := failingNodeStore{err: errors.New("db down")}
	id := domainfile.NewIntentID()
	if err := intents.Enqueue(ctx, domainfile.Intent{
		ID: id, Op: domainfile.IntentOpCreateFile, AbsPath: abs,
		Actor: testActor(), NodeType: domainfile.NodeTypeFile,
	}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	rec := NewRecoverer(intents, nodes, infrafs.NewInodeInspector(), nil)
	var final domainfile.Intent
	for i := 0; i < defaultRecoveryMaxTries+1; i++ {
		if _, err := rec.Recover(ctx); err != nil {
			t.Fatalf("recover pass %d: %v", i, err)
		}
		got, _ := intents.Get(id)
		final = got
		if got.Status == domainfile.IntentAborted {
			break
		}
	}

	if final.Status != domainfile.IntentAborted {
		t.Fatalf("intent status = %v (attempts=%d), want aborted", final.Status, final.Attempts)
	}
}

// A disabled journal (nil stores) makes recovery a safe no-op.
func TestRecovererNoopWhenDisabled(t *testing.T) {
	t.Parallel()
	stats, err := NewRecoverer(nil, nil, nil, nil).Recover(context.Background())
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if (stats != RecoveryStats{}) {
		t.Fatalf("stats = %+v, want zero", stats)
	}
}
