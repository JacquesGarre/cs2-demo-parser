package processing

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
)

type CS2DemoAnalyzer struct{}

type playerAggregate struct {
	ID                   string
	PlayerName           string
	Team                 string
	Assists              int
	TeamDamage           int
	HeadshotKills        int
	TwoKs                int
	ThreeKs              int
	FourKs               int
	FiveKs               int
	BestRoundPlayerCount int
	OpeningDuels         int
	OpeningWins          int
	KillPoints           []entities.HeatPoint
	DeathPoints          []entities.HeatPoint
}

type roundState struct {
	CurrentRound       int
	ActiveRound        bool
	RoundCommitted     bool
	PendingWinner      common.Team
	RoundKills         map[string]int
	RoundKillDetails   map[string][]entities.RoundKillDetail
	RoundDamage        map[string]int
	CTEconomy          string
	TEconomy           string
	CTStartMoney       int
	TStartMoney        int
	CTMoneyByPlayer    []entities.RoundPlayerMoney
	TMoneyByPlayer     []entities.RoundPlayerMoney
	EconomyCaptureTick int
	EconomyCaptured    bool
	OpeningTaken       map[int]bool
	CTWins             int
	TWins              int
	RoundHistory       []entities.RoundSummary
}

const maxRoundHistoryEntries = 30
const economyGraceTicks = 64

func NewCS2DemoAnalyzer() *CS2DemoAnalyzer {
	return &CS2DemoAnalyzer{}
}

func (a *CS2DemoAnalyzer) Analyze(demo entities.Demo) (entities.MatchSummary, error) {
	demoFile, err := os.Open(demo.StoragePath)
	if err != nil {
		return entities.MatchSummary{}, fmt.Errorf("failed to open demo: %w", err)
	}
	defer demoFile.Close()

	parser := demoinfocs.NewParser(demoFile)
	defer parser.Close()

	state := roundState{OpeningTaken: map[int]bool{}, RoundKills: map[string]int{}, RoundKillDetails: map[string][]entities.RoundKillDetail{}, RoundDamage: map[string]int{}, RoundHistory: []entities.RoundSummary{}}
	players := map[string]*playerAggregate{}
	var ctTeamName, tTeamName string
	var mapNameFromServerInfo string

	parser.RegisterNetMessageHandler(func(m *msg.CSVCMsg_ServerInfo) {
		if m == nil {
			return
		}

		if rawMap := strings.TrimSpace(m.GetMapName()); rawMap != "" {
			mapNameFromServerInfo = normalizeMapName(rawMap)
		}
	})

	parser.RegisterEventHandler(func(_ events.RoundStart) {
		if !shouldTrackLiveRound(parser) {
			state.ActiveRound = false
			return
		}

		if ctTeamName == "" || tTeamName == "" {
			if gameState := parser.GameState(); gameState != nil {
				if ct := gameState.TeamCounterTerrorists(); ct != nil {
					ctTeamName = ct.ClanName()
				}
				if t := gameState.TeamTerrorists(); t != nil {
					tTeamName = t.ClanName()
				}
			}
		}

		state.CurrentRound++
		state.ActiveRound = true
		state.RoundCommitted = false
		state.PendingWinner = common.TeamUnassigned
		state.RoundKills = map[string]int{}
		state.RoundKillDetails = map[string][]entities.RoundKillDetail{}
		state.RoundDamage = map[string]int{}
		state.CTEconomy = "Unknown"
		state.TEconomy = "Unknown"
		state.CTStartMoney = 0
		state.TStartMoney = 0
		state.CTMoneyByPlayer = nil
		state.TMoneyByPlayer = nil
		state.EconomyCaptureTick = 0
		state.EconomyCaptured = false

		gameState := parser.GameState()
		if gameState != nil {
			ctState := gameState.TeamCounterTerrorists()
			tState := gameState.TeamTerrorists()
			state.CTStartMoney = captureTeamTotalMoney(ctState)
			state.TStartMoney = captureTeamTotalMoney(tState)
		}
	})

	parser.RegisterEventHandler(func(_ events.RoundFreezetimeEnd) {
		if !state.ActiveRound {
			return
		}

		gameState := parser.GameState()
		if gameState == nil {
			return
		}

		state.EconomyCaptureTick = gameState.IngameTick() + economyGraceTicks
	})

	parser.RegisterEventHandler(func(_ events.FrameDone) {
		captureRoundEconomyIfReady(parser, &state)
	})

	parser.RegisterEventHandler(func(e events.RoundEnd) {
		captureRoundEconomyNow(parser, &state)

		if !state.ActiveRound {
			return
		}

		if isCompetitiveTeam(e.Winner) {
			state.PendingWinner = e.Winner
		}
	})

	parser.RegisterEventHandler(func(_ events.RoundEndOfficial) {
		if state.RoundCommitted {
			return
		}

		winner := inferWinnerFromScoreAtOfficial(parser, &state)
		if !isCompetitiveTeam(winner) {
			winner = state.PendingWinner
		}

		if !isCompetitiveTeam(winner) {
			state.ActiveRound = false
			state.PendingWinner = common.TeamUnassigned
			return
		}

		if !commitRoundResult(&state, players, winner) {
			state.ActiveRound = false
			state.PendingWinner = common.TeamUnassigned
			return
		}

		state.RoundCommitted = true
		state.ActiveRound = false
		state.PendingWinner = common.TeamUnassigned
	})

	parser.RegisterEventHandler(func(e events.PlayerHurt) {
		if !state.ActiveRound {
			return
		}

		if e.Attacker == nil || e.Player == nil {
			return
		}
		if e.Attacker.SteamID64 == e.Player.SteamID64 {
			return
		}
		if !isCompetitiveTeam(e.Attacker.Team) || !isCompetitiveTeam(e.Player.Team) {
			return
		}

		agg := upsertPlayer(players, e.Attacker)

		if !areOpposingTeams(e.Attacker.Team, e.Player.Team) {
			agg.TeamDamage += e.HealthDamageTaken
			return
		}

		state.RoundDamage[agg.ID] += e.HealthDamageTaken
	})

	parser.RegisterEventHandler(func(e events.Kill) {
		if !state.ActiveRound {
			return
		}

		if e.Victim == nil {
			return
		}

		victimAgg := upsertPlayer(players, e.Victim)
		killerName := "World"
		killerSide := "Unknown"
		killerWeapon := "Unknown"
		if e.Killer != nil {
			killerName = safePlayerName(e.Killer)
			killerSide = teamName(e.Killer.Team)
			killerWeapon = playerWeaponLabel(e.Killer)
		}
		victimWeapon := playerWeaponLabel(e.Victim)
		victimAgg.DeathPoints = append(victimAgg.DeathPoints, newHeatPoint(
			state.CurrentRound,
			killerSide,
			teamName(e.Victim.Team),
			float64(e.Victim.Position().X),
			float64(e.Victim.Position().Y),
			float64(e.Victim.Position().Z),
			killerName,
			safePlayerName(e.Victim),
			killerWeapon,
			victimWeapon,
		))

		if e.Killer == nil {
			return
		}
		if e.Killer.SteamID64 == e.Victim.SteamID64 {
			return
		}
		if !isCompetitiveTeam(e.Killer.Team) || !isCompetitiveTeam(e.Victim.Team) {
			return
		}
		if !areOpposingTeams(e.Killer.Team, e.Victim.Team) {
			return
		}

		killerAgg := upsertPlayer(players, e.Killer)
		state.RoundKills[killerAgg.ID]++
		state.RoundKillDetails[killerAgg.ID] = append(state.RoundKillDetails[killerAgg.ID], entities.RoundKillDetail{
			VictimName: safePlayerName(e.Victim),
			Weapon:     killWeaponLabel(e),
		})
		killerAgg.KillPoints = append(killerAgg.KillPoints, newHeatPoint(
			state.CurrentRound,
			killerSide,
			teamName(e.Victim.Team),
			float64(e.Victim.Position().X),
			float64(e.Victim.Position().Y),
			float64(e.Victim.Position().Z),
			safePlayerName(e.Killer),
			safePlayerName(e.Victim),
			killerWeapon,
			victimWeapon,
		))
		if e.IsHeadshot {
			killerAgg.HeadshotKills++
		}

		if e.Assister != nil && !e.AssistedFlash && isCompetitiveTeam(e.Assister.Team) && e.Assister.SteamID64 != e.Killer.SteamID64 && e.Assister.SteamID64 != e.Victim.SteamID64 && e.Assister.Team == e.Killer.Team {
			assisterAgg := upsertPlayer(players, e.Assister)
			assisterAgg.Assists++
		}

		if !state.OpeningTaken[state.CurrentRound] {
			killerAgg.OpeningDuels++
			killerAgg.OpeningWins++
			victimAgg.OpeningDuels++
			state.OpeningTaken[state.CurrentRound] = true
		}
	})

	parser.RegisterEventHandler(func(e events.RoundMVPAnnouncement) {
		if e.Player == nil {
			return
		}

		aggregate := upsertPlayer(players, e.Player)

		if len(state.RoundHistory) == 0 {
			return
		}

		lastRound := &state.RoundHistory[len(state.RoundHistory)-1]
		lastRound.MVP = &entities.RoundMVP{
			PlayerName: aggregate.PlayerName,
			Team:       aggregate.Team,
			Reason:     mvpReasonLabel(e.Reason),
		}
		applyRoundMVPStats(lastRound, lastRound.MVP)
	})

	if err := parser.ParseToEnd(); err != nil {
		return entities.MatchSummary{}, fmt.Errorf("failed to parse demo events: %w", err)
	}

	if state.ActiveRound && !state.RoundCommitted && isCompetitiveTeam(state.PendingWinner) {
		if commitRoundResult(&state, players, state.PendingWinner) {
			state.RoundCommitted = true
		}
		state.ActiveRound = false
		state.PendingWinner = common.TeamUnassigned
	}

	reconcileMissingFinalRound(parser, &state, players)

	finalCTScore := state.CTWins
	finalTScore := state.TWins
	finalRounds := 0
	finalCTTeamName := ""
	finalTTeamName := ""
	if gameState := parser.GameState(); gameState != nil {
		if ct := gameState.TeamCounterTerrorists(); ct != nil {
			finalCTScore = ct.Score()
			finalCTTeamName = strings.TrimSpace(ct.ClanName())
		}
		if t := gameState.TeamTerrorists(); t != nil {
			finalTScore = t.Score()
			finalTTeamName = strings.TrimSpace(t.ClanName())
		}
		if finalCTScore >= 0 && finalTScore >= 0 {
			finalRounds = finalCTScore + finalTScore
		}
	}

	if finalRounds > 0 {
		state.RoundHistory = normalizeRoundHistory(state.RoundHistory, finalRounds)
	}

	rounds := finalRounds
	if rounds <= 0 {
		rounds = state.CTWins + state.TWins
	}
	if rounds <= 0 {
		rounds = len(state.RoundHistory)
	}
	if rounds <= 0 {
		rounds = 1
	}

	participants := parser.GameState().Participants().All()

	playing := make([]*common.Player, 0, len(participants))
	for _, player := range participants {
		if player == nil {
			continue
		}
		if player.IsUnknown {
			continue
		}
		if player.IsBot {
			continue
		}
		if player.Team != common.TeamCounterTerrorists && player.Team != common.TeamTerrorists {
			continue
		}
		if player.SteamID64 == 0 && strings.TrimSpace(player.Name) == "" {
			continue
		}
		playing = append(playing, player)
	}

	mvpTargets := make(map[string]int, len(playing))
	for _, player := range playing {
		mvpTargets[safePlayerName(player)] = player.MVPs()
	}

	reconcileRoundMVPs(state.RoundHistory, mvpTargets)

	playerStats := make([]entities.PlayerSummary, 0, len(playing))
	playerHeatmaps := make([]entities.PlayerHeatmap, 0, len(playing))
	aggregatedKills := []entities.HeatPoint{}
	aggregatedDeaths := []entities.HeatPoint{}

	for _, player := range playing {
		aggregate := upsertPlayer(players, player)

		entryKillRate := 0.0
		if aggregate.OpeningDuels > 0 {
			entryKillRate = float64(aggregate.OpeningWins) / float64(aggregate.OpeningDuels)
		}

		damage := player.TotalDamage() - aggregate.TeamDamage
		if damage < 0 {
			damage = 0
		}

		adr := 0.0
		if rounds > 0 {
			adr = float64(damage) / float64(rounds)
		}

		kdRatio := float64(player.Kills())
		if player.Deaths() > 0 {
			kdRatio = float64(player.Kills()) / float64(player.Deaths())
		}

		killsPerRound := 0.0
		if rounds > 0 {
			killsPerRound = float64(player.Kills()) / float64(rounds)
		}

		hsPercentage := 0.0
		if player.Kills() > 0 {
			hsPercentage = (float64(aggregate.HeadshotKills) / float64(player.Kills())) * 100
		}

		killHeat := cloneHeatPoints(aggregate.KillPoints)
		deathHeat := cloneHeatPoints(aggregate.DeathPoints)
		sortHeatPoints(killHeat)
		sortHeatPoints(deathHeat)

		playerStats = append(playerStats, entities.PlayerSummary{
			PlayerName:           safePlayerName(player),
			Team:                 teamName(player.Team),
			Kills:                player.Kills(),
			Deaths:               player.Deaths(),
			Assists:              aggregate.Assists,
			KDRatio:              kdRatio,
			KillsPerRound:        killsPerRound,
			HSPercentage:         hsPercentage,
			ADR:                  adr,
			KAST:                 0,
			OpeningDuels:         aggregate.OpeningDuels,
			OpeningWins:          aggregate.OpeningWins,
			TradeKills:           0,
			TradeDeaths:          0,
			ClutchWon:            0,
			ClutchPlayed:         0,
			TwoKs:                aggregate.TwoKs,
			ThreeKs:              aggregate.ThreeKs,
			FourKs:               aggregate.FourKs,
			FiveKs:               aggregate.FiveKs,
			BestRoundPlayerCount: player.MVPs(),
			EntryKillRate:        entryKillRate,
		})

		playerHeatmaps = append(playerHeatmaps, entities.PlayerHeatmap{
			PlayerName: safePlayerName(player),
			Kills:      killHeat,
			Deaths:     deathHeat,
		})

		aggregatedKills = append(aggregatedKills, killHeat...)
		aggregatedDeaths = append(aggregatedDeaths, deathHeat...)
	}

	sort.Slice(playerStats, func(i, j int) bool {
		if playerStats[i].Kills == playerStats[j].Kills {
			return playerStats[i].Deaths < playerStats[j].Deaths
		}
		return playerStats[i].Kills > playerStats[j].Kills
	})

	sort.Slice(playerHeatmaps, func(i, j int) bool {
		return playerHeatmaps[i].PlayerName < playerHeatmaps[j].PlayerName
	})
	sortHeatPoints(aggregatedKills)
	sortHeatPoints(aggregatedDeaths)

	// Team A/B are anchored to scoreboard mapping where Team A score equals final CT score.
	// Keep names aligned with that same anchor to avoid team-name/player/score inversion.
	teamAName := finalCTTeamName
	if teamAName == "" {
		teamAName = ctTeamName
	}
	teamBName := finalTTeamName
	if teamBName == "" {
		teamBName = tTeamName
	}
	if teamAName == "" {
		teamAName = "Team A"
	}
	if teamBName == "" {
		teamBName = "Team B"
	}

	mapName := inferMapName(demo.FileName)
	if mapNameFromServerInfo != "" {
		mapName = mapNameFromServerInfo
	}

	return entities.MatchSummary{
		DemoID:         demo.ID,
		MapName:        mapName,
		Rounds:         rounds,
		TeamAScore:     finalCTScore,
		TeamBScore:     finalTScore,
		TeamAName:      teamAName,
		TeamBName:      teamBName,
		PlayerStats:    playerStats,
		KillHeatmap:    aggregatedKills,
		DeathHeatmap:   aggregatedDeaths,
		PlayerHeatmaps: playerHeatmaps,
		RoundHistory:   state.RoundHistory,
		AnalysisSource: "demoinfocs-v5-faceit-like-adr",
	}, nil
}

func inferMapName(fileName string) string {
	// Extract the actual filename from path
	base := strings.ToLower(filepath.Base(fileName))

	// Remove common file extensions first to avoid confusion
	base = strings.TrimSuffix(base, ".dem")
	base = strings.TrimSuffix(base, ".dem.bz2")
	base = strings.TrimSuffix(base, ".dem.zip")

	// Map keywords to full map names, ordered by specificity (longest first to catch multi-word variations)
	// Check more specific names first to avoid false positives
	mapMappings := []struct {
		keyword  string
		fullName string
	}{
		{"ancient", "de_ancient"},
		{"anubis", "de_anubis"},
		{"inferno", "de_inferno"},
		{"overpass", "de_overpass"},
		{"vertigo", "de_vertigo"},
		{"dust2", "de_dust2"},
		{"dust 2", "de_dust2"},
		{"nuke", "de_nuke"},
		{"mirage", "de_mirage"},
	}

	for _, mapping := range mapMappings {
		if strings.Contains(base, mapping.keyword) {
			return mapping.fullName
		}
	}

	// If no keyword matched, return default
	return "de_mirage"
}

func normalizeMapName(name string) string {
	// The demo parser might return map names like "Ancient", "de_ancient", or "ancient"
	// Convert to our standard format: "de_<mapname>"
	base := strings.ToLower(strings.TrimSpace(name))

	// Remove "de_" prefix if it exists
	base = strings.TrimPrefix(base, "de_")

	// Map known map keywords
	mapKeywords := map[string]string{
		"ancient":  "de_ancient",
		"anubis":   "de_anubis",
		"inferno":  "de_inferno",
		"overpass": "de_overpass",
		"vertigo":  "de_vertigo",
		"dust2":    "de_dust2",
		"nuke":     "de_nuke",
		"mirage":   "de_mirage",
	}

	if fullName, ok := mapKeywords[base]; ok {
		return fullName
	}

	// Fallback: if it didn't match, return as-is with de_ prefix
	return "de_" + base
}

func upsertPlayer(items map[string]*playerAggregate, player *common.Player) *playerAggregate {
	id := playerID(player)
	if existing, found := items[id]; found {
		existing.PlayerName = safePlayerName(player)
		existing.Team = teamName(player.Team)
		return existing
	}

	created := &playerAggregate{
		ID:          id,
		PlayerName:  safePlayerName(player),
		Team:        teamName(player.Team),
		KillPoints:  make([]entities.HeatPoint, 0),
		DeathPoints: make([]entities.HeatPoint, 0),
	}
	items[id] = created
	return created
}

func playerID(player *common.Player) string {
	if player.SteamID64 != 0 {
		return fmt.Sprintf("%d", player.SteamID64)
	}
	return safePlayerName(player)
}

func safePlayerName(player *common.Player) string {
	if player.Name != "" {
		return player.Name
	}
	if player.SteamID64 != 0 {
		return fmt.Sprintf("player-%d", player.SteamID64)
	}
	return "unknown"
}

func teamName(team common.Team) string {
	switch team {
	case common.TeamCounterTerrorists:
		return "CT"
	case common.TeamTerrorists:
		return "T"
	default:
		return "Unknown"
	}
}

func playerWeaponLabel(player *common.Player) string {
	if player == nil {
		return "Unknown"
	}

	bestName := ""
	bestPriority := -1
	for _, weapon := range player.Weapons() {
		if weapon == nil {
			continue
		}

		class := weapon.Class()
		if class == common.EqClassUnknown || class == common.EqClassGrenade || class == common.EqClassEquipment {
			continue
		}

		name := strings.TrimSpace(weapon.String())
		if name == "" || name == "Unknown" {
			continue
		}

		priority := classPriority(class)
		if priority > bestPriority {
			bestPriority = priority
			bestName = name
		}
	}

	if bestName != "" {
		return bestName
	}

	return "Unknown"
}

func newHeatPoint(roundNumber int, killerSide string, victimSide string, x float64, y float64, z float64, killerName string, victimName string, killWeapon string, victimWeapon string) entities.HeatPoint {
	return entities.HeatPoint{
		X:            x,
		Y:            y,
		Z:            z,
		Count:        1,
		RoundNumber:  roundNumber,
		Side:         killerSide,
		KillerSide:   killerSide,
		VictimSide:   victimSide,
		KillerName:   killerName,
		VictimName:   victimName,
		KillWeapon:   killWeapon,
		VictimWeapon: victimWeapon,
	}
}

func cloneHeatPoints(points []entities.HeatPoint) []entities.HeatPoint {
	cloned := make([]entities.HeatPoint, len(points))
	copy(cloned, points)
	return cloned
}

func sortHeatPoints(points []entities.HeatPoint) {
	sort.Slice(points, func(i, j int) bool {
		if points[i].RoundNumber == points[j].RoundNumber {
			if points[i].X == points[j].X {
				if points[i].Y == points[j].Y {
					return points[i].Z < points[j].Z
				}
				return points[i].Y < points[j].Y
			}
			return points[i].X < points[j].X
		}
		return points[i].RoundNumber < points[j].RoundNumber
	})
}

func maxRound(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func commitRoundAwards(state *roundState, players map[string]*playerAggregate, winningTeam common.Team, roundNumber int) {
	bestPlayerID := ""
	bestKills := -1
	bestDamage := -1
	multiKills := make([]entities.RoundPerformance, 0)
	playerDamages := make([]entities.RoundPerformance, 0, len(state.RoundDamage))

	for id, damage := range state.RoundDamage {
		aggregate, found := players[id]
		if !found {
			continue
		}

		playerDamages = append(playerDamages, entities.RoundPerformance{
			PlayerName: aggregate.PlayerName,
			Team:       aggregate.Team,
			Kills:      state.RoundKills[id],
			Damage:     damage,
			Label:      "Damage dealt",
		})
	}

	for id, kills := range state.RoundKills {
		aggregate, found := players[id]
		if !found {
			continue
		}

		switch kills {
		case 2:
			aggregate.TwoKs++
		case 3:
			aggregate.ThreeKs++
		case 4:
			aggregate.FourKs++
		default:
			if kills >= 5 {
				aggregate.FiveKs++
			}
		}

		if kills >= 2 {
			killDetails := state.RoundKillDetails[id]
			multiKills = append(multiKills, entities.RoundPerformance{
				PlayerName:  aggregate.PlayerName,
				Team:        aggregate.Team,
				Kills:       kills,
				Damage:      state.RoundDamage[id],
				Label:       multiKillLabel(kills),
				KillDetails: killDetails,
			})
		}

		if parseAggregateTeam(aggregate.Team) != winningTeam {
			continue
		}

		damage := state.RoundDamage[id]
		if kills > bestKills || (kills == bestKills && damage > bestDamage) {
			bestPlayerID = id
			bestKills = kills
			bestDamage = damage
		}
	}

	var bestPlayer *entities.RoundPerformance
	if bestPlayerID != "" {
		aggregate := players[bestPlayerID]
		bestPlayer = &entities.RoundPerformance{
			PlayerName: aggregate.PlayerName,
			Team:       aggregate.Team,
			Kills:      state.RoundKills[bestPlayerID],
			Damage:     state.RoundDamage[bestPlayerID],
			Label:      "Best of round",
		}
	}

	sort.Slice(multiKills, func(i, j int) bool {
		if multiKills[i].Kills == multiKills[j].Kills {
			return multiKills[i].Damage > multiKills[j].Damage
		}
		return multiKills[i].Kills > multiKills[j].Kills
	})

	sort.Slice(playerDamages, func(i, j int) bool {
		if playerDamages[i].Damage == playerDamages[j].Damage {
			return playerDamages[i].PlayerName < playerDamages[j].PlayerName
		}
		return playerDamages[i].Damage > playerDamages[j].Damage
	})

	state.RoundHistory = append(state.RoundHistory, entities.RoundSummary{
		RoundNumber:     roundNumber,
		WinnerTeam:      teamName(winningTeam),
		BestPlayer:      bestPlayer,
		PlayerDamages:   playerDamages,
		CTEconomy:       state.CTEconomy,
		TEconomy:        state.TEconomy,
		CTStartMoney:    state.CTStartMoney,
		TStartMoney:     state.TStartMoney,
		CTMoneyByPlayer: state.CTMoneyByPlayer,
		TMoneyByPlayer:  state.TMoneyByPlayer,
		MultiKills:      multiKills,
	})
}

func captureRoundEconomyIfReady(parser demoinfocs.Parser, state *roundState) {
	if !state.ActiveRound || state.EconomyCaptured || state.EconomyCaptureTick <= 0 {
		return
	}

	gameState := parser.GameState()
	if gameState == nil {
		return
	}

	if gameState.IngameTick() < state.EconomyCaptureTick {
		return
	}

	captureRoundEconomyNow(parser, state)
}

func captureRoundEconomyNow(parser demoinfocs.Parser, state *roundState) {
	if !state.ActiveRound || state.EconomyCaptured {
		return
	}

	gameState := parser.GameState()
	if gameState == nil {
		return
	}

	ctState := gameState.TeamCounterTerrorists()
	tState := gameState.TeamTerrorists()
	if ctState == nil || tState == nil {
		return
	}

	ctSnapshot := buildTeamLoadoutSnapshot(ctState)
	tSnapshot := buildTeamLoadoutSnapshot(tState)
	_, state.CTMoneyByPlayer = captureTeamMoneySnapshot(state.CurrentRound, common.TeamCounterTerrorists, ctState, state.RoundHistory)
	_, state.TMoneyByPlayer = captureTeamMoneySnapshot(state.CurrentRound, common.TeamTerrorists, tState, state.RoundHistory)

	state.CTEconomy = classifyTeamEconomy(state.CurrentRound, common.TeamCounterTerrorists, state.CTStartMoney, state.CTMoneyByPlayer, ctSnapshot, state.RoundHistory)
	state.TEconomy = classifyTeamEconomy(state.CurrentRound, common.TeamTerrorists, state.TStartMoney, state.TMoneyByPlayer, tSnapshot, state.RoundHistory)
	state.EconomyCaptured = true
}

func buildTeamLoadoutSnapshot(teamState *common.TeamState) teamLoadoutSnapshot {
	snapshot := teamLoadoutSnapshot{}
	if teamState == nil {
		return snapshot
	}

	members := teamState.Members()
	for _, player := range members {
		if player == nil || player.IsUnknown {
			continue
		}

		snapshot.Players++
		snapshot.TeamEquipValue += player.EquipmentValueCurrent()

		if player.Armor() > 0 {
			snapshot.ArmorPlayers++
		}
		if player.Armor() >= 100 && player.HasHelmet() {
			snapshot.FullArmorPlayers++
		}
		if player.HasDefuseKit() {
			snapshot.DefuseKits++
		}

		primaryClass := common.EqClassUnknown
		for _, weapon := range player.Weapons() {
			if weapon == nil {
				continue
			}

			class := weapon.Class()
			if class == common.EqClassGrenade {
				snapshot.TotalGrenades++
				continue
			}

			if class == common.EqClassEquipment {
				continue
			}

			if classPriority(class) > classPriority(primaryClass) {
				primaryClass = class
			}
		}

		switch primaryClass {
		case common.EqClassRifle:
			snapshot.RiflePlayers++
		case common.EqClassSMG:
			snapshot.SMGPlayers++
		case common.EqClassHeavy:
			snapshot.HeavyPlayers++
		default:
			snapshot.PistolOnlyPlayers++
		}
	}

	return snapshot
}

func reconcileRoundMVPs(rounds []entities.RoundSummary, targetCounts map[string]int) {
	if len(rounds) == 0 || len(targetCounts) == 0 {
		return
	}

	remaining := make(map[string]int, len(targetCounts))
	for playerName, target := range targetCounts {
		remaining[playerName] = target
	}

	assignableRounds := make([]int, 0, len(rounds))
	for i := range rounds {
		mvp := rounds[i].MVP
		isInferred := mvp != nil && strings.Contains(strings.ToLower(mvp.Reason), "inferred")
		if mvp != nil && !isInferred {
			if remaining[mvp.PlayerName] > 0 {
				remaining[mvp.PlayerName]--
			}
			continue
		}

		rounds[i].MVP = nil
		assignableRounds = append(assignableRounds, i)
	}

	for _, idx := range assignableRounds {
		round := &rounds[idx]
		if chosen := chooseMVPFromBest(round, remaining); chosen != nil {
			round.MVP = chosen
			applyRoundMVPStats(round, round.MVP)
			remaining[chosen.PlayerName]--
			continue
		}
		if chosen := chooseMVPFromMultiKills(round, remaining); chosen != nil {
			round.MVP = chosen
			applyRoundMVPStats(round, round.MVP)
			remaining[chosen.PlayerName]--
			continue
		}
		if chosen := chooseMVPFromRemaining(remaining); chosen != "" {
			round.MVP = &entities.RoundMVP{PlayerName: chosen, Team: "Unknown", Reason: "Round MVP"}
			applyRoundMVPStats(round, round.MVP)
			remaining[chosen]--
		}
	}
}

func chooseMVPFromBest(round *entities.RoundSummary, remaining map[string]int) *entities.RoundMVP {
	if round.BestPlayer == nil {
		return nil
	}
	if remaining[round.BestPlayer.PlayerName] <= 0 {
		return nil
	}
	return &entities.RoundMVP{
		PlayerName: round.BestPlayer.PlayerName,
		Team:       round.BestPlayer.Team,
		Reason:     "Most eliminations",
	}
}

func chooseMVPFromMultiKills(round *entities.RoundSummary, remaining map[string]int) *entities.RoundMVP {
	for _, perf := range round.MultiKills {
		if remaining[perf.PlayerName] <= 0 {
			continue
		}
		return &entities.RoundMVP{PlayerName: perf.PlayerName, Team: perf.Team, Reason: "Round impact"}
	}
	return nil
}

func chooseMVPFromRemaining(remaining map[string]int) string {
	bestPlayer := ""
	bestRemaining := 0
	for playerName, count := range remaining {
		if count > bestRemaining {
			bestRemaining = count
			bestPlayer = playerName
		}
	}
	return bestPlayer
}

func applyRoundMVPStats(round *entities.RoundSummary, mvp *entities.RoundMVP) {
	if round == nil || mvp == nil {
		return
	}

	for _, perf := range round.MultiKills {
		if perf.PlayerName != mvp.PlayerName {
			continue
		}
		mvp.Kills = perf.Kills
		mvp.Damage = perf.Damage
		if mvp.Team == "Unknown" || mvp.Team == "" {
			mvp.Team = perf.Team
		}
		return
	}

	if round.BestPlayer != nil && round.BestPlayer.PlayerName == mvp.PlayerName {
		mvp.Kills = round.BestPlayer.Kills
		mvp.Damage = round.BestPlayer.Damage
		if mvp.Team == "Unknown" || mvp.Team == "" {
			mvp.Team = round.BestPlayer.Team
		}
	}
}

func multiKillLabel(kills int) string {
	switch kills {
	case 2:
		return "2K"
	case 3:
		return "3K"
	case 4:
		return "4K"
	default:
		if kills >= 5 {
			return "5K"
		}
		return "Round impact"
	}
}

func killWeaponLabel(e events.Kill) string {
	if e.Weapon == nil {
		return "Unknown weapon"
	}

	name := strings.TrimSpace(e.Weapon.String())
	if name == "" {
		return "Unknown weapon"
	}

	return name
}

func mvpReasonLabel(reason events.RoundMVPReason) string {
	switch reason {
	case events.MVPReasonMostEliminations:
		return "Most eliminations"
	case events.MVPReasonBombDefused:
		return "Bomb defused"
	case events.MVPReasonBombPlanted:
		return "Bomb planted"
	default:
		return "Round impact"
	}
}

func parseAggregateTeam(team string) common.Team {
	switch team {
	case "CT":
		return common.TeamCounterTerrorists
	case "T":
		return common.TeamTerrorists
	default:
		return common.TeamSpectators
	}
}

func areOpposingTeams(left common.Team, right common.Team) bool {
	return (left == common.TeamTerrorists && right == common.TeamCounterTerrorists) ||
		(left == common.TeamCounterTerrorists && right == common.TeamTerrorists)
}

func isCompetitiveTeam(team common.Team) bool {
	return team == common.TeamTerrorists || team == common.TeamCounterTerrorists
}

func shouldTrackLiveRound(parser demoinfocs.Parser) bool {
	state := parser.GameState()
	if state == nil {
		return false
	}

	if !state.IsMatchStarted() {
		return false
	}

	if state.IsWarmupPeriod() {
		return false
	}

	return true
}

func inferWinnerFromScoreAtOfficial(parser demoinfocs.Parser, state *roundState) common.Team {
	gameState := parser.GameState()
	if gameState == nil {
		return common.TeamUnassigned
	}

	ct := gameState.TeamCounterTerrorists()
	t := gameState.TeamTerrorists()
	if ct == nil || t == nil {
		return common.TeamUnassigned
	}

	if ct.Score() == state.CTWins+1 && t.Score() == state.TWins {
		return common.TeamCounterTerrorists
	}

	if t.Score() == state.TWins+1 && ct.Score() == state.CTWins {
		return common.TeamTerrorists
	}

	return common.TeamUnassigned
}

func commitRoundResult(state *roundState, players map[string]*playerAggregate, winner common.Team) bool {
	if !isCompetitiveTeam(winner) {
		return false
	}

	nextRoundNumber := state.CTWins + state.TWins + 1
	if nextRoundNumber > maxRoundHistoryEntries {
		return false
	}

	switch winner {
	case common.TeamCounterTerrorists:
		state.CTWins++
		commitRoundAwards(state, players, common.TeamCounterTerrorists, nextRoundNumber)
	case common.TeamTerrorists:
		state.TWins++
		commitRoundAwards(state, players, common.TeamTerrorists, nextRoundNumber)
	default:
		return false
	}

	return true
}

func reconcileMissingFinalRound(parser demoinfocs.Parser, state *roundState, players map[string]*playerAggregate) {
	gameState := parser.GameState()
	if gameState == nil {
		return
	}

	ct := gameState.TeamCounterTerrorists()
	t := gameState.TeamTerrorists()
	if ct == nil || t == nil {
		return
	}

	ctDelta := ct.Score() - state.CTWins
	tDelta := t.Score() - state.TWins
	if ctDelta < 0 || tDelta < 0 {
		return
	}

	if ctDelta+tDelta != 1 {
		return
	}

	nextRoundNumber := state.CTWins + state.TWins + 1
	if nextRoundNumber > maxRoundHistoryEntries {
		return
	}

	winner := common.TeamUnassigned
	if ctDelta == 1 {
		winner = common.TeamCounterTerrorists
	}
	if tDelta == 1 {
		winner = common.TeamTerrorists
	}
	if !isCompetitiveTeam(winner) {
		return
	}

	switch winner {
	case common.TeamCounterTerrorists:
		state.CTWins++
		commitRoundAwards(state, players, common.TeamCounterTerrorists, nextRoundNumber)
	case common.TeamTerrorists:
		state.TWins++
		commitRoundAwards(state, players, common.TeamTerrorists, nextRoundNumber)
	}

	state.ActiveRound = false
	state.PendingWinner = common.TeamUnassigned
}
