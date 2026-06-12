package fs

import (
	"context"
	"os"
	"syscall"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
	domainfs "git.woa.com/leondli/workspace-service/internal/domain/fs"
)

func nodeActor(actor domainfs.Actor) domainfile.NodeActor {
	return domainfile.NewNodeActor(actor.OwnerUIN, actor.UIN, actor.AppID, actor.WorkspaceID)
}

// RecordInode upserts ws_file_node for a path on the mount (used by FS and Git usecases).
func RecordInode(ctx context.Context, store domainfile.NodeStore, actor domainfs.Actor, absPath, nodeType string) {
	if store == nil {
		return
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return
	}
	inodeID, ok := inodeFromFileInfo(info)
	if !ok {
		return
	}
	if nodeType == "" {
		nodeType = domainfile.NodeTypeFromDir(info.IsDir())
	}
	_ = store.UpsertCreatedOrUpdated(ctx, domainfile.NodeMutation{
		InodeID:  inodeID,
		Actor:    nodeActor(actor),
		NodeType: nodeType,
	})
}

func recordUpsert(ctx context.Context, store domainfile.NodeStore, actor domainfs.Actor, absPath string) {
	RecordInode(ctx, store, actor, absPath, "")
}

func inodeFromFileInfo(info os.FileInfo) (uint64, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(stat.Ino), true
}
