package identity

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// ErrInvalidContext is returned when required identity fields are missing.
var ErrInvalidContext = errors.New("invalid request context")

// RequestContext carries the WeData caller and workspace scope for a single request.
// Field names align with wedata3-monorepo:
//   - application/ai-assistant/common/auth.Identity (OwnerUin, UserUin, AppID, ProjectID)
//   - application/wedata-application/.../RequestBaseInfo (ownerUin, uin, appId, workspaceId)
//
// ProjectID in AI assistant is often the same as WorkspaceID; this service uses WorkspaceID.
type RequestContext struct {
	OwnerUIN    string // 主账号 UIN
	UIN         string // 子账号 / 操作人 UIN
	AppID       string // 租户 ID（tenantId）
	WorkspaceID string // 工作空间 ID
}

// Validate checks required identity fields.
func (r RequestContext) Validate() error {
	if strings.TrimSpace(r.OwnerUIN) == "" {
		return fmt.Errorf("%w: owner_uin is required", ErrInvalidContext)
	}
	if strings.TrimSpace(r.UIN) == "" {
		return fmt.Errorf("%w: uin is required", ErrInvalidContext)
	}
	if strings.TrimSpace(r.AppID) == "" {
		return fmt.Errorf("%w: app_id is required", ErrInvalidContext)
	}
	if strings.TrimSpace(r.WorkspaceID) == "" {
		return fmt.Errorf("%w: workspace_id is required", ErrInvalidContext)
	}
	return nil
}

// Normalize trims string fields.
func (r RequestContext) Normalize() RequestContext {
	r.OwnerUIN = strings.TrimSpace(r.OwnerUIN)
	r.UIN = strings.TrimSpace(r.UIN)
	r.AppID = strings.TrimSpace(r.AppID)
	r.WorkspaceID = strings.TrimSpace(r.WorkspaceID)
	if r.UIN == "" {
		r.UIN = r.OwnerUIN
	}
	if r.OwnerUIN == "" {
		r.OwnerUIN = r.UIN
	}
	return r
}

// UserPathPrefix returns the canonical mount-relative prefix for this user:
// {appId}/{workspaceId}/users/{uin}/
func (r RequestContext) UserPathPrefix() string {
	return filepath.Join(r.AppID, r.WorkspaceID, "users", r.UIN)
}

// ResolveRelativePath joins userPath under UserPathPrefix when userPath is not already
// under that prefix. API callers may pass either "demo/file.txt" or the full
// "{appId}/{workspaceId}/users/{uin}/demo/file.txt".
func (r RequestContext) ResolveRelativePath(userPath string) (string, error) {
	userPath = strings.TrimSpace(userPath)
	if userPath == "" {
		return "", fmt.Errorf("%w: path is required", ErrInvalidContext)
	}
	if filepath.IsAbs(userPath) {
		return "", fmt.Errorf("%w: path must be relative to workspace mount root", ErrInvalidContext)
	}
	cleanPath := filepath.Clean(userPath)
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: path escapes workspace mount root", ErrInvalidContext)
	}

	prefix := r.UserPathPrefix()
	if cleanPath == prefix || strings.HasPrefix(cleanPath, prefix+string(filepath.Separator)) {
		return cleanPath, nil
	}
	return filepath.Join(prefix, cleanPath), nil
}
