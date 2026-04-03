package processing

import (
	"fmt"
	"time"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
	"github.com/jcqsg/cs2-demos/backend/internal/domain/repositories"
)

type DemoAnalyzer interface {
	Analyze(demo entities.Demo) (entities.MatchSummary, error)
}

type AnalysisQueue struct {
	demoRepository    repositories.DemoRepository
	jobRepository     repositories.JobRepository
	summaryRepository repositories.SummaryRepository
	analyzer          DemoAnalyzer
	queue             chan string
}

func NewAnalysisQueue(
	demoRepository repositories.DemoRepository,
	jobRepository repositories.JobRepository,
	summaryRepository repositories.SummaryRepository,
	analyzer DemoAnalyzer,
) *AnalysisQueue {
	q := &AnalysisQueue{
		demoRepository:    demoRepository,
		jobRepository:     jobRepository,
		summaryRepository: summaryRepository,
		analyzer:          analyzer,
		queue:             make(chan string, 100),
	}

	go q.worker()
	return q
}

func NewFakeAnalysisQueue(
	demoRepository repositories.DemoRepository,
	jobRepository repositories.JobRepository,
	summaryRepository repositories.SummaryRepository,
) *AnalysisQueue {
	return NewAnalysisQueue(demoRepository, jobRepository, summaryRepository, NewCS2DemoAnalyzer())
}

func (q *AnalysisQueue) Enqueue(jobID string) error {
	select {
	case q.queue <- jobID:
		return nil
	default:
		return fmt.Errorf("analysis queue is full")
	}
}

func (q *AnalysisQueue) worker() {
	for jobID := range q.queue {
		job, found := q.jobRepository.GetByID(jobID)
		if !found {
			continue
		}

		job.Status = entities.JobStatusProcessing
		job.UpdatedAt = time.Now().UTC()
		_ = q.jobRepository.Update(job)

		demo, found := q.demoRepository.GetByID(job.DemoID)
		if !found {
			job.Status = entities.JobStatusFailed
			job.Error = "demo not found"
			job.UpdatedAt = time.Now().UTC()
			_ = q.jobRepository.Update(job)
			continue
		}

		summary, err := q.analyzer.Analyze(demo)
		if err != nil {
			job.Status = entities.JobStatusFailed
			job.Error = err.Error()
			job.UpdatedAt = time.Now().UTC()
			_ = q.jobRepository.Update(job)
			continue
		}

		if err := q.summaryRepository.Save(summary); err != nil {
			job.Status = entities.JobStatusFailed
			job.Error = err.Error()
			job.UpdatedAt = time.Now().UTC()
			_ = q.jobRepository.Update(job)
			continue
		}

		job.Status = entities.JobStatusCompleted
		job.Error = ""
		job.UpdatedAt = time.Now().UTC()
		_ = q.jobRepository.Update(job)
	}
}
