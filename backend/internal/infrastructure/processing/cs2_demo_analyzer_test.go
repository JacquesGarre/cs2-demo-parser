package processing

import (
	"testing"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
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

func TestAnnotateRoundEventsWithMatchState(t *testing.T) {
	events := []entities.RoundEvent{
		{Tick: 10, EventType: "utility", Team: "T", Description: "flash"},
		{Tick: 20, EventType: "kill", Team: "T", TargetName: "ct-one", Description: "t kill"},
		{Tick: 30, EventType: "damage", Team: "CT", Description: "spam damage"},
		{Tick: 40, EventType: "kill", Team: "CT", TargetName: "t-one", Description: "ct trade"},
	}

	annotateRoundEventsWithMatchState(events, 5, 5)

	if events[0].MatchState != "T 5v5 CT" {
		t.Fatalf("expected first event state 'T 5v5 CT', got %q", events[0].MatchState)
	}
	if events[1].MatchState != "T 5v4 CT" {
		t.Fatalf("expected kill event state 'T 5v4 CT', got %q", events[1].MatchState)
	}
	if events[2].MatchState != "T 5v4 CT" {
		t.Fatalf("expected damage event state 'T 5v4 CT', got %q", events[2].MatchState)
	}
	if events[3].MatchState != "T 4v4 CT" {
		t.Fatalf("expected trade event state 'T 4v4 CT', got %q", events[3].MatchState)
	}
}
