package memory

import (
	"context"
	"sync"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
)

// FileNodeStore is an in-memory NodeStore for unit tests.
type FileNodeStore struct {
	mu    sync.RWMutex
	nodes map[uint64]domainfile.NodeRecord
}

func NewFileNodeStore() *FileNodeStore {
	return &FileNodeStore{nodes: make(map[uint64]domainfile.NodeRecord)}
}

func (s *FileNodeStore) UpsertCreatedOrUpdated(_ context.Context, mutation domainfile.NodeMutation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	nodeType := domainfile.NormalizeNodeType(mutation.NodeType)
	if nodeType == "" {
		nodeType = domainfile.NodeTypeFile
	}
	s.nodes[mutation.InodeID] = domainfile.NodeRecord{
		InodeID:     mutation.InodeID,
		OwnerUIN:    mutation.Actor.OwnerUIN,
		UIN:         mutation.Actor.UIN,
		AppID:       mutation.Actor.AppID,
		WorkspaceID: mutation.Actor.WorkspaceID,
		NodeType:    nodeType,
	}
	return nil
}

func (s *FileNodeStore) MarkDeleted(_ context.Context, mutation domainfile.NodeMutation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.nodes, mutation.InodeID)
	return nil
}

func (s *FileNodeStore) LookupByInodeIDs(_ context.Context, inodeIDs []uint64) (map[uint64]domainfile.NodeRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[uint64]domainfile.NodeRecord, len(inodeIDs))
	for _, id := range inodeIDs {
		if rec, ok := s.nodes[id]; ok {
			out[id] = rec
		}
	}
	return out, nil
}
