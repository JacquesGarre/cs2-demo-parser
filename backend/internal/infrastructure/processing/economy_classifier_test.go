package processing

import (
	"testing"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
)

func TestEconomyFromAverageMoney(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		players   int
		expected  string
	}{
		{name: "full buy threshold", total: 20000, players: 5, expected: "Full buy"},
		{name: "small buy threshold", total: 15000, players: 5, expected: "Small buy"},
		{name: "force buy threshold", total: 7500, players: 5, expected: "Force buy"},
		{name: "eco round", total: 5000, players: 5, expected: "Eco round"},
		{name: "non-positive player count fallback", total: 0, players: 0, expected: "Eco round"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := economyFromAverageMoney(tc.total, tc.players)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestTeamEconomyFromPlayerLabels(t *testing.T) {
	players := []entities.RoundPlayerMoney{
		{Economy: "Full buy"},
		{Economy: "Full buy"},
		{Economy: "Full buy"},
		{Economy: "Eco round"},
		{Economy: "Force buy"},
	}

	got := teamEconomyFromPlayerLabels(players)
	if got != "Full buy" {
		t.Fatalf("expected Full buy, got %q", got)
	}
}

func TestNormalizeEconomyLabel_DefaultsToForceBuy(t *testing.T) {
	got := normalizeEconomyLabel("some-unknown")
	if got != "Force buy" {
		t.Fatalf("expected Force buy, got %q", got)
	}
}

func TestClassifyPlayerEconomy_PistolRound(t *testing.T) {
	got := classifyPlayerEconomy(1, common.TeamTerrorists, 800, false, false, false, "No armor", 0, nil)
	if got != "Pistol round" {
		t.Fatalf("expected Pistol round, got %q", got)
	}
}

func TestClassifyPlayerEconomy_FullBuyRifleArmor(t *testing.T) {
	got := classifyPlayerEconomy(5, common.TeamCounterTerrorists, 4200, false, true, false, "Armor + Helmet", 2, nil)
	if got != "Full buy" {
		t.Fatalf("expected Full buy, got %q", got)
	}
}
