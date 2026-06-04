package file

import "context"

type NodeActor struct {
	OwnerUIN string
	UIN      string
}

type NodeMutation struct {
	InodeID uint64
	Actor   NodeActor
}

type NodeStore interface {
	UpsertCreatedOrUpdated(ctx context.Context, mutation NodeMutation) error
	MarkDeleted(ctx context.Context, mutation NodeMutation) error
}
