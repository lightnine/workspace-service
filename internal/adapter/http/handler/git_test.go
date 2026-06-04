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

func TestGitHandlerCloneRepository(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	commandService := &fakeGitCommandService{}
	handler := NewGitHandler(commandService)
	router := gin.New()
	router.POST("/CloneRepo", handler.CloneRepository)

	body := req.CloneRepositoryReq{
		OwnerUIN:   "100001",
		UIN:        "200001",
		RepoURL:    "https://example.com/repo.git",
		TargetPath: "users/200001/repo",
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
	if commandService.req.OwnerUIN != "100001" || commandService.req.UIN != "200001" {
		t.Fatalf("actor = %q/%q, want 100001/200001", commandService.req.OwnerUIN, commandService.req.UIN)
	}
	if commandService.req.RepoURL != "https://example.com/repo.git" {
		t.Fatalf("repo url = %q", commandService.req.RepoURL)
	}
	if commandService.req.TargetPath != "users/200001/repo" {
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

	body := req.CloneRepositoryReq{
		OwnerUIN:   "100001",
		UIN:        "200001",
		RepoURL:    "https://example.com/repo.git",
		TargetPath: "users/200001/repo",
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
