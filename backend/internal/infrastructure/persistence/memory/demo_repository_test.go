package memory

import (
	"testing"
	"time"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
)

func TestDemoRepository_SaveAndGetByID(t *testing.T) {
	repo := NewDemoRepository()
	demo := entities.Demo{
		ID:          "demo-1",
		FileName:    "match.dem",
		StoragePath: "/tmp/match.dem",
		UploadedAt:  time.Now().UTC(),
	}

	if err := repo.Save(demo); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	got, found := repo.GetByID("demo-1")
	if !found {
		t.Fatalf("expected demo to be found")
	}

	if got.ID != demo.ID || got.FileName != demo.FileName || got.StoragePath != demo.StoragePath {
		t.Fatalf("unexpected demo returned: %#v", got)
	}
}

func TestDemoRepository_GetByID_NotFound(t *testing.T) {
	repo := NewDemoRepository()

	_, found := repo.GetByID("missing")
	if found {
		t.Fatalf("expected missing demo to not be found")
	}
}
