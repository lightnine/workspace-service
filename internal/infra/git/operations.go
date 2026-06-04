package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	domaingit "git.woa.com/leondli/workspace-service/internal/domain/git"
	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

const (
	defaultRemoteName  = "origin"
	defaultHistorySize = 50
)

// openRepo opens an already-cloned repository using the same storage + identity
// worktree as Clone, so write operations (pull, checkout, ...) keep recording
// file ownership in the node store.
func (c *WorkspaceGitClient) openRepo(ctx context.Context, path string, actor domaingit.Actor) (*gogit.Repository, error) {
	gitDir, err := c.gitDir(path)
	if err != nil {
		return nil, err
	}
	gitStorage := filesystem.NewStorage(
		osfs.New(gitDir),
		cache.NewObjectLRUDefault(),
	)
	worktree := newIdentityFS(ctx, osfs.New(path), actor, c.fileNodeStore)

	repo, err := gogit.Open(gitStorage, worktree)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	return repo, nil
}

func (c *WorkspaceGitClient) Pull(ctx context.Context, req domaingit.PullReq) (domaingit.PullResult, error) {
	repo, err := c.openRepo(ctx, req.Path, req.Actor)
	if err != nil {
		return domaingit.PullResult{}, err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return domaingit.PullResult{}, fmt.Errorf("worktree: %w", err)
	}

	options := &gogit.PullOptions{
		RemoteName: remoteOrDefault(req.RemoteName),
		Auth:       basicAuth(req.Credentials),
	}
	if req.Branch != "" {
		options.ReferenceName = plumbing.NewBranchReferenceName(req.Branch)
	}

	err = worktree.PullContext(ctx, options)
	upToDate := errors.Is(err, gogit.NoErrAlreadyUpToDate)
	if err != nil && !upToDate {
		return domaingit.PullResult{}, fmt.Errorf("pull: %w", err)
	}

	result := domaingit.PullResult{AlreadyUpToDate: upToDate}
	if head, headErr := repo.Head(); headErr == nil {
		result.HeadCommit = head.Hash().String()
		result.CurrentBranch = head.Name().Short()
	}
	return result, nil
}

func (c *WorkspaceGitClient) Stage(ctx context.Context, req domaingit.StageReq) error {
	repo, err := c.openRepo(ctx, req.Path, req.Actor)
	if err != nil {
		return err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}
	if req.All {
		return worktree.AddWithOptions(&gogit.AddOptions{All: true})
	}
	for _, file := range req.Files {
		if err := worktree.AddWithOptions(&gogit.AddOptions{Path: file}); err != nil {
			return fmt.Errorf("stage %s: %w", file, err)
		}
	}
	return nil
}

func (c *WorkspaceGitClient) Unstage(ctx context.Context, req domaingit.UnstageReq) error {
	repo, err := c.openRepo(ctx, req.Path, req.Actor)
	if err != nil {
		return err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}
	if req.All {
		return worktree.Reset(&gogit.ResetOptions{Mode: gogit.MixedReset})
	}
	for _, file := range req.Files {
		if _, err := worktree.Remove(file); err != nil {
			return fmt.Errorf("unstage %s: %w", file, err)
		}
	}
	return nil
}

func (c *WorkspaceGitClient) Commit(ctx context.Context, req domaingit.CommitReq) (domaingit.CommitResult, error) {
	repo, err := c.openRepo(ctx, req.Path, req.Actor)
	if err != nil {
		return domaingit.CommitResult{}, err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return domaingit.CommitResult{}, fmt.Errorf("worktree: %w", err)
	}
	status, err := worktree.Status()
	if err != nil {
		return domaingit.CommitResult{}, fmt.Errorf("status: %w", err)
	}
	if status.IsClean() {
		return domaingit.CommitResult{NothingToCommit: true}, nil
	}
	hash, err := worktree.Commit(req.Message, &gogit.CommitOptions{
		Author: &object.Signature{
			Name: req.AuthorName, Email: req.AuthorEmail, When: time.Now(),
		},
	})
	if err != nil {
		return domaingit.CommitResult{}, fmt.Errorf("commit: %w", err)
	}
	return domaingit.CommitResult{CommitHash: hash.String()}, nil
}

func (c *WorkspaceGitClient) Push(ctx context.Context, req domaingit.PushReq) (domaingit.PushResult, error) {
	repo, err := c.openRepo(ctx, req.Path, req.Actor)
	if err != nil {
		return domaingit.PushResult{}, err
	}
	err = repo.PushContext(ctx, &gogit.PushOptions{
		RemoteName: remoteOrDefault(req.RemoteName),
		Auth:       basicAuth(req.Credentials),
	})
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return domaingit.PushResult{}, fmt.Errorf("push: %w", err)
	}
	return domaingit.PushResult{Pushed: true}, nil
}

func (c *WorkspaceGitClient) CommitAndPush(ctx context.Context, req domaingit.CommitAndPushReq) (domaingit.CommitAndPushResult, error) {
	if err := c.Stage(ctx, domaingit.StageReq{Actor: req.Actor, Path: req.Path, All: true}); err != nil {
		return domaingit.CommitAndPushResult{}, err
	}
	commitResult, err := c.Commit(ctx, domaingit.CommitReq{
		Actor: req.Actor, Path: req.Path, Message: req.Message,
		AuthorName: req.AuthorName, AuthorEmail: req.AuthorEmail,
	})
	if err != nil {
		return domaingit.CommitAndPushResult{}, err
	}
	result := domaingit.CommitAndPushResult{
		NothingToCommit: commitResult.NothingToCommit,
		CommitHash:      commitResult.CommitHash,
	}
	if !req.Push || commitResult.NothingToCommit {
		return result, nil
	}
	pushResult, err := c.Push(ctx, domaingit.PushReq{
		Actor: req.Actor, Path: req.Path,
		RemoteName: req.RemoteName, Credentials: req.Credentials,
	})
	if err != nil {
		return result, err
	}
	result.Pushed = pushResult.Pushed
	return result, nil
}

func (c *WorkspaceGitClient) CreateBranch(ctx context.Context, req domaingit.CreateBranchReq) (domaingit.BranchResult, error) {
	repo, err := c.openRepo(ctx, req.Path, req.Actor)
	if err != nil {
		return domaingit.BranchResult{}, err
	}
	refName := plumbing.NewBranchReferenceName(req.Branch)

	if req.Checkout {
		worktree, wtErr := repo.Worktree()
		if wtErr != nil {
			return domaingit.BranchResult{}, fmt.Errorf("worktree: %w", wtErr)
		}
		if err := worktree.Checkout(&gogit.CheckoutOptions{Branch: refName, Create: true}); err != nil {
			return domaingit.BranchResult{}, fmt.Errorf("create branch: %w", err)
		}
	} else {
		head, headErr := repo.Head()
		if headErr != nil {
			return domaingit.BranchResult{}, fmt.Errorf("resolve HEAD: %w", headErr)
		}
		if err := repo.Storer.SetReference(plumbing.NewHashReference(refName, head.Hash())); err != nil {
			return domaingit.BranchResult{}, fmt.Errorf("create branch: %w", err)
		}
	}

	return domaingit.BranchResult{CurrentBranch: currentBranch(repo)}, nil
}

func (c *WorkspaceGitClient) CheckoutBranch(ctx context.Context, req domaingit.CheckoutBranchReq) (domaingit.BranchResult, error) {
	repo, err := c.openRepo(ctx, req.Path, req.Actor)
	if err != nil {
		return domaingit.BranchResult{}, err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return domaingit.BranchResult{}, fmt.Errorf("worktree: %w", err)
	}

	refName := plumbing.NewBranchReferenceName(req.Branch)
	err = worktree.Checkout(&gogit.CheckoutOptions{Branch: refName, Create: req.Create})
	if err != nil && !req.Create {
		// Fall back to creating a local branch from the remote tracking ref so
		// users can switch to a branch that only exists on the remote.
		if remoteRef, refErr := repo.Reference(plumbing.NewRemoteReferenceName(defaultRemoteName, req.Branch), true); refErr == nil {
			if setErr := repo.Storer.SetReference(plumbing.NewHashReference(refName, remoteRef.Hash())); setErr == nil {
				err = worktree.Checkout(&gogit.CheckoutOptions{Branch: refName})
			}
		}
	}
	if err != nil {
		return domaingit.BranchResult{}, fmt.Errorf("checkout branch: %w", err)
	}

	return domaingit.BranchResult{CurrentBranch: currentBranch(repo)}, nil
}

func (c *WorkspaceGitClient) ListBranches(ctx context.Context, req domaingit.ListBranchesReq) (domaingit.ListBranchesResult, error) {
	repo, err := c.openRepo(ctx, req.Path, req.Actor)
	if err != nil {
		return domaingit.ListBranchesResult{}, err
	}

	iter, err := repo.Branches()
	if err != nil {
		return domaingit.ListBranchesResult{}, fmt.Errorf("list branches: %w", err)
	}
	defer iter.Close()

	var branches []string
	if err := iter.ForEach(func(ref *plumbing.Reference) error {
		branches = append(branches, ref.Name().Short())
		return nil
	}); err != nil {
		return domaingit.ListBranchesResult{}, fmt.Errorf("iterate branches: %w", err)
	}

	return domaingit.ListBranchesResult{
		CurrentBranch: currentBranch(repo),
		Branches:      branches,
	}, nil
}

func (c *WorkspaceGitClient) Status(ctx context.Context, req domaingit.StatusReq) (domaingit.StatusResult, error) {
	repo, err := c.openRepo(ctx, req.Path, req.Actor)
	if err != nil {
		return domaingit.StatusResult{}, err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return domaingit.StatusResult{}, fmt.Errorf("worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return domaingit.StatusResult{}, fmt.Errorf("status: %w", err)
	}

	result := domaingit.StatusResult{Clean: status.IsClean()}
	for path, fileStatus := range status {
		result.Files = append(result.Files, domaingit.FileStatus{
			Path:     path,
			Staging:  statusCodeString(fileStatus.Staging),
			Worktree: statusCodeString(fileStatus.Worktree),
		})
	}
	return result, nil
}

func (c *WorkspaceGitClient) CommitHistory(ctx context.Context, req domaingit.CommitHistoryReq) (domaingit.CommitHistoryResult, error) {
	repo, err := c.openRepo(ctx, req.Path, req.Actor)
	if err != nil {
		return domaingit.CommitHistoryResult{}, err
	}

	iter, err := repo.Log(&gogit.LogOptions{})
	if err != nil {
		return domaingit.CommitHistoryResult{}, fmt.Errorf("read log: %w", err)
	}
	defer iter.Close()

	limit := req.Limit
	if limit <= 0 {
		limit = defaultHistorySize
	}

	var commits []domaingit.CommitInfo
	err = iter.ForEach(func(commit *object.Commit) error {
		if len(commits) >= limit {
			return storer.ErrStop
		}
		commits = append(commits, domaingit.CommitInfo{
			Hash:    commit.Hash.String(),
			Author:  commit.Author.Name,
			Email:   commit.Author.Email,
			Message: strings.TrimSpace(commit.Message),
			When:    commit.Author.When,
		})
		return nil
	})
	if err != nil {
		return domaingit.CommitHistoryResult{}, fmt.Errorf("iterate log: %w", err)
	}

	return domaingit.CommitHistoryResult{Commits: commits}, nil
}

func (c *WorkspaceGitClient) DiscardChanges(ctx context.Context, req domaingit.DiscardChangesReq) error {
	repo, err := c.openRepo(ctx, req.Path, req.Actor)
	if err != nil {
		return err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	if err := worktree.Reset(&gogit.ResetOptions{Mode: gogit.HardReset}); err != nil {
		return fmt.Errorf("reset worktree: %w", err)
	}
	if err := worktree.Clean(&gogit.CleanOptions{Dir: true}); err != nil {
		return fmt.Errorf("clean worktree: %w", err)
	}
	return nil
}

func (c *WorkspaceGitClient) DeleteRepo(_ context.Context, req domaingit.DeleteRepoReq) error {
	if strings.TrimSpace(req.Path) == "" {
		return fmt.Errorf("repo path is required")
	}
	gitDir, err := c.gitDir(req.Path)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(req.Path); err != nil {
		return fmt.Errorf("delete repo worktree: %w", err)
	}
	if !isSubpath(req.Path, gitDir) {
		if err := os.RemoveAll(gitDir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete repo git metadata: %w", err)
		}
	}
	return nil
}

func remoteOrDefault(name string) string {
	if strings.TrimSpace(name) == "" {
		return defaultRemoteName
	}
	return name
}

func basicAuth(cred domaingit.Credentials) transport.AuthMethod {
	if strings.TrimSpace(cred.Token) == "" {
		return nil
	}
	username := cred.Username
	if username == "" {
		// Token-based providers (e.g. GitHub PAT) accept any non-empty username.
		username = "git"
	}
	return &githttp.BasicAuth{Username: username, Password: cred.Token}
}

func currentBranch(repo *gogit.Repository) string {
	head, err := repo.Head()
	if err != nil {
		return ""
	}
	return head.Name().Short()
}

func statusCodeString(code gogit.StatusCode) string {
	switch code {
	case gogit.Unmodified:
		return "unmodified"
	case gogit.Untracked:
		return "untracked"
	case gogit.Modified:
		return "modified"
	case gogit.Added:
		return "added"
	case gogit.Deleted:
		return "deleted"
	case gogit.Renamed:
		return "renamed"
	case gogit.Copied:
		return "copied"
	case gogit.UpdatedButUnmerged:
		return "unmerged"
	default:
		return "unknown"
	}
}
