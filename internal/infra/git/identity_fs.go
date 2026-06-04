package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	domaingit "git.woa.com/leondli/workspace-service/internal/domain/git"
	"github.com/go-git/go-billy/v5"
)

type identityFS struct {
	billy.Filesystem
	ctx   context.Context
	actor domaingit.Actor
	store domainfile.NodeStore
}

func newIdentityFS(
	ctx context.Context,
	base billy.Filesystem,
	actor domaingit.Actor,
	store domainfile.NodeStore,
) *identityFS {
	return &identityFS{
		Filesystem: base,
		ctx:        ctx,
		actor:      actor,
		store:      store,
	}
}

func (fs *identityFS) Create(path string) (billy.File, error) {
	f, err := fs.Filesystem.Create(path)
	if err != nil {
		return nil, err
	}
	_ = fs.recordCreatedOrUpdated(path)
	return &identityFile{File: f, fs: fs, path: path, dirty: true}, nil
}

func (fs *identityFS) OpenFile(path string, flag int, perm os.FileMode) (billy.File, error) {
	f, err := fs.Filesystem.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}

	dirty := flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_TRUNC|os.O_CREATE) != 0
	if dirty && flag&os.O_CREATE != 0 {
		_ = fs.recordCreatedOrUpdated(path)
	}
	return &identityFile{File: f, fs: fs, path: path, dirty: dirty}, nil
}

func (fs *identityFS) Rename(oldPath, newPath string) error {
	info, statErr := fs.Filesystem.Stat(oldPath)
	if err := fs.Filesystem.Rename(oldPath, newPath); err != nil {
		return err
	}
	if statErr == nil {
		_ = fs.upsert(newPath, info)
	}
	return nil
}

func (fs *identityFS) Remove(path string) error {
	info, statErr := fs.Filesystem.Stat(path)
	if err := fs.Filesystem.Remove(path); err != nil {
		return err
	}
	if statErr == nil {
		_ = fs.markDeleted(path, info)
	}
	return nil
}

func (fs *identityFS) MkdirAll(path string, perm os.FileMode) error {
	if err := fs.Filesystem.MkdirAll(path, perm); err != nil {
		return err
	}
	_ = fs.recordCreatedOrUpdated(path)
	return nil
}

func (fs *identityFS) recordCreatedOrUpdated(path string) error {
	if fs.shouldSkip(path) || fs.store == nil {
		return nil
	}
	info, err := fs.Filesystem.Stat(path)
	if err != nil {
		return err
	}
	return fs.upsert(path, info)
}

func (fs *identityFS) upsert(path string, info os.FileInfo) error {
	if fs.shouldSkip(path) || fs.store == nil {
		return nil
	}
	inodeID, ok := inodeFromFileInfo(info)
	if !ok {
		return nil
	}
	return fs.store.UpsertCreatedOrUpdated(fs.ctx, domainfile.NodeMutation{
		InodeID: inodeID,
		Actor: domainfile.NewNodeActor(
			fs.actor.OwnerUIN, fs.actor.UIN, fs.actor.AppID, fs.actor.WorkspaceID,
		),
		NodeType: domainfile.NodeTypeFromDir(info.IsDir()),
	})
}

func (fs *identityFS) markDeleted(path string, info os.FileInfo) error {
	if fs.shouldSkip(path) || fs.store == nil {
		return nil
	}
	inodeID, ok := inodeFromFileInfo(info)
	if !ok {
		return nil
	}
	return fs.store.MarkDeleted(fs.ctx, domainfile.NodeMutation{
		InodeID: inodeID,
		Actor: domainfile.NewNodeActor(
			fs.actor.OwnerUIN, fs.actor.UIN, fs.actor.AppID, fs.actor.WorkspaceID,
		),
	})
}

func (fs *identityFS) shouldSkip(path string) bool {
	cleanPath := filepath.ToSlash(filepath.Clean(path))
	return cleanPath == ".git" || strings.HasPrefix(cleanPath, ".git/")
}

func inodeFromFileInfo(info os.FileInfo) (uint64, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(stat.Ino), true
}

type identityFile struct {
	billy.File
	fs    *identityFS
	path  string
	dirty bool
}

func (f *identityFile) Write(p []byte) (int, error) {
	n, err := f.File.Write(p)
	if n > 0 {
		f.dirty = true
	}
	return n, err
}

func (f *identityFile) Truncate(size int64) error {
	if err := f.File.Truncate(size); err != nil {
		return err
	}
	f.dirty = true
	return nil
}

func (f *identityFile) Close() error {
	err := f.File.Close()
	if err == nil && f.dirty {
		_ = f.fs.recordCreatedOrUpdated(f.path)
	}
	return err
}
