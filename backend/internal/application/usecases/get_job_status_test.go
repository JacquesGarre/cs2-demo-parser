package usecases

import (
	"strings"
	"testing"
	"time"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
	"github.com/jcqsg/cs2-demos/backend/internal/infrastructure/persistence/memory"
)

func TestGetJobStatusUseCase_Execute_Success(t *testing.T) {
	jobRepo := memory.NewJobRepository()
	job := entities.AnalysisJob{
		ID:        "job-1",
		DemoID:    "demo-1",
		Status:    entities.JobStatusQueued,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := jobRepo.Save(job); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	uc := NewGetJobStatusUseCase(jobRepo)
	got, err := uc.Execute("job-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if got.ID != "job-1" {
		t.Fatalf("expected job id job-1, got %q", got.ID)
	}
}

func TestGetJobStatusUseCase_Execute_NotFound(t *testing.T) {
	uc := NewGetJobStatusUseCase(memory.NewJobRepository())

	_, err := uc.Execute("missing")
	if err == nil {
		t.Fatalf("expected error for missing job")
	}

	if !strings.Contains(err.Error(), "job not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
