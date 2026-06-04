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
	var req req.CloneRepositoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresponse.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}

	output, err := h.commandService.CloneRepository(c.Request.Context(), usecasegit.CloneRepositoryReq{
		OwnerUIN:   req.OwnerUIN,
		UIN:        req.UIN,
		RepoURL:    req.RepoURL,
		TargetPath: req.TargetPath,
		Branch:     req.Branch,
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
	})
}
