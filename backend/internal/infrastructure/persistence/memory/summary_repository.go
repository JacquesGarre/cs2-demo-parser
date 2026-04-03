package memory

import (
	"sync"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
)

type SummaryRepository struct {
	mu    sync.RWMutex
	items map[string]entities.MatchSummary
}

func NewSummaryRepository() *SummaryRepository {
	return &SummaryRepository{items: map[string]entities.MatchSummary{}}
}

func (r *SummaryRepository) Save(summary entities.MatchSummary) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[summary.DemoID] = summary
	return nil
}

func (r *SummaryRepository) GetByDemoID(demoID string) (entities.MatchSummary, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, found := r.items[demoID]
	return value, found
}
