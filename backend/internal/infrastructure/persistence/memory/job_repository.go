package memory

import (
	"sync"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
)

type JobRepository struct {
	mu    sync.RWMutex
	items map[string]entities.AnalysisJob
}

func NewJobRepository() *JobRepository {
	return &JobRepository{items: map[string]entities.AnalysisJob{}}
}

func (r *JobRepository) Save(job entities.AnalysisJob) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[job.ID] = job
	return nil
}

func (r *JobRepository) Update(job entities.AnalysisJob) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[job.ID] = job
	return nil
}

func (r *JobRepository) GetByID(id string) (entities.AnalysisJob, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, found := r.items[id]
	return value, found
}
