package fs

import (
	"context"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	domainfs "git.woa.com/leondli/workspace-service/internal/domain/fs"
)

// nodeRecorder orchestrates the "write storage, then write DB" sequence with a
// write-ahead intent so a crash in between is recoverable.
//
// Sequence for a create/write operation:
//
//  1. Enqueue a pending intent (durable) BEFORE touching storage.
//  2. Run the storage action (atomic write on the mount).
//  3. On success: upsert ws_file_node, then mark the intent done.
//     On storage error: mark the intent aborted (no file => no node row).
//
// If the process crashes after step 2 but before step 3 completes, the intent
// stays pending and the Recoverer reconciles it on restart.
//
// A nil nodeRecorder, or one without stores, degrades to running the action
// directly (used when MySQL is disabled).
type nodeRecorder struct {
	nodes   domainfile.NodeStore
	intents domainfile.IntentStore
}

func newNodeRecorder(nodes domainfile.NodeStore, intents domainfile.IntentStore) *nodeRecorder {
	if nodes == nil && intents == nil {
		return nil
	}
	return &nodeRecorder{nodes: nodes, intents: intents}
}

// run executes action wrapped in the write-ahead protocol and returns the
// storage result. Metadata bookkeeping never turns a successful storage write
// into a failure: finalize errors leave the intent pending for recovery.
func (r *nodeRecorder) run(
	ctx context.Context,
	op domainfile.IntentOp,
	absPath string,
	actor domainfs.Actor,
	nodeType string,
	action func() (domainfs.FileInfo, error),
) (domainfs.FileInfo, error) {
	if r == nil {
		return action()
	}

	intentID := r.enqueue(ctx, op, absPath, actor, nodeType)

	info, err := action()
	if err != nil {
		if intentID != "" {
			_ = r.intents.MarkAborted(ctx, intentID, err.Error())
		}
		return domainfs.FileInfo{}, err
	}

	r.finalize(ctx, intentID, info.InodeID, actor, nodeType)
	return info, nil
}

// enqueue durably records a pending intent and returns its id. On failure it
// returns "" so the operation degrades to best-effort recording instead of
// blocking the user's write.
func (r *nodeRecorder) enqueue(
	ctx context.Context,
	op domainfile.IntentOp,
	absPath string,
	actor domainfs.Actor,
	nodeType string,
) string {
	if r.intents == nil {
		return ""
	}
	id := domainfile.NewIntentID()
	intent := domainfile.Intent{
		ID:       id,
		Op:       op,
		AbsPath:  absPath,
		Actor:    toNodeActor(actor),
		NodeType: nodeType,
	}
	if err := r.intents.Enqueue(ctx, intent); err != nil {
		return ""
	}
	return id
}

func (r *nodeRecorder) finalize(
	ctx context.Context,
	intentID string,
	inode uint64,
	actor domainfs.Actor,
	nodeType string,
) {
	if r.nodes != nil {
		if inode == 0 {
			// Inode unresolved; leave the intent pending for the recovery scan
			// to re-stat the path and complete.
			return
		}
		if err := r.nodes.UpsertCreatedOrUpdated(ctx, domainfile.NodeMutation{
			InodeID:  inode,
			Actor:    toNodeActor(actor),
			NodeType: nodeType,
		}); err != nil {
			// Leave pending; recovery retries.
			return
		}
	}
	if intentID != "" {
		_ = r.intents.MarkDone(ctx, intentID)
	}
}

func toNodeActor(actor domainfs.Actor) domainfile.NodeActor {
	return domainfile.NewNodeActor(actor.OwnerUIN, actor.UIN, actor.AppID, actor.WorkspaceID)
}
