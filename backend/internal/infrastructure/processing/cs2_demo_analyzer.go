package processing

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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

type recentDeathEvent struct {
	KillerID   string
	VictimID   string
	VictimTeam common.Team
	Tick       int
}

type roundState struct {
	CurrentRound       int
	ActiveRound        bool
	RoundCommitted     bool
	PendingWinner      common.Team
	PendingEndReason   events.RoundEndReason
	PendingEndTick     int
	PendingEndTime     string
	LiveStartTick      int
	RoundKills         map[string]int
	RoundKillDetails   map[string][]entities.RoundKillDetail
	RoundDamage        map[string]int
	RoundEvents        []entities.RoundEvent
	RecentDeaths       []recentDeathEvent
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
const defaultRoundTimeSeconds = 115
const tradeWindowSeconds = 5
const roundResultTickOffset = 1000000

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
		state.PendingEndReason = events.RoundEndReasonStillInProgress
		state.PendingEndTick = 0
		state.PendingEndTime = ""
		state.LiveStartTick = 0
		state.RoundKills = map[string]int{}
		state.RoundKillDetails = map[string][]entities.RoundKillDetail{}
		state.RoundDamage = map[string]int{}
		state.RoundEvents = nil
		state.RecentDeaths = nil
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

		state.LiveStartTick = gameState.IngameTick()
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
			state.PendingEndReason = e.Reason
			state.PendingEndTick = currentIngameTick(parser)
			state.PendingEndTime = roundTimeLabel(parser, &state)
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

		if !commitRoundResult(&state, players, winner, state.PendingEndReason, state.PendingEndTick, state.PendingEndTime) {
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

		if isNoteworthyDamage(e) {
			state.RoundEvents = append(state.RoundEvents, newRoundEvent(
				currentIngameTick(parser),
				roundTimeLabel(parser, &state),
				"damage",
				teamName(e.Attacker.Team),
				safePlayerName(e.Attacker),
				safePlayerName(e.Player),
				"",
				hurtWeaponLabel(e),
				"",
				float64(e.Player.Position().X),
				float64(e.Player.Position().Y),
				float64(e.Player.Position().Z),
				false,
				false,
				e.HitGroup == events.HitGroupHead,
				false,
				false,
				buildDamageEventDescription(e),
			))
		}
	})

	parser.RegisterEventHandler(func(e events.Kill) {
		if !state.ActiveRound {
			return
		}

		if e.Victim == nil {
			return
		}

		eventTick := currentIngameTick(parser)
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
			state.RoundEvents = append(state.RoundEvents, newRoundEvent(
				eventTick,
				roundTimeLabel(parser, &state),
				"kill",
				"Unknown",
				"World",
				safePlayerName(e.Victim),
				"",
				killWeaponLabel(e),
				"",
				float64(e.Victim.Position().X),
				float64(e.Victim.Position().Y),
				float64(e.Victim.Position().Z),
				false,
				false,
				false,
				false,
				false,
				fmt.Sprintf("%s died to world damage", safePlayerName(e.Victim)),
			))
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

		assistantName := ""
		if e.Assister != nil && !e.AssistedFlash && isCompetitiveTeam(e.Assister.Team) && e.Assister.SteamID64 != e.Killer.SteamID64 && e.Assister.SteamID64 != e.Victim.SteamID64 && e.Assister.Team == e.Killer.Team {
			assisterAgg := upsertPlayer(players, e.Assister)
			assisterAgg.Assists++
			assistantName = safePlayerName(e.Assister)
		} else if e.Assister != nil && e.Assister.SteamID64 != e.Killer.SteamID64 && e.Assister.SteamID64 != e.Victim.SteamID64 {
			assistantName = safePlayerName(e.Assister)
		}

		isEntry := !state.OpeningTaken[state.CurrentRound]
		isTrade := isTradeKill(state.RecentDeaths, eventTick, int(parser.TickRate()), e.Killer, e.Victim)
		state.RoundEvents = append(state.RoundEvents, newRoundEvent(
			eventTick,
			roundTimeLabel(parser, &state),
			"kill",
			teamName(e.Killer.Team),
			safePlayerName(e.Killer),
			safePlayerName(e.Victim),
			assistantName,
			killWeaponLabel(e),
			"",
			float64(e.Victim.Position().X),
			float64(e.Victim.Position().Y),
			float64(e.Victim.Position().Z),
			isEntry,
			isTrade,
			e.IsHeadshot,
			e.IsWallBang(),
			e.ThroughSmoke,
			buildKillEventDescription(e, isEntry, isTrade, assistantName),
		))
		state.RecentDeaths = appendPrunedRecentDeaths(state.RecentDeaths, recentDeathEvent{
			KillerID: playerID(e.Killer),
			VictimID: playerID(e.Victim),
			VictimTeam: e.Victim.Team,
			Tick: eventTick,
		}, eventTick, int(parser.TickRate()))

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
		mvpEvent := newRoundEvent(
			currentIngameTick(parser),
			roundTimeLabel(parser, &state),
			"mvp",
			aggregate.Team,
			aggregate.PlayerName,
			"",
			"",
			"",
			"",
			0,
			0,
			0,
			false,
			false,
			false,
			false,
			false,
			fmt.Sprintf("Round MVP awarded to %s (%s)", aggregate.PlayerName, mvpReasonLabel(e.Reason)),
		)
		mvpEvent.MatchState = currentMatchStateLabel(parser)
		if mvpEvent.MatchState == "" && len(lastRound.Events) > 0 {
			mvpEvent.MatchState = lastRound.Events[len(lastRound.Events)-1].MatchState
		}
		if lastRound.WinnerTeam == "CT" {
			mvpEvent.CTWinProbability = 100
			mvpEvent.TWinProbability = 0
		} else if lastRound.WinnerTeam == "T" {
			mvpEvent.CTWinProbability = 0
			mvpEvent.TWinProbability = 100
		}
		lastRound.Events = append(lastRound.Events, mvpEvent)
		sortRoundEventsInPlace(lastRound.Events)
	})

	parser.RegisterEventHandler(func(e events.BombPlantBegin) {
		if !state.ActiveRound {
			return
		}
		state.RoundEvents = append(state.RoundEvents, newRoundEvent(
			currentIngameTick(parser),
			roundTimeLabel(parser, &state),
			"plant",
			teamName(playerTeam(e.Player)),
			safePlayerName(e.Player),
			"",
			"",
			"C4",
			bombSiteLabel(e.Site),
			0,
			0,
			0,
			false,
			false,
			false,
			false,
			false,
			buildBombEventDescription("starting plant", e.Player, e.Site),
		))
	})

	parser.RegisterEventHandler(func(e events.BombPlanted) {
		if !state.ActiveRound {
			return
		}
		state.RoundEvents = append(state.RoundEvents, newRoundEvent(
			currentIngameTick(parser),
			roundTimeLabel(parser, &state),
			"plant",
			teamName(playerTeam(e.Player)),
			safePlayerName(e.Player),
			"",
			"",
			"C4",
			bombSiteLabel(e.Site),
			0,
			0,
			0,
			false,
			false,
			false,
			false,
			false,
			buildBombEventDescription("planted the bomb", e.Player, e.Site),
		))
	})

	parser.RegisterEventHandler(func(e events.BombDefuseStart) {
		if !state.ActiveRound {
			return
		}
		kitLabel := ""
		if e.HasKit {
			kitLabel = " with a kit"
		}
		state.RoundEvents = append(state.RoundEvents, newRoundEvent(
			currentIngameTick(parser),
			roundTimeLabel(parser, &state),
			"defuse",
			teamName(playerTeam(e.Player)),
			safePlayerName(e.Player),
			"",
			"",
			"Defuse kit",
			"",
			0,
			0,
			0,
			false,
			false,
			false,
			false,
			false,
			fmt.Sprintf("%s started defusing%s", safePlayerName(e.Player), kitLabel),
		))
	})

	parser.RegisterEventHandler(func(e events.BombDefused) {
		if !state.ActiveRound {
			return
		}
		state.RoundEvents = append(state.RoundEvents, newRoundEvent(
			currentIngameTick(parser),
			roundTimeLabel(parser, &state),
			"defuse",
			teamName(playerTeam(e.Player)),
			safePlayerName(e.Player),
			"",
			"",
			"C4",
			bombSiteLabel(e.Site),
			0,
			0,
			0,
			false,
			false,
			false,
			false,
			false,
			buildBombEventDescription("defused the bomb", e.Player, e.Site),
		))
	})

	parser.RegisterEventHandler(func(e events.BombExplode) {
		if !state.ActiveRound {
			return
		}
		state.RoundEvents = append(state.RoundEvents, newRoundEvent(
			currentIngameTick(parser),
			roundTimeLabel(parser, &state),
			"bomb",
			teamName(playerTeam(e.Player)),
			safePlayerName(e.Player),
			"",
			"",
			"C4",
			bombSiteLabel(e.Site),
			0,
			0,
			0,
			false,
			false,
			false,
			false,
			false,
			buildBombEventDescription("detonated the bomb", e.Player, e.Site),
		))
	})

	parser.RegisterEventHandler(func(e events.BombDropped) {
		if !state.ActiveRound {
			return
		}
		state.RoundEvents = append(state.RoundEvents, newRoundEvent(
			currentIngameTick(parser),
			roundTimeLabel(parser, &state),
			"bomb",
			teamName(playerTeam(e.Player)),
			safePlayerName(e.Player),
			"",
			"",
			"C4",
			"",
			0,
			0,
			0,
			false,
			false,
			false,
			false,
			false,
			fmt.Sprintf("%s dropped the bomb", safePlayerName(e.Player)),
		))
	})

	parser.RegisterEventHandler(func(e events.BombPickup) {
		if !state.ActiveRound {
			return
		}
		state.RoundEvents = append(state.RoundEvents, newRoundEvent(
			currentIngameTick(parser),
			roundTimeLabel(parser, &state),
			"bomb",
			teamName(playerTeam(e.Player)),
			safePlayerName(e.Player),
			"",
			"",
			"C4",
			"",
			0,
			0,
			0,
			false,
			false,
			false,
			false,
			false,
			fmt.Sprintf("%s picked up the bomb", safePlayerName(e.Player)),
		))
	})

	parser.RegisterEventHandler(func(e events.PlayerFlashed) {
		if !state.ActiveRound || e.Player == nil || e.Attacker == nil {
			return
		}
		if e.Attacker.SteamID64 == e.Player.SteamID64 || !areOpposingTeams(e.Attacker.Team, e.Player.Team) {
			return
		}
		if e.FlashDuration() < 1500*time.Millisecond {
			return
		}
		state.RoundEvents = append(state.RoundEvents, newRoundEvent(
			currentIngameTick(parser),
			roundTimeLabel(parser, &state),
			"utility",
			teamName(e.Attacker.Team),
			safePlayerName(e.Attacker),
			safePlayerName(e.Player),
			"",
			"Flashbang",
			"",
			0,
			0,
			0,
			false,
			false,
			false,
			false,
			false,
			fmt.Sprintf("%s flashed %s for %.1fs", safePlayerName(e.Attacker), safePlayerName(e.Player), e.FlashDuration().Seconds()),
		))
	})

	parser.RegisterEventHandler(func(e events.SmokeStart) {
		if !state.ActiveRound {
			return
		}
		state.RoundEvents = append(state.RoundEvents, newRoundEvent(
			currentIngameTick(parser),
			roundTimeLabel(parser, &state),
			"utility",
			teamName(playerTeam(e.Thrower)),
			safePlayerName(e.Thrower),
			"",
			"",
			grenadeTypeLabel(e.GrenadeType, "Smoke"),
			"",
			float64(e.Position.X),
			float64(e.Position.Y),
			float64(e.Position.Z),
			false,
			false,
			false,
			false,
			false,
			buildGrenadeEventDescription("bloomed", e.Thrower, grenadeTypeLabel(e.GrenadeType, "Smoke")),
		))
	})

	parser.RegisterEventHandler(func(e events.HeExplode) {
		if !state.ActiveRound {
			return
		}
		state.RoundEvents = append(state.RoundEvents, newRoundEvent(
			currentIngameTick(parser),
			roundTimeLabel(parser, &state),
			"utility",
			teamName(playerTeam(e.Thrower)),
			safePlayerName(e.Thrower),
			"",
			"",
			grenadeTypeLabel(e.GrenadeType, "HE Grenade"),
			"",
			float64(e.Position.X),
			float64(e.Position.Y),
			float64(e.Position.Z),
			false,
			false,
			false,
			false,
			false,
			buildGrenadeEventDescription("exploded", e.Thrower, grenadeTypeLabel(e.GrenadeType, "HE Grenade")),
		))
	})

	parser.RegisterEventHandler(func(e events.FlashExplode) {
		if !state.ActiveRound {
			return
		}
		state.RoundEvents = append(state.RoundEvents, newRoundEvent(
			currentIngameTick(parser),
			roundTimeLabel(parser, &state),
			"utility",
			teamName(playerTeam(e.Thrower)),
			safePlayerName(e.Thrower),
			"",
			"",
			grenadeTypeLabel(e.GrenadeType, "Flashbang"),
			"",
			float64(e.Position.X),
			float64(e.Position.Y),
			float64(e.Position.Z),
			false,
			false,
			false,
			false,
			false,
			buildGrenadeEventDescription("popped", e.Thrower, grenadeTypeLabel(e.GrenadeType, "Flashbang")),
		))
	})

	if err := parser.ParseToEnd(); err != nil {
		return entities.MatchSummary{}, fmt.Errorf("failed to parse demo events: %w", err)
	}

	if state.ActiveRound && !state.RoundCommitted && isCompetitiveTeam(state.PendingWinner) {
		if commitRoundResult(&state, players, state.PendingWinner, state.PendingEndReason, state.PendingEndTick, state.PendingEndTime) {
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
	if player == nil {
		return "unknown"
	}
	if player.SteamID64 != 0 {
		return fmt.Sprintf("%d", player.SteamID64)
	}
	return safePlayerName(player)
}

func safePlayerName(player *common.Player) string {
	if player == nil {
		return "Unknown player"
	}
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

func cloneRoundEvents(events []entities.RoundEvent) []entities.RoundEvent {
	cloned := make([]entities.RoundEvent, len(events))
	copy(cloned, events)
	sortRoundEventsInPlace(cloned)
	return cloned
}

func sortRoundEventsInPlace(events []entities.RoundEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Tick < events[j].Tick
	})
}

func currentIngameTick(parser demoinfocs.Parser) int {
	gameState := parser.GameState()
	if gameState == nil {
		return 0
	}
	return gameState.IngameTick()
}

func currentMatchStateLabel(parser demoinfocs.Parser) string {
	gameState := parser.GameState()
	if gameState == nil {
		return ""
	}

	ctAlive := countAlivePlayers(gameState.TeamCounterTerrorists())
	tAlive := countAlivePlayers(gameState.TeamTerrorists())
	if ctAlive == 0 && tAlive == 0 {
		return ""
	}

	return formatMatchStateLabel(tAlive, ctAlive)
}

func countAlivePlayers(teamState *common.TeamState) int {
	if teamState == nil {
		return 0
	}

	count := 0
	for _, player := range teamState.Members() {
		if player == nil || player.IsUnknown || !player.IsAlive() {
			continue
		}
		count++
	}
	return count
}

func roundTimeLabel(parser demoinfocs.Parser, state *roundState) string {
	return formatRoundClock(state.LiveStartTick, currentIngameTick(parser), parser.TickRate())
}

func formatRoundClock(liveStartTick int, eventTick int, tickRate float64) string {
	if tickRate <= 0 {
		tickRate = 64
	}
	if eventTick <= 0 {
		return formatClockSeconds(defaultRoundTimeSeconds)
	}
	if liveStartTick <= 0 {
		return formatClockSeconds(defaultRoundTimeSeconds)
	}

	elapsedTicks := eventTick - liveStartTick
	if elapsedTicks < 0 {
		elapsedTicks = 0
	}

	remaining := defaultRoundTimeSeconds - int(float64(elapsedTicks)/tickRate)
	if remaining < 0 {
		remaining = 0
	}
	return formatClockSeconds(remaining)
}

func formatClockSeconds(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

func playerTeam(player *common.Player) common.Team {
	if player == nil {
		return common.TeamUnassigned
	}
	return player.Team
}

func hurtWeaponLabel(e events.PlayerHurt) string {
	if e.Weapon != nil {
		if name := strings.TrimSpace(e.Weapon.String()); name != "" && name != "Unknown" {
			return name
		}
	}
	if name := strings.TrimSpace(e.WeaponString); name != "" {
		return name
	}
	return "Unknown weapon"
}

func bombSiteLabel(site events.Bombsite) string {
	switch site {
	case events.BombsiteA:
		return "A"
	case events.BombsiteB:
		return "B"
	default:
		return ""
	}
}

func grenadeTypeLabel(grenadeType common.EquipmentType, fallback string) string {
	name := strings.TrimSpace(grenadeType.String())
	if name == "" || name == "EqUnknown" || name == "Unknown" {
		return fallback
	}
	return name
}

func buildBombEventDescription(action string, player *common.Player, site events.Bombsite) string {
	siteLabel := bombSiteLabel(site)
	if siteLabel != "" {
		return fmt.Sprintf("%s %s on %s", safePlayerName(player), action, siteLabel)
	}
	return fmt.Sprintf("%s %s", safePlayerName(player), action)
}

func buildGrenadeEventDescription(action string, thrower *common.Player, grenade string) string {
	if thrower == nil {
		return action
	}
	if grenade == "" {
		return fmt.Sprintf("%s %s", safePlayerName(thrower), action)
	}
	return fmt.Sprintf("%s threw a %s that %s", safePlayerName(thrower), grenade, action)
}

func roundEndReasonLabel(reason events.RoundEndReason, winner common.Team) string {
	switch reason {
	case events.RoundEndReasonTargetBombed:
		return "bomb exploded"
	case events.RoundEndReasonBombDefused:
		return "bomb defused"
	case events.RoundEndReasonTargetSaved:
		return "time expired"
	case events.RoundEndReasonCTWin, events.RoundEndReasonTerroristsWin:
		return "no enemies remaining"
	case events.RoundEndReasonTerroristsSurrender, events.RoundEndReasonCTSurrender:
		return "opponents surrendered"
	case events.RoundEndReasonDraw:
		return "round ended in a draw"
	case events.RoundEndReasonHostagesRescued:
		return "hostages rescued"
	case events.RoundEndReasonHostagesNotRescued:
		return "hostages not rescued"
	case events.RoundEndReasonTerroristsPlanted:
		return "bomb planted"
	default:
		if isCompetitiveTeam(winner) {
			return "won the round"
		}
		return "round ended"
	}
}

func buildRoundEndDescription(winner common.Team, reason events.RoundEndReason) string {
	winnerLabel := teamName(winner)
	reasonLabel := roundEndReasonLabel(reason, winner)
	if reasonLabel == "won the round" || reasonLabel == "round ended" {
		return fmt.Sprintf("%s won the round", winnerLabel)
	}
	return fmt.Sprintf("%s won the round (%s)", winnerLabel, reasonLabel)
}

func formatMatchStateLabel(tAlive int, ctAlive int) string {
	if tAlive < 0 {
		tAlive = 0
	}
	if ctAlive < 0 {
		ctAlive = 0
	}
	return fmt.Sprintf("T %dv%d CT", tAlive, ctAlive)
}

func annotateRoundEventsWithMatchState(events []entities.RoundEvent, ctPlayers int, tPlayers int) {
	if len(events) == 0 {
		return
	}
	if ctPlayers <= 0 {
		ctPlayers = 5
	}
	if tPlayers <= 0 {
		tPlayers = 5
	}

	ctAlive := ctPlayers
	tAlive := tPlayers
	sortRoundEventsInPlace(events)

	for i := range events {
		if events[i].EventType == "kill" {
			switch events[i].Team {
			case "T":
				if ctAlive > 0 {
					ctAlive--
				}
			case "CT":
				if tAlive > 0 {
					tAlive--
				}
			}
		}
		events[i].MatchState = formatMatchStateLabel(tAlive, ctAlive)
	}
}

func annotateRoundEventsWithWinProbabilities(events []entities.RoundEvent, ctPlayers []entities.RoundPlayerMoney, tPlayers []entities.RoundPlayerMoney) {
	if len(events) == 0 {
		return
	}

	ctAlivePlayers := make(map[string]entities.RoundPlayerMoney, len(ctPlayers))
	for _, player := range ctPlayers {
		ctAlivePlayers[player.PlayerName] = player
	}
	tAlivePlayers := make(map[string]entities.RoundPlayerMoney, len(tPlayers))
	for _, player := range tPlayers {
		tAlivePlayers[player.PlayerName] = player
	}

	ctAlive := len(ctPlayers)
	tAlive := len(tPlayers)
	if ctAlive <= 0 {
		ctAlive = 5
	}
	if tAlive <= 0 {
		tAlive = 5
	}

	bombPlanted := false
	bombDropped := false
	defuseInProgress := false

	sortRoundEventsInPlace(events)

	for i := range events {
		event := &events[i]
		description := strings.ToLower(event.Description)

		switch event.EventType {
		case "kill":
			applyKillToAliveState(*event, ctAlivePlayers, tAlivePlayers, &ctAlive, &tAlive)
			if event.Team == "T" {
				defuseInProgress = false
			}
		case "plant":
			bombDropped = false
			if !strings.Contains(description, "starting plant") {
				bombPlanted = true
			}
		case "defuse":
			if strings.Contains(description, "started defusing") {
				defuseInProgress = true
			}
			if strings.Contains(description, "defused the bomb") {
				bombPlanted = false
				defuseInProgress = false
			}
		case "bomb":
			switch {
			case strings.Contains(description, "dropped the bomb"):
				bombDropped = true
			case strings.Contains(description, "picked up the bomb"):
				bombDropped = false
			case strings.Contains(description, "detonated the bomb"):
				bombPlanted = false
				defuseInProgress = false
			}
		case "result":
			if event.Team == "CT" {
				event.CTWinProbability = 100
				event.TWinProbability = 0
				event.MatchState = formatMatchStateLabel(tAlive, ctAlive)
				continue
			}
			if event.Team == "T" {
				event.CTWinProbability = 0
				event.TWinProbability = 100
				event.MatchState = formatMatchStateLabel(tAlive, ctAlive)
				continue
			}
		}

		event.MatchState = formatMatchStateLabel(tAlive, ctAlive)
		event.CTWinProbability, event.TWinProbability = estimateRoundWinProbabilities(ctAlivePlayers, tAlivePlayers, ctAlive, tAlive, event.TimeLabel, bombPlanted, bombDropped, defuseInProgress)
	}
}

func applyKillToAliveState(event entities.RoundEvent, ctAlivePlayers map[string]entities.RoundPlayerMoney, tAlivePlayers map[string]entities.RoundPlayerMoney, ctAlive *int, tAlive *int) {
	target := strings.TrimSpace(event.TargetName)
	if target != "" {
		if _, ok := ctAlivePlayers[target]; ok {
			delete(ctAlivePlayers, target)
			if *ctAlive > 0 {
				*ctAlive = *ctAlive - 1
			}
			return
		}
		if _, ok := tAlivePlayers[target]; ok {
			delete(tAlivePlayers, target)
			if *tAlive > 0 {
				*tAlive = *tAlive - 1
			}
			return
		}
	}

	switch event.Team {
	case "T":
		if *ctAlive > 0 {
			*ctAlive = *ctAlive - 1
		}
		removeAnyAlivePlayer(ctAlivePlayers)
	case "CT":
		if *tAlive > 0 {
			*tAlive = *tAlive - 1
		}
		removeAnyAlivePlayer(tAlivePlayers)
	}
}

func removeAnyAlivePlayer(players map[string]entities.RoundPlayerMoney) {
	for playerName := range players {
		delete(players, playerName)
		return
	}
}

func estimateRoundWinProbabilities(ctAlivePlayers map[string]entities.RoundPlayerMoney, tAlivePlayers map[string]entities.RoundPlayerMoney, ctAlive int, tAlive int, timeLabel string, bombPlanted bool, bombDropped bool, defuseInProgress bool) (int, int) {
	if ctAlive <= 0 && tAlive <= 0 {
		return 50, 50
	}
	if tAlive <= 0 {
		return 100, 0
	}
	if ctAlive <= 0 {
		return 0, 100
	}

	ctScore := aliveTeamStrength(ctAlivePlayers, ctAlive, "CT")
	tScore := aliveTeamStrength(tAlivePlayers, tAlive, "T")
	timeLeft := parseRoundClockSeconds(timeLabel)

	if bombPlanted {
		tScore += 10
		if timeLeft <= 25 {
			tScore += 8
		}
		if timeLeft <= 15 {
			tScore += 6
		}
		if defuseInProgress {
			ctScore += 12
		}
	} else {
		if timeLeft <= 35 {
			ctScore += 2
		}
		if timeLeft <= 20 {
			ctScore += 8
		}
		if timeLeft <= 10 {
			ctScore += 10
		}
		if bombDropped {
			ctScore += 5
		}
	}

	total := ctScore + tScore
	if total <= 0 {
		return 50, 50
	}

	ctWin := int(math.Round((ctScore / total) * 100))
	if ctWin < 0 {
		ctWin = 0
	}
	if ctWin > 100 {
		ctWin = 100
	}
	tWin := 100 - ctWin

	if ctWin > 0 && tWin > 0 {
		if ctWin < 5 {
			ctWin, tWin = 5, 95
		} else if tWin < 5 {
			ctWin, tWin = 95, 5
		}
	}

	return ctWin, tWin
}

func aliveTeamStrength(alivePlayers map[string]entities.RoundPlayerMoney, aliveCount int, team string) float64 {
	if aliveCount <= 0 {
		return 0
	}

	score := 0.0
	counted := 0
	for _, player := range alivePlayers {
		score += playerWinStrength(player, team)
		counted++
	}
	for counted < aliveCount {
		score += playerWinStrength(entities.RoundPlayerMoney{}, team)
		counted++
	}
	return score
}

func playerWinStrength(player entities.RoundPlayerMoney, team string) float64 {
	score := 18.0
	weapon := strings.ToLower(strings.TrimSpace(player.MainWeapon))
	switch {
	case weapon == "":
		score += 2
	case strings.Contains(weapon, "awp"):
		score += 11
	case strings.Contains(weapon, "ak-47"), strings.Contains(weapon, "m4a1"), strings.Contains(weapon, "m4a4"), strings.Contains(weapon, "aug"), strings.Contains(weapon, "sg 553"), strings.Contains(weapon, "famas"), strings.Contains(weapon, "galil"):
		score += 8.5
	case strings.Contains(weapon, "ssg 08"):
		score += 7
	case strings.Contains(weapon, "mac-10"), strings.Contains(weapon, "mp9"), strings.Contains(weapon, "mp7"), strings.Contains(weapon, "ump"), strings.Contains(weapon, "p90"):
		score += 5.5
	case strings.Contains(weapon, "nova"), strings.Contains(weapon, "xm1014"), strings.Contains(weapon, "mag-7"), strings.Contains(weapon, "sawed-off"), strings.Contains(weapon, "m249"), strings.Contains(weapon, "negev"):
		score += 4.5
	case strings.Contains(weapon, "deagle"), strings.Contains(weapon, "desert eagle"), strings.Contains(weapon, "tec-9"), strings.Contains(weapon, "five-seven"), strings.Contains(weapon, "cz75"), strings.Contains(weapon, "p250"):
		score += 3
	default:
		score += 1.5
	}

	switch player.Armor {
	case "Armor + Helmet":
		score += 3.5
	case "Armor":
		score += 2
	}

	score += math.Min(float64(player.Utility), 4) * 0.45
	if team == "CT" && strings.Contains(strings.ToLower(player.Armor), "helmet") {
		score += 0.25
	}

	return score
}

func parseRoundClockSeconds(label string) int {
	label = strings.TrimSpace(label)
	if label == "" {
		return defaultRoundTimeSeconds
	}

	parts := strings.Split(label, ":")
	if len(parts) != 2 {
		return defaultRoundTimeSeconds
	}

	minutes, errMin := strconv.Atoi(parts[0])
	seconds, errSec := strconv.Atoi(parts[1])
	if errMin != nil || errSec != nil {
		return defaultRoundTimeSeconds
	}

	total := minutes*60 + seconds
	if total < 0 {
		return 0
	}
	return total
}

func isNoteworthyDamage(e events.PlayerHurt) bool {
	if e.HealthDamageTaken >= 40 {
		return true
	}
	if e.Health <= 25 {
		return true
	}
	return e.HitGroup == events.HitGroupHead
}

func buildDamageEventDescription(e events.PlayerHurt) string {
	text := fmt.Sprintf("%s hit %s for %d damage with %s", safePlayerName(e.Attacker), safePlayerName(e.Player), e.HealthDamageTaken, hurtWeaponLabel(e))
	if e.HitGroup == events.HitGroupHead {
		text += " (headshot)"
	}
	return text
}

func buildKillEventDescription(e events.Kill, isEntry bool, isTrade bool, assistantName string) string {
	action := "eliminated"
	switch {
	case isEntry:
		action = "entry fragged"
	case isTrade:
		action = "traded"
	}

	text := fmt.Sprintf("%s %s %s with %s", safePlayerName(e.Killer), action, safePlayerName(e.Victim), killWeaponLabel(e))
	extra := make([]string, 0, 5)
	if e.IsHeadshot {
		extra = append(extra, "headshot")
	}
	if e.IsWallBang() {
		extra = append(extra, "wallbang")
	}
	if e.ThroughSmoke {
		extra = append(extra, "through smoke")
	}
	if e.NoScope {
		extra = append(extra, "noscope")
	}
	if e.AssistedFlash && assistantName != "" {
		extra = append(extra, fmt.Sprintf("flash assist: %s", assistantName))
	} else if assistantName != "" {
		extra = append(extra, fmt.Sprintf("assist: %s", assistantName))
	}
	if len(extra) > 0 {
		text += fmt.Sprintf(" (%s)", strings.Join(extra, ", "))
	}
	return text
}

func newRoundEvent(tick int, timeLabel string, eventType string, team string, actorName string, targetName string, assistantName string, weapon string, site string, x float64, y float64, z float64, isEntry bool, isTrade bool, isHeadshot bool, isWallbang bool, throughSmoke bool, description string) entities.RoundEvent {
	return entities.RoundEvent{
		Tick:          tick,
		TimeLabel:     timeLabel,
		EventType:     eventType,
		Description:   description,
		Team:          team,
		ActorName:     actorName,
		TargetName:    targetName,
		AssistantName: assistantName,
		Weapon:        weapon,
		Site:          site,
		X:             x,
		Y:             y,
		Z:             z,
		IsEntry:       isEntry,
		IsTrade:       isTrade,
		IsHeadshot:    isHeadshot,
		IsWallbang:    isWallbang,
		ThroughSmoke:  throughSmoke,
	}
}

func appendRoundEndEvent(state *roundState, winner common.Team, reason events.RoundEndReason, tick int, timeLabel string) {
	if state == nil || !isCompetitiveTeam(winner) {
		return
	}

	if timeLabel == "" {
		timeLabel = formatClockSeconds(0)
	}

	maxTick := tick
	for _, event := range state.RoundEvents {
		if event.Tick > maxTick {
			maxTick = event.Tick
		}
	}

	state.RoundEvents = append(state.RoundEvents, newRoundEvent(
		maxTick+roundResultTickOffset,
		timeLabel,
		"result",
		teamName(winner),
		"",
		"",
		"",
		"",
		"",
		0,
		0,
		0,
		false,
		false,
		false,
		false,
		false,
		buildRoundEndDescription(winner, reason),
	))
}

func appendPrunedRecentDeaths(existing []recentDeathEvent, latest recentDeathEvent, currentTick int, tickRate int) []recentDeathEvent {
	if tickRate <= 0 {
		tickRate = 64
	}
	window := tickRate * tradeWindowSeconds
	kept := make([]recentDeathEvent, 0, len(existing)+1)
	for _, item := range existing {
		if currentTick-item.Tick <= window {
			kept = append(kept, item)
		}
	}
	return append(kept, latest)
}

func isTradeKill(existing []recentDeathEvent, currentTick int, tickRate int, killer *common.Player, victim *common.Player) bool {
	if killer == nil || victim == nil {
		return false
	}
	if tickRate <= 0 {
		tickRate = 64
	}
	window := tickRate * tradeWindowSeconds
	killerID := playerID(killer)
	victimID := playerID(victim)
	for _, item := range existing {
		if currentTick-item.Tick > window {
			continue
		}
		if item.KillerID == victimID && item.VictimTeam == killer.Team && item.VictimID != killerID {
			return true
		}
	}
	return false
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

	roundEvents := cloneRoundEvents(state.RoundEvents)
	annotateRoundEventsWithMatchState(roundEvents, len(state.CTMoneyByPlayer), len(state.TMoneyByPlayer))
	annotateRoundEventsWithWinProbabilities(roundEvents, state.CTMoneyByPlayer, state.TMoneyByPlayer)

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
		Events:          roundEvents,
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

func commitRoundResult(state *roundState, players map[string]*playerAggregate, winner common.Team, reason events.RoundEndReason, tick int, timeLabel string) bool {
	if !isCompetitiveTeam(winner) {
		return false
	}

	nextRoundNumber := state.CTWins + state.TWins + 1
	if nextRoundNumber > maxRoundHistoryEntries {
		return false
	}

	appendRoundEndEvent(state, winner, reason, tick, timeLabel)

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

	_ = commitRoundResult(state, players, winner, state.PendingEndReason, state.PendingEndTick, state.PendingEndTime)

	state.ActiveRound = false
	state.PendingWinner = common.TeamUnassigned
}
