package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appconfig "git.woa.com/leondli/workspace-service/internal/config"
)

func TestProxyHTTPForwardsMethodPathAndBody(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/kernels/execute_task/save_outputs" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("file_id") != "f-1" {
			t.Fatalf("query file_id = %q", r.URL.Query().Get("file_id"))
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"ok":true}` {
			t.Fatalf("body = %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"saved":1}`))
	}))
	defer backend.Close()

	gw, err := NewOptionalGateway(appconfig.GatewayConfig{URL: backend.URL, AuthToken: "secret"}, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/kernels/execute_task/save_outputs?file_id=f-1", strings.NewReader(`{"ok":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gw.ProxyHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content-type = %q", rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != `{"saved":1}` {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestProxyHTTPSparkStageForwardsClusterHeader(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/sessions/spark-app/stage" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("cluster"); got != "eg-1" {
			t.Fatalf("cluster header = %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"stage":"running"}`))
	}))
	defer backend.Close()

	gw, err := NewOptionalGateway(appconfig.GatewayConfig{URL: backend.URL}, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/spark-app/stage?file_id=f-1", nil)
	req.Header.Set("cluster", "eg-1")
	rec := httptest.NewRecorder()

	gw.ProxyHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != `{"stage":"running"}` {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
