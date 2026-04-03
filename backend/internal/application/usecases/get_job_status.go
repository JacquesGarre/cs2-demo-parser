package usecases

import (
	"fmt"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
	"github.com/jcqsg/cs2-demos/backend/internal/domain/repositories"
)

type GetJobStatusUseCase struct {
	jobRepository repositories.JobRepository
}

func NewGetJobStatusUseCase(jobRepository repositories.JobRepository) GetJobStatusUseCase {
	return GetJobStatusUseCase{jobRepository: jobRepository}
}

func (uc GetJobStatusUseCase) Execute(jobID string) (entities.AnalysisJob, error) {
	job, found := uc.jobRepository.GetByID(jobID)
	if !found {
		return entities.AnalysisJob{}, fmt.Errorf("job not found")
	}

	return job, nil
}
