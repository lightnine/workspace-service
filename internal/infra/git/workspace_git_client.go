package git

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	domaingit "git.woa.com/leondli/workspace-service/internal/domain/git"
	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

type WorkspaceGitClient struct {
	fileNodeStore domainfile.NodeStore
	mountRoot     string
	gitMetaRoot   string
}

func NewWorkspaceGitClient(fileNodeStore domainfile.NodeStore, mountRoot, gitMetaRoot string) *WorkspaceGitClient {
	return &WorkspaceGitClient{
		fileNodeStore: fileNodeStore,
		mountRoot:     mountRoot,
		gitMetaRoot:   gitMetaRoot,
	}
}

func (c *WorkspaceGitClient) gitDir(worktreePath string) (string, error) {
	return resolveGitDir(c.mountRoot, c.gitMetaRoot, worktreePath)
}

func (c *WorkspaceGitClient) Clone(ctx context.Context, req domaingit.CloneReq) (domaingit.CloneResult, error) {
	if err := prepareCloneTarget(req.TargetPath); err != nil {
		return domaingit.CloneResult{}, err
	}

	branch := normalizeBranchName(req.Branch)
	if branch == "" {
		return domaingit.CloneResult{}, fmt.Errorf("branch is required")
	}

	gitDir, err := c.gitDir(req.TargetPath)
	if err != nil {
		return domaingit.CloneResult{}, err
	}
	if err := ensureGitDirParent(gitDir); err != nil {
		return domaingit.CloneResult{}, err
	}

	options := &gogit.CloneOptions{
		URL:           req.RepoURL,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Tags:          gogit.NoTags,
	}

	gitStorage := filesystem.NewStorage(
		osfs.New(gitDir),
		cache.NewObjectLRUDefault(),
	)
	worktree := newIdentityFS(
		ctx,
		osfs.New(req.TargetPath),
		req.Actor,
		c.fileNodeStore,
	)

	if _, err := gogit.CloneContext(ctx, gitStorage, worktree, options); err != nil {
		return domaingit.CloneResult{}, err
	}

	return domaingit.CloneResult{
		RepoURL: req.RepoURL,
		Path:    req.TargetPath,
		Branch:  branch,
	}, nil
}

func normalizeBranchName(branch string) string {
	branch = strings.TrimSpace(branch)
	branch = strings.TrimPrefix(branch, "refs/heads/")
	return branch
}

func prepareCloneTarget(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return os.MkdirAll(path, 0o755)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("clone target is not a directory: %s", path)
	}

	empty, err := isDirEmpty(path)
	if err != nil {
		return err
	}
	if !empty {
		return fmt.Errorf("clone target directory is not empty: %s", path)
	}
	return nil
}

func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = f.Close()
	}()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, nil
}
