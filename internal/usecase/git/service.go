package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	domaingit "git.woa.com/leondli/workspace-service/internal/domain/git"
)

var ErrInvalidCloneReq = errors.New("invalid git clone request")

type CommandService interface {
	CloneRepository(ctx context.Context, req CloneRepositoryReq) (CloneRepositoryResp, error)
}

type CloneRepositoryReq struct {
	OwnerUIN   string
	UIN        string
	RepoURL    string
	TargetPath string
	Branch     string
}

type CloneRepositoryResp struct {
	RepoURL string `json:"repo_url"`
	Path    string `json:"path"`
}

type Service struct {
	gitClient domaingit.GitClient
	mountRoot string
}

func NewService(gitClient domaingit.GitClient, mountRoot string) *Service {
	return &Service{
		gitClient: gitClient,
		mountRoot: cleanMountRoot(mountRoot),
	}
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
	}, nil
}

func (s *Service) buildCloneReq(input CloneRepositoryReq) (domaingit.CloneReq, error) {
	ownerUIN := strings.TrimSpace(input.OwnerUIN)
	uin := strings.TrimSpace(input.UIN)
	repoURL := strings.TrimSpace(input.RepoURL)
	targetPath := strings.TrimSpace(input.TargetPath)
	branch := strings.TrimSpace(input.Branch)

	if ownerUIN == "" {
		return domaingit.CloneReq{}, fmt.Errorf("%w: owner_uin is required", ErrInvalidCloneReq)
	}
	if uin == "" {
		return domaingit.CloneReq{}, fmt.Errorf("%w: uin is required", ErrInvalidCloneReq)
	}
	if repoURL == "" {
		return domaingit.CloneReq{}, fmt.Errorf("%w: repo_url is required", ErrInvalidCloneReq)
	}
	if targetPath == "" {
		return domaingit.CloneReq{}, fmt.Errorf("%w: target_path is required", ErrInvalidCloneReq)
	}
	absoluteTargetPath, err := s.resolveWorkspacePath(targetPath)
	if err != nil {
		return domaingit.CloneReq{}, err
	}

	return domaingit.CloneReq{
		Actor: domaingit.Actor{
			OwnerUIN: ownerUIN,
			UIN:      uin,
		},
		RepoURL:    repoURL,
		TargetPath: absoluteTargetPath,
		Branch:     branch,
	}, nil
}

func (s *Service) resolveWorkspacePath(workspacePath string) (string, error) {
	if s.mountRoot == "" {
		return "", fmt.Errorf("%w: workspace mount root is required", ErrInvalidCloneReq)
	}
	if filepath.IsAbs(workspacePath) {
		return "", fmt.Errorf("%w: target_path must be relative to workspace mount root", ErrInvalidCloneReq)
	}

	cleanPath := filepath.Clean(workspacePath)
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: target_path escapes workspace mount root", ErrInvalidCloneReq)
	}

	return filepath.Join(s.mountRoot, cleanPath), nil
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
