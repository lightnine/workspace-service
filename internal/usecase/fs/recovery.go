package fs

import (
	"context"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	"go.uber.org/zap"
)

const (
	defaultRecoveryBatch    = 200
	defaultRecoveryMaxTries = 5
)

// StorageInspector re-stats a mount path during recovery. Existence (with a
// resolvable inode) is the source of truth for "the storage write landed".
type StorageInspector interface {
	StatInode(absPath string) (inode uint64, exists bool, err error)
}

// Recoverer reconciles pending write-ahead intents after a restart. For each
// pending intent it re-stats the target path:
//
//	exists  -> upsert ws_file_node, mark intent done
//	missing -> mark intent aborted (no file + no node row == consistent)
//	transient error -> increment attempt; abort once attempts exhaust maxTries
type Recoverer struct {
	intents   domainfile.IntentStore
	nodes     domainfile.NodeStore
	inspector StorageInspector
	batchSize int
	maxTries  int
	logger    *zap.Logger
}

// RecoveryStats summarizes a single Recover pass.
type RecoveryStats struct {
	Scanned   int
	Completed int
	Aborted   int
	Retried   int
}

// NewRecoverer builds a Recoverer. Any of intents/nodes/inspector may be nil
// (e.g. MySQL disabled); Recover then becomes a no-op.
func NewRecoverer(
	intents domainfile.IntentStore,
	nodes domainfile.NodeStore,
	inspector StorageInspector,
	logger *zap.Logger,
) *Recoverer {
	return &Recoverer{
		intents:   intents,
		nodes:     nodes,
		inspector: inspector,
		batchSize: defaultRecoveryBatch,
		maxTries:  defaultRecoveryMaxTries,
		logger:    logger,
	}
}

// Recover scans pending intents in batches until none remain (or a batch makes
// no terminal progress) and returns the aggregate stats.
func (r *Recoverer) Recover(ctx context.Context) (RecoveryStats, error) {
	var total RecoveryStats
	if r == nil || r.intents == nil || r.inspector == nil {
		return total, nil
	}

	for {
		pending, err := r.intents.ListPending(ctx, r.batchSize)
		if err != nil {
			return total, err
		}
		if len(pending) == 0 {
			break
		}

		progressed := false
		for _, intent := range pending {
			outcome := r.reconcile(ctx, intent)
			total.Scanned++
			switch outcome {
			case outcomeCompleted:
				total.Completed++
				progressed = true
			case outcomeAborted:
				total.Aborted++
				progressed = true
			case outcomeRetried:
				total.Retried++
			}
			if err := ctx.Err(); err != nil {
				return total, err
			}
		}

		// If a full batch only produced retries (no intent left pending state),
		// stop to avoid spinning on the same rows within one pass.
		if !progressed || len(pending) < r.batchSize {
			break
		}
	}

	if r.logger != nil && total.Scanned > 0 {
		r.logger.Info("file intent recovery completed",
			zap.Int("scanned", total.Scanned),
			zap.Int("completed", total.Completed),
			zap.Int("aborted", total.Aborted),
			zap.Int("retried", total.Retried),
		)
	}
	return total, nil
}

type recoverOutcome int

const (
	outcomeCompleted recoverOutcome = iota
	outcomeAborted
	outcomeRetried
)

func (r *Recoverer) reconcile(ctx context.Context, intent domainfile.Intent) recoverOutcome {
	inode, exists, err := r.inspector.StatInode(intent.AbsPath)
	if err != nil {
		return r.retryOrAbort(ctx, intent, "stat target: "+err.Error())
	}
	if !exists {
		_ = r.intents.MarkAborted(ctx, intent.ID, "target path absent on storage")
		return outcomeAborted
	}
	if inode == 0 {
		return r.retryOrAbort(ctx, intent, "inode unresolved for existing path")
	}

	if r.nodes != nil {
		if upErr := r.nodes.UpsertCreatedOrUpdated(ctx, domainfile.NodeMutation{
			InodeID:  inode,
			Actor:    intent.Actor,
			NodeType: intent.NodeType,
		}); upErr != nil {
			return r.retryOrAbort(ctx, intent, "upsert node: "+upErr.Error())
		}
	}

	_ = r.intents.MarkDone(ctx, intent.ID)
	return outcomeCompleted
}

// retryOrAbort bumps the attempt counter, aborting once the intent has
// exhausted maxTries so it cannot stay pending forever.
func (r *Recoverer) retryOrAbort(ctx context.Context, intent domainfile.Intent, reason string) recoverOutcome {
	if intent.Attempts+1 >= r.maxTries {
		_ = r.intents.MarkAborted(ctx, intent.ID, "max attempts exceeded: "+reason)
		return outcomeAborted
	}
	_ = r.intents.IncrementAttempt(ctx, intent.ID, reason)
	return outcomeRetried
}
