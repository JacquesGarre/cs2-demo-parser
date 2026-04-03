package memory

import (
	"testing"
	"time"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
)

func TestJobRepository_SaveUpdateAndGetByID(t *testing.T) {
	repo := NewJobRepository()
	job := entities.AnalysisJob{
		ID:        "job-1",
		DemoID:    "demo-1",
		Status:    entities.JobStatusQueued,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := repo.Save(job); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	job.Status = entities.JobStatusProcessing
	if err := repo.Update(job); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, found := repo.GetByID("job-1")
	if !found {
		t.Fatalf("expected job to be found")
	}

	if got.Status != entities.JobStatusProcessing {
		t.Fatalf("expected updated status, got %q", got.Status)
	}
}

func TestJobRepository_GetByID_NotFound(t *testing.T) {
	repo := NewJobRepository()

	_, found := repo.GetByID("missing")
	if found {
		t.Fatalf("expected missing job to not be found")
	}
}
