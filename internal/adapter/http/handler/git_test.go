package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.woa.com/leondli/workspace-service/internal/adapter/http/req"
	"git.woa.com/leondli/workspace-service/internal/testutil"
	usecasegit "git.woa.com/leondli/workspace-service/internal/usecase/git"
	"github.com/gin-gonic/gin"
)

type fakeGitCommandService struct {
	req usecasegit.CloneRepositoryReq
	err error
}

func (f *fakeGitCommandService) CloneRepository(ctx context.Context, req usecasegit.CloneRepositoryReq) (usecasegit.CloneRepositoryResp, error) {
	f.req = req
	if f.err != nil {
		return usecasegit.CloneRepositoryResp{}, f.err
	}
	return usecasegit.CloneRepositoryResp{
		RepoURL: req.RepoURL,
		Path:    req.TargetPath,
	}, nil
}

func (f *fakeGitCommandService) CreateGitFolder(context.Context, usecasegit.CreateGitFolderReq) (usecasegit.CreateGitFolderResp, error) {
	return usecasegit.CreateGitFolderResp{}, f.err
}
func (f *fakeGitCommandService) GetGitFolderStatus(context.Context, usecasegit.GetGitFolderStatusReq) (usecasegit.GetGitFolderStatusResp, error) {
	return usecasegit.GetGitFolderStatusResp{}, f.err
}
func (f *fakeGitCommandService) PullRepository(context.Context, usecasegit.PullRepositoryReq) (usecasegit.PullRepositoryResp, error) {
	return usecasegit.PullRepositoryResp{}, f.err
}
func (f *fakeGitCommandService) Commit(context.Context, usecasegit.CommitReq) (usecasegit.CommitResp, error) {
	return usecasegit.CommitResp{}, f.err
}
func (f *fakeGitCommandService) PushRepository(context.Context, usecasegit.PushRepositoryReq) (usecasegit.PushRepositoryResp, error) {
	return usecasegit.PushRepositoryResp{}, f.err
}
func (f *fakeGitCommandService) StageFiles(context.Context, usecasegit.StageFilesReq) error { return f.err }
func (f *fakeGitCommandService) UnstageFiles(context.Context, usecasegit.UnstageFilesReq) error { return f.err }
func (f *fakeGitCommandService) CommitAndPush(context.Context, usecasegit.CommitAndPushReq) (usecasegit.CommitAndPushResp, error) {
	return usecasegit.CommitAndPushResp{}, f.err
}
func (f *fakeGitCommandService) CreateBranch(context.Context, usecasegit.CreateBranchReq) (usecasegit.BranchResp, error) {
	return usecasegit.BranchResp{}, f.err
}
func (f *fakeGitCommandService) CheckoutBranch(context.Context, usecasegit.CheckoutBranchReq) (usecasegit.BranchResp, error) {
	return usecasegit.BranchResp{}, f.err
}
func (f *fakeGitCommandService) ListBranches(context.Context, usecasegit.ListBranchesReq) (usecasegit.ListBranchesResp, error) {
	return usecasegit.ListBranchesResp{}, f.err
}
func (f *fakeGitCommandService) GetStatus(context.Context, usecasegit.StatusReq) (usecasegit.StatusResp, error) {
	return usecasegit.StatusResp{}, f.err
}
func (f *fakeGitCommandService) GetCommitHistory(context.Context, usecasegit.CommitHistoryReq) (usecasegit.CommitHistoryResp, error) {
	return usecasegit.CommitHistoryResp{}, f.err
}
func (f *fakeGitCommandService) DiscardChanges(context.Context, usecasegit.DiscardChangesReq) error {
	return f.err
}
func (f *fakeGitCommandService) DeleteRepository(context.Context, usecasegit.DeleteRepositoryReq) error {
	return f.err
}

func TestGitHandlerCloneRepository(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	commandService := &fakeGitCommandService{}
	handler := NewGitHandler(commandService)
	router := gin.New()
	router.POST("/CloneRepo", handler.CloneRepository)

	ctx := testutil.RequestContext()
	body := req.CloneRepositoryReq{
		WorkspaceContext: req.WorkspaceContext{
			OwnerUIN: ctx.OwnerUIN, UIN: ctx.UIN,
			AppID: ctx.AppID, WorkspaceID: ctx.WorkspaceID,
		},
		RepoURL:    "https://example.com/repo.git",
		Branch:     "main",
		TargetPath: "repo",
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/CloneRepo", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if commandService.req.Context.OwnerUIN != ctx.OwnerUIN || commandService.req.Context.UIN != ctx.UIN {
		t.Fatalf("context = %+v, want %+v", commandService.req.Context, ctx)
	}
	if commandService.req.RepoURL != "https://example.com/repo.git" {
		t.Fatalf("repo url = %q", commandService.req.RepoURL)
	}
	if commandService.req.TargetPath != "repo" {
		t.Fatalf("target path = %q", commandService.req.TargetPath)
	}
}

func TestGitHandlerCloneRepositoryReturnsCommonErrorBody(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	handler := NewGitHandler(&fakeGitCommandService{
		err: usecasegit.ErrInvalidCloneReq,
	})
	router := gin.New()
	router.POST("/CloneRepo", handler.CloneRepository)

	ctx := testutil.RequestContext()
	body := req.CloneRepositoryReq{
		WorkspaceContext: req.WorkspaceContext{
			OwnerUIN: ctx.OwnerUIN, UIN: ctx.UIN,
			AppID: ctx.AppID, WorkspaceID: ctx.WorkspaceID,
		},
		RepoURL:    "https://example.com/repo.git",
		Branch:     "main",
		TargetPath: "repo",
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/CloneRepo", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var got struct {
		Common struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		} `json:"common"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if got.Common.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want %d", got.Common.Code, http.StatusBadRequest)
	}
	if !strings.Contains(got.Common.Msg, "invalid git clone request") {
		t.Fatalf("msg = %q", got.Common.Msg)
	}
}
