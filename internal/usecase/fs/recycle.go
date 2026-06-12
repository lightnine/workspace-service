package fs

import (
	"context"
	"fmt"

	domainfs "git.woa.com/leondli/workspace-service/internal/domain/fs"
	"git.woa.com/leondli/workspace-service/internal/domain/identity"
)

type ListRecycleBinReq struct {
	Context identity.RequestContext
}

type ListRecycleBinResp struct {
	Files []FileInfoResp `json:"files"`
}

type RestorePathReq struct {
	Context      identity.RequestContext
	TrashPath    string
	TargetParent string
}

type EmptyRecycleBinReq struct {
	Context identity.RequestContext
}

func (s *Service) ListRecycleBin(ctx context.Context, input ListRecycleBinReq) (ListRecycleBinResp, error) {
	ident := input.Context.Normalize()
	if err := ident.Validate(); err != nil {
		return ListRecycleBinResp{}, fmt.Errorf("%w: %w", ErrInvalidFileRequest, err)
	}
	result, err := s.fsClient.ListRecycleBin(ctx, domainfs.ListRecycleBinReq{Actor: ident})
	if err != nil {
		return ListRecycleBinResp{}, fmt.Errorf("list recycle bin: %w", err)
	}
	resp := ListRecycleBinResp{}
	for _, f := range result.Files {
		if f.Name == "_meta" {
			continue
		}
		resp.Files = append(resp.Files, mapFileInfo(f, s.mountRoot))
	}
	return resp, nil
}

func (s *Service) RestorePath(ctx context.Context, input RestorePathReq) (FileInfoResp, error) {
	actor, trashAbs, err := s.resolveActorAndPath(input.Context, input.TrashPath)
	if err != nil {
		return FileInfoResp{}, err
	}
	info, err := s.fsClient.RestorePath(ctx, domainfs.RestorePathReq{
		Actor: actor, TrashPath: trashAbs, TargetParent: input.TargetParent,
	})
	if err != nil {
		return FileInfoResp{}, fmt.Errorf("restore path: %w", err)
	}
	return mapFileInfo(info, s.mountRoot), nil
}

func (s *Service) EmptyRecycleBin(ctx context.Context, input EmptyRecycleBinReq) error {
	ident := input.Context.Normalize()
	if err := ident.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidFileRequest, err)
	}
	if err := s.fsClient.EmptyRecycleBin(ctx, domainfs.EmptyRecycleBinReq{Actor: ident}); err != nil {
		return fmt.Errorf("empty recycle bin: %w", err)
	}
	return nil
}
