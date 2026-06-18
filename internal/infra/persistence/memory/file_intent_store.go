package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	domainfile "git.woa.com/leondli/workspace-service/internal/domain/file"
)

// FileIntentStore is an in-memory IntentStore for unit tests.
type FileIntentStore struct {
	mu      sync.Mutex
	seq     int64
	order   map[string]int64
	intents map[string]domainfile.Intent

	// EnqueueErr, when set, is returned by Enqueue to simulate a durable-write
	// failure (used to assert the orchestrator's behavior).
	EnqueueErr error
}

func NewFileIntentStore() *FileIntentStore {
	return &FileIntentStore{
		order:   make(map[string]int64),
		intents: make(map[string]domainfile.Intent),
	}
}

func (s *FileIntentStore) Enqueue(_ context.Context, intent domainfile.Intent) error {
	if s.EnqueueErr != nil {
		return s.EnqueueErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	intent.Status = domainfile.IntentPending
	intent.LastError = ""
	if _, ok := s.intents[intent.ID]; !ok {
		s.seq++
		s.order[intent.ID] = s.seq
		intent.CreatedAt = now
	}
	intent.UpdatedAt = now
	s.intents[intent.ID] = intent
	return nil
}

func (s *FileIntentStore) MarkDone(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if intent, ok := s.intents[id]; ok {
		intent.Status = domainfile.IntentDone
		intent.UpdatedAt = time.Now()
		s.intents[id] = intent
	}
	return nil
}

func (s *FileIntentStore) MarkAborted(_ context.Context, id, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if intent, ok := s.intents[id]; ok {
		intent.Status = domainfile.IntentAborted
		intent.LastError = reason
		intent.UpdatedAt = time.Now()
		s.intents[id] = intent
	}
	return nil
}

func (s *FileIntentStore) IncrementAttempt(_ context.Context, id, lastErr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if intent, ok := s.intents[id]; ok {
		intent.Attempts++
		intent.LastError = lastErr
		intent.UpdatedAt = time.Now()
		s.intents[id] = intent
	}
	return nil
}

func (s *FileIntentStore) ListPending(_ context.Context, limit int) ([]domainfile.Intent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var pending []domainfile.Intent
	for _, intent := range s.intents {
		if intent.Status == domainfile.IntentPending {
			pending = append(pending, intent)
		}
	}
	sort.Slice(pending, func(i, j int) bool {
		return s.order[pending[i].ID] < s.order[pending[j].ID]
	})
	if limit > 0 && len(pending) > limit {
		pending = pending[:limit]
	}
	return pending, nil
}

// Get exposes a stored intent for assertions in tests.
func (s *FileIntentStore) Get(id string) (domainfile.Intent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	intent, ok := s.intents[id]
	return intent, ok
}
