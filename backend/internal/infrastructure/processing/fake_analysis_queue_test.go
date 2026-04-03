package processing

import (
	"testing"
	"time"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
	"github.com/jcqsg/cs2-demos/backend/internal/infrastructure/persistence/memory"
)

func TestAnalysisQueue_Enqueue_FullQueue(t *testing.T) {
	q := &AnalysisQueue{queue: make(chan string, 1)}

	if err := q.Enqueue("job-1"); err != nil {
		t.Fatalf("first enqueue should succeed, got %v", err)
	}

	err := q.Enqueue("job-2")
	if err == nil {
		t.Fatalf("expected queue full error")
	}
}

func TestAnalysisQueue_Worker_FailsWhenDemoMissing(t *testing.T) {
	demoRepo := memory.NewDemoRepository()
	jobRepo := memory.NewJobRepository()
	summaryRepo := memory.NewSummaryRepository()

	job := entities.AnalysisJob{
		ID:     "job-1",
		DemoID: "missing-demo",
		Status: entities.JobStatusQueued,
	}
	if err := jobRepo.Save(job); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	queue := NewAnalysisQueue(demoRepo, jobRepo, summaryRepo, NewCS2DemoAnalyzer())
	if err := queue.Enqueue("job-1"); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		updated, found := jobRepo.GetByID("job-1")
		if found && updated.Status == entities.JobStatusFailed {
			if updated.Error != "demo not found" {
				t.Fatalf("expected demo not found error, got %q", updated.Error)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	updated, _ := jobRepo.GetByID("job-1")
	t.Fatalf("expected failed status within timeout, got status=%q error=%q", updated.Status, updated.Error)
}
