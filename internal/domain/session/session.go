package session

import (
	"context"
	"encoding/json"

	domaingateway "git.woa.com/leondli/workspace-service/internal/domain/gateway"
	"git.woa.com/leondli/workspace-service/internal/domain/identity"
)

// Record is the persisted Jupyter session row (kernel_session table).
type Record struct {
	SessionID   string
	Path        string
	Name        string
	Type        string
	KernelID    string
	KernelName  string
	Cluster     string
	CustomEnvs  json.RawMessage
	LaunchPlan  json.RawMessage
	State       string
	OwnerUIN    string
	UIN         string
	AppID       string
	WorkspaceID string
}

// Store persists session metadata after gateway operations succeed.
type Store interface {
	Upsert(ctx context.Context, record Record) error
	UpdateState(ctx context.Context, sessionID, state string) error
	MarkDeleted(ctx context.Context, sessionID string) error
	GetBySessionID(ctx context.Context, sessionID string) (Record, error)
	GetByKernelID(ctx context.Context, kernelID string) (Record, error)
}

// RecordFromCreate builds a store record from a successful gateway create response.
func RecordFromCreate(
	sess domaingateway.Session,
	actor identity.RequestContext,
	req domaingateway.CreateSessionRequest,
) Record {
	rec := Record{
		SessionID:   sess.ID,
		Path:        firstNonEmpty(sess.Path, req.Path),
		Name:        firstNonEmpty(sess.Name, req.Name),
		Type:        firstNonEmpty(sess.Type, req.Type),
		OwnerUIN:    actor.OwnerUIN,
		UIN:         actor.UIN,
		AppID:       actor.AppID,
		WorkspaceID: actor.WorkspaceID,
		Cluster:     req.Cluster,
	}
	if sess.Kernel != nil {
		rec.KernelID = sess.Kernel.ID
		rec.KernelName = firstNonEmpty(sess.Kernel.Name, kernelNameFromRef(req.Kernel))
		rec.State = sess.Kernel.ExecutionState
	} else if req.Kernel != nil {
		rec.KernelID = req.Kernel.ID
		rec.KernelName = req.Kernel.Name
	}
	if req.Kernel != nil && len(req.Kernel.CustomEnvs) > 0 {
		rec.CustomEnvs = append(json.RawMessage(nil), req.Kernel.CustomEnvs...)
	}
	return rec
}

// RecordFromGateway merges gateway session view into an existing persisted row shape.
func RecordFromGateway(sess domaingateway.Session, actor identity.RequestContext) Record {
	rec := Record{
		SessionID:   sess.ID,
		Path:        sess.Path,
		Name:        sess.Name,
		Type:        sess.Type,
		OwnerUIN:    actor.OwnerUIN,
		UIN:         actor.UIN,
		AppID:       actor.AppID,
		WorkspaceID: actor.WorkspaceID,
	}
	if sess.Kernel != nil {
		rec.KernelID = sess.Kernel.ID
		rec.KernelName = sess.Kernel.Name
		rec.State = sess.Kernel.ExecutionState
	}
	return rec
}

func kernelNameFromRef(ref *domaingateway.KernelRef) string {
	if ref == nil {
		return ""
	}
	return ref.Name
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
