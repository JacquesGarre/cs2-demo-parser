package entities

type PlayerSummary struct {
	PlayerName           string  `json:"playerName"`
	Team                 string  `json:"team"`
	Kills                int     `json:"kills"`
	Deaths               int     `json:"deaths"`
	Assists              int     `json:"assists"`
	KDRatio              float64 `json:"kdRatio"`
	KillsPerRound        float64 `json:"killsPerRound"`
	HSPercentage         float64 `json:"hsPercentage"`
	ADR                  float64 `json:"adr"`
	KAST                 float64 `json:"kast"`
	OpeningDuels         int     `json:"openingDuels"`
	OpeningWins          int     `json:"openingWins"`
	TradeKills           int     `json:"tradeKills"`
	TradeDeaths          int     `json:"tradeDeaths"`
	ClutchWon            int     `json:"clutchWon"`
	ClutchPlayed         int     `json:"clutchPlayed"`
	TwoKs                int     `json:"twoKs"`
	ThreeKs              int     `json:"threeKs"`
	FourKs               int     `json:"fourKs"`
	FiveKs               int     `json:"fiveKs"`
	BestRoundPlayerCount int     `json:"bestRoundPlayerCount"`
	EntryKillRate        float64 `json:"entryKillRate"`
}

type HeatPoint struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Count int     `json:"count"`
}

type PlayerHeatmap struct {
	PlayerName string      `json:"playerName"`
	Kills      []HeatPoint `json:"kills"`
	Deaths     []HeatPoint `json:"deaths"`
}

type RoundPerformance struct {
	PlayerName string `json:"playerName"`
	Team       string `json:"team"`
	Kills      int    `json:"kills"`
	Damage     int    `json:"damage"`
	Label      string `json:"label"`
	KillDetails []RoundKillDetail `json:"killDetails,omitempty"`
}

type RoundKillDetail struct {
	VictimName string `json:"victimName"`
	Weapon     string `json:"weapon"`
}

type RoundMVP struct {
	PlayerName string `json:"playerName"`
	Team       string `json:"team"`
	Reason     string `json:"reason"`
	Kills      int    `json:"kills,omitempty"`
	Damage     int    `json:"damage,omitempty"`
}

type RoundPlayerMoney struct {
	PlayerName string `json:"playerName"`
	Money      int    `json:"money"`
	Economy    string `json:"economy,omitempty"`
	MainWeapon string `json:"mainWeapon,omitempty"`
	Utility    int    `json:"utility,omitempty"`
	Armor      string `json:"armor,omitempty"`
}

type RoundSummary struct {
	RoundNumber     int                `json:"roundNumber"`
	WinnerTeam      string             `json:"winnerTeam"`
	BestPlayer      *RoundPerformance  `json:"bestPlayer,omitempty"`
	MVP             *RoundMVP          `json:"mvp,omitempty"`
	PlayerDamages   []RoundPerformance `json:"playerDamages,omitempty"`
	CTEconomy       string             `json:"ctEconomy,omitempty"`
	TEconomy        string             `json:"tEconomy,omitempty"`
	CTStartMoney    int                `json:"ctStartMoney,omitempty"`
	TStartMoney     int                `json:"tStartMoney,omitempty"`
	CTMoneyByPlayer []RoundPlayerMoney `json:"ctMoneyByPlayer,omitempty"`
	TMoneyByPlayer  []RoundPlayerMoney `json:"tMoneyByPlayer,omitempty"`
	MultiKills      []RoundPerformance `json:"multiKills"`
}

type MatchSummary struct {
	DemoID         string          `json:"demoId"`
	MapName        string          `json:"mapName"`
	Rounds         int             `json:"rounds"`
	TeamAScore     int             `json:"teamAScore"`
	TeamBScore     int             `json:"teamBScore"`
	TeamAName      string          `json:"teamAName"`
	TeamBName      string          `json:"teamBName"`
	PlayerStats    []PlayerSummary `json:"playerStats"`
	KillHeatmap    []HeatPoint     `json:"killHeatmap"`
	DeathHeatmap   []HeatPoint     `json:"deathHeatmap"`
	PlayerHeatmaps []PlayerHeatmap `json:"playerHeatmaps"`
	RoundHistory   []RoundSummary  `json:"roundHistory"`
	AnalysisSource string          `json:"analysisSource"`
}
