package postgres

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
)

type SummaryRepository struct {
	db *sql.DB
}

func NewSummaryRepository(db *sql.DB) *SummaryRepository {
	return &SummaryRepository{db: db}
}

func (r *SummaryRepository) Save(summary entities.MatchSummary) error {
	payload, err := json.Marshal(summary)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(
		`INSERT INTO match_summaries (demo_id, payload, updated_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (demo_id) DO UPDATE SET payload = EXCLUDED.payload, updated_at = EXCLUDED.updated_at`,
		summary.DemoID,
		payload,
		time.Now().UTC(),
	)
	return err
}

func (r *SummaryRepository) GetByDemoID(demoID string) (entities.MatchSummary, bool) {
	var payload []byte
	err := r.db.QueryRow(`SELECT payload FROM match_summaries WHERE demo_id = $1`, demoID).Scan(&payload)
	if err != nil {
		return entities.MatchSummary{}, false
	}

	var summary entities.MatchSummary
	if err := json.Unmarshal(payload, &summary); err != nil {
		return entities.MatchSummary{}, false
	}

	return summary, true
}
