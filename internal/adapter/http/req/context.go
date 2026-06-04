package req

import (
	"errors"
	"strings"

	"git.woa.com/leondli/workspace-service/internal/domain/identity"
	"github.com/gin-gonic/gin"
)

// WorkspaceContext is the common JSON block for workspace-service APIs.
// Frontend should send it on every Verb+Noun request (with global user + current workspace).
type WorkspaceContext struct {
	OwnerUIN    string `json:"owner_uin"`
	UIN         string `json:"uin"`
	AppID       string `json:"app_id"`
	WorkspaceID string `json:"workspace_id"`
}

// ToIdentity converts the JSON DTO to a domain RequestContext.
func (w WorkspaceContext) ToIdentity() identity.RequestContext {
	return identity.RequestContext{
		OwnerUIN:    strings.TrimSpace(w.OwnerUIN),
		UIN:         strings.TrimSpace(w.UIN),
		AppID:       strings.TrimSpace(w.AppID),
		WorkspaceID: strings.TrimSpace(w.WorkspaceID),
	}.Normalize()
}

// Header names used as fallback when JSON fields are empty (gateway / BFF may inject).
const (
	HeaderOwnerUIN    = "X-Wedata-Owner-Uin"
	HeaderUIN         = "X-Wedata-Uin"
	HeaderAppID       = "X-Wedata-App-Id"
	HeaderWorkspaceID = "X-Wedata-Workspace-Id"
)

// ResolveWorkspaceContext builds identity from JSON body, then fills empty fields from headers.
func ResolveWorkspaceContext(c *gin.Context, ws WorkspaceContext) (identity.RequestContext, error) {
	rc := ws.ToIdentity()
	if rc.OwnerUIN == "" {
		rc.OwnerUIN = strings.TrimSpace(c.GetHeader(HeaderOwnerUIN))
	}
	if rc.UIN == "" {
		rc.UIN = strings.TrimSpace(c.GetHeader(HeaderUIN))
	}
	if rc.AppID == "" {
		rc.AppID = strings.TrimSpace(c.GetHeader(HeaderAppID))
	}
	if rc.WorkspaceID == "" {
		rc.WorkspaceID = strings.TrimSpace(c.GetHeader(HeaderWorkspaceID))
	}
	rc = rc.Normalize()
	if err := rc.Validate(); err != nil {
		return identity.RequestContext{}, err
	}
	return rc, nil
}

// IsInvalidContext reports whether err is a missing/invalid identity error.
func IsInvalidContext(err error) bool {
	return errors.Is(err, identity.ErrInvalidContext)
}
