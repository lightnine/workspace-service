package fs

import (
	"context"

	"git.woa.com/leondli/workspace-service/internal/domain/identity"
)

// GitBranchLookup resolves the current branch for a Git folder (workspace-relative path).
type GitBranchLookup interface {
	LookupGitBranch(ctx context.Context, ident identity.RequestContext, relPath string) (string, error)
}

func (s *Service) enrichGitBranches(ctx context.Context, ident identity.RequestContext, files []FileInfoResp) []FileInfoResp {
	if s.gitBranches == nil {
		return files
	}
	for i := range files {
		if !files[i].IsGitFolder {
			continue
		}
		branch, err := s.gitBranches.LookupGitBranch(ctx, ident, files[i].Path)
		if err != nil || branch == "" {
			continue
		}
		files[i].GitBranch = branch
	}
	return files
}
