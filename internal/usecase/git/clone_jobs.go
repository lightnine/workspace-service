package git

import (
	"sync"
)

const (
	GitFolderStatusUnspecified = 0
	GitFolderStatusWaiting     = 1
	GitFolderStatusCloning     = 2
	GitFolderStatusCheckingOut = 3
	GitFolderStatusReady       = 4
	GitFolderStatusFailed      = 5
)

type gitFolderJob struct {
	Status  int
	Message string
	RepoURL string
	Branch  string
	Path    string
}

type cloneJobRegistry struct {
	mu   sync.RWMutex
	jobs map[string]*gitFolderJob
}

func newCloneJobRegistry() *cloneJobRegistry {
	return &cloneJobRegistry{jobs: make(map[string]*gitFolderJob)}
}

func (r *cloneJobRegistry) set(key string, job *gitFolderJob) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[key] = job
}

func (r *cloneJobRegistry) get(key string) (gitFolderJob, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	job, ok := r.jobs[key]
	if !ok || job == nil {
		return gitFolderJob{}, false
	}
	return *job, true
}
