package fs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	domainfs "git.woa.com/leondli/workspace-service/internal/domain/fs"
	"git.woa.com/leondli/workspace-service/internal/domain/identity"
)

var ErrInvalidFileRequest = errors.New("invalid file request")

type FileService interface {
	CreateFolder(ctx context.Context, req CreateFolderReq) (FileInfoResp, error)
	CreateFile(ctx context.Context, req CreateFileReq) (FileInfoResp, error)
	CreateNotebook(ctx context.Context, req CreateNotebookReq) (FileInfoResp, error)
	DeletePath(ctx context.Context, req DeletePathReq) error
	MovePath(ctx context.Context, req MovePathReq) (FileInfoResp, error)
	CopyPath(ctx context.Context, req CopyPathReq) (FileInfoResp, error)
	RenamePath(ctx context.Context, req RenamePathReq) (FileInfoResp, error)
	ListFiles(ctx context.Context, req ListFilesReq) (ListFilesResp, error)
	ValidatePath(ctx context.Context, req ValidatePathReq) (ValidatePathResp, error)
	GetFolderNodePath(ctx context.Context, req GetFolderNodePathReq) (GetFolderNodePathResp, error)
	ListRecycleBin(ctx context.Context, req ListRecycleBinReq) (ListRecycleBinResp, error)
	RestorePath(ctx context.Context, req RestorePathReq) (FileInfoResp, error)
	EmptyRecycleBin(ctx context.Context, req EmptyRecycleBinReq) error
	GetFileInfo(ctx context.Context, req GetFileInfoReq) (FileInfoResp, error)
	ReadFile(ctx context.Context, req ReadFileReq) (ReadFileResp, error)
	WriteFile(ctx context.Context, req WriteFileReq) (FileInfoResp, error)
	DownloadFile(ctx context.Context, req ReadFileReq) ([]byte, string, error)
}

type Service struct {
	fsClient    domainfs.WorkspaceFS
	mountRoot   string
	gitBranches GitBranchLookup
	recorder    *nodeRecorder
}

func NewService(fsClient domainfs.WorkspaceFS, mountRoot string, gitBranches GitBranchLookup) *Service {
	return NewServiceWithStores(fsClient, mountRoot, gitBranches, nil, nil)
}

// NewServiceWithStores builds a Service that orchestrates the storage write and
// the ws_file_node metadata write itself, using a write-ahead intent for crash
// recovery. nodes and intents may be nil (MySQL disabled), in which case the
// service degrades to a plain storage write without metadata recording.
func NewServiceWithStores(
	fsClient domainfs.WorkspaceFS,
	mountRoot string,
	gitBranches GitBranchLookup,
	nodes domainfile.NodeStore,
	intents domainfile.IntentStore,
) *Service {
	return &Service{
		fsClient:    fsClient,
		mountRoot:   cleanMountRoot(mountRoot),
		gitBranches: gitBranches,
		recorder:    newNodeRecorder(nodes, intents),
	}
}

func (s *Service) resolveActorAndPath(ctx identity.RequestContext, path string) (domainfs.Actor, string, error) {
	ctx = ctx.Normalize()
	if err := ctx.Validate(); err != nil {
		return domainfs.Actor{}, "", fmt.Errorf("%w: %w", ErrInvalidFileRequest, err)
	}
	abs, err := s.resolveAbsPath(ctx, path)
	if err != nil {
		return domainfs.Actor{}, "", err
	}
	return ctx, abs, nil
}

func (s *Service) resolveAbsPath(ctx identity.RequestContext, relPath string) (string, error) {
	if s.mountRoot == "" {
		return "", fmt.Errorf("%w: workspace mount root is required", ErrInvalidFileRequest)
	}
	// Empty path means the caller's workspace user root (ListFiles/Home, etc.).
	if strings.TrimSpace(relPath) == "" {
		return filepath.Join(s.mountRoot, ctx.Normalize().UserPathPrefix()), nil
	}
	resolved, err := ctx.ResolveRelativePath(relPath)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrInvalidFileRequest, err)
	}
	abs := filepath.Join(s.mountRoot, resolved)
	mount := s.mountRoot + string(filepath.Separator)
	if !strings.HasPrefix(abs, mount) && abs != s.mountRoot {
		return "", fmt.Errorf("%w: path escapes workspace mount root", ErrInvalidFileRequest)
	}
	return abs, nil
}

type FileInfoResp struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	IsDir       bool   `json:"is_dir"`
	Size        int64  `json:"size"`
	ModifyTime  string `json:"modify_time"`
	InodeID     uint64 `json:"inode_id,omitempty"`
	OwnerUIN    string `json:"owner_uin,omitempty"`
	CreatorUIN  string `json:"creator_uin,omitempty"`
	NodeType    string `json:"node_type,omitempty"`
	IsGitFolder bool   `json:"is_git_folder"`
	GitBranch   string `json:"git_branch,omitempty"`
	InRecycle   bool   `json:"in_recycle,omitempty"`
	OriginPath  string `json:"origin_path,omitempty"`
	FileID      string `json:"file_id,omitempty"`
}

func mapFileInfo(info domainfs.FileInfo, mountRoot string) FileInfoResp {
	resp := mapFolderNode(info, mountRoot)
	return FileInfoResp{
		Name: resp.Name, Path: resp.Path, IsDir: resp.IsDir, Size: resp.Size,
		ModifyTime: resp.ModifyTime, InodeID: resp.InodeID, OwnerUIN: resp.OwnerUIN,
		CreatorUIN: resp.CreatorUIN, NodeType: resp.NodeType, IsGitFolder: resp.IsGitFolder,
		GitBranch: resp.GitBranch,
		InRecycle: info.InRecycle, OriginPath: toUserRelativePath(info.OriginPath),
		FileID: resp.FileID,
	}
}

// resolveFileID returns JuiceFS inode_id as a decimal string for API consumers.
// Full path remains in the "path" field. Future ide_code_file.code_file_id (UUID)
// may replace inode for business objects.
func resolveFileID(info domainfs.FileInfo) string {
	if info.InodeID == 0 {
		return ""
	}
	return strconv.FormatUint(info.InodeID, 10)
}

func mapFolderNode(info domainfs.FileInfo, mountRoot string) FolderNodeResp {
	rel := relPathFromMount(info.Path, mountRoot)
	fileID := resolveFileID(info)
	return FolderNodeResp{
		Name: info.Name, Path: rel, IsDir: info.IsDir, Size: info.Size,
		ModifyTime: info.ModifyTime.UTC().Format(time.RFC3339),
		InodeID: info.InodeID, OwnerUIN: info.OwnerUIN, CreatorUIN: info.CreatorUIN,
		NodeType: info.NodeType, IsGitFolder: info.IsGitFolder, FileID: fileID,
	}
}

func relPathFromMount(absPath, mountRoot string) string {
	rel := absPath
	if strings.HasPrefix(absPath, mountRoot) {
		if r, err := filepath.Rel(mountRoot, absPath); err == nil {
			rel = filepath.ToSlash(r)
		}
	}
	return rel
}

func toUserRelativePath(path string) string {
	return filepath.ToSlash(path)
}

func cleanMountRoot(mountRoot string) string {
	mountRoot = strings.TrimSpace(mountRoot)
	if mountRoot == "" {
		return ""
	}
	if mountRoot == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return mountRoot
	}
	if strings.HasPrefix(mountRoot, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(mountRoot, "~/"))
		}
	}
	return filepath.Clean(mountRoot)
}
