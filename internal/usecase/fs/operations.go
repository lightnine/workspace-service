package fs

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	domainfs "git.woa.com/leondli/workspace-service/internal/domain/fs"
	"git.woa.com/leondli/workspace-service/internal/domain/identity"
)

type CreateFolderReq struct {
	Context identity.RequestContext
	Path    string
}

type CreateFileReq struct {
	Context    identity.RequestContext
	Path       string
	Content    []byte
	ContentB64 string
	Overwrite  bool
}

type CreateNotebookReq struct {
	Context    identity.RequestContext
	Path       string
	KernelName string
	Overwrite  bool
}

type DeletePathReq struct {
	Context    identity.RequestContext
	Path       string
	SoftDelete bool
	Permanent  bool
}

type MovePathReq struct {
	Context   identity.RequestContext
	SrcPath   string
	DestPath  string
	Overwrite bool
}

type CopyPathReq struct {
	Context   identity.RequestContext
	SrcPath   string
	DestPath  string
	Overwrite bool
}

type RenamePathReq struct {
	Context   identity.RequestContext
	Path      string
	NewName   string
	Overwrite bool
}

type ListFilesReq struct {
	Context   identity.RequestContext
	Path      string
	Recursive bool
}

type ListFilesResp struct {
	Files []FileInfoResp `json:"files"`
}

type GetFileInfoReq struct {
	Context identity.RequestContext
	Path    string
}

type ReadFileReq struct {
	Context identity.RequestContext
	Path    string
}

type ReadFileResp struct {
	File FileInfoResp `json:"file"`
	B64  string       `json:"content_base64"`
	Size int          `json:"size"`
}

type WriteFileReq struct {
	Context    identity.RequestContext
	Path       string
	Content    []byte
	ContentB64 string
	Overwrite  bool
}

func decodeContent(raw []byte, b64 string) ([]byte, error) {
	if len(raw) > 0 {
		return raw, nil
	}
	b64 = strings.TrimSpace(b64)
	if b64 == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(b64)
}

func (s *Service) CreateFolder(ctx context.Context, input CreateFolderReq) (FileInfoResp, error) {
	actor, abs, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return FileInfoResp{}, err
	}
	info, err := s.recorder.run(ctx, domainfile.IntentOpCreateFolder, abs, actor, domainfile.NodeTypeDirectory,
		func() (domainfs.FileInfo, error) {
			return s.fsClient.CreateFolder(ctx, domainfs.CreateFolderReq{Actor: actor, Path: abs})
		})
	if err != nil {
		return FileInfoResp{}, fmt.Errorf("create folder: %w", err)
	}
	return mapFileInfo(info, s.mountRoot), nil
}

func (s *Service) CreateFile(ctx context.Context, input CreateFileReq) (FileInfoResp, error) {
	actor, abs, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return FileInfoResp{}, err
	}
	content, err := decodeContent(input.Content, input.ContentB64)
	if err != nil {
		return FileInfoResp{}, fmt.Errorf("%w: invalid content_base64", ErrInvalidFileRequest)
	}
	info, err := s.recorder.run(ctx, domainfile.IntentOpCreateFile, abs, actor, domainfile.NodeTypeFile,
		func() (domainfs.FileInfo, error) {
			return s.fsClient.CreateFile(ctx, domainfs.CreateFileReq{
				Actor: actor, Path: abs, Content: content, Overwrite: input.Overwrite,
			})
		})
	if err != nil {
		return FileInfoResp{}, fmt.Errorf("create file: %w", err)
	}
	return mapFileInfo(info, s.mountRoot), nil
}

func (s *Service) CreateNotebook(ctx context.Context, input CreateNotebookReq) (FileInfoResp, error) {
	path, err := normalizeNotebookPath(input.Path)
	if err != nil {
		return FileInfoResp{}, err
	}
	actor, abs, err := s.resolveActorAndPath(input.Context, path)
	if err != nil {
		return FileInfoResp{}, err
	}
	info, err := s.recorder.run(ctx, domainfile.IntentOpCreateNotebook, abs, actor, domainfile.NodeTypeNotebook,
		func() (domainfs.FileInfo, error) {
			return s.fsClient.CreateNotebook(ctx, domainfs.CreateNotebookReq{
				Actor: actor, Path: abs, KernelName: input.KernelName, Overwrite: input.Overwrite,
			})
		})
	if err != nil {
		return FileInfoResp{}, fmt.Errorf("create notebook: %w", err)
	}
	return mapFileInfo(info, s.mountRoot), nil
}

func normalizeNotebookPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("%w: path is required", ErrInvalidFileRequest)
	}
	base := filepath.Base(path)
	if base == "." || base == ".." || strings.ContainsAny(base, `/\`) {
		return "", fmt.Errorf("%w: invalid notebook name", ErrInvalidFileRequest)
	}
	if !strings.HasSuffix(strings.ToLower(base), ".ipynb") {
		path = path + ".ipynb"
	}
	return path, nil
}

func (s *Service) DeletePath(ctx context.Context, input DeletePathReq) error {
	actor, abs, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return err
	}
	if err := s.fsClient.DeletePath(ctx, domainfs.DeletePathReq{
		Actor: actor, Path: abs,
		SoftDelete: input.SoftDelete,
		Permanent:  input.Permanent,
	}); err != nil {
		return fmt.Errorf("delete path: %w", err)
	}
	return nil
}

func (s *Service) MovePath(ctx context.Context, input MovePathReq) (FileInfoResp, error) {
	ident := input.Context.Normalize()
	if err := ident.Validate(); err != nil {
		return FileInfoResp{}, fmt.Errorf("%w: %w", ErrInvalidFileRequest, err)
	}
	src, err := s.resolveAbsPath(ident, input.SrcPath)
	if err != nil {
		return FileInfoResp{}, err
	}
	dest, err := s.resolveAbsPath(ident, input.DestPath)
	if err != nil {
		return FileInfoResp{}, err
	}
	info, err := s.fsClient.MovePath(ctx, domainfs.MovePathReq{
		Actor: ident, SrcPath: src, DestPath: dest, Overwrite: input.Overwrite,
	})
	if err != nil {
		return FileInfoResp{}, fmt.Errorf("move path: %w", err)
	}
	return mapFileInfo(info, s.mountRoot), nil
}

func (s *Service) CopyPath(ctx context.Context, input CopyPathReq) (FileInfoResp, error) {
	ident := input.Context.Normalize()
	if err := ident.Validate(); err != nil {
		return FileInfoResp{}, fmt.Errorf("%w: %w", ErrInvalidFileRequest, err)
	}
	src, err := s.resolveAbsPath(ident, input.SrcPath)
	if err != nil {
		return FileInfoResp{}, err
	}
	dest, err := s.resolveAbsPath(ident, input.DestPath)
	if err != nil {
		return FileInfoResp{}, err
	}
	info, err := s.fsClient.CopyPath(ctx, domainfs.CopyPathReq{
		Actor: ident, SrcPath: src, DestPath: dest, Overwrite: input.Overwrite,
	})
	if err != nil {
		return FileInfoResp{}, fmt.Errorf("copy path: %w", err)
	}
	return mapFileInfo(info, s.mountRoot), nil
}

func (s *Service) RenamePath(ctx context.Context, input RenamePathReq) (FileInfoResp, error) {
	actor, abs, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return FileInfoResp{}, err
	}
	name := strings.TrimSpace(input.NewName)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return FileInfoResp{}, fmt.Errorf("%w: new_name is invalid", ErrInvalidFileRequest)
	}
	info, err := s.fsClient.RenamePath(ctx, domainfs.RenamePathReq{
		Actor: actor, Path: abs, NewName: name, Overwrite: input.Overwrite,
	})
	if err != nil {
		return FileInfoResp{}, fmt.Errorf("rename path: %w", err)
	}
	return mapFileInfo(info, s.mountRoot), nil
}

func (s *Service) ListFiles(ctx context.Context, input ListFilesReq) (ListFilesResp, error) {
	actor, abs, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return ListFilesResp{}, err
	}
	result, err := s.fsClient.ListFiles(ctx, domainfs.ListFilesReq{
		Actor: actor, Path: abs, Recursive: input.Recursive,
	})
	if err != nil {
		return ListFilesResp{}, fmt.Errorf("list files: %w", err)
	}
	resp := ListFilesResp{}
	for _, f := range result.Files {
		resp.Files = append(resp.Files, mapFileInfo(f, s.mountRoot))
	}
	resp.Files = s.enrichGitBranches(ctx, input.Context.Normalize(), resp.Files)
	return resp, nil
}

func (s *Service) GetFileInfo(ctx context.Context, input GetFileInfoReq) (FileInfoResp, error) {
	_, abs, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return FileInfoResp{}, err
	}
	info, err := s.fsClient.GetFileInfo(ctx, abs)
	if err != nil {
		return FileInfoResp{}, fmt.Errorf("get file info: %w", err)
	}
	return mapFileInfo(info, s.mountRoot), nil
}

func (s *Service) ReadFile(ctx context.Context, input ReadFileReq) (ReadFileResp, error) {
	actor, abs, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return ReadFileResp{}, err
	}
	result, err := s.fsClient.ReadFile(ctx, domainfs.ReadFileReq{Actor: actor, Path: abs})
	if err != nil {
		return ReadFileResp{}, fmt.Errorf("read file: %w", err)
	}
	return ReadFileResp{
		File: mapFileInfo(result.Info, s.mountRoot),
		B64:  base64.StdEncoding.EncodeToString(result.Content),
		Size: len(result.Content),
	}, nil
}

func (s *Service) WriteFile(ctx context.Context, input WriteFileReq) (FileInfoResp, error) {
	actor, abs, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return FileInfoResp{}, err
	}
	content, err := decodeContent(input.Content, input.ContentB64)
	if err != nil {
		return FileInfoResp{}, fmt.Errorf("%w: invalid content_base64", ErrInvalidFileRequest)
	}
	info, err := s.recorder.run(ctx, domainfile.IntentOpWriteFile, abs, actor, domainfile.NodeTypeFile,
		func() (domainfs.FileInfo, error) {
			return s.fsClient.WriteFile(ctx, domainfs.WriteFileReq{
				Actor: actor, Path: abs, Content: content, Overwrite: input.Overwrite,
			})
		})
	if err != nil {
		return FileInfoResp{}, fmt.Errorf("write file: %w", err)
	}
	return mapFileInfo(info, s.mountRoot), nil
}

func (s *Service) DownloadFile(ctx context.Context, input ReadFileReq) ([]byte, string, error) {
	actor, abs, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return nil, "", err
	}
	result, err := s.fsClient.ReadFile(ctx, domainfs.ReadFileReq{Actor: actor, Path: abs})
	if err != nil {
		return nil, "", fmt.Errorf("download file: %w", err)
	}
	return result.Content, result.Info.Name, nil
}
