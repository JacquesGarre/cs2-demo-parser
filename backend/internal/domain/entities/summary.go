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
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Z            float64 `json:"z,omitempty"`
	Count        int     `json:"count"`
	RoundNumber  int     `json:"roundNumber,omitempty"`
	Side         string  `json:"side,omitempty"`
	KillerSide   string  `json:"killerSide,omitempty"`
	VictimSide   string  `json:"victimSide,omitempty"`
	KillerName   string  `json:"killerName,omitempty"`
	VictimName   string  `json:"victimName,omitempty"`
	KillWeapon   string  `json:"killWeapon,omitempty"`
	VictimWeapon string  `json:"victimWeapon,omitempty"`
}

type PlayerHeatmap struct {
	PlayerName string      `json:"playerName"`
	Kills      []HeatPoint `json:"kills"`
	Deaths     []HeatPoint `json:"deaths"`
}

type RoundPerformance struct {
	PlayerName  string            `json:"playerName"`
	Team        string            `json:"team"`
	Kills       int               `json:"kills"`
	Damage      int               `json:"damage"`
	Label       string            `json:"label"`
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

type RoundEvent struct {
	Tick          int     `json:"tick"`
	TimeLabel     string  `json:"timeLabel,omitempty"`
	EventType     string  `json:"eventType"`
	Description   string  `json:"description"`
	Team          string  `json:"team,omitempty"`
	ActorName     string  `json:"actorName,omitempty"`
	TargetName    string  `json:"targetName,omitempty"`
	AssistantName string  `json:"assistantName,omitempty"`
	Weapon        string  `json:"weapon,omitempty"`
	Site          string  `json:"site,omitempty"`
	LocationLabel string  `json:"locationLabel,omitempty"`
	X             float64 `json:"x,omitempty"`
	Y             float64 `json:"y,omitempty"`
	Z             float64 `json:"z,omitempty"`
	IsEntry       bool    `json:"isEntry,omitempty"`
	IsTrade       bool    `json:"isTrade,omitempty"`
	IsHeadshot    bool    `json:"isHeadshot,omitempty"`
	IsWallbang    bool    `json:"isWallbang,omitempty"`
	ThroughSmoke  bool    `json:"throughSmoke,omitempty"`
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
	Events          []RoundEvent       `json:"events,omitempty"`
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
