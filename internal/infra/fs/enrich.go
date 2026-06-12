package fs

import (
	"context"
	"os"
	"path/filepath"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	domainfs "git.woa.com/leondli/workspace-service/internal/domain/fs"
)

func (c *WorkspaceFSClient) enrichFileInfo(ctx context.Context, actor domainfs.Actor, info domainfs.FileInfo) domainfs.FileInfo {
	if info.InodeID == 0 {
		if inodeID, ok := inodeFromPath(info.Path); ok {
			info.InodeID = inodeID
		}
	}
	if info.InodeID > 0 {
		if c.store != nil {
			records, err := c.store.LookupByInodeIDs(ctx, []uint64{info.InodeID})
			if err == nil {
				if rec, ok := records[info.InodeID]; ok {
					applyNodeRecord(&info, rec)
				}
			}
		}
	}
	if info.NodeType == "" {
		info.NodeType = domainfile.InferNodeTypeFromName(info.Name, info.IsDir)
	}
	if !info.IsGitFolder {
		info.IsGitFolder = info.NodeType == domainfile.NodeTypeGitFolder || isGitFolderPath(info.Path)
	}
	if c.isUnderTrash(actor, info.Path) {
		info.InRecycle = true
		if entry := filepath.Base(info.Path); entry != trashDirName && entry != "_meta" {
			if meta, err := c.readTrashMeta(actor, entry); err == nil {
				info.OriginPath = meta.OriginalPath
			}
		}
	}
	return info
}

func (c *WorkspaceFSClient) enrichMany(ctx context.Context, actor domainfs.Actor, files []domainfs.FileInfo) []domainfs.FileInfo {
	var inodeIDs []uint64
	for i := range files {
		if files[i].InodeID == 0 {
			if id, ok := inodeFromPath(files[i].Path); ok {
				files[i].InodeID = id
			}
		}
		if files[i].InodeID > 0 {
			inodeIDs = append(inodeIDs, files[i].InodeID)
		}
	}

	records := map[uint64]domainfile.NodeRecord{}
	if c.store != nil && len(inodeIDs) > 0 {
		if looked, err := c.store.LookupByInodeIDs(ctx, inodeIDs); err == nil {
			records = looked
		}
	}

	for i := range files {
		if rec, ok := records[files[i].InodeID]; ok {
			applyNodeRecord(&files[i], rec)
		}
		if files[i].NodeType == "" {
			files[i].NodeType = domainfile.InferNodeTypeFromName(files[i].Name, files[i].IsDir)
		}
		if !files[i].IsGitFolder {
			files[i].IsGitFolder = files[i].NodeType == domainfile.NodeTypeGitFolder ||
				isGitFolderPath(files[i].Path)
		}
		if c.isUnderTrash(actor, files[i].Path) {
			files[i].InRecycle = true
			entry := filepath.Base(files[i].Path)
			if meta, err := c.readTrashMeta(actor, entry); err == nil {
				files[i].OriginPath = meta.OriginalPath
			}
		}
	}
	return files
}

func applyNodeRecord(info *domainfs.FileInfo, rec domainfile.NodeRecord) {
	info.OwnerUIN = rec.OwnerUIN
	info.CreatorUIN = rec.UIN
	if rec.NodeType != "" {
		info.NodeType = rec.NodeType
		info.IsGitFolder = rec.NodeType == domainfile.NodeTypeGitFolder
	}
}

func inodeFromPath(absPath string) (uint64, bool) {
	info, err := os.Stat(absPath)
	if err != nil {
		return 0, false
	}
	return inodeFromFileInfo(info)
}

func isGitFolderPath(absPath string) bool {
	_, err := os.Stat(filepath.Join(absPath, ".git"))
	return err == nil
}
