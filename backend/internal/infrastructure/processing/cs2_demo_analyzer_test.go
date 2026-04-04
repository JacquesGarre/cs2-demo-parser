package processing

import (
	"testing"

	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
)

func TestRoundEndReasonLabel(t *testing.T) {
	tests := []struct {
		name   string
		reason events.RoundEndReason
		want   string
	}{
		{name: "bomb exploded", reason: events.RoundEndReasonTargetBombed, want: "bomb exploded"},
		{name: "bomb defused", reason: events.RoundEndReasonBombDefused, want: "bomb defused"},
		{name: "target saved", reason: events.RoundEndReasonTargetSaved, want: "time expired"},
		{name: "terrorists win", reason: events.RoundEndReasonTerroristsWin, want: "no enemies remaining"},
		{name: "fallback", reason: events.RoundEndReasonStillInProgress, want: "won the round"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := roundEndReasonLabel(tc.reason, common.TeamCounterTerrorists); got != tc.want {
				t.Fatalf("roundEndReasonLabel(%v) = %q, want %q", tc.reason, got, tc.want)
			}
		})
	}
}

func TestAppendRoundEndEvent(t *testing.T) {
	state := &roundState{}

	appendRoundEndEvent(state, common.TeamCounterTerrorists, events.RoundEndReasonBombDefused, 999, "0:00")

	if len(state.RoundEvents) != 1 {
		t.Fatalf("expected 1 round event, got %d", len(state.RoundEvents))
	}

	event := state.RoundEvents[0]
	if event.EventType != "result" {
		t.Fatalf("expected result event type, got %q", event.EventType)
	}
	if event.Team != "CT" {
		t.Fatalf("expected winning team CT, got %q", event.Team)
	}
	if event.TimeLabel != "0:00" {
		t.Fatalf("expected time label 0:00, got %q", event.TimeLabel)
	}
	if event.Description != "CT won the round (bomb defused)" {
		t.Fatalf("unexpected description: %q", event.Description)
	}
}
