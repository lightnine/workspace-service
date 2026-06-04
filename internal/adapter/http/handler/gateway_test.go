package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domaingateway "git.woa.com/leondli/workspace-service/internal/domain/gateway"
	"github.com/gin-gonic/gin"
)

type fakeGateway struct {
	createdKernel  domaingateway.CreateKernelRequest
	createdSession domaingateway.CreateSessionRequest
	interruptedID  string
	err            error
}

func (f *fakeGateway) CreateSession(_ context.Context, req domaingateway.CreateSessionRequest) (domaingateway.Session, error) {
	f.createdSession = req
	if f.err != nil {
		return domaingateway.Session{}, f.err
	}
	return domaingateway.Session{ID: "s-1", Path: req.Path, Type: req.Type, Kernel: &domaingateway.Kernel{ID: "k-1", Name: "python3"}}, nil
}
func (f *fakeGateway) ListSessions(context.Context) ([]domaingateway.Session, error) {
	return []domaingateway.Session{{ID: "s-1"}}, f.err
}
func (f *fakeGateway) GetSession(_ context.Context, id string) (domaingateway.Session, error) {
	return domaingateway.Session{ID: id}, f.err
}
func (f *fakeGateway) UpdateSession(_ context.Context, id string, _ domaingateway.UpdateSessionRequest) (domaingateway.Session, error) {
	return domaingateway.Session{ID: id}, f.err
}
func (f *fakeGateway) DeleteSession(context.Context, string) error { return f.err }

func (f *fakeGateway) CreateKernel(_ context.Context, req domaingateway.CreateKernelRequest) (domaingateway.Kernel, error) {
	f.createdKernel = req
	if f.err != nil {
		return domaingateway.Kernel{}, f.err
	}
	return domaingateway.Kernel{ID: "k-1", Name: req.Name}, nil
}
func (f *fakeGateway) ListKernels(context.Context) ([]domaingateway.Kernel, error) {
	return []domaingateway.Kernel{{ID: "k-1"}}, f.err
}
func (f *fakeGateway) GetKernel(_ context.Context, id string) (domaingateway.Kernel, error) {
	return domaingateway.Kernel{ID: id}, f.err
}
func (f *fakeGateway) DeleteKernel(context.Context, string) error { return f.err }
func (f *fakeGateway) InterruptKernel(_ context.Context, id string) error {
	f.interruptedID = id
	return f.err
}
func (f *fakeGateway) RestartKernel(_ context.Context, id string) (domaingateway.Kernel, error) {
	return domaingateway.Kernel{ID: id}, f.err
}
func (f *fakeGateway) ListKernelSpecs(context.Context) (domaingateway.KernelSpecs, error) {
	return domaingateway.KernelSpecs{Default: "python3"}, f.err
}
func (f *fakeGateway) GetKernelSpec(_ context.Context, name string) (domaingateway.KernelSpec, error) {
	return domaingateway.KernelSpec{Name: name}, f.err
}
func (f *fakeGateway) GetKernelSpecResource(context.Context, string) (domaingateway.KernelSpecResource, error) {
	return domaingateway.KernelSpecResource{ContentType: "image/png", Data: []byte("png")}, f.err
}
func (f *fakeGateway) ProxyWebSocket(http.ResponseWriter, *http.Request) {}

func newGatewayTestRouter(gw domaingateway.Gateway) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewGatewayHandler(gw)
	r := gin.New()
	r.POST("/api/sessions", h.CreateSession)
	r.POST("/api/kernels", h.CreateKernel)
	r.POST("/api/kernels/:kernel_id/interrupt", h.InterruptKernel)
	return r
}

func TestGatewayHandlerCreateKernelReturns201(t *testing.T) {
	t.Parallel()

	gw := &fakeGateway{}
	r := newGatewayTestRouter(gw)

	req := httptest.NewRequest(http.MethodPost, "/api/kernels", strings.NewReader(`{"name":"python3"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", rec.Code, rec.Body.String())
	}
	if gw.createdKernel.Name != "python3" {
		t.Fatalf("created kernel name = %q", gw.createdKernel.Name)
	}
	var kernel domaingateway.Kernel
	if err := json.Unmarshal(rec.Body.Bytes(), &kernel); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if kernel.ID != "k-1" {
		t.Fatalf("kernel id = %q", kernel.ID)
	}
}

func TestGatewayHandlerCreateKernelWithEmptyBody(t *testing.T) {
	t.Parallel()

	gw := &fakeGateway{}
	r := newGatewayTestRouter(gw)

	req := httptest.NewRequest(http.MethodPost, "/api/kernels", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", rec.Code, rec.Body.String())
	}
}

func TestGatewayHandlerInterruptReturns204(t *testing.T) {
	t.Parallel()

	gw := &fakeGateway{}
	r := newGatewayTestRouter(gw)

	req := httptest.NewRequest(http.MethodPost, "/api/kernels/k-9/interrupt", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if gw.interruptedID != "k-9" {
		t.Fatalf("interrupted id = %q, want k-9", gw.interruptedID)
	}
}

func TestGatewayHandlerRelaysGatewayAPIError(t *testing.T) {
	t.Parallel()

	gw := &fakeGateway{err: &domaingateway.APIError{
		StatusCode:  http.StatusConflict,
		ContentType: "application/json",
		Body:        []byte(`{"message":"boom"}`),
	}}
	r := newGatewayTestRouter(gw)

	req := httptest.NewRequest(http.MethodPost, "/api/kernels", strings.NewReader(`{"name":"python3"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if rec.Body.String() != `{"message":"boom"}` {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestGatewayHandlerInvalidBodyReturns400(t *testing.T) {
	t.Parallel()

	gw := &fakeGateway{}
	r := newGatewayTestRouter(gw)

	req := httptest.NewRequest(http.MethodPost, "/api/sessions", strings.NewReader(`{not json`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
