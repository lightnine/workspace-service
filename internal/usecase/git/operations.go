package git

import (
	"context"
	"fmt"
	"strings"

	domaingit "git.woa.com/leondli/workspace-service/internal/domain/git"
	"git.woa.com/leondli/workspace-service/internal/domain/identity"
)

// timeFormat renders commit timestamps as RFC3339 in API responses.
const timeFormat = "2006-01-02T15:04:05Z07:00"

// Credentials carries optional per-request Git auth for remote operations.
type Credentials struct {
	Username string
	Token    string
}

func (c Credentials) toDomain() domaingit.Credentials {
	return domaingit.Credentials{Username: c.Username, Token: c.Token}
}

type PullRepositoryReq struct {
	Context     identity.RequestContext
	Path        string
	RemoteName  string
	Branch      string
	Credentials Credentials
}

type PullRepositoryResp struct {
	AlreadyUpToDate bool   `json:"already_up_to_date"`
	HeadCommit      string `json:"head_commit"`
	CurrentBranch   string `json:"current_branch"`
}

type CommitAndPushReq struct {
	Context     identity.RequestContext
	Path        string
	Message     string
	AuthorName  string
	AuthorEmail string
	RemoteName  string
	Push        bool
	Credentials Credentials
}

type CommitAndPushResp struct {
	NothingToCommit bool   `json:"nothing_to_commit"`
	CommitHash      string `json:"commit_hash"`
	Pushed          bool   `json:"pushed"`
}

type CreateBranchReq struct {
	Context  identity.RequestContext
	Path     string
	Branch   string
	Checkout bool
}

type CheckoutBranchReq struct {
	Context  identity.RequestContext
	Path     string
	Branch   string
	Create   bool
}

type BranchResp struct {
	CurrentBranch string `json:"current_branch"`
}

type ListBranchesReq struct {
	Context identity.RequestContext
	Path    string
}

type ListBranchesResp struct {
	CurrentBranch string   `json:"current_branch"`
	Branches      []string `json:"branches"`
}

type StatusReq struct {
	Context identity.RequestContext
	Path    string
}

type FileStatus struct {
	Path     string `json:"path"`
	Staging  string `json:"staging"`
	Worktree string `json:"worktree"`
}

type StatusResp struct {
	Clean bool         `json:"clean"`
	Files []FileStatus `json:"files"`
}

type CommitHistoryReq struct {
	Context identity.RequestContext
	Path    string
	Limit   int
}

type CommitInfo struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Email   string `json:"email"`
	Message string `json:"message"`
	When    string `json:"when"`
}

type CommitHistoryResp struct {
	Commits []CommitInfo `json:"commits"`
}

type DiscardChangesReq struct {
	Context identity.RequestContext
	Path    string
}

type DeleteRepositoryReq struct {
	Context identity.RequestContext
	Path    string
}

func (s *Service) PullRepository(ctx context.Context, input PullRepositoryReq) (PullRepositoryResp, error) {
	actor, path, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return PullRepositoryResp{}, err
	}

	result, err := s.gitClient.Pull(ctx, domaingit.PullReq{
		Actor:       actor,
		Path:        path,
		RemoteName:  strings.TrimSpace(input.RemoteName),
		Branch:      strings.TrimSpace(input.Branch),
		Credentials: input.Credentials.toDomain(),
	})
	if err != nil {
		return PullRepositoryResp{}, fmt.Errorf("pull repository: %w", err)
	}

	return PullRepositoryResp{
		AlreadyUpToDate: result.AlreadyUpToDate,
		HeadCommit:      result.HeadCommit,
		CurrentBranch:   result.CurrentBranch,
	}, nil
}

func (s *Service) CommitAndPush(ctx context.Context, input CommitAndPushReq) (CommitAndPushResp, error) {
	actor, path, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return CommitAndPushResp{}, err
	}

	message := strings.TrimSpace(input.Message)
	if message == "" {
		return CommitAndPushResp{}, fmt.Errorf("%w: message is required", ErrInvalidGitRequest)
	}

	authorName := strings.TrimSpace(input.AuthorName)
	if authorName == "" {
		authorName = actor.UIN
	}
	authorEmail := strings.TrimSpace(input.AuthorEmail)
	if authorEmail == "" {
		authorEmail = fmt.Sprintf("%s@wedata", actor.UIN)
	}

	result, err := s.gitClient.CommitAndPush(ctx, domaingit.CommitAndPushReq{
		Actor:       actor,
		Path:        path,
		Message:     message,
		AuthorName:  authorName,
		AuthorEmail: authorEmail,
		RemoteName:  strings.TrimSpace(input.RemoteName),
		Push:        input.Push,
		Credentials: input.Credentials.toDomain(),
	})
	if err != nil {
		return CommitAndPushResp{}, fmt.Errorf("commit and push: %w", err)
	}

	return CommitAndPushResp{
		NothingToCommit: result.NothingToCommit,
		CommitHash:      result.CommitHash,
		Pushed:          result.Pushed,
	}, nil
}

func (s *Service) CreateBranch(ctx context.Context, input CreateBranchReq) (BranchResp, error) {
	actor, path, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return BranchResp{}, err
	}
	branch := strings.TrimSpace(input.Branch)
	if branch == "" {
		return BranchResp{}, fmt.Errorf("%w: branch is required", ErrInvalidGitRequest)
	}

	result, err := s.gitClient.CreateBranch(ctx, domaingit.CreateBranchReq{
		Actor:    actor,
		Path:     path,
		Branch:   branch,
		Checkout: input.Checkout,
	})
	if err != nil {
		return BranchResp{}, fmt.Errorf("create branch: %w", err)
	}
	return BranchResp{CurrentBranch: result.CurrentBranch}, nil
}

func (s *Service) CheckoutBranch(ctx context.Context, input CheckoutBranchReq) (BranchResp, error) {
	actor, path, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return BranchResp{}, err
	}
	branch := strings.TrimSpace(input.Branch)
	if branch == "" {
		return BranchResp{}, fmt.Errorf("%w: branch is required", ErrInvalidGitRequest)
	}

	result, err := s.gitClient.CheckoutBranch(ctx, domaingit.CheckoutBranchReq{
		Actor:  actor,
		Path:   path,
		Branch: branch,
		Create: input.Create,
	})
	if err != nil {
		return BranchResp{}, fmt.Errorf("checkout branch: %w", err)
	}
	return BranchResp{CurrentBranch: result.CurrentBranch}, nil
}

func (s *Service) ListBranches(ctx context.Context, input ListBranchesReq) (ListBranchesResp, error) {
	actor, path, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return ListBranchesResp{}, err
	}

	result, err := s.gitClient.ListBranches(ctx, domaingit.ListBranchesReq{Actor: actor, Path: path})
	if err != nil {
		return ListBranchesResp{}, fmt.Errorf("list branches: %w", err)
	}
	return ListBranchesResp{
		CurrentBranch: result.CurrentBranch,
		Branches:      result.Branches,
	}, nil
}

func (s *Service) GetStatus(ctx context.Context, input StatusReq) (StatusResp, error) {
	actor, path, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return StatusResp{}, err
	}

	result, err := s.gitClient.Status(ctx, domaingit.StatusReq{Actor: actor, Path: path})
	if err != nil {
		return StatusResp{}, fmt.Errorf("get status: %w", err)
	}

	resp := StatusResp{Clean: result.Clean}
	for _, file := range result.Files {
		resp.Files = append(resp.Files, FileStatus{
			Path:     file.Path,
			Staging:  file.Staging,
			Worktree: file.Worktree,
		})
	}
	return resp, nil
}

func (s *Service) GetCommitHistory(ctx context.Context, input CommitHistoryReq) (CommitHistoryResp, error) {
	actor, path, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return CommitHistoryResp{}, err
	}

	result, err := s.gitClient.CommitHistory(ctx, domaingit.CommitHistoryReq{
		Actor: actor,
		Path:  path,
		Limit: input.Limit,
	})
	if err != nil {
		return CommitHistoryResp{}, fmt.Errorf("get commit history: %w", err)
	}

	resp := CommitHistoryResp{}
	for _, commit := range result.Commits {
		resp.Commits = append(resp.Commits, CommitInfo{
			Hash:    commit.Hash,
			Author:  commit.Author,
			Email:   commit.Email,
			Message: commit.Message,
			When:    commit.When.Format(timeFormat),
		})
	}
	return resp, nil
}

func (s *Service) DiscardChanges(ctx context.Context, input DiscardChangesReq) error {
	actor, path, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return err
	}
	if err := s.gitClient.DiscardChanges(ctx, domaingit.DiscardChangesReq{Actor: actor, Path: path}); err != nil {
		return fmt.Errorf("discard changes: %w", err)
	}
	return nil
}

func (s *Service) DeleteRepository(ctx context.Context, input DeleteRepositoryReq) error {
	actor, path, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return err
	}
	if err := s.gitClient.DeleteRepo(ctx, domaingit.DeleteRepoReq{Actor: actor, Path: path}); err != nil {
		return fmt.Errorf("delete repository: %w", err)
	}
	return nil
}
