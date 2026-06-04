package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"git.woa.com/leondli/workspace-service/pkg/ctxmeta"
	"github.com/gin-gonic/gin"
)

func TestRequestIDGeneratesWhenHeaderMissing(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) {
		requestID, ok := ctxmeta.RequestIDFromContext(c.Request.Context())
		if !ok || requestID == "" {
			t.Fatal("request id missing from context")
		}
		c.String(http.StatusOK, requestID)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	got := rec.Header().Get(ctxmeta.HeaderRequestID)
	if got == "" {
		t.Fatal("response request id header is empty")
	}
	if rec.Body.String() != got {
		t.Fatalf("context request id = %q, response header = %q", rec.Body.String(), got)
	}
}

func TestRequestIDKeepsExistingHeader(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) {
		requestID, _ := ctxmeta.RequestIDFromContext(c.Request.Context())
		c.String(http.StatusOK, requestID)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set(ctxmeta.HeaderRequestID, "req-123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if got := rec.Header().Get(ctxmeta.HeaderRequestID); got != "req-123" {
		t.Fatalf("response request id = %q, want %q", got, "req-123")
	}
	if rec.Body.String() != "req-123" {
		t.Fatalf("context request id = %q, want %q", rec.Body.String(), "req-123")
	}
}
