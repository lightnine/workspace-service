package git

import (
	"context"
	"time"

	"git.woa.com/leondli/workspace-service/internal/domain/identity"
)

// Actor is the Git operation principal (aligned with wedata3 RequestBaseInfo / auth.Identity).
type Actor = identity.RequestContext

// Credentials carries per-request Git auth (e.g. a personal access token) used
// for remote operations against private repositories.
type Credentials struct {
	Username string
	Token    string
}

type CloneReq struct {
	Actor      Actor
	RepoURL    string
	TargetPath string
	Branch     string
}

type CloneResult struct {
	RepoURL string
	Path    string
	Branch  string
}

type PullReq struct {
	Actor       Actor
	Path        string
	RemoteName  string
	Branch      string
	Credentials Credentials
}

type PullResult struct {
	AlreadyUpToDate bool
	HeadCommit      string
	CurrentBranch   string
}

type StageReq struct {
	Actor Actor
	Path  string
	Files []string
	All   bool
}

type UnstageReq struct {
	Actor Actor
	Path  string
	Files []string
	All   bool
}

type CommitReq struct {
	Actor       Actor
	Path        string
	Message     string
	AuthorName  string
	AuthorEmail string
}

type CommitResult struct {
	NothingToCommit bool
	CommitHash      string
}

type PushReq struct {
	Actor       Actor
	Path        string
	RemoteName  string
	Branch      string
	Credentials Credentials
}

type PushResult struct {
	Pushed bool
}

type CommitAndPushReq struct {
	Actor       Actor
	Path        string
	Message     string
	AuthorName  string
	AuthorEmail string
	RemoteName  string
	Push        bool
	Credentials Credentials
}

type CommitAndPushResult struct {
	NothingToCommit bool
	CommitHash      string
	Pushed          bool
}

type CreateBranchReq struct {
	Actor    Actor
	Path     string
	Branch   string
	Checkout bool
}

type CheckoutBranchReq struct {
	Actor  Actor
	Path   string
	Branch string
	Create bool
}

type BranchResult struct {
	CurrentBranch string
}

type ListBranchesReq struct {
	Actor Actor
	Path  string
}

type ListBranchesResult struct {
	CurrentBranch string
	Branches      []string
}

type StatusReq struct {
	Actor Actor
	Path  string
}

type RepoInfoReq struct {
	Actor Actor
	Path  string
}

type RepoInfoResult struct {
	CurrentBranch string
	RemoteURL     string
}

type FileStatus struct {
	Path     string
	Staging  string
	Worktree string
}

type StatusResult struct {
	Clean bool
	Files []FileStatus
}

type CommitHistoryReq struct {
	Actor Actor
	Path  string
	Limit int
}

type CommitInfo struct {
	Hash    string
	Author  string
	Email   string
	Message string
	When    time.Time
}

type CommitHistoryResult struct {
	Commits []CommitInfo
}

type FileDiffReq struct {
	Actor Actor
	Path  string
	File  string
}

type FileDiffResult struct {
	File                  string
	HeadContentBase64     string
	WorktreeContentBase64 string
	HeadMissing           bool
	WorktreeMissing       bool
}

type DiscardChangesReq struct {
	Actor Actor
	Path  string
}

type DeleteRepoReq struct {
	Actor Actor
	Path  string
}

// GitClient performs Git operations against repositories checked out in the
// workspace, mirroring the operations available in Databricks Git folders.
type GitClient interface {
	Clone(ctx context.Context, req CloneReq) (CloneResult, error)
	Pull(ctx context.Context, req PullReq) (PullResult, error)
	Stage(ctx context.Context, req StageReq) error
	Unstage(ctx context.Context, req UnstageReq) error
	Commit(ctx context.Context, req CommitReq) (CommitResult, error)
	Push(ctx context.Context, req PushReq) (PushResult, error)
	CommitAndPush(ctx context.Context, req CommitAndPushReq) (CommitAndPushResult, error)
	CreateBranch(ctx context.Context, req CreateBranchReq) (BranchResult, error)
	CheckoutBranch(ctx context.Context, req CheckoutBranchReq) (BranchResult, error)
	ListBranches(ctx context.Context, req ListBranchesReq) (ListBranchesResult, error)
	Status(ctx context.Context, req StatusReq) (StatusResult, error)
	RepoInfo(ctx context.Context, req RepoInfoReq) (RepoInfoResult, error)
	FileDiff(ctx context.Context, req FileDiffReq) (FileDiffResult, error)
	CommitHistory(ctx context.Context, req CommitHistoryReq) (CommitHistoryResult, error)
	DiscardChanges(ctx context.Context, req DiscardChangesReq) error
	DeleteRepo(ctx context.Context, req DeleteRepoReq) error
}
