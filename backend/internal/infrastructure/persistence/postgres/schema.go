package postgres

import "database/sql"

func EnsureSchema(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS demos (
			id TEXT PRIMARY KEY,
			file_name TEXT NOT NULL,
			storage_path TEXT NOT NULL,
			uploaded_at TIMESTAMPTZ NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS analysis_jobs (
			id TEXT PRIMARY KEY,
			demo_id TEXT NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			error TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_analysis_jobs_demo_id ON analysis_jobs(demo_id);`,
		`CREATE TABLE IF NOT EXISTS match_summaries (
			demo_id TEXT PRIMARY KEY REFERENCES demos(id) ON DELETE CASCADE,
			payload JSONB NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		);`,
	}

	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}

	return nil
}
