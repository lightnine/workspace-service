package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveGitDir returns the absolute path to the repository's .git directory.
// When gitMetaRoot is set, metadata is stored outside the JuiceFS worktree mount.
func resolveGitDir(mountRoot, gitMetaRoot, worktreePath string) (string, error) {
	worktreePath = filepath.Clean(worktreePath)
	if strings.TrimSpace(gitMetaRoot) == "" {
		return filepath.Join(worktreePath, ".git"), nil
	}

	mountRoot = filepath.Clean(mountRoot)
	gitMetaRoot = filepath.Clean(gitMetaRoot)
	if mountRoot == "" {
		return "", fmt.Errorf("workspace mount root is required when git_meta_root is set")
	}

	rel, err := filepath.Rel(mountRoot, worktreePath)
	if err != nil {
		return "", fmt.Errorf("worktree path relative to mount root: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("worktree path escapes workspace mount root")
	}

	gitDir := filepath.Join(gitMetaRoot, rel, ".git")
	return gitDir, nil
}

func ensureGitDirParent(gitDir string) error {
	if err := os.MkdirAll(filepath.Dir(gitDir), 0o755); err != nil {
		return fmt.Errorf("create git metadata parent dir: %w", err)
	}
	return nil
}

func isSubpath(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if parent == child {
		return true
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
