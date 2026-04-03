package processing

import (
	"sort"
	"strings"

	"github.com/jcqsg/cs2-demos/backend/internal/domain/entities"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
)

type teamLoadoutSnapshot struct {
	Players           int
	RiflePlayers      int
	SMGPlayers        int
	HeavyPlayers      int
	PistolOnlyPlayers int
	ArmorPlayers      int
	FullArmorPlayers  int
	DefuseKits        int
	TotalGrenades     int
	TeamEquipValue    int
}

func classPriority(class common.EquipmentClass) int {
	switch class {
	case common.EqClassRifle:
		return 4
	case common.EqClassHeavy:
		return 3
	case common.EqClassSMG:
		return 2
	case common.EqClassPistols:
		return 1
	default:
		return 0
	}
}

func classifyTeamEconomy(roundNumber int, team common.Team, teamStartMoney int, players []entities.RoundPlayerMoney, snapshot teamLoadoutSnapshot, history []entities.RoundSummary) string {
	_ = team
	_ = snapshot
	_ = history

	if len(players) == 0 {
		return economyFromAverageMoney(teamStartMoney, 5)
	}

	return teamEconomyFromPlayerLabels(players)
}

func isPistolRound(roundNumber int) bool {
	return roundNumber == 1 || roundNumber == 13
}

func isBonusRound(team common.Team, snapshot teamLoadoutSnapshot, history []entities.RoundSummary) bool {
	if len(history) == 0 {
		return false
	}

	lastRound := history[len(history)-1]
	if lastRound.WinnerTeam != teamName(team) {
		return false
	}

	opponentEconomy := lastRound.CTEconomy
	if team == common.TeamCounterTerrorists {
		opponentEconomy = lastRound.TEconomy
	}

	if opponentEconomy != "Eco round" && opponentEconomy != "Small buy" {
		return false
	}

	if snapshot.SMGPlayers < 2 {
		return false
	}

	return snapshot.RiflePlayers <= 2
}

func isSecondRoundAfterPistolWin(roundNumber int, team common.Team, history []entities.RoundSummary) bool {
	if roundNumber != 2 && roundNumber != 14 {
		return false
	}

	if len(history) == 0 {
		return false
	}

	lastRound := history[len(history)-1]
	return lastRound.WinnerTeam == teamName(team)
}

func economyFromAverageMoney(totalMoney int, playerCount int) string {
	if playerCount <= 0 {
		if totalMoney <= 0 {
			return "Eco round"
		}
		playerCount = 5
	}

	avgMoney := totalMoney / playerCount
	switch {
	case avgMoney >= 4000:
		return "Full buy"
	case avgMoney >= 3000:
		return "Small buy"
	case avgMoney >= 1500:
		return "Force buy"
	default:
		return "Eco round"
	}
}

func teamEconomyFromPlayerLabels(players []entities.RoundPlayerMoney) string {
	if len(players) == 0 {
		return "Unknown"
	}

	counts := map[string]int{}
	for _, player := range players {
		label := normalizeEconomyLabel(player.Economy)
		counts[label]++
	}

	ordered := []string{"Pistol round", "Full buy", "Small buy", "Force buy", "Eco round"}
	for _, label := range ordered {
		if counts[label] >= 3 {
			return label
		}
	}

	bestLabel := "Unknown"
	bestCount := -1
	for _, label := range ordered {
		if counts[label] > bestCount {
			bestCount = counts[label]
			bestLabel = label
		}
	}

	return bestLabel
}

func normalizeEconomyLabel(label string) string {
	switch label {
	case "Pistol round", "Full buy", "Small buy", "Force buy", "Eco round":
		return label
	default:
		return "Force buy"
	}
}

func classifyPlayerEconomy(roundNumber int, team common.Team, money int, hasSMG bool, hasRifleOrAWP bool, hasHeavy bool, armor string, utility int, history []entities.RoundSummary) string {
	if isPistolRound(roundNumber) {
		return "Pistol round"
	}

	hasArmor := armor != "No armor"
	hasPrimary := hasSMG || hasRifleOrAWP || hasHeavy

	if isSecondRoundAfterPistolWin(roundNumber, team, history) {
		if hasArmor && ((hasPrimary && utility > 0) || hasRifleOrAWP) {
			return "Full buy"
		}
	}

	if hasRifleOrAWP && hasArmor {
		return "Full buy"
	}

	if hasSMG {
		if money > 1500 {
			return "Small buy"
		}
		return "Force buy"
	}

	if hasRifleOrAWP {
		if money > 1500 {
			return "Small buy"
		}
		return "Force buy"
	}

	if !hasArmor || utility == 0 {
		if money > 1500 {
			return "Eco round"
		}
		return "Force buy"
	}

	if isSecondRoundAfterPistolWin(roundNumber, team, history) && money > 1500 {
		return "Small buy"
	}

	if money > 1500 {
		return "Eco round"
	}

	return "Force buy"
}

func captureTeamMoneySnapshot(roundNumber int, team common.Team, teamState *common.TeamState, history []entities.RoundSummary) (int, []entities.RoundPlayerMoney) {
	if teamState == nil {
		return 0, nil
	}

	members := teamState.Members()
	total := 0
	details := make([]entities.RoundPlayerMoney, 0, len(members))

	for _, player := range members {
		if player == nil || player.IsUnknown || player.IsBot {
			continue
		}

		money := player.Money()
		total += money
		weapon, hasSMG, hasRifleOrAWP, hasHeavy := mainWeaponSnapshot(player)
		util := grenadeCount(player)
		armor := armorLabel(player)
		details = append(details, entities.RoundPlayerMoney{
			PlayerName: safePlayerName(player),
			Money:      money,
			Economy:    classifyPlayerEconomy(roundNumber, team, money, hasSMG, hasRifleOrAWP, hasHeavy, armor, util, history),
			MainWeapon: weapon,
			Utility:    util,
			Armor:      armor,
		})
	}

	sort.Slice(details, func(i, j int) bool {
		if details[i].Money == details[j].Money {
			return details[i].PlayerName < details[j].PlayerName
		}
		return details[i].Money > details[j].Money
	})

	return total, details
}

func captureTeamTotalMoney(teamState *common.TeamState) int {
	if teamState == nil {
		return 0
	}

	total := 0
	for _, player := range teamState.Members() {
		if player == nil || player.IsUnknown || player.IsBot {
			continue
		}
		total += player.Money()
	}

	return total
}

func mainWeaponSnapshot(player *common.Player) (string, bool, bool, bool) {
	if player == nil {
		return "Unknown", false, false, false
	}

	bestPriority := -1
	bestClass := common.EqClassUnknown
	bestName := ""
	hasSMG := false
	hasRifleOrAWP := false
	hasHeavy := false

	for _, weapon := range player.Weapons() {
		if weapon == nil {
			continue
		}

		class := weapon.Class()
		if class == common.EqClassGrenade || class == common.EqClassEquipment {
			continue
		}

		name := strings.TrimSpace(weapon.String())
		priority := classPriority(class)
		if strings.EqualFold(name, "awp") {
			priority = 5
			hasRifleOrAWP = true
		}

		if class == common.EqClassRifle {
			hasRifleOrAWP = true
		}
		if class == common.EqClassSMG {
			hasSMG = true
		}
		if class == common.EqClassHeavy {
			hasHeavy = true
		}

		if priority > bestPriority {
			bestPriority = priority
			bestClass = class
			bestName = name
		}
	}

	if bestPriority < 0 {
		return "Pistol (None)", hasSMG, hasRifleOrAWP, hasHeavy
	}

	if strings.EqualFold(bestName, "awp") {
		return "AWP", hasSMG, hasRifleOrAWP, hasHeavy
	}

	if bestName == "" {
		bestName = "Unknown"
	}

	switch bestClass {
	case common.EqClassRifle:
		return "Rifle (" + bestName + ")", hasSMG, hasRifleOrAWP, hasHeavy
	case common.EqClassSMG:
		return "SMG (" + bestName + ")", hasSMG, hasRifleOrAWP, hasHeavy
	case common.EqClassHeavy:
		return "Heavy (" + bestName + ")", hasSMG, hasRifleOrAWP, hasHeavy
	default:
		return "Pistol (" + bestName + ")", hasSMG, hasRifleOrAWP, hasHeavy
	}
}

func grenadeCount(player *common.Player) int {
	if player == nil {
		return 0
	}

	count := 0
	for _, weapon := range player.Weapons() {
		if weapon == nil {
			continue
		}
		if weapon.Class() == common.EqClassGrenade {
			count++
		}
	}

	return count
}

func armorLabel(player *common.Player) string {
	if player == nil || player.Armor() <= 0 {
		return "No armor"
	}

	if player.HasHelmet() {
		return "Armor + Helmet"
	}

	return "Armor"
}

func normalizeRoundHistory(history []entities.RoundSummary, expectedRounds int) []entities.RoundSummary {
	if expectedRounds <= 0 {
		return history
	}

	if len(history) > expectedRounds {
		history = history[len(history)-expectedRounds:]
	}

	for i := range history {
		history[i].RoundNumber = i + 1
		history[i].CTMoneyByPlayer = normalizePlayerMoneyEconomies(history[i].CTMoneyByPlayer, history[i].RoundNumber, common.TeamCounterTerrorists, history, i)
		history[i].TMoneyByPlayer = normalizePlayerMoneyEconomies(history[i].TMoneyByPlayer, history[i].RoundNumber, common.TeamTerrorists, history, i)
		history[i].CTEconomy = classifyTeamEconomy(history[i].RoundNumber, common.TeamCounterTerrorists, history[i].CTStartMoney, history[i].CTMoneyByPlayer, teamLoadoutSnapshot{}, history)
		history[i].TEconomy = classifyTeamEconomy(history[i].RoundNumber, common.TeamTerrorists, history[i].TStartMoney, history[i].TMoneyByPlayer, teamLoadoutSnapshot{}, history)
	}

	return history
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func normalizePlayerMoneyEconomies(players []entities.RoundPlayerMoney, roundNumber int, team common.Team, history []entities.RoundSummary, roundIndex int) []entities.RoundPlayerMoney {
	if len(players) == 0 {
		return players
	}

	for i := range players {
		players[i].Economy = normalizePlayerEconomyLabel(players[i], roundNumber, team, history, roundIndex)
	}

	return players
}

func normalizePlayerEconomyLabel(player entities.RoundPlayerMoney, roundNumber int, team common.Team, history []entities.RoundSummary, roundIndex int) string {
	if isPistolRound(roundNumber) {
		return "Pistol round"
	}

	hasSMG := strings.HasPrefix(player.MainWeapon, "SMG (")
	hasRifle := strings.HasPrefix(player.MainWeapon, "Rifle (") || strings.EqualFold(player.MainWeapon, "AWP")
	hasHeavy := strings.HasPrefix(player.MainWeapon, "Heavy (")
	hasArmor := player.Armor != "No armor"
	hasUtility := player.Utility > 0

	if roundNumber == 2 || roundNumber == 14 {
		if roundIndex > 0 && history[roundIndex-1].WinnerTeam == teamName(team) {
			if hasArmor && ((hasSMG || hasRifle || hasHeavy) && hasUtility || hasRifle) {
				return "Full buy"
			}
		}
	}

	if hasRifle && hasArmor {
		return "Full buy"
	}

	if hasSMG || hasHeavy || hasRifle {
		if player.Money > 1500 {
			return "Small buy"
		}
		return "Force buy"
	}

	if !hasArmor || !hasUtility {
		if player.Money > 1500 {
			return "Eco round"
		}
		return "Force buy"
	}

	if player.Money > 1500 {
		return "Eco round"
	}

	return "Force buy"
}
