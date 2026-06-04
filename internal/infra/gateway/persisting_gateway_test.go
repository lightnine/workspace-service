package gateway

import (
	"context"
	"net/http"
	"testing"

	domaingateway "git.woa.com/leondli/workspace-service/internal/domain/gateway"
	"git.woa.com/leondli/workspace-service/internal/domain/identity"
	memorysession "git.woa.com/leondli/workspace-service/internal/infra/persistence/memory"
	"git.woa.com/leondli/workspace-service/pkg/ctxmeta"
)

type stubGateway struct {
	created domaingateway.CreateSessionRequest
	deleted string
}

func (s *stubGateway) CreateSession(_ context.Context, req domaingateway.CreateSessionRequest) (domaingateway.Session, error) {
	s.created = req
	return domaingateway.Session{
		ID:   "sess-1",
		Path: req.Path,
		Type: req.Type,
		Kernel: &domaingateway.Kernel{
			ID:             "kern-1",
			Name:           "python3",
			ExecutionState: "starting",
		},
	}, nil
}
func (s *stubGateway) ListSessions(context.Context) ([]domaingateway.Session, error) { return nil, nil }
func (s *stubGateway) GetSession(context.Context, string) (domaingateway.Session, error) {
	return domaingateway.Session{}, nil
}
func (s *stubGateway) UpdateSession(context.Context, string, domaingateway.UpdateSessionRequest) (domaingateway.Session, error) {
	return domaingateway.Session{}, nil
}
func (s *stubGateway) DeleteSession(_ context.Context, sessionID string) error {
	s.deleted = sessionID
	return nil
}
func (s *stubGateway) CreateKernel(context.Context, domaingateway.CreateKernelRequest) (domaingateway.Kernel, error) {
	return domaingateway.Kernel{}, nil
}
func (s *stubGateway) ListKernels(context.Context) ([]domaingateway.Kernel, error) { return nil, nil }
func (s *stubGateway) GetKernel(context.Context, string) (domaingateway.Kernel, error) {
	return domaingateway.Kernel{}, nil
}
func (s *stubGateway) DeleteKernel(context.Context, string) error { return nil }
func (s *stubGateway) InterruptKernel(context.Context, string) error { return nil }
func (s *stubGateway) RestartKernel(context.Context, string) (domaingateway.Kernel, error) {
	return domaingateway.Kernel{}, nil
}
func (s *stubGateway) ListKernelSpecs(context.Context) (domaingateway.KernelSpecs, error) {
	return domaingateway.KernelSpecs{}, nil
}
func (s *stubGateway) GetKernelSpec(context.Context, string) (domaingateway.KernelSpec, error) {
	return domaingateway.KernelSpec{}, nil
}
func (s *stubGateway) GetKernelSpecResource(context.Context, string) (domaingateway.KernelSpecResource, error) {
	return domaingateway.KernelSpecResource{}, nil
}
func (s *stubGateway) ProxyHTTP(http.ResponseWriter, *http.Request)     {}
func (s *stubGateway) ProxyWebSocket(http.ResponseWriter, *http.Request) {}

func TestPersistingGatewayCreateSessionWritesStore(t *testing.T) {
	t.Parallel()

	inner := &stubGateway{}
	store := memorysession.NewKernelSessionStore()
	gw := WrapWithSessionStore(inner, store, nil)

	ctx := ctxmeta.WithRequestContext(context.Background(), identity.RequestContext{
		OwnerUIN: "100", UIN: "200", AppID: "app-1", WorkspaceID: "ws-1",
	})
	_, err := gw.CreateSession(ctx, domaingateway.CreateSessionRequest{
		Path: "demo/a.ipynb",
		Type: "notebook",
		Kernel: &domaingateway.KernelRef{
			Name:       "python3",
			CustomEnvs: []byte(`{"KERNEL_SID":"spark-1"}`),
		},
		Cluster: `{"cluster_id":"c1"}`,
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	rec, err := store.GetBySessionID(ctx, "sess-1")
	if err != nil {
		t.Fatalf("GetBySessionID: %v", err)
	}
	if rec.KernelID != "kern-1" || rec.AppID != "app-1" || rec.Path != "demo/a.ipynb" {
		t.Fatalf("record = %+v", rec)
	}
	if string(rec.CustomEnvs) != `{"KERNEL_SID":"spark-1"}` {
		t.Fatalf("custom_envs = %s", rec.CustomEnvs)
	}
}

func TestPersistingGatewayDeleteSessionMarksDeleted(t *testing.T) {
	t.Parallel()

	inner := &stubGateway{}
	store := memorysession.NewKernelSessionStore()
	gw := WrapWithSessionStore(inner, store, nil)
	ctx := context.Background()

	_, _ = gw.CreateSession(ctx, domaingateway.CreateSessionRequest{Path: "a.ipynb", Type: "notebook"})
	if err := gw.DeleteSession(ctx, "sess-1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := store.GetBySessionID(ctx, "sess-1"); err == nil {
		t.Fatal("expected session removed from active store")
	}
}
