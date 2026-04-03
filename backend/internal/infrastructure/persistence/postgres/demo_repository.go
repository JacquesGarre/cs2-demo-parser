package postgres

import (
	"database/sql"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
)

type DemoRepository struct {
	db *sql.DB
}

func NewDemoRepository(db *sql.DB) *DemoRepository {
	return &DemoRepository{db: db}
}

func (r *DemoRepository) Save(demo entities.Demo) error {
	_, err := r.db.Exec(
		`INSERT INTO demos (id, file_name, storage_path, uploaded_at) VALUES ($1, $2, $3, $4)`,
		demo.ID,
		demo.FileName,
		demo.StoragePath,
		demo.UploadedAt,
	)
	return err
}

func (r *DemoRepository) GetByID(id string) (entities.Demo, bool) {
	var demo entities.Demo
	err := r.db.QueryRow(
		`SELECT id, file_name, storage_path, uploaded_at FROM demos WHERE id = $1`,
		id,
	).Scan(&demo.ID, &demo.FileName, &demo.StoragePath, &demo.UploadedAt)
	if err != nil {
		return entities.Demo{}, false
	}

	return demo, true
}
