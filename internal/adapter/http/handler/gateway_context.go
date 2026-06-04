package handler

import (
	"strings"

	"git.woa.com/leondli/workspace-service/internal/adapter/http/req"
	"git.woa.com/leondli/workspace-service/internal/domain/identity"
	"git.woa.com/leondli/workspace-service/pkg/ctxmeta"
	"github.com/gin-gonic/gin"
)

// bindGatewayContext attaches tenant identity from Wedata headers when present.
// Jupyter session APIs do not require a full WorkspaceContext JSON body; headers
// are enough to populate kernel_session.owner_uin / app_id fields.
func bindGatewayContext(c *gin.Context) {
	rc := identity.RequestContext{
		OwnerUIN:    strings.TrimSpace(c.GetHeader(req.HeaderOwnerUIN)),
		UIN:         strings.TrimSpace(c.GetHeader(req.HeaderUIN)),
		AppID:       strings.TrimSpace(c.GetHeader(req.HeaderAppID)),
		WorkspaceID: strings.TrimSpace(c.GetHeader(req.HeaderWorkspaceID)),
	}.Normalize()
	if rc.OwnerUIN == "" && rc.UIN == "" && rc.AppID == "" && rc.WorkspaceID == "" {
		return
	}
	c.Request = c.Request.WithContext(ctxmeta.WithRequestContext(c.Request.Context(), rc))
}
