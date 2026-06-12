package handler

import (
	"errors"
	"net/http"

	"git.woa.com/leondli/workspace-service/internal/adapter/http/req"
	httpresponse "git.woa.com/leondli/workspace-service/internal/adapter/http/response"
	usecasegit "git.woa.com/leondli/workspace-service/internal/usecase/git"
	"github.com/gin-gonic/gin"
)

type GitHandler struct {
	commandService usecasegit.CommandService
}

func NewGitHandler(commandService usecasegit.CommandService) *GitHandler {
	return &GitHandler{commandService: commandService}
}

func (h *GitHandler) CloneRepository(c *gin.Context) {
	var body req.CloneRepositoryReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}

	output, err := h.commandService.CloneRepository(c.Request.Context(), usecasegit.CloneRepositoryReq{
		Context:    rc,
		RepoURL:    body.RepoURL,
		TargetPath: body.TargetPath,
		Branch:     body.Branch,
	})
	if err != nil {
		if errors.Is(err, usecasegit.ErrInvalidCloneReq) {
			httpresponse.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		httpresponse.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	httpresponse.OK(c, gin.H{
		"repo_url": output.RepoURL,
		"path":     output.Path,
		"branch":   output.Branch,
	})
}

func (h *GitHandler) CreateGitFolder(c *gin.Context) {
	var body req.CreateGitFolderReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	output, err := h.commandService.CreateGitFolder(c.Request.Context(), usecasegit.CreateGitFolderReq{
		Context: rc, TargetPath: body.TargetPath, RepoURL: body.RepoURL, Branch: body.Branch,
		GitUsername: body.GitUsername, GitToken: body.GitToken,
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, output)
}

func (h *GitHandler) GetGitFolderStatus(c *gin.Context) {
	var body req.GetGitFolderStatusReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	output, err := h.commandService.GetGitFolderStatus(c.Request.Context(), usecasegit.GetGitFolderStatusReq{
		Context: rc, Path: body.Path,
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, output)
}

func (h *GitHandler) StageFiles(c *gin.Context) {
	var body req.StageFilesReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	err := h.commandService.StageFiles(c.Request.Context(), usecasegit.StageFilesReq{
		Context: rc, Path: body.Path, Files: body.Files, All: body.All,
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, gin.H{"staged": true})
}

func (h *GitHandler) UnstageFiles(c *gin.Context) {
	var body req.UnstageFilesReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	err := h.commandService.UnstageFiles(c.Request.Context(), usecasegit.UnstageFilesReq{
		Context: rc, Path: body.Path, Files: body.Files, All: body.All,
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, gin.H{"unstaged": true})
}

func (h *GitHandler) Commit(c *gin.Context) {
	var body req.CommitReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	output, err := h.commandService.Commit(c.Request.Context(), usecasegit.CommitReq{
		Context: rc, Path: body.Path, Message: body.Message,
		AuthorName: body.AuthorName, AuthorEmail: body.AuthorEmail,
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, output)
}

func (h *GitHandler) PushRepo(c *gin.Context) {
	var body req.PushRepoReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	output, err := h.commandService.PushRepository(c.Request.Context(), usecasegit.PushRepositoryReq{
		Context: rc, Path: body.Path, RemoteName: body.RemoteName, Branch: body.Branch,
		GitUsername: body.GitUsername, GitToken: body.GitToken,
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, output)
}

func (h *GitHandler) PullRepo(c *gin.Context) {
	var body req.PullRepoReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}

	output, err := h.commandService.PullRepository(c.Request.Context(), usecasegit.PullRepositoryReq{
		Context:     rc,
		Path:        body.Path,
		RemoteName:  body.RemoteName,
		Branch:      body.Branch,
		Credentials: usecasegit.Credentials{Username: body.GitUsername, Token: body.GitToken},
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, output)
}

func (h *GitHandler) CommitAndPush(c *gin.Context) {
	var body req.CommitAndPushReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}

	push := true
	if body.Push != nil {
		push = *body.Push
	}

	output, err := h.commandService.CommitAndPush(c.Request.Context(), usecasegit.CommitAndPushReq{
		Context:     rc,
		Path:        body.Path,
		Message:     body.Message,
		AuthorName:  body.AuthorName,
		AuthorEmail: body.AuthorEmail,
		RemoteName:  body.RemoteName,
		Push:        push,
		Credentials: usecasegit.Credentials{Username: body.GitUsername, Token: body.GitToken},
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, output)
}

func (h *GitHandler) CreateBranch(c *gin.Context) {
	var body req.CreateBranchReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}

	output, err := h.commandService.CreateBranch(c.Request.Context(), usecasegit.CreateBranchReq{
		Context:  rc,
		Path:     body.Path,
		Branch:   body.Branch,
		Checkout: body.Checkout,
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, output)
}

func (h *GitHandler) CheckoutBranch(c *gin.Context) {
	var body req.CheckoutBranchReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}

	output, err := h.commandService.CheckoutBranch(c.Request.Context(), usecasegit.CheckoutBranchReq{
		Context: rc,
		Path:    body.Path,
		Branch:  body.Branch,
		Create:  body.Create,
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, output)
}

func (h *GitHandler) ListBranches(c *gin.Context) {
	var body req.ListBranchesReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}

	output, err := h.commandService.ListBranches(c.Request.Context(), usecasegit.ListBranchesReq{
		Context: rc,
		Path:    body.Path,
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, output)
}

func (h *GitHandler) GetStatus(c *gin.Context) {
	var body req.GetStatusReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}

	output, err := h.commandService.GetStatus(c.Request.Context(), usecasegit.StatusReq{
		Context: rc,
		Path:    body.Path,
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, output)
}

func (h *GitHandler) GetFileDiff(c *gin.Context) {
	var body req.GetFileDiffReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	output, err := h.commandService.GetFileDiff(c.Request.Context(), usecasegit.FileDiffReq{
		Context: rc,
		Path:    body.Path,
		File:    body.File,
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, output)
}

func (h *GitHandler) GetCommitHistory(c *gin.Context) {
	var body req.GetCommitHistoryReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}

	output, err := h.commandService.GetCommitHistory(c.Request.Context(), usecasegit.CommitHistoryReq{
		Context: rc,
		Path:    body.Path,
		Limit:   body.Limit,
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, output)
}

func (h *GitHandler) DiscardChanges(c *gin.Context) {
	var body req.DiscardChangesReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}

	err := h.commandService.DiscardChanges(c.Request.Context(), usecasegit.DiscardChangesReq{
		Context: rc,
		Path:    body.Path,
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, gin.H{"discarded": true})
}

func (h *GitHandler) DeleteRepo(c *gin.Context) {
	var body req.DeleteRepoReq
	if !bindGitJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}

	err := h.commandService.DeleteRepository(c.Request.Context(), usecasegit.DeleteRepositoryReq{
		Context: rc,
		Path:    body.Path,
	})
	if err != nil {
		writeGitError(c, err)
		return
	}
	httpresponse.OK(c, gin.H{"deleted": true})
}

func bindGitJSON(c *gin.Context, out any) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		httpresponse.Error(c, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func writeGitError(c *gin.Context, err error) {
	if errors.Is(err, usecasegit.ErrInvalidCloneReq) || errors.Is(err, usecasegit.ErrInvalidGitRequest) {
		httpresponse.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.IsInvalidContext(err) {
		httpresponse.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	httpresponse.Error(c, http.StatusInternalServerError, err.Error())
}
