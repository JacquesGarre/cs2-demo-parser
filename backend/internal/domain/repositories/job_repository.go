package repositories

import "github.com/jcqsg/cs2-demos/backend/internal/domain/entities"

type JobRepository interface {
	Save(job entities.AnalysisJob) error
	Update(job entities.AnalysisJob) error
	GetByID(id string) (entities.AnalysisJob, bool)
}
