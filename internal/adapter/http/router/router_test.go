package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apphandler "git.woa.com/leondli/workspace-service/internal/adapter/http/handler"
	usecasegit "git.woa.com/leondli/workspace-service/internal/usecase/git"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type fakeGitCommandService struct{}

func (f *fakeGitCommandService) CloneRepository(ctx context.Context, req usecasegit.CloneRepositoryReq) (usecasegit.CloneRepositoryResp, error) {
	return usecasegit.CloneRepositoryResp{
		RepoURL: req.RepoURL,
		Path:    req.TargetPath,
	}, nil
}

func TestNewRegistersHealthRoutes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	engine := New(zap.NewNop())

	tests := []struct {
		name string
		path string
	}{
		{name: "healthz", path: "/healthz"},
		{name: "readyz", path: "/readyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			engine.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
		})
	}
}

func TestNewWithHandlersRegistersCloneRepoWithEmptyPrefix(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	engine := NewWithHandlers(zap.NewNop(), "", apphandler.NewGitHandler(&fakeGitCommandService{}), nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/CloneRepo", strings.NewReader(`{
		"owner_uin": "100001",
		"uin": "200001",
		"repo_url": "https://example.com/repo.git",
		"target_path": "users/200001/repo"
	}`))
	req.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestNewWithHandlersRegistersCloneRepoWithPrefix(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	engine := NewWithHandlers(zap.NewNop(), "/api", apphandler.NewGitHandler(&fakeGitCommandService{}), nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/CloneRepo", strings.NewReader(`{
		"owner_uin": "100001",
		"uin": "200001",
		"repo_url": "https://example.com/repo.git",
		"target_path": "users/200001/repo"
	}`))
	req.Header.Set("Content-Type", "application/json")

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got struct {
		Common struct {
			Code int `json:"code"`
		} `json:"common"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Common.Code != 0 {
		t.Fatalf("common code = %d, want 0", got.Common.Code)
	}
}
