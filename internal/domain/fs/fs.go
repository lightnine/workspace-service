// Package fs defines workspace file operations against the JuiceFS mount,
// aligned with Databricks Files API capabilities and wedata3 fs-proxy operations.
package fs

import (
	"context"
	"errors"
	"time"

	"git.woa.com/leondli/workspace-service/internal/domain/identity"
)

var (
	ErrNotFound       = errors.New("path not found")
	ErrAlreadyExists  = errors.New("path already exists")
	ErrNotAFile       = errors.New("not a file")
	ErrNotADirectory  = errors.New("not a directory")
	ErrFileTooLarge   = errors.New("file too large for inline read")
	ErrInvalidRequest = errors.New("invalid file request")
)

// Actor is the file operation principal (aligned with wedata3 RequestBaseInfo).
type Actor = identity.RequestContext

// FileInfo describes a file or directory on the workspace mount.
type FileInfo struct {
	Name       string
	Path       string // workspace-relative path (absolute on mount in infra layer)
	IsDir      bool
	Size       int64
	ModifyTime time.Time
	InodeID    uint64
	OwnerUIN   string
	CreatorUIN  string
	NodeType    string // file | directory | git_folder | notebook (from ws_file_node when known)
	IsGitFolder bool
	InRecycle  bool
	OriginPath string // set for recycle-bin entries
}

type CreateFolderReq struct {
	Actor Actor
	Path  string // workspace-relative
}

type CreateFileReq struct {
	Actor     Actor
	Path      string
	Content   []byte // empty => touch empty file
	Overwrite bool
}

type CreateNotebookReq struct {
	Actor      Actor
	Path       string // must end with .ipynb
	KernelName string // optional; default python3
	Overwrite  bool
}

type DeletePathReq struct {
	Actor      Actor
	Path       string
	SoftDelete bool // move to recycle bin instead of permanent delete
	Permanent  bool // when true, skip recycle and remove from disk
}

type MovePathReq struct {
	Actor     Actor
	SrcPath   string
	DestPath  string
	Overwrite bool
}

type CopyPathReq struct {
	Actor     Actor
	SrcPath   string
	DestPath  string
	Overwrite bool
}

type RenamePathReq struct {
	Actor    Actor
	Path     string
	NewName  string // basename only
	Overwrite bool
}

type ListFilesReq struct {
	Actor     Actor
	Path      string
	Recursive bool
}

type ListFilesResult struct {
	Files []FileInfo
}

type ValidatePathReq struct {
	Actor      Actor
	ParentPath string // directory path; empty means user root
	Name       string // basename to check
}

type ValidatePathResult struct {
	Exists bool
}

type GetFolderNodePathReq struct {
	Actor Actor
	Path  string // target folder path
}

type GetFolderNodePathResult struct {
	Nodes []FileInfo
}

type ListRecycleBinReq struct {
	Actor Actor
}

type ListRecycleBinResult struct {
	Files []FileInfo
}

type RestorePathReq struct {
	Actor        Actor
	TrashPath    string // path under trash/
	TargetParent string // optional override parent; empty uses OriginPath parent
}

type EmptyRecycleBinReq struct {
	Actor Actor
}

type ReadFileReq struct {
	Actor Actor
	Path  string
}

type ReadFileResult struct {
	Info    FileInfo
	Content []byte
}

type WriteFileReq struct {
	Actor     Actor
	Path      string
	Content   []byte
	Overwrite bool
}

// WorkspaceFS performs POSIX operations on the mounted workspace tree.
type WorkspaceFS interface {
	CreateFolder(ctx context.Context, req CreateFolderReq) (FileInfo, error)
	CreateFile(ctx context.Context, req CreateFileReq) (FileInfo, error)
	CreateNotebook(ctx context.Context, req CreateNotebookReq) (FileInfo, error)
	DeletePath(ctx context.Context, req DeletePathReq) error
	MovePath(ctx context.Context, req MovePathReq) (FileInfo, error)
	CopyPath(ctx context.Context, req CopyPathReq) (FileInfo, error)
	RenamePath(ctx context.Context, req RenamePathReq) (FileInfo, error)
	ListFiles(ctx context.Context, req ListFilesReq) (ListFilesResult, error)
	ValidatePath(ctx context.Context, req ValidatePathReq) (ValidatePathResult, error)
	GetFolderNodePath(ctx context.Context, req GetFolderNodePathReq) (GetFolderNodePathResult, error)
	ListRecycleBin(ctx context.Context, req ListRecycleBinReq) (ListRecycleBinResult, error)
	RestorePath(ctx context.Context, req RestorePathReq) (FileInfo, error)
	EmptyRecycleBin(ctx context.Context, req EmptyRecycleBinReq) error
	GetFileInfo(ctx context.Context, path string) (FileInfo, error)
	ReadFile(ctx context.Context, req ReadFileReq) (ReadFileResult, error)
	WriteFile(ctx context.Context, req WriteFileReq) (FileInfo, error)
}
