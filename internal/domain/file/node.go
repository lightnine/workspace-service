package file

import "context"

// NodeActor identifies who performed a file operation within a tenant workspace.
type NodeActor struct {
	OwnerUIN    string
	UIN         string
	AppID       string
	WorkspaceID string
}

type NodeMutation struct {
	InodeID  uint64
	Actor    NodeActor
	NodeType string // file | directory | git_folder
}

// NodeRecord is the persisted ownership row for a JuiceFS inode.
type NodeRecord struct {
	InodeID     uint64
	OwnerUIN    string
	UIN         string
	AppID       string
	WorkspaceID string
	NodeType    string
}

type NodeStore interface {
	UpsertCreatedOrUpdated(ctx context.Context, mutation NodeMutation) error
	MarkDeleted(ctx context.Context, mutation NodeMutation) error
	LookupByInodeIDs(ctx context.Context, inodeIDs []uint64) (map[uint64]NodeRecord, error)
}

// NewNodeActor builds a NodeActor from request scope fields.
func NewNodeActor(ownerUIN, uin, appID, workspaceID string) NodeActor {
	return NodeActor{
		OwnerUIN:    ownerUIN,
		UIN:         uin,
		AppID:       appID,
		WorkspaceID: workspaceID,
	}
}
