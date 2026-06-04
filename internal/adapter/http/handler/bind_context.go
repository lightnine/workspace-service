package handler

import (
	"net/http"

	"git.woa.com/leondli/workspace-service/internal/adapter/http/req"
	httpresponse "git.woa.com/leondli/workspace-service/internal/adapter/http/response"
	"git.woa.com/leondli/workspace-service/internal/domain/identity"
	"git.woa.com/leondli/workspace-service/pkg/ctxmeta"
	"github.com/gin-gonic/gin"
)

func bindWorkspaceContext(c *gin.Context, ws *req.WorkspaceContext) (identity.RequestContext, bool) {
	rc, err := req.ResolveWorkspaceContext(c, *ws)
	if err != nil {
		if req.IsInvalidContext(err) {
			httpresponse.Error(c, http.StatusBadRequest, err.Error())
			return identity.RequestContext{}, false
		}
		httpresponse.Error(c, http.StatusBadRequest, "invalid request context")
		return identity.RequestContext{}, false
	}
	c.Request = c.Request.WithContext(ctxmeta.WithRequestContext(c.Request.Context(), rc))
	return rc, true
}
