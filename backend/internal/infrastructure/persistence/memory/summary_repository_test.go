package memory

import (
	"testing"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
)

func TestSummaryRepository_SaveAndGetByDemoID(t *testing.T) {
	repo := NewSummaryRepository()
	summary := entities.MatchSummary{
		DemoID:  "demo-1",
		MapName: "de_inferno",
		Rounds:  24,
	}

	if err := repo.Save(summary); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	got, found := repo.GetByDemoID("demo-1")
	if !found {
		t.Fatalf("expected summary to be found")
	}

	if got.DemoID != summary.DemoID || got.MapName != summary.MapName || got.Rounds != summary.Rounds {
		t.Fatalf("unexpected summary returned: %#v", got)
	}
}

func TestSummaryRepository_GetByDemoID_NotFound(t *testing.T) {
	repo := NewSummaryRepository()

	_, found := repo.GetByDemoID("missing")
	if found {
		t.Fatalf("expected missing summary to not be found")
	}
}
