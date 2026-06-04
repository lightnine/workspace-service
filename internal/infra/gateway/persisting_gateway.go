package gateway

import (
	"context"
	"net/http"

	domaingateway "git.woa.com/leondli/workspace-service/internal/domain/gateway"
	domainsession "git.woa.com/leondli/workspace-service/internal/domain/session"
	"git.woa.com/leondli/workspace-service/pkg/ctxmeta"
	"go.uber.org/zap"
)

// PersistingGateway wraps a gateway client and records session lifecycle in kernel_session.
type PersistingGateway struct {
	inner domaingateway.Gateway
	store domainsession.Store
	log   *zap.Logger
}

var _ domaingateway.Gateway = (*PersistingGateway)(nil)

// WrapWithSessionStore returns gw unchanged when store is nil.
func WrapWithSessionStore(gw domaingateway.Gateway, store domainsession.Store, log *zap.Logger) domaingateway.Gateway {
	if gw == nil || store == nil {
		return gw
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &PersistingGateway{inner: gw, store: store, log: log}
}

func (g *PersistingGateway) CreateSession(ctx context.Context, req domaingateway.CreateSessionRequest) (domaingateway.Session, error) {
	sess, err := g.inner.CreateSession(ctx, req)
	if err != nil {
		return sess, err
	}
	actor, _ := ctxmeta.RequestContextFrom(ctx)
	if err := g.store.Upsert(ctx, domainsession.RecordFromCreate(sess, actor, req)); err != nil {
		g.log.Warn("persist kernel session after create failed",
			zap.String("session_id", sess.ID),
			zap.Error(err),
		)
	}
	return sess, nil
}

func (g *PersistingGateway) UpdateSession(ctx context.Context, sessionID string, req domaingateway.UpdateSessionRequest) (domaingateway.Session, error) {
	sess, err := g.inner.UpdateSession(ctx, sessionID, req)
	if err != nil {
		return sess, err
	}
	actor, _ := ctxmeta.RequestContextFrom(ctx)
	if err := g.store.Upsert(ctx, domainsession.RecordFromGateway(sess, actor)); err != nil {
		g.log.Warn("persist kernel session after update failed",
			zap.String("session_id", sess.ID),
			zap.Error(err),
		)
	}
	return sess, nil
}

func (g *PersistingGateway) DeleteSession(ctx context.Context, sessionID string) error {
	if err := g.inner.DeleteSession(ctx, sessionID); err != nil {
		return err
	}
	if err := g.store.MarkDeleted(ctx, sessionID); err != nil {
		g.log.Warn("mark kernel session deleted failed",
			zap.String("session_id", sessionID),
			zap.Error(err),
		)
	}
	return nil
}

func (g *PersistingGateway) ListSessions(ctx context.Context) ([]domaingateway.Session, error) {
	return g.inner.ListSessions(ctx)
}

func (g *PersistingGateway) GetSession(ctx context.Context, sessionID string) (domaingateway.Session, error) {
	return g.inner.GetSession(ctx, sessionID)
}

func (g *PersistingGateway) CreateKernel(ctx context.Context, req domaingateway.CreateKernelRequest) (domaingateway.Kernel, error) {
	return g.inner.CreateKernel(ctx, req)
}

func (g *PersistingGateway) ListKernels(ctx context.Context) ([]domaingateway.Kernel, error) {
	return g.inner.ListKernels(ctx)
}

func (g *PersistingGateway) GetKernel(ctx context.Context, kernelID string) (domaingateway.Kernel, error) {
	return g.inner.GetKernel(ctx, kernelID)
}

func (g *PersistingGateway) DeleteKernel(ctx context.Context, kernelID string) error {
	return g.inner.DeleteKernel(ctx, kernelID)
}

func (g *PersistingGateway) InterruptKernel(ctx context.Context, kernelID string) error {
	return g.inner.InterruptKernel(ctx, kernelID)
}

func (g *PersistingGateway) RestartKernel(ctx context.Context, kernelID string) (domaingateway.Kernel, error) {
	return g.inner.RestartKernel(ctx, kernelID)
}

func (g *PersistingGateway) ListKernelSpecs(ctx context.Context) (domaingateway.KernelSpecs, error) {
	return g.inner.ListKernelSpecs(ctx)
}

func (g *PersistingGateway) GetKernelSpec(ctx context.Context, name string) (domaingateway.KernelSpec, error) {
	return g.inner.GetKernelSpec(ctx, name)
}

func (g *PersistingGateway) GetKernelSpecResource(ctx context.Context, resourcePath string) (domaingateway.KernelSpecResource, error) {
	return g.inner.GetKernelSpecResource(ctx, resourcePath)
}

func (g *PersistingGateway) ProxyHTTP(w http.ResponseWriter, r *http.Request) {
	g.inner.ProxyHTTP(w, r)
}

func (g *PersistingGateway) ProxyWebSocket(w http.ResponseWriter, r *http.Request) {
	g.inner.ProxyWebSocket(w, r)
}
