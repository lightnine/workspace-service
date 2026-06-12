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

func (f *fakeGitCommandService) CreateGitFolder(context.Context, usecasegit.CreateGitFolderReq) (usecasegit.CreateGitFolderResp, error) {
	return usecasegit.CreateGitFolderResp{}, nil
}
func (f *fakeGitCommandService) GetGitFolderStatus(context.Context, usecasegit.GetGitFolderStatusReq) (usecasegit.GetGitFolderStatusResp, error) {
	return usecasegit.GetGitFolderStatusResp{}, nil
}
func (f *fakeGitCommandService) PullRepository(context.Context, usecasegit.PullRepositoryReq) (usecasegit.PullRepositoryResp, error) {
	return usecasegit.PullRepositoryResp{}, nil
}
func (f *fakeGitCommandService) Commit(context.Context, usecasegit.CommitReq) (usecasegit.CommitResp, error) {
	return usecasegit.CommitResp{}, nil
}
func (f *fakeGitCommandService) PushRepository(context.Context, usecasegit.PushRepositoryReq) (usecasegit.PushRepositoryResp, error) {
	return usecasegit.PushRepositoryResp{}, nil
}
func (f *fakeGitCommandService) StageFiles(context.Context, usecasegit.StageFilesReq) error { return nil }
func (f *fakeGitCommandService) UnstageFiles(context.Context, usecasegit.UnstageFilesReq) error { return nil }
func (f *fakeGitCommandService) CommitAndPush(context.Context, usecasegit.CommitAndPushReq) (usecasegit.CommitAndPushResp, error) {
	return usecasegit.CommitAndPushResp{}, nil
}
func (f *fakeGitCommandService) CreateBranch(context.Context, usecasegit.CreateBranchReq) (usecasegit.BranchResp, error) {
	return usecasegit.BranchResp{}, nil
}
func (f *fakeGitCommandService) CheckoutBranch(context.Context, usecasegit.CheckoutBranchReq) (usecasegit.BranchResp, error) {
	return usecasegit.BranchResp{}, nil
}
func (f *fakeGitCommandService) ListBranches(context.Context, usecasegit.ListBranchesReq) (usecasegit.ListBranchesResp, error) {
	return usecasegit.ListBranchesResp{}, nil
}
func (f *fakeGitCommandService) GetStatus(context.Context, usecasegit.StatusReq) (usecasegit.StatusResp, error) {
	return usecasegit.StatusResp{}, nil
}
func (f *fakeGitCommandService) GetFileDiff(context.Context, usecasegit.FileDiffReq) (usecasegit.FileDiffResp, error) {
	return usecasegit.FileDiffResp{}, nil
}
func (f *fakeGitCommandService) GetCommitHistory(context.Context, usecasegit.CommitHistoryReq) (usecasegit.CommitHistoryResp, error) {
	return usecasegit.CommitHistoryResp{}, nil
}
func (f *fakeGitCommandService) DiscardChanges(context.Context, usecasegit.DiscardChangesReq) error {
	return nil
}
func (f *fakeGitCommandService) DeleteRepository(context.Context, usecasegit.DeleteRepositoryReq) error {
	return nil
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

	engine := NewWithHandlers(zap.NewNop(), "", &Handlers{
		Git: apphandler.NewGitHandler(&fakeGitCommandService{}),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/CloneRepo", strings.NewReader(`{
		"owner_uin": "100001",
		"uin": "200001",
		"app_id": "260073493",
		"workspace_id": "ws-test",
		"repo_url": "https://example.com/repo.git",
		"target_path": "repo",
		"branch": "main"
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

	engine := NewWithHandlers(zap.NewNop(), "/api", &Handlers{
		Git: apphandler.NewGitHandler(&fakeGitCommandService{}),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/CloneRepo", strings.NewReader(`{
		"owner_uin": "100001",
		"uin": "200001",
		"app_id": "260073493",
		"workspace_id": "ws-test",
		"repo_url": "https://example.com/repo.git",
		"target_path": "repo",
		"branch": "main"
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
