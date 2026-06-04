package fs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	domainfs "git.woa.com/leondli/workspace-service/internal/domain/fs"
)

const trashDirName = "trash"

type trashMeta struct {
	OriginalPath string    `json:"original_path"`
	DeletedAt    time.Time `json:"deleted_at"`
}

func (c *WorkspaceFSClient) userRootAbs(actor domainfs.Actor) string {
	if c.mountRoot != "" {
		return filepath.Join(c.mountRoot, actor.UserPathPrefix())
	}
	return filepath.Clean(actor.UserPathPrefix())
}

func (c *WorkspaceFSClient) userTrashDirAbs(actor domainfs.Actor) string {
	return filepath.Join(c.userRootAbs(actor), trashDirName)
}

func (c *WorkspaceFSClient) userTrashMetaDirAbs(actor domainfs.Actor) string {
	return filepath.Join(c.userTrashDirAbs(actor), "_meta")
}

func trashEntryName(relPath string) string {
	return strings.ReplaceAll(relPath, "/", "__")
}

func (c *WorkspaceFSClient) trashMetaPath(actor domainfs.Actor, entryName string) string {
	return filepath.Join(c.userTrashMetaDirAbs(actor), entryName+".json")
}

func (c *WorkspaceFSClient) writeTrashMeta(actor domainfs.Actor, entryName, originalPath string) error {
	if err := os.MkdirAll(c.userTrashMetaDirAbs(actor), 0o755); err != nil {
		return err
	}
	payload, err := json.Marshal(trashMeta{
		OriginalPath: originalPath,
		DeletedAt:    time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	return os.WriteFile(c.trashMetaPath(actor, entryName), payload, 0o644)
}

func (c *WorkspaceFSClient) readTrashMeta(actor domainfs.Actor, entryName string) (trashMeta, error) {
	data, err := os.ReadFile(c.trashMetaPath(actor, entryName))
	if err != nil {
		return trashMeta{}, err
	}
	var meta trashMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return trashMeta{}, err
	}
	return meta, nil
}

func (c *WorkspaceFSClient) removeTrashMeta(actor domainfs.Actor, entryName string) error {
	err := os.Remove(c.trashMetaPath(actor, entryName))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (c *WorkspaceFSClient) isUnderTrash(actor domainfs.Actor, absPath string) bool {
	trash := c.userTrashDirAbs(actor)
	clean := filepath.Clean(absPath)
	return clean == trash || strings.HasPrefix(clean, trash+string(filepath.Separator))
}

func skipListEntry(name string) bool {
	return name == "_meta" || strings.HasPrefix(name, ".")
}
