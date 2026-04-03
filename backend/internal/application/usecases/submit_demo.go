package usecases

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/jcqsg/cs2-demos/backend/internal/application/ports"
	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
	"github.com/jcqsg/cs2-demos/backend/internal/domain/repositories"
	"github.com/jcqsg/cs2-demos/backend/internal/domain/valueobjects"
)

type SubmitDemoUseCase struct {
	demoRepository repositories.DemoRepository
	jobRepository  repositories.JobRepository
	storage        ports.DemoStorage
	queue          ports.AnalysisQueue
}

func NewSubmitDemoUseCase(
	demoRepository repositories.DemoRepository,
	jobRepository repositories.JobRepository,
	storage ports.DemoStorage,
	queue ports.AnalysisQueue,
) SubmitDemoUseCase {
	return SubmitDemoUseCase{
		demoRepository: demoRepository,
		jobRepository:  jobRepository,
		storage:        storage,
		queue:          queue,
	}
}

func (uc SubmitDemoUseCase) Execute(file io.Reader, fileName string) (entities.AnalysisJob, error) {
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext != ".dem" {
		return entities.AnalysisJob{}, fmt.Errorf("invalid file type: expected .dem")
	}

	demoID := valueobjects.NewID()
	jobID := valueobjects.NewID()

	storagePath, err := uc.storage.Save(file, fileName)
	if err != nil {
		return entities.AnalysisJob{}, fmt.Errorf("failed to store demo: %w", err)
	}

	demo := entities.Demo{
		ID:          demoID,
		FileName:    fileName,
		StoragePath: storagePath,
		UploadedAt:  time.Now().UTC(),
	}
	if err := uc.demoRepository.Save(demo); err != nil {
		return entities.AnalysisJob{}, fmt.Errorf("failed to save demo: %w", err)
	}

	job := entities.AnalysisJob{
		ID:        jobID,
		DemoID:    demoID,
		Status:    entities.JobStatusQueued,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := uc.jobRepository.Save(job); err != nil {
		return entities.AnalysisJob{}, fmt.Errorf("failed to save job: %w", err)
	}

	if err := uc.queue.Enqueue(job.ID); err != nil {
		job.Status = entities.JobStatusFailed
		job.Error = err.Error()
		job.UpdatedAt = time.Now().UTC()
		_ = uc.jobRepository.Update(job)
		return job, fmt.Errorf("failed to enqueue job: %w", err)
	}

	return job, nil
}
