package usecases

import (
	"strings"
	"testing"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
	"github.com/jcqsg/cs2-demos/backend/internal/infrastructure/persistence/memory"
)

func TestGetMatchSummaryUseCase_Execute_Success(t *testing.T) {
	summaryRepo := memory.NewSummaryRepository()
	summary := entities.MatchSummary{
		DemoID:  "demo-1",
		MapName: "de_ancient",
		Rounds:  24,
	}
	if err := summaryRepo.Save(summary); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	uc := NewGetMatchSummaryUseCase(summaryRepo)
	got, err := uc.Execute("demo-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if got.DemoID != "demo-1" {
		t.Fatalf("expected demo id demo-1, got %q", got.DemoID)
	}
}

func TestGetMatchSummaryUseCase_Execute_NotFound(t *testing.T) {
	uc := NewGetMatchSummaryUseCase(memory.NewSummaryRepository())

	_, err := uc.Execute("missing")
	if err == nil {
		t.Fatalf("expected error for missing summary")
	}

	if !strings.Contains(err.Error(), "summary not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
