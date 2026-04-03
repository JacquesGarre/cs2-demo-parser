package usecases

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
	"github.com/jcqsg/cs2-demos/backend/internal/infrastructure/persistence/memory"
	"github.com/jcqsg/cs2-demos/backend/internal/infrastructure/processing"
	"github.com/jcqsg/cs2-demos/backend/internal/infrastructure/storage/local"
)

func TestSubmitDemoUseCase_Execute_InvalidExtension(t *testing.T) {
	uc := SubmitDemoUseCase{}

	_, err := uc.Execute(strings.NewReader("data"), "invalid.txt")
	if err == nil {
		t.Fatalf("expected error for invalid extension")
	}

	if !strings.Contains(err.Error(), "invalid file type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubmitDemoUseCase_Execute_Success(t *testing.T) {
	demoRepo := memory.NewDemoRepository()
	jobRepo := memory.NewJobRepository()
	summaryRepo := memory.NewSummaryRepository()
	storage := local.NewDemoStorage(t.TempDir())
	queue := processing.NewAnalysisQueue(demoRepo, jobRepo, summaryRepo, processing.NewCS2DemoAnalyzer())

	uc := NewSubmitDemoUseCase(demoRepo, jobRepo, storage, queue)
	job, err := uc.Execute(bytes.NewBufferString("not-a-real-demo"), "match.dem")
	if err != nil {
		t.Fatalf("expected submit to succeed, got error: %v", err)
	}

	if job.ID == "" || job.DemoID == "" {
		t.Fatalf("expected generated IDs, got job=%#v", job)
	}

	if job.Status != entities.JobStatusQueued {
		t.Fatalf("expected queued status on submit response, got %q", job.Status)
	}

	storedDemo, found := demoRepo.GetByID(job.DemoID)
	if !found {
		t.Fatalf("expected stored demo to exist")
	}

	if storedDemo.FileName != "match.dem" {
		t.Fatalf("expected stored filename match.dem, got %q", storedDemo.FileName)
	}
}
