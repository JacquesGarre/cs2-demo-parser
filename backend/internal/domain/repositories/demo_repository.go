package repositories

import "github.com/jcqsg/cs2-demos/backend/internal/domain/entities"

type DemoRepository interface {
	Save(demo entities.Demo) error
	GetByID(id string) (entities.Demo, bool)
}
