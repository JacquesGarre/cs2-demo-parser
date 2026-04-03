package memory

import (
	"sync"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
)

type DemoRepository struct {
	mu    sync.RWMutex
	items map[string]entities.Demo
}

func NewDemoRepository() *DemoRepository {
	return &DemoRepository{items: map[string]entities.Demo{}}
}

func (r *DemoRepository) Save(demo entities.Demo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[demo.ID] = demo
	return nil
}

func (r *DemoRepository) GetByID(id string) (entities.Demo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, found := r.items[id]
	return value, found
}
