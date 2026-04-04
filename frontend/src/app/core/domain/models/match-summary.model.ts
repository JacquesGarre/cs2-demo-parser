export interface PlayerSummary {
  playerName: string;
  team: string;
  kills: number;
  deaths: number;
  assists: number;
  kdRatio: number;
  killsPerRound: number;
  hsPercentage: number;
  adr: number;
  kast: number;
  openingDuels: number;
  openingWins: number;
  tradeKills: number;
  tradeDeaths: number;
  clutchWon: number;
  clutchPlayed: number;
  twoKs: number;
  threeKs: number;
  fourKs: number;
  fiveKs: number;
  bestRoundPlayerCount: number;
  entryKillRate: number;
}

export interface HeatPoint {
  x: number;
  y: number;
  z?: number;
  count: number;
  roundNumber?: number;
  side?: string;
  killerSide?: string;
  victimSide?: string;
  killerName?: string;
  victimName?: string;
  killWeapon?: string;
  victimWeapon?: string;
}

export interface PlayerHeatmap {
  playerName: string;
  kills: HeatPoint[];
  deaths: HeatPoint[];
}

export interface RoundPerformance {
  playerName: string;
  team: string;
  kills: number;
  damage: number;
  label: string;
  killDetails?: RoundKillDetail[];
}

export interface RoundKillDetail {
  victimName: string;
  weapon: string;
}

export interface RoundMVP {
  playerName: string;
  team: string;
  reason: string;
  kills: number;
  damage: number;
}

export interface RoundPlayerMoney {
  playerName: string;
  money: number;
  economy?: string;
  mainWeapon?: string;
  utility?: number;
  armor?: string;
}

export interface RoundEvent {
  tick: number;
  timeLabel?: string;
  eventType: string;
  description: string;
  team?: string;
  actorName?: string;
  targetName?: string;
  assistantName?: string;
  weapon?: string;
  site?: string;
  locationLabel?: string;
  x?: number;
  y?: number;
  z?: number;
  isEntry?: boolean;
  isTrade?: boolean;
  isHeadshot?: boolean;
  isWallbang?: boolean;
  throughSmoke?: boolean;
}

export interface RoundSummary {
  roundNumber: number;
  winnerTeam: string;
  bestPlayer?: RoundPerformance;
  mvp?: RoundMVP;
  playerDamages?: RoundPerformance[];
  ctEconomy?: string;
  tEconomy?: string;
  ctStartMoney?: number;
  tStartMoney?: number;
  ctMoneyByPlayer?: RoundPlayerMoney[];
  tMoneyByPlayer?: RoundPlayerMoney[];
  multiKills: RoundPerformance[];
  events?: RoundEvent[];
}

export interface MatchSummary {
  demoId: string;
  mapName: string;
  rounds: number;
  teamAScore: number;
  teamBScore: number;
  teamAName: string;
  teamBName: string;
  playerStats: PlayerSummary[];
  killHeatmap: HeatPoint[];
  deathHeatmap: HeatPoint[];
  playerHeatmaps: PlayerHeatmap[];
  roundHistory: RoundSummary[];
  analysisSource: string;
}
