package git

import (
	"context"
	"fmt"
	"os"
	"strings"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	domaingit "git.woa.com/leondli/workspace-service/internal/domain/git"
	"git.woa.com/leondli/workspace-service/internal/domain/identity"
	infrafs "git.woa.com/leondli/workspace-service/internal/infra/fs"
)

type CreateGitFolderReq struct {
	Context     identity.RequestContext
	TargetPath  string
	RepoURL     string
	Branch      string
	GitUsername string
	GitToken    string
}

type CreateGitFolderResp struct {
	Path   string `json:"path"`
	Status int    `json:"status"`
	Branch string `json:"branch"`
	RepoURL string `json:"repo_url"`
}

type GetGitFolderStatusReq struct {
	Context identity.RequestContext
	Path    string
}

type GetGitFolderStatusResp struct {
	Status  int    `json:"status"`
	Message string `json:"message,omitempty"`
	Path    string `json:"path"`
	Branch  string `json:"branch,omitempty"`
	RepoURL string `json:"repo_url,omitempty"`
}

func (s *Service) cloneJobKey(ctx identity.RequestContext, relPath string) string {
	return strings.Join([]string{ctx.AppID, ctx.WorkspaceID, ctx.UIN, relPath}, "|")
}

func (s *Service) CreateGitFolder(ctx context.Context, input CreateGitFolderReq) (CreateGitFolderResp, error) {
	cloneReq, err := s.buildCloneReq(CloneRepositoryReq{
		Context: input.Context, RepoURL: input.RepoURL,
		TargetPath: input.TargetPath, Branch: input.Branch,
	})
	if err != nil {
		return CreateGitFolderResp{}, err
	}

	if err := os.MkdirAll(cloneReq.TargetPath, 0o755); err != nil {
		return CreateGitFolderResp{}, fmt.Errorf("prepare git folder: %w", err)
	}

	relPath := strings.TrimSpace(input.TargetPath)
	key := s.cloneJobKey(cloneReq.Actor, relPath)
	s.cloneJobs.set(key, &gitFolderJob{
		Status: GitFolderStatusWaiting, RepoURL: cloneReq.RepoURL,
		Branch: cloneReq.Branch, Path: relPath,
	})

	go func() {
		bg := context.Background()
		s.cloneJobs.set(key, &gitFolderJob{
			Status: GitFolderStatusCloning, RepoURL: cloneReq.RepoURL,
			Branch: cloneReq.Branch, Path: relPath,
		})
		_, err := s.gitClient.Clone(bg, cloneReq)
		if err != nil {
			s.cloneJobs.set(key, &gitFolderJob{
				Status: GitFolderStatusFailed, Message: err.Error(),
				RepoURL: cloneReq.RepoURL, Branch: cloneReq.Branch, Path: relPath,
			})
			return
		}
		s.cloneJobs.set(key, &gitFolderJob{
			Status: GitFolderStatusReady, RepoURL: cloneReq.RepoURL,
			Branch: cloneReq.Branch, Path: relPath,
		})
		infrafs.RecordInode(bg, s.nodeStore, cloneReq.Actor, cloneReq.TargetPath, domainfile.NodeTypeGitFolder)
	}()

	return CreateGitFolderResp{
		Path: relPath, Status: GitFolderStatusWaiting,
		Branch: cloneReq.Branch, RepoURL: cloneReq.RepoURL,
	}, nil
}

func (s *Service) GetGitFolderStatus(ctx context.Context, input GetGitFolderStatusReq) (GetGitFolderStatusResp, error) {
	ident := input.Context.Normalize()
	if err := ident.Validate(); err != nil {
		return GetGitFolderStatusResp{}, fmt.Errorf("%w: %w", ErrInvalidGitRequest, err)
	}
	relPath := strings.TrimSpace(input.Path)
	if relPath == "" {
		return GetGitFolderStatusResp{}, fmt.Errorf("%w: path is required", ErrInvalidGitRequest)
	}

	key := s.cloneJobKey(ident, relPath)
	if job, ok := s.cloneJobs.get(key); ok {
		return GetGitFolderStatusResp{
			Status: job.Status, Message: job.Message,
			Path: job.Path, Branch: job.Branch, RepoURL: job.RepoURL,
		}, nil
	}

	absPath, err := s.resolveAbsPath(ident, relPath, ErrInvalidGitRequest)
	if err != nil {
		return GetGitFolderStatusResp{}, err
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return GetGitFolderStatusResp{Status: GitFolderStatusUnspecified, Path: relPath}, nil
	}
	if _, err := s.gitClient.Status(ctx, domaingit.StatusReq{Actor: ident, Path: absPath}); err == nil {
		resp := GetGitFolderStatusResp{Status: GitFolderStatusReady, Path: relPath}
		if info, err := s.gitClient.RepoInfo(ctx, domaingit.RepoInfoReq{Actor: ident, Path: absPath}); err == nil {
			resp.Branch = info.CurrentBranch
			resp.RepoURL = info.RemoteURL
		}
		return resp, nil
	}
	return GetGitFolderStatusResp{Status: GitFolderStatusUnspecified, Path: relPath}, nil
}
