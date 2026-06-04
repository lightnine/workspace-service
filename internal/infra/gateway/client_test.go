package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	appconfig "git.woa.com/leondli/workspace-service/internal/config"
	domaingateway "git.woa.com/leondli/workspace-service/internal/domain/gateway"
)

func TestNewOptionalGatewayDisabledWhenURLEmpty(t *testing.T) {
	t.Parallel()

	gw, err := NewOptionalGateway(appconfig.GatewayConfig{URL: ""}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gw != nil {
		t.Fatalf("gateway = %v, want nil when url empty", gw)
	}
}

func newTestGateway(t *testing.T, url string) domaingateway.Gateway {
	t.Helper()
	gw, err := NewOptionalGateway(appconfig.GatewayConfig{
		URL:           url,
		AuthHeaderKey: "Authorization",
		AuthScheme:    "token",
		AuthToken:     "secret-token",
		ValidateCert:  true,
	}, nil)
	if err != nil {
		t.Fatalf("build gateway: %v", err)
	}
	return gw
}

func TestCreateKernelSendsBodyAndAuth(t *testing.T) {
	t.Parallel()

	var (
		gotMethod string
		gotPath   string
		gotAuth   string
		gotBody   domaingateway.CreateKernelRequest
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"k-1","name":"python3","execution_state":"starting"}`)
	}))
	defer server.Close()

	gw := newTestGateway(t, server.URL)

	kernel, err := gw.CreateKernel(context.Background(), domaingateway.CreateKernelRequest{Name: "python3"})
	if err != nil {
		t.Fatalf("create kernel: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/kernels" {
		t.Fatalf("path = %q, want /api/kernels", gotPath)
	}
	if gotAuth != "token secret-token" {
		t.Fatalf("auth = %q, want %q", gotAuth, "token secret-token")
	}
	if gotBody.Name != "python3" {
		t.Fatalf("body name = %q, want python3", gotBody.Name)
	}
	if kernel.ID != "k-1" || kernel.Name != "python3" || kernel.ExecutionState != "starting" {
		t.Fatalf("kernel = %+v", kernel)
	}
}

func TestCreateSessionDecodesNestedKernel(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"s-1","path":"/a.ipynb","type":"notebook","kernel":{"id":"k-1","name":"python3"}}`)
	}))
	defer server.Close()

	gw := newTestGateway(t, server.URL)

	session, err := gw.CreateSession(context.Background(), domaingateway.CreateSessionRequest{
		Path:   "/a.ipynb",
		Type:   "notebook",
		Kernel: &domaingateway.KernelRef{Name: "python3"},
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.ID != "s-1" || session.Kernel == nil || session.Kernel.ID != "k-1" {
		t.Fatalf("session = %+v", session)
	}
}

func TestGatewayReturnsAPIErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"kernel not found","reason":"Not Found"}`)
	}))
	defer server.Close()

	gw := newTestGateway(t, server.URL)

	_, err := gw.GetKernel(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *domaingateway.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", apiErr.StatusCode)
	}
	if string(apiErr.Body) != `{"message":"kernel not found","reason":"Not Found"}` {
		t.Fatalf("body = %s", apiErr.Body)
	}
}

func TestDeleteKernelTreats404AsSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	gw := newTestGateway(t, server.URL)

	if err := gw.DeleteKernel(context.Background(), "missing"); err != nil {
		t.Fatalf("delete kernel: %v, want nil (404 idempotent)", err)
	}
}

func TestListKernelsRetriesOnServiceUnavailable(t *testing.T) {
	t.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `[{"id":"k-1","name":"python3"}]`)
	}))
	defer server.Close()

	gw, err := NewOptionalGateway(appconfig.GatewayConfig{URL: server.URL, MaxRequestRetries: 2}, nil)
	if err != nil {
		t.Fatalf("build gateway: %v", err)
	}

	kernels, err := gw.ListKernels(context.Background())
	if err != nil {
		t.Fatalf("list kernels: %v", err)
	}
	if len(kernels) != 1 || kernels[0].ID != "k-1" {
		t.Fatalf("kernels = %+v", kernels)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
}

func TestRestartKernelRefetchesModel(t *testing.T) {
	t.Parallel()

	var restartCalled, getCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/kernels/k-1/restart":
			restartCalled = true
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/api/kernels/k-1":
			getCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"id":"k-1","name":"python3","execution_state":"idle"}`)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	gw := newTestGateway(t, server.URL)

	kernel, err := gw.RestartKernel(context.Background(), "k-1")
	if err != nil {
		t.Fatalf("restart kernel: %v", err)
	}
	if !restartCalled || !getCalled {
		t.Fatalf("restartCalled=%v getCalled=%v, want both true", restartCalled, getCalled)
	}
	if kernel.ExecutionState != "idle" {
		t.Fatalf("kernel = %+v", kernel)
	}
}

func TestResolveWSTargetDerivesFromHTTP(t *testing.T) {
	t.Parallel()

	httpTarget, err := url.Parse("https://gw.example.com:8888/base")
	if err != nil {
		t.Fatalf("parse http target: %v", err)
	}
	ws, err := resolveWSTarget("", httpTarget)
	if err != nil {
		t.Fatalf("resolve ws target: %v", err)
	}
	if ws.Scheme != "wss" {
		t.Fatalf("scheme = %q, want wss", ws.Scheme)
	}
	if ws.Host != "gw.example.com:8888" {
		t.Fatalf("host = %q", ws.Host)
	}
}
