package file

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// IntentStatus is the lifecycle state of a write-ahead file intent.
type IntentStatus string

const (
	// IntentPending means the storage write has been requested but the
	// ws_file_node metadata has not yet been confirmed. Pending intents are
	// the units the recovery scan reconciles after a crash.
	IntentPending IntentStatus = "pending"
	// IntentDone means storage + ws_file_node are consistent.
	IntentDone IntentStatus = "done"
	// IntentAborted means the storage write did not durably land (the target
	// path does not exist), so the operation is treated as never-happened.
	IntentAborted IntentStatus = "aborted"
)

// IntentOp identifies which create-style operation an intent journals.
type IntentOp string

const (
	IntentOpCreateFolder   IntentOp = "create_folder"
	IntentOpCreateFile     IntentOp = "create_file"
	IntentOpWriteFile      IntentOp = "write_file"
	IntentOpCreateNotebook IntentOp = "create_notebook"
)

// Intent is a durable write-ahead record of a create/write operation. It is
// persisted BEFORE the storage write so that, after a crash, a recovery scan
// can deterministically reconcile storage and ws_file_node:
//
//	target path exists  -> upsert ws_file_node, mark done
//	target path missing -> mark aborted (consistent: no file, no node row)
type Intent struct {
	ID        string
	Op        IntentOp
	AbsPath   string // absolute path on the mount; re-stat target during recovery
	Actor     NodeActor
	NodeType  string
	Status    IntentStatus
	Attempts  int
	LastError string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IntentStore persists write-ahead intents. Implementations must make Enqueue
// durable before returning so the recovery scan can always find in-flight work.
type IntentStore interface {
	// Enqueue durably records a pending intent.
	Enqueue(ctx context.Context, intent Intent) error
	// MarkDone marks an intent reconciled.
	MarkDone(ctx context.Context, id string) error
	// MarkAborted marks an intent as not-applicable (storage write never landed).
	MarkAborted(ctx context.Context, id, reason string) error
	// IncrementAttempt bumps the retry counter and records the last error,
	// leaving the intent pending for a later scan.
	IncrementAttempt(ctx context.Context, id, lastErr string) error
	// ListPending returns up to limit pending intents, oldest first.
	ListPending(ctx context.Context, limit int) ([]Intent, error)
}

// NewIntentID returns a random 128-bit hex identifier for an intent.
func NewIntentID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// rand.Read essentially never fails; fall back to a time-based id.
		return "intent-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
