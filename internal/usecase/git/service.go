package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	domaingit "git.woa.com/leondli/workspace-service/internal/domain/git"
	"git.woa.com/leondli/workspace-service/internal/domain/identity"
)

var (
	ErrInvalidCloneReq   = errors.New("invalid git clone request")
	ErrInvalidGitRequest = errors.New("invalid git request")
)

type CommandService interface {
	CloneRepository(ctx context.Context, req CloneRepositoryReq) (CloneRepositoryResp, error)
	CreateGitFolder(ctx context.Context, req CreateGitFolderReq) (CreateGitFolderResp, error)
	GetGitFolderStatus(ctx context.Context, req GetGitFolderStatusReq) (GetGitFolderStatusResp, error)
	PullRepository(ctx context.Context, req PullRepositoryReq) (PullRepositoryResp, error)
	Commit(ctx context.Context, req CommitReq) (CommitResp, error)
	PushRepository(ctx context.Context, req PushRepositoryReq) (PushRepositoryResp, error)
	CommitAndPush(ctx context.Context, req CommitAndPushReq) (CommitAndPushResp, error)
	StageFiles(ctx context.Context, req StageFilesReq) error
	UnstageFiles(ctx context.Context, req UnstageFilesReq) error
	CreateBranch(ctx context.Context, req CreateBranchReq) (BranchResp, error)
	CheckoutBranch(ctx context.Context, req CheckoutBranchReq) (BranchResp, error)
	ListBranches(ctx context.Context, req ListBranchesReq) (ListBranchesResp, error)
	GetStatus(ctx context.Context, req StatusReq) (StatusResp, error)
	GetFileDiff(ctx context.Context, req FileDiffReq) (FileDiffResp, error)
	GetCommitHistory(ctx context.Context, req CommitHistoryReq) (CommitHistoryResp, error)
	DiscardChanges(ctx context.Context, req DiscardChangesReq) error
	DeleteRepository(ctx context.Context, req DeleteRepositoryReq) error
}

type CloneRepositoryReq struct {
	Context    identity.RequestContext
	RepoURL    string
	TargetPath string
	Branch     string
}

type CloneRepositoryResp struct {
	RepoURL string `json:"repo_url"`
	Path    string `json:"path"`
	Branch  string `json:"branch"`
}

type Service struct {
	gitClient     domaingit.GitClient
	mountRoot     string
	nodeStore     domainfile.NodeStore
	cloneJobs     *cloneJobRegistry
}

func NewService(gitClient domaingit.GitClient, mountRoot string, nodeStore domainfile.NodeStore) *Service {
	return &Service{
		gitClient: gitClient,
		mountRoot: CleanMountRoot(mountRoot),
		nodeStore: nodeStore,
		cloneJobs: newCloneJobRegistry(),
	}
}

// CleanMountRoot expands "~" and cleans a configured filesystem path.
func CleanMountRoot(mountRoot string) string {
	return cleanMountRoot(mountRoot)
}

func (s *Service) CloneRepository(ctx context.Context, input CloneRepositoryReq) (CloneRepositoryResp, error) {
	req, err := s.buildCloneReq(input)
	if err != nil {
		return CloneRepositoryResp{}, err
	}

	result, err := s.gitClient.Clone(ctx, req)
	if err != nil {
		return CloneRepositoryResp{}, fmt.Errorf("clone repository: %w", err)
	}

	return CloneRepositoryResp{
		RepoURL: result.RepoURL,
		Path:    result.Path,
		Branch:  result.Branch,
	}, nil
}

func (s *Service) buildCloneReq(input CloneRepositoryReq) (domaingit.CloneReq, error) {
	ctx := input.Context.Normalize()
	repoURL := strings.TrimSpace(input.RepoURL)
	targetPath := strings.TrimSpace(input.TargetPath)
	branch := strings.TrimSpace(input.Branch)

	if err := ctx.Validate(); err != nil {
		return domaingit.CloneReq{}, fmt.Errorf("%w: %w", ErrInvalidCloneReq, err)
	}
	if repoURL == "" {
		return domaingit.CloneReq{}, fmt.Errorf("%w: repo_url is required", ErrInvalidCloneReq)
	}
	if targetPath == "" {
		return domaingit.CloneReq{}, fmt.Errorf("%w: target_path is required", ErrInvalidCloneReq)
	}
	if branch == "" {
		return domaingit.CloneReq{}, fmt.Errorf("%w: branch is required", ErrInvalidCloneReq)
	}
	absoluteTargetPath, err := s.resolveAbsPath(ctx, targetPath, ErrInvalidCloneReq)
	if err != nil {
		return domaingit.CloneReq{}, err
	}

	return domaingit.CloneReq{
		Actor:      ctx,
		RepoURL:    repoURL,
		TargetPath: absoluteTargetPath,
		Branch:     branch,
	}, nil
}

func (s *Service) resolveActorAndPath(ctx identity.RequestContext, path string) (domaingit.Actor, string, error) {
	ctx = ctx.Normalize()
	if err := ctx.Validate(); err != nil {
		return domaingit.Actor{}, "", fmt.Errorf("%w: %w", ErrInvalidGitRequest, err)
	}
	absPath, err := s.resolveAbsPath(ctx, path, ErrInvalidGitRequest)
	if err != nil {
		return domaingit.Actor{}, "", err
	}
	return ctx, absPath, nil
}

func (s *Service) resolveAbsPath(ctx identity.RequestContext, relPath string, wrap error) (string, error) {
	if s.mountRoot == "" {
		return "", fmt.Errorf("%w: workspace mount root is required", wrap)
	}
	resolved, err := ctx.ResolveRelativePath(relPath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", wrap, err)
	}
	abs := filepath.Join(s.mountRoot, resolved)
	mount := s.mountRoot + string(filepath.Separator)
	if !strings.HasPrefix(abs, mount) && abs != s.mountRoot {
		return "", fmt.Errorf("%w: path escapes workspace mount root", wrap)
	}
	return abs, nil
}

// LookupGitBranch implements fs.GitBranchLookup for ListFiles enrichment.
func (s *Service) LookupGitBranch(ctx context.Context, ident identity.RequestContext, relPath string) (string, error) {
	actor, path, err := s.resolveActorAndPath(ident, relPath)
	if err != nil {
		return "", err
	}
	result, err := s.gitClient.ListBranches(ctx, domaingit.ListBranchesReq{Actor: actor, Path: path})
	if err != nil {
		return "", err
	}
	return result.CurrentBranch, nil
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
