package git

import (
	"context"
	"fmt"
	"strings"

	domaingit "git.woa.com/leondli/workspace-service/internal/domain/git"
	"git.woa.com/leondli/workspace-service/internal/domain/identity"
)

type StageFilesReq struct {
	Context identity.RequestContext
	Path    string
	Files   []string
	All     bool
}

type UnstageFilesReq struct {
	Context identity.RequestContext
	Path    string
	Files   []string
	All     bool
}

type CommitReq struct {
	Context     identity.RequestContext
	Path        string
	Message     string
	AuthorName  string
	AuthorEmail string
}

type CommitResp struct {
	NothingToCommit bool   `json:"nothing_to_commit"`
	CommitHash      string `json:"commit_hash"`
}

type PushRepositoryReq struct {
	Context     identity.RequestContext
	Path        string
	RemoteName  string
	Branch      string
	GitUsername string
	GitToken    string
}

type PushRepositoryResp struct {
	Pushed bool `json:"pushed"`
}

func (s *Service) StageFiles(ctx context.Context, input StageFilesReq) error {
	actor, path, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return err
	}
	if err := s.gitClient.Stage(ctx, domaingit.StageReq{
		Actor: actor, Path: path, Files: input.Files, All: input.All,
	}); err != nil {
		return fmt.Errorf("stage files: %w", err)
	}
	return nil
}

func (s *Service) UnstageFiles(ctx context.Context, input UnstageFilesReq) error {
	actor, path, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return err
	}
	if err := s.gitClient.Unstage(ctx, domaingit.UnstageReq{
		Actor: actor, Path: path, Files: input.Files, All: input.All,
	}); err != nil {
		return fmt.Errorf("unstage files: %w", err)
	}
	return nil
}

func (s *Service) Commit(ctx context.Context, input CommitReq) (CommitResp, error) {
	actor, path, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return CommitResp{}, err
	}
	message := strings.TrimSpace(input.Message)
	if message == "" {
		return CommitResp{}, fmt.Errorf("%w: message is required", ErrInvalidGitRequest)
	}
	authorName := strings.TrimSpace(input.AuthorName)
	if authorName == "" {
		authorName = actor.UIN
	}
	authorEmail := strings.TrimSpace(input.AuthorEmail)
	if authorEmail == "" {
		authorEmail = fmt.Sprintf("%s@wedata", actor.UIN)
	}
	result, err := s.gitClient.Commit(ctx, domaingit.CommitReq{
		Actor: actor, Path: path, Message: message,
		AuthorName: authorName, AuthorEmail: authorEmail,
	})
	if err != nil {
		return CommitResp{}, fmt.Errorf("commit: %w", err)
	}
	return CommitResp{
		NothingToCommit: result.NothingToCommit,
		CommitHash:      result.CommitHash,
	}, nil
}

func (s *Service) PushRepository(ctx context.Context, input PushRepositoryReq) (PushRepositoryResp, error) {
	actor, path, err := s.resolveActorAndPath(input.Context, input.Path)
	if err != nil {
		return PushRepositoryResp{}, err
	}
	result, err := s.gitClient.Push(ctx, domaingit.PushReq{
		Actor:       actor,
		Path:        path,
		RemoteName:  strings.TrimSpace(input.RemoteName),
		Branch:      strings.TrimSpace(input.Branch),
		Credentials: Credentials{Username: input.GitUsername, Token: input.GitToken}.toDomain(),
	})
	if err != nil {
		return PushRepositoryResp{}, fmt.Errorf("push: %w", err)
	}
	return PushRepositoryResp{Pushed: result.Pushed}, nil
}
