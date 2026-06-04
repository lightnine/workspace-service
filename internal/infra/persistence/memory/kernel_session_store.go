package memory

import (
	"context"
	"errors"
	"sync"

	domainsession "git.woa.com/leondli/workspace-service/internal/domain/session"
)

// KernelSessionStore is an in-memory session store for tests.
type KernelSessionStore struct {
	mu       sync.RWMutex
	byID     map[string]domainsession.Record
	deleted  map[string]struct{}
}

func NewKernelSessionStore() *KernelSessionStore {
	return &KernelSessionStore{
		byID:    make(map[string]domainsession.Record),
		deleted: make(map[string]struct{}),
	}
}

func (s *KernelSessionStore) Upsert(_ context.Context, record domainsession.Record) error {
	if record.SessionID == "" {
		return errors.New("session_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.deleted, record.SessionID)
	s.byID[record.SessionID] = record
	return nil
}

func (s *KernelSessionStore) UpdateState(_ context.Context, sessionID, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.byID[sessionID]
	if !ok {
		return errors.New("session not found")
	}
	rec.State = state
	s.byID[sessionID] = rec
	return nil
}

func (s *KernelSessionStore) MarkDeleted(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted[sessionID] = struct{}{}
	delete(s.byID, sessionID)
	return nil
}

func (s *KernelSessionStore) GetBySessionID(_ context.Context, sessionID string) (domainsession.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.deleted[sessionID]; ok {
		return domainsession.Record{}, errors.New("session not found")
	}
	rec, ok := s.byID[sessionID]
	if !ok {
		return domainsession.Record{}, errors.New("session not found")
	}
	return rec, nil
}

func (s *KernelSessionStore) GetByKernelID(_ context.Context, kernelID string) (domainsession.Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for id, rec := range s.byID {
		if _, ok := s.deleted[id]; ok {
			continue
		}
		if rec.KernelID == kernelID {
			return rec, nil
		}
	}
	return domainsession.Record{}, errors.New("session not found")
}
