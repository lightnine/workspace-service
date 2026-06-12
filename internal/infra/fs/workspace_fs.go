package fs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	domainfs "git.woa.com/leondli/workspace-service/internal/domain/fs"
	"git.woa.com/leondli/workspace-service/internal/domain/notebook"
	"git.woa.com/leondli/workspace-service/pkg/ctxmeta"
)

const maxInlineReadSize = 32 * 1024 * 1024 // 32 MiB

// WorkspaceFSClient implements domain/fs.WorkspaceFS on the JuiceFS mount.
type WorkspaceFSClient struct {
	store     domainfile.NodeStore
	mountRoot string
}

func NewWorkspaceFSClient(store domainfile.NodeStore, mountRoot string) *WorkspaceFSClient {
	return &WorkspaceFSClient{store: store, mountRoot: cleanMountRoot(mountRoot)}
}

func (c *WorkspaceFSClient) CreateFolder(ctx context.Context, req domainfs.CreateFolderReq) (domainfs.FileInfo, error) {
	if err := os.MkdirAll(req.Path, 0o755); err != nil {
		return domainfs.FileInfo{}, fmt.Errorf("create folder: %w", err)
	}
	RecordInode(ctx, c.store, req.Actor, req.Path, domainfile.NodeTypeDirectory)
	return c.buildEnrichedFileInfo(ctx, req.Actor, req.Path)
}

func (c *WorkspaceFSClient) CreateFile(ctx context.Context, req domainfs.CreateFileReq) (domainfs.FileInfo, error) {
	if err := os.MkdirAll(filepath.Dir(req.Path), 0o755); err != nil {
		return domainfs.FileInfo{}, fmt.Errorf("create parent dir: %w", err)
	}

	if info, err := os.Stat(req.Path); err == nil {
		if info.IsDir() {
			return domainfs.FileInfo{}, domainfs.ErrNotAFile
		}
		if !req.Overwrite {
			return domainfs.FileInfo{}, domainfs.ErrAlreadyExists
		}
	} else if !os.IsNotExist(err) {
		return domainfs.FileInfo{}, err
	}

	if err := os.WriteFile(req.Path, req.Content, 0o644); err != nil {
		return domainfs.FileInfo{}, fmt.Errorf("write file: %w", err)
	}
	recordUpsert(ctx, c.store, req.Actor, req.Path)
	return c.buildEnrichedFileInfo(ctx, req.Actor, req.Path)
}

func (c *WorkspaceFSClient) CreateNotebook(ctx context.Context, req domainfs.CreateNotebookReq) (domainfs.FileInfo, error) {
	content := notebook.DefaultNotebook(req.KernelName)
	if _, err := c.CreateFile(ctx, domainfs.CreateFileReq{
		Actor: req.Actor, Path: req.Path, Content: content, Overwrite: req.Overwrite,
	}); err != nil {
		return domainfs.FileInfo{}, err
	}
	RecordInode(ctx, c.store, req.Actor, req.Path, domainfile.NodeTypeNotebook)
	info, err := c.buildEnrichedFileInfo(ctx, req.Actor, req.Path)
	if err != nil {
		return domainfs.FileInfo{}, err
	}
	info.NodeType = domainfile.NodeTypeNotebook
	return info, nil
}

func (c *WorkspaceFSClient) DeletePath(ctx context.Context, req domainfs.DeletePathReq) error {
	info, err := os.Stat(req.Path)
	if os.IsNotExist(err) {
		return domainfs.ErrNotFound
	}
	if err != nil {
		return err
	}

	if req.SoftDelete && !req.Permanent {
		return c.softDelete(ctx, req.Actor, req.Path, info)
	}

	if inodeID, ok := inodeFromFileInfo(info); ok && c.store != nil {
		_ = c.store.MarkDeleted(ctx, domainfile.NodeMutation{
			InodeID: inodeID,
			Actor:   nodeActor(req.Actor),
		})
	}

	if info.IsDir() {
		if err := os.RemoveAll(req.Path); err != nil {
			return fmt.Errorf("delete folder: %w", err)
		}
		return nil
	}
	if err := os.Remove(req.Path); err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

func (c *WorkspaceFSClient) softDelete(ctx context.Context, actor domainfs.Actor, absPath string, info os.FileInfo) error {
	userRel, err := c.userRelPath(actor, absPath)
	if err != nil {
		return domainfs.ErrInvalidRequest
	}
	if userRel == "" || userRel == trashDirName || strings.HasPrefix(userRel, trashDirName+"/") {
		return domainfs.ErrInvalidRequest
	}

	entryName := trashEntryName(userRel)
	trashDest := filepath.Join(c.userTrashDirAbs(actor), entryName)
	if err := os.MkdirAll(c.userTrashDirAbs(actor), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(trashDest); err == nil {
		return domainfs.ErrAlreadyExists
	}
	if err := os.Rename(absPath, trashDest); err != nil {
		return fmt.Errorf("move to trash: %w", err)
	}
	if err := c.writeTrashMeta(actor, entryName, userRel); err != nil {
		_ = os.Rename(trashDest, absPath)
		return err
	}
	recordUpsert(ctx, c.store, actor, trashDest)
	return nil
}

func (c *WorkspaceFSClient) MovePath(ctx context.Context, req domainfs.MovePathReq) (domainfs.FileInfo, error) {
	return c.renameOrMove(ctx, req.Actor, req.SrcPath, req.DestPath, req.Overwrite)
}

func (c *WorkspaceFSClient) RenamePath(ctx context.Context, req domainfs.RenamePathReq) (domainfs.FileInfo, error) {
	dest := filepath.Join(filepath.Dir(req.Path), req.NewName)
	return c.renameOrMove(ctx, req.Actor, req.Path, dest, req.Overwrite)
}

func (c *WorkspaceFSClient) renameOrMove(ctx context.Context, actor domainfs.Actor, src, dest string, overwrite bool) (domainfs.FileInfo, error) {
	srcInfo, err := os.Stat(src)
	if os.IsNotExist(err) {
		return domainfs.FileInfo{}, domainfs.ErrNotFound
	}
	if err != nil {
		return domainfs.FileInfo{}, err
	}

	if samePath(src, dest) {
		return c.buildEnrichedFileInfo(ctx, actor, src)
	}

	if destInfo, err := os.Stat(dest); err == nil {
		if srcInfo.IsDir() != destInfo.IsDir() {
			return domainfs.FileInfo{}, domainfs.ErrInvalidRequest
		}
		if !overwrite {
			return domainfs.FileInfo{}, domainfs.ErrAlreadyExists
		}
	} else if !os.IsNotExist(err) {
		return domainfs.FileInfo{}, err
	} else if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return domainfs.FileInfo{}, err
	}

	if inodeID, ok := inodeFromFileInfo(srcInfo); ok && c.store != nil {
		_ = c.store.MarkDeleted(ctx, domainfile.NodeMutation{
			InodeID: inodeID,
			Actor:   nodeActor(actor),
		})
	}

	if err := os.Rename(src, dest); err != nil {
		return domainfs.FileInfo{}, fmt.Errorf("move: %w", err)
	}
	recordUpsert(ctx, c.store, actor, dest)
	return c.buildEnrichedFileInfo(ctx, actor, dest)
}

func (c *WorkspaceFSClient) CopyPath(ctx context.Context, req domainfs.CopyPathReq) (domainfs.FileInfo, error) {
	srcInfo, err := os.Stat(req.SrcPath)
	if os.IsNotExist(err) {
		return domainfs.FileInfo{}, domainfs.ErrNotFound
	}
	if err != nil {
		return domainfs.FileInfo{}, err
	}

	if destInfo, err := os.Stat(req.DestPath); err == nil {
		if srcInfo.IsDir() != destInfo.IsDir() {
			return domainfs.FileInfo{}, domainfs.ErrInvalidRequest
		}
		if !req.Overwrite {
			return domainfs.FileInfo{}, domainfs.ErrAlreadyExists
		}
	} else if !os.IsNotExist(err) {
		return domainfs.FileInfo{}, err
	} else if err := os.MkdirAll(filepath.Dir(req.DestPath), 0o755); err != nil {
		return domainfs.FileInfo{}, err
	}

	if srcInfo.IsDir() {
		if err := copyDir(req.SrcPath, req.DestPath); err != nil {
			return domainfs.FileInfo{}, err
		}
	} else {
		if err := copyFile(req.SrcPath, req.DestPath); err != nil {
			return domainfs.FileInfo{}, err
		}
	}
	recordUpsert(ctx, c.store, req.Actor, req.DestPath)
	return c.buildEnrichedFileInfo(ctx, req.Actor, req.DestPath)
}

func (c *WorkspaceFSClient) ListFiles(ctx context.Context, req domainfs.ListFilesReq) (domainfs.ListFilesResult, error) {
	info, err := os.Stat(req.Path)
	if os.IsNotExist(err) {
		return domainfs.ListFilesResult{}, domainfs.ErrNotFound
	}
	if err != nil {
		return domainfs.ListFilesResult{}, err
	}
	if !info.IsDir() {
		return domainfs.ListFilesResult{}, domainfs.ErrNotADirectory
	}

	var files []domainfs.FileInfo
	if req.Recursive {
		err = filepath.WalkDir(req.Path, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == req.Path {
				return nil
			}
			fi, err := c.buildFileInfo(path)
			if err != nil {
				return err
			}
			files = append(files, fi)
			return nil
		})
	} else {
		entries, err := os.ReadDir(req.Path)
		if err != nil {
			return domainfs.ListFilesResult{}, err
		}
		for _, entry := range entries {
			if skipListEntry(entry.Name()) {
				continue
			}
			entryPath := filepath.Join(req.Path, entry.Name())
			fi, err := c.buildFileInfo(entryPath)
			if err != nil {
				continue
			}
			files = append(files, fi)
		}
	}
	if err != nil {
		return domainfs.ListFilesResult{}, err
	}
	files = c.enrichMany(ctx, req.Actor, files)
	return domainfs.ListFilesResult{Files: files}, nil
}

func (c *WorkspaceFSClient) GetFileInfo(ctx context.Context, absPath string) (domainfs.FileInfo, error) {
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return domainfs.FileInfo{}, domainfs.ErrNotFound
	} else if err != nil {
		return domainfs.FileInfo{}, err
	}
	actor, _ := ctxmeta.RequestContextFrom(ctx)
	return c.buildEnrichedFileInfo(ctx, actor, absPath)
}

func (c *WorkspaceFSClient) ReadFile(ctx context.Context, req domainfs.ReadFileReq) (domainfs.ReadFileResult, error) {
	info, err := os.Stat(req.Path)
	if os.IsNotExist(err) {
		return domainfs.ReadFileResult{}, domainfs.ErrNotFound
	}
	if err != nil {
		return domainfs.ReadFileResult{}, err
	}
	if info.IsDir() {
		return domainfs.ReadFileResult{}, domainfs.ErrNotAFile
	}
	if info.Size() > maxInlineReadSize {
		return domainfs.ReadFileResult{}, domainfs.ErrFileTooLarge
	}

	content, err := os.ReadFile(req.Path)
	if err != nil {
		return domainfs.ReadFileResult{}, err
	}

	fi, err := c.buildEnrichedFileInfo(ctx, req.Actor, req.Path)
	if err != nil {
		return domainfs.ReadFileResult{}, err
	}
	return domainfs.ReadFileResult{Info: fi, Content: content}, nil
}

func (c *WorkspaceFSClient) WriteFile(ctx context.Context, req domainfs.WriteFileReq) (domainfs.FileInfo, error) {
	return c.CreateFile(ctx, domainfs.CreateFileReq{
		Actor:     req.Actor,
		Path:      req.Path,
		Content:   req.Content,
		Overwrite: req.Overwrite,
	})
}

func (c *WorkspaceFSClient) ValidatePath(_ context.Context, req domainfs.ValidatePathReq) (domainfs.ValidatePathResult, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return domainfs.ValidatePathResult{}, domainfs.ErrInvalidRequest
	}

	parent := strings.TrimSpace(req.ParentPath)
	if parent == "" || parent == "." {
		parent = c.userRootAbs(req.Actor)
	}
	target := filepath.Join(parent, name)

	if _, err := os.Stat(target); os.IsNotExist(err) {
		return domainfs.ValidatePathResult{Exists: false}, nil
	} else if err != nil {
		return domainfs.ValidatePathResult{}, err
	}
	return domainfs.ValidatePathResult{Exists: true}, nil
}

func (c *WorkspaceFSClient) GetFolderNodePath(ctx context.Context, req domainfs.GetFolderNodePathReq) (domainfs.GetFolderNodePathResult, error) {
	targetAbs := filepath.Clean(req.Path)
	info, err := os.Stat(targetAbs)
	if os.IsNotExist(err) {
		return domainfs.GetFolderNodePathResult{}, domainfs.ErrNotFound
	}
	if err != nil {
		return domainfs.GetFolderNodePathResult{}, err
	}
	if !info.IsDir() {
		return domainfs.GetFolderNodePathResult{}, domainfs.ErrNotADirectory
	}

	userRoot := c.userRootAbs(req.Actor)
	var nodes []domainfs.FileInfo
	current := targetAbs
	for {
		fi, err := c.buildFileInfo(current)
		if err != nil {
			return domainfs.GetFolderNodePathResult{}, err
		}
		fi = c.enrichFileInfo(ctx, req.Actor, fi)
		nodes = append([]domainfs.FileInfo{fi}, nodes...)
		if current == userRoot {
			break
		}
		parent := filepath.Dir(current)
		if parent == current || !strings.HasPrefix(parent, userRoot) {
			break
		}
		current = parent
	}
	return domainfs.GetFolderNodePathResult{Nodes: nodes}, nil
}

func (c *WorkspaceFSClient) ListRecycleBin(ctx context.Context, req domainfs.ListRecycleBinReq) (domainfs.ListRecycleBinResult, error) {
	trashPath := c.userTrashDirAbs(req.Actor)
	if err := os.MkdirAll(trashPath, 0o755); err != nil {
		return domainfs.ListRecycleBinResult{}, err
	}
	result, err := c.ListFiles(ctx, domainfs.ListFilesReq{Actor: req.Actor, Path: trashPath, Recursive: false})
	return domainfs.ListRecycleBinResult{Files: result.Files}, err
}

func (c *WorkspaceFSClient) RestorePath(ctx context.Context, req domainfs.RestorePathReq) (domainfs.FileInfo, error) {
	trashAbs := filepath.Clean(req.TrashPath)
	if !c.isUnderTrash(req.Actor, trashAbs) {
		return domainfs.FileInfo{}, domainfs.ErrInvalidRequest
	}

	entryName := filepath.Base(trashAbs)
	meta, err := c.readTrashMeta(req.Actor, entryName)
	if err != nil {
		return domainfs.FileInfo{}, domainfs.ErrNotFound
	}

	original := meta.OriginalPath
	if parent := strings.TrimSpace(req.TargetParent); parent != "" {
		original = filepath.ToSlash(filepath.Join(parent, filepath.Base(original)))
	}

	destAbs := filepath.Join(c.userRootAbs(req.Actor), filepath.FromSlash(original))
	if err := os.MkdirAll(filepath.Dir(destAbs), 0o755); err != nil {
		return domainfs.FileInfo{}, err
	}
	if _, err := os.Stat(destAbs); err == nil {
		return domainfs.FileInfo{}, domainfs.ErrAlreadyExists
	}
	if err := os.Rename(trashAbs, destAbs); err != nil {
		return domainfs.FileInfo{}, fmt.Errorf("restore: %w", err)
	}
	_ = c.removeTrashMeta(req.Actor, entryName)
	recordUpsert(ctx, c.store, req.Actor, destAbs)
	return c.buildEnrichedFileInfo(ctx, req.Actor, destAbs)
}

func (c *WorkspaceFSClient) EmptyRecycleBin(ctx context.Context, req domainfs.EmptyRecycleBinReq) error {
	trashPath := c.userTrashDirAbs(req.Actor)
	entries, err := os.ReadDir(trashPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "_meta" {
			continue
		}
		entryPath := filepath.Join(trashPath, entry.Name())
		if err := c.DeletePath(ctx, domainfs.DeletePathReq{
			Actor: req.Actor, Path: entryPath, Permanent: true,
		}); err != nil {
			return err
		}
		_ = c.removeTrashMeta(req.Actor, entry.Name())
	}
	_ = os.RemoveAll(c.userTrashMetaDirAbs(req.Actor))
	return nil
}

func (c *WorkspaceFSClient) userRelPath(actor domainfs.Actor, absPath string) (string, error) {
	clean := filepath.Clean(absPath)
	root := c.userRootAbs(actor)
	if clean == root {
		return "", nil
	}
	suffix := root + string(filepath.Separator)
	if strings.HasPrefix(clean, suffix) {
		return filepath.ToSlash(strings.TrimPrefix(clean, suffix)), nil
	}
	return "", fmt.Errorf("path not under user prefix")
}

func (c *WorkspaceFSClient) buildFileInfo(absPath string) (domainfs.FileInfo, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return domainfs.FileInfo{}, err
	}
	fi := domainfs.FileInfo{
		Name:       info.Name(),
		Path:       absPath,
		IsDir:      info.IsDir(),
		Size:       info.Size(),
		ModifyTime: info.ModTime(),
	}
	if inodeID, ok := inodeFromFileInfo(info); ok {
		fi.InodeID = inodeID
	}
	return fi, nil
}

func (c *WorkspaceFSClient) buildEnrichedFileInfo(ctx context.Context, actor domainfs.Actor, absPath string) (domainfs.FileInfo, error) {
	fi, err := c.buildFileInfo(absPath)
	if err != nil {
		return domainfs.FileInfo{}, err
	}
	return c.enrichFileInfo(ctx, actor, fi), nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
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

var _ domainfs.WorkspaceFS = (*WorkspaceFSClient)(nil)
