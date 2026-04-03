package repositories

import "github.com/jcqsg/cs2-demos/backend/internal/domain/entities"

type SummaryRepository interface {
	Save(summary entities.MatchSummary) error
	GetByDemoID(demoID string) (entities.MatchSummary, bool)
}
