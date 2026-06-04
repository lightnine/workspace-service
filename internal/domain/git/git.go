package git

import "context"

type Actor struct {
	OwnerUIN string
	UIN      string
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
}

type GitClient interface {
	Clone(ctx context.Context, req CloneReq) (CloneResult, error)
}
