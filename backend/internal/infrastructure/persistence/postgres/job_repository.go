package postgres

import (
	"database/sql"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
)

type JobRepository struct {
	db *sql.DB
}

func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) Save(job entities.AnalysisJob) error {
	_, err := r.db.Exec(
		`INSERT INTO analysis_jobs (id, demo_id, status, error, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		job.ID,
		job.DemoID,
		job.Status,
		job.Error,
		job.CreatedAt,
		job.UpdatedAt,
	)
	return err
}

func (r *JobRepository) Update(job entities.AnalysisJob) error {
	_, err := r.db.Exec(
		`UPDATE analysis_jobs SET status = $2, error = $3, updated_at = $4 WHERE id = $1`,
		job.ID,
		job.Status,
		job.Error,
		job.UpdatedAt,
	)
	return err
}

func (r *JobRepository) GetByID(id string) (entities.AnalysisJob, bool) {
	var job entities.AnalysisJob
	err := r.db.QueryRow(
		`SELECT id, demo_id, status, error, created_at, updated_at FROM analysis_jobs WHERE id = $1`,
		id,
	).Scan(&job.ID, &job.DemoID, &job.Status, &job.Error, &job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		return entities.AnalysisJob{}, false
	}

	return job, true
}
