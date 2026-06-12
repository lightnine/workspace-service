package fs

import (
	"context"
	"fmt"
	"path/filepath"

	domainfs "git.woa.com/leondli/workspace-service/internal/domain/fs"
	"git.woa.com/leondli/workspace-service/internal/domain/identity"
)

type ValidatePathReq struct {
	Context    identity.RequestContext
	ParentPath string
	Name       string
}

type ValidatePathResp struct {
	Exists bool `json:"exists"`
}

type GetFolderNodePathReq struct {
	Context identity.RequestContext
	Path    string
}

type FolderNodeResp struct {
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
	FileID      string `json:"file_id,omitempty"`
}

type GetFolderNodePathResp struct {
	Nodes []FolderNodeResp `json:"nodes"`
}

func (s *Service) ValidatePath(ctx context.Context, input ValidatePathReq) (ValidatePathResp, error) {
	ident := input.Context.Normalize()
	if err := ident.Validate(); err != nil {
		return ValidatePathResp{}, fmt.Errorf("%w: %w", ErrInvalidFileRequest, err)
	}
	parentAbs := s.userRootAbs(ident)
	if p := input.ParentPath; p != "" {
		var err error
		parentAbs, err = s.resolveAbsPath(ident, p)
		if err != nil {
			return ValidatePathResp{}, err
		}
	}
	result, err := s.fsClient.ValidatePath(ctx, domainfs.ValidatePathReq{
		Actor: ident, ParentPath: parentAbs, Name: input.Name,
	})
	if err != nil {
		return ValidatePathResp{}, fmt.Errorf("validate path: %w", err)
	}
	return ValidatePathResp{Exists: result.Exists}, nil
}

func (s *Service) GetFolderNodePath(ctx context.Context, input GetFolderNodePathReq) (GetFolderNodePathResp, error) {
	_, abs, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return GetFolderNodePathResp{}, err
	}
	result, err := s.fsClient.GetFolderNodePath(ctx, domainfs.GetFolderNodePathReq{
		Actor: input.Context.Normalize(), Path: abs,
	})
	if err != nil {
		return GetFolderNodePathResp{}, fmt.Errorf("get folder node path: %w", err)
	}
	resp := GetFolderNodePathResp{}
	nodes := make([]FileInfoResp, 0, len(result.Nodes))
	for _, node := range result.Nodes {
		nodes = append(nodes, mapFileInfo(node, s.mountRoot))
	}
	nodes = s.enrichGitBranches(ctx, input.Context.Normalize(), nodes)
	for _, n := range nodes {
		resp.Nodes = append(resp.Nodes, FolderNodeResp{
			Name: n.Name, Path: n.Path, IsDir: n.IsDir, Size: n.Size,
			ModifyTime: n.ModifyTime, InodeID: n.InodeID, OwnerUIN: n.OwnerUIN,
			CreatorUIN: n.CreatorUIN, NodeType: n.NodeType, IsGitFolder: n.IsGitFolder,
			GitBranch: n.GitBranch, FileID: n.FileID,
		})
	}
	return resp, nil
}

func (s *Service) userRootAbs(ctx identity.RequestContext) string {
	return filepath.Join(s.mountRoot, ctx.UserPathPrefix())
}