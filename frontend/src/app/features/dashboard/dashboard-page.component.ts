import { CommonModule, DecimalPipe } from '@angular/common';
import { AfterViewInit, Component, ElementRef, OnDestroy, OnInit, Renderer2, ViewChild, inject, signal } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { Subscription, interval, switchMap } from 'rxjs';
import { Chart, registerables } from 'chart.js';
import { GetJobStatusUseCase } from '../../core/application/use-cases/get-job-status.use-case';
import { GetMatchSummaryUseCase } from '../../core/application/use-cases/get-match-summary.use-case';
import { AnalysisJob } from '../../core/domain/models/analysis-job.model';
import { HeatPoint, MatchSummary, PlayerSummary, RoundPerformance, RoundPlayerMoney } from '../../core/domain/models/match-summary.model';
import { RadarMinimapComponent } from './components/radar-minimap/radar-minimap.component';

type PlayerStatColumn = {
  label: string;
  value: (player: PlayerSummary) => string;
};

type TimelineWinner = 'TEAM A' | 'TEAM B' | 'UNKNOWN';

type TimelineRound = {
  roundNumber: number;
  winner: TimelineWinner;
  winnerSide: 'left' | 'right' | 'center';
  winnerRole: 'CT' | 'T' | 'UNKNOWN';
  mvpLabel: string;
  teamAEconomy: string;
  teamBEconomy: string;
  teamAMoneyLabel: string;
  teamBMoneyLabel: string;
  teamAMoneyDetails: RoundPlayerMoney[];
  teamBMoneyDetails: RoundPlayerMoney[];
  teamAMultiKills: RoundPerformance[];
  teamBMultiKills: RoundPerformance[];
};

type PlayerDamageRound = {
  roundNumber: number;
  damage: number;
};

type StandoutRound = {
  roundNumber: number;
  side: 'CT' | 'T';
  title: string;
  details: string[];
  score: number;
};

Chart.register(...registerables);

@Component({
  selector: 'app-dashboard-page',
  imports: [CommonModule, DecimalPipe, RadarMinimapComponent],
  templateUrl: './dashboard-page.component.html',
  styleUrl: './dashboard-page.component.scss',
})
export class DashboardPageComponent implements OnInit, AfterViewInit, OnDestroy {
  @ViewChild('mapHeader') private mapHeaderRef?: ElementRef<HTMLElement>;
  @ViewChild('damageChartCanvas') private damageChartCanvasRef?: ElementRef<HTMLCanvasElement>;

  private readonly route = inject(ActivatedRoute);
  private readonly getJobStatus = inject(GetJobStatusUseCase);
  private readonly getMatchSummary = inject(GetMatchSummaryUseCase);
  private readonly renderer = inject(Renderer2);

  readonly job = signal<AnalysisJob | null>(null);
  readonly summary = signal<MatchSummary | null>(null);
  readonly selectedPlayer = signal<PlayerSummary | null>(null);
  readonly selectedPlayerIndex = signal<number | null>(null);
  readonly isPlayerDrawerOpen = signal(false);
  readonly heatmapMode = signal<'kills' | 'deaths'>('kills');
  readonly errorMessage = signal('');
  readonly fakeProgress = signal(0);
  private progressInterval?: ReturnType<typeof setInterval>;
  readonly statColumns: PlayerStatColumn[] = [
    { label: 'K/D/A', value: (player) => `${player.kills}/${player.deaths}/${player.assists}` },
    { label: 'KD', value: (player) => player.kdRatio.toFixed(2) },
    { label: 'KPR', value: (player) => player.killsPerRound.toFixed(2) },
    { label: 'ADR', value: (player) => player.adr.toFixed(1) },
    { label: 'HS%', value: (player) => `${player.hsPercentage.toFixed(1)}%` },
    { label: '5K', value: (player) => `${player.fiveKs}` },
    { label: '4K', value: (player) => `${player.fourKs}` },
    { label: '3K', value: (player) => `${player.threeKs}` },
    { label: '2K', value: (player) => `${player.twoKs}` },
    { label: 'MVP', value: (player) => `${player.bestRoundPlayerCount}` },
  ];

  private pollingSubscription?: Subscription;
  private damageChart?: Chart;

  ngAfterViewInit(): void {
    this.applyMapBackground();
  }

  private applyMapBackground(): void {
    const el = this.mapHeaderRef?.nativeElement;
    const mapName = this.summary()?.mapName;
    if (!el || !mapName) return;
    const mapKey = this.normalizeMapKey(mapName).replace('map-', 'de_');
    this.renderer.setStyle(el, 'background-image', `url('/maps/${mapKey}.png')`);
  }

  ngOnInit(): void {
    const jobId = this.route.snapshot.queryParamMap.get('jobId') ?? '';
    const demoId = this.route.snapshot.queryParamMap.get('demoId') ?? '';

    if (!jobId || !demoId) {
      this.errorMessage.set('Missing job context. Upload a demo first.');
      return;
    }

    this.startPolling(jobId, demoId);
  }

  ngOnDestroy(): void {
    this.pollingSubscription?.unsubscribe();
    clearInterval(this.progressInterval);
    this.destroyDamageChart();
  }

  setPlayer(player: PlayerSummary): void {
    this.selectPlayer(player);
    this.queueDamageChartRender();
  }

  openPlayerBreakdown(player: PlayerSummary): void {
    this.selectPlayer(player);
    this.isPlayerDrawerOpen.set(true);
    this.queueDamageChartRender();
  }

  closePlayerBreakdown(): void {
    this.isPlayerDrawerOpen.set(false);
  }

  setPlayerByName(playerName: string): void {
    const summary = this.summary();
    if (!summary) {
      return;
    }

    const nextPlayer = summary.playerStats.find((player) => player.playerName === playerName) ?? null;
    this.selectPlayer(nextPlayer);
    this.queueDamageChartRender();
  }

  setPlayerByIndex(indexValue: string): void {
    const summary = this.summary();
    if (!summary) {
      return;
    }

    const parsedIndex = Number(indexValue);
    if (Number.isInteger(parsedIndex) && parsedIndex >= 0 && parsedIndex < summary.playerStats.length) {
      this.selectPlayer(summary.playerStats[parsedIndex]);
      this.queueDamageChartRender();
      return;
    }

    this.selectPlayer(null);
    this.queueDamageChartRender();
  }

  private selectPlayer(player: PlayerSummary | null): void {
    this.selectedPlayer.set(player);

    if (!player) {
      this.selectedPlayerIndex.set(null);
      return;
    }

    const players = this.summary()?.playerStats ?? [];
    const index = players.indexOf(player);
    if (index >= 0) {
      this.selectedPlayerIndex.set(index);
      return;
    }

    const fallbackIndex = players.findIndex(
      (candidate) => candidate.playerName === player.playerName && candidate.team === player.team,
    );
    this.selectedPlayerIndex.set(fallbackIndex >= 0 ? fallbackIndex : null);
  }

  get allPlayers(): PlayerSummary[] {
    return this.summary()?.playerStats ?? [];
  }

  setHeatmapMode(mode: 'kills' | 'deaths'): void {
    this.heatmapMode.set(mode);
  }

  mapDisplayName(mapName: string): string {
    const normalized = this.normalizeMapKey(mapName);
    return normalized.replace('map-', '').replace(/_/g, ' ').toUpperCase();
  }

  mapImageUrl(mapName: string): string {
    return `/maps/${this.normalizeMapKey(mapName)}.png`;
  }

  mapBgVar(mapName: string): string {
    return `url('/maps/${this.normalizeMapKey(mapName)}.png')`;
  }

  get teamOneLabel(): string {
    return this.teams[0] ?? 'Team A';
  }

  get teamTwoLabel(): string {
    return this.teams[1] ?? 'Team B';
  }

  get teamAPlayers(): PlayerSummary[] {
    return this.playersForTeam('TEAM A');
  }

  get teamBPlayers(): PlayerSummary[] {
    return this.playersForTeam('TEAM B');
  }

  get halftimeRound(): number {
    const rounds = this.summary()?.rounds ?? 0;
    return Math.min(12, Math.max(rounds, 0));
  }

  get teamAScore(): number {
    return this.roundTimeline.reduce((total, round) => total + (round.winner === 'TEAM A' ? 1 : 0), 0);
  }

  get teamBScore(): number {
    return this.roundTimeline.reduce((total, round) => total + (round.winner === 'TEAM B' ? 1 : 0), 0);
  }

  get halftimeTeamAScore(): number {
    return this.roundTimeline
      .filter((round) => round.roundNumber <= this.halftimeRound)
      .reduce((total, round) => total + (round.winner === 'TEAM A' ? 1 : 0), 0);
  }

  get halftimeTeamBScore(): number {
    return this.roundTimeline
      .filter((round) => round.roundNumber <= this.halftimeRound)
      .reduce((total, round) => total + (round.winner === 'TEAM B' ? 1 : 0), 0);
  }

  get roundTimeline(): TimelineRound[] {
    const summary = this.summary();
    if (!summary) {
      return [];
    }

    return summary.roundHistory.map((round) => {
      const winner = this.resolveWinner(round.roundNumber, round.winnerTeam);
      const winnerSide = winner === 'TEAM A' ? 'left' : winner === 'TEAM B' ? 'right' : 'center';

      const mvpLabel = round.mvp
        ? round.mvp.reason === 'Round MVP'
          ? `${round.mvp.playerName} - ${round.mvp.reason}`
          : `${round.mvp.playerName} - ${round.mvp.reason} - ${round.mvp.kills}K / ${round.mvp.damage} ADR`
        : 'No MVP event';

      const teamASide = this.teamASideForRound(round.roundNumber);
      const teamAEconomy = teamASide === 'CT' ? (round.ctEconomy ?? 'Unknown') : (round.tEconomy ?? 'Unknown');
      const teamBEconomy = teamASide === 'CT' ? (round.tEconomy ?? 'Unknown') : (round.ctEconomy ?? 'Unknown');
      const teamAStartMoney = teamASide === 'CT' ? (round.ctStartMoney ?? 0) : (round.tStartMoney ?? 0);
      const teamBStartMoney = teamASide === 'CT' ? (round.tStartMoney ?? 0) : (round.ctStartMoney ?? 0);
      const teamAMoneyByPlayer = teamASide === 'CT' ? (round.ctMoneyByPlayer ?? []) : (round.tMoneyByPlayer ?? []);
      const teamBMoneyByPlayer = teamASide === 'CT' ? (round.tMoneyByPlayer ?? []) : (round.ctMoneyByPlayer ?? []);
      const teamAMultiKills: RoundPerformance[] = [];
      const teamBMultiKills: RoundPerformance[] = [];

      for (const award of round.multiKills) {
        const awardTeam = this.resolveAwardTeam(round.roundNumber, award.team);
        if (awardTeam === 'TEAM A') {
          teamAMultiKills.push(award);
          continue;
        }
        if (awardTeam === 'TEAM B') {
          teamBMultiKills.push(award);
          continue;
        }
        if (winner === 'TEAM A') {
          teamAMultiKills.push(award);
        } else if (winner === 'TEAM B') {
          teamBMultiKills.push(award);
        }
      }

      return {
        roundNumber: round.roundNumber,
        winner,
        winnerSide,
        winnerRole: round.winnerTeam === 'CT' || round.winnerTeam === 'T' ? round.winnerTeam : 'UNKNOWN',
        mvpLabel,
        teamAEconomy,
        teamBEconomy,
        teamAMoneyLabel: `$${teamAStartMoney.toLocaleString()}`,
        teamBMoneyLabel: `$${teamBStartMoney.toLocaleString()}`,
        teamAMoneyDetails: teamAMoneyByPlayer,
        teamBMoneyDetails: teamBMoneyByPlayer,
        teamAMultiKills,
        teamBMultiKills,
      };
    });
  }

  private resolveAwardTeam(roundNumber: number, awardTeam: string): TimelineWinner {
    if (awardTeam !== 'CT' && awardTeam !== 'T') {
      return 'UNKNOWN';
    }

    const teamASide = this.teamASideForRound(roundNumber);
    if (awardTeam === teamASide) {
      return 'TEAM A';
    }

    return 'TEAM B';
  }

  private get teams(): string[] {
    const summary = this.summary();
    if (!summary) {
      return [];
    }

    const unique = Array.from(new Set(summary.playerStats.map((player) => player.team))).filter(
      (team) => !!team && team !== 'Unknown',
    );

    if (unique.length >= 2) {
      return unique.slice(0, 2);
    }

    return unique.length === 1 ? [unique[0], 'Unknown'] : [];
  }

  private resolveWinner(roundNumber: number, winnerTeam: string): TimelineWinner {
    if (winnerTeam !== 'CT' && winnerTeam !== 'T') {
      return 'UNKNOWN';
    }

    const teamASide = this.teamASideForRound(roundNumber);
    if (winnerTeam === teamASide) {
      return 'TEAM A';
    }

    return 'TEAM B';
  }

  private teamASideForRound(roundNumber: number): 'CT' | 'T' {
    const summary = this.summary();
    if (!summary) {
      return 'CT';
    }

    // Team A is defined as the roster that ends on CT (TeamAScore == finalCTScore).
    // Before halftime swap exists, Team A is CT. After swap, Team A starts as T then becomes CT.
    if (summary.rounds <= 12) {
      return 'CT';
    }

    if (roundNumber <= this.halftimeRound) {
      return 'T';
    }

    return 'CT';
  }

  private playersForTeam(team: 'TEAM A' | 'TEAM B'): PlayerSummary[] {
    const summary = this.summary();
    if (!summary) {
      return [];
    }

    const teamASideAtEnd = this.teamASideForRound(summary.rounds);
    const side = team === 'TEAM A' ? teamASideAtEnd : teamASideAtEnd === 'CT' ? 'T' : 'CT';
    const players = summary.playerStats.filter((player) => player.team === side);

    if (players.length > 0) {
      return players;
    }

    return summary.playerStats;
  }

  teamRoleForRound(team: 'TEAM A' | 'TEAM B', roundNumber: number): 'CT' | 'T' {
    const teamASide = this.teamASideForRound(roundNumber);
    return team === 'TEAM A' ? teamASide : teamASide === 'CT' ? 'T' : 'CT';
  }

  normalizeMapKey(mapName: string): string {
    const key = mapName.trim().toLowerCase().replace(/\s+/g, '_');
    const withoutDe = key.startsWith('de_') ? key.slice(3) : key;
    return `map-${withoutDe}`;
  }

  get selectedHeatPoints(): HeatPoint[] {
    const summary = this.summary();
    const player = this.selectedPlayer();

    if (!summary || !player) {
      return [];
    }

    const playerHeatmap = summary.playerHeatmaps.find(
      (entry) => entry.playerName === player.playerName,
    );
    if (!playerHeatmap) {
      return this.heatmapMode() === 'kills' ? summary.killHeatmap : summary.deathHeatmap;
    }

    return this.heatmapMode() === 'kills' ? playerHeatmap.kills : playerHeatmap.deaths;
  }

  selectedPlayerTeamName(player: PlayerSummary): string {
    if (this.teamAPlayers.some((candidate) => candidate.playerName === player.playerName)) {
      return this.summary()?.teamAName ?? 'Team A';
    }

    if (this.teamBPlayers.some((candidate) => candidate.playerName === player.playerName)) {
      return this.summary()?.teamBName ?? 'Team B';
    }

    return 'Unknown Team';
  }

  get playerDamageByRound(): PlayerDamageRound[] {
    const summary = this.summary();
    const player = this.selectedPlayer();

    if (!summary || !player) {
      return [];
    }

    return summary.roundHistory.map((round) => {
      const roundDamage = round.playerDamages?.find((entry) => entry.playerName === player.playerName)?.damage ?? 0;
      return {
        roundNumber: round.roundNumber,
        damage: roundDamage,
      };
    });
  }

  get playerDamageMax(): number {
    return Math.max(1, ...this.playerDamageByRound.map((entry) => entry.damage));
  }

  get matchDamageScaleMax(): number {
    const summary = this.summary();
    if (!summary) {
      return 100;
    }

    const maxDamage = summary.roundHistory.reduce((max, round) => {
      const roundMax = Math.max(0, ...(round.playerDamages ?? []).map((entry) => entry.damage));
      return Math.max(max, roundMax);
    }, 0);

    // Keep slight headroom so peaks remain readable when comparing players.
    const scaledMax = Math.ceil(maxDamage * 1.1);
    return Math.max(100, scaledMax);
  }

  get playerDamageAverage(): number {
    const points = this.playerDamageByRound;
    if (points.length === 0) {
      return 0;
    }

    const total = points.reduce((sum, point) => sum + point.damage, 0);
    return total / points.length;
  }

  get playerStandoutRounds(): StandoutRound[] {
    const summary = this.summary();
    const player = this.selectedPlayer();

    if (!summary || !player) {
      return [];
    }

    const standoutRounds: StandoutRound[] = [];

    for (const round of summary.roundHistory) {
      const details: string[] = [];
      let title = 'Impact round';
      let score = 0;

      const multiKill = round.multiKills.find((entry) => entry.playerName === player.playerName);
      const roundDamage = round.playerDamages?.find((entry) => entry.playerName === player.playerName)?.damage ?? 0;
      const roundMvp = round.mvp?.playerName === player.playerName ? round.mvp : null;
      const bestPlayer = round.bestPlayer?.playerName === player.playerName ? round.bestPlayer : null;

      if (multiKill) {
        title = multiKill.label;
        details.push(`${multiKill.kills}K with ${multiKill.damage} damage`);
        if (multiKill.killDetails && multiKill.killDetails.length > 0) {
          const victims = multiKill.killDetails.map((kill) => `${kill.victimName} (${kill.weapon})`).join(', ');
          details.push(`Eliminations: ${victims}`);
        }
        score += multiKill.kills * 40 + multiKill.damage;
      }

      if (roundMvp) {
        if (title === 'Impact round') {
          title = roundMvp.reason === 'Round MVP' ? 'Round MVP' : roundMvp.reason;
        }

        const normalizedReason = roundMvp.reason.trim().toLowerCase();
        const isGenericMvpReason = normalizedReason === 'round mvp' || normalizedReason === 'most eliminations';

        if (!multiKill && !isGenericMvpReason) {
          details.push(roundMvp.reason);
        }

        if (!multiKill && (roundMvp.kills > 0 || roundMvp.damage > 0)) {
          details.push(`${roundMvp.kills}K and ${roundMvp.damage} damage`);
        }

        score += 35 + roundMvp.damage + roundMvp.kills * 12;
      }

      if (bestPlayer && !roundMvp && !multiKill) {
        title = 'Best of round';
        details.push('Best player of the round');
        score += 25 + bestPlayer.damage + bestPlayer.kills * 10;
      }

      const damageAlreadyMentioned = details.some((detail) => detail.includes(`${roundDamage} damage`));
      if (roundDamage >= 100 && !damageAlreadyMentioned) {
        details.push(`${roundDamage} damage dealt`);
        score += roundDamage;
      }

      const uniqueDetails = Array.from(new Set(details));
      if (uniqueDetails.length === 0) {
        continue;
      }

      standoutRounds.push({
        roundNumber: round.roundNumber,
        side: player.team === this.teamASideForRound(round.roundNumber) ? 'CT' : 'T',
        title,
        details: uniqueDetails,
        score,
      });
    }

    return standoutRounds
      .sort((a, b) => (b.score === a.score ? a.roundNumber - b.roundNumber : b.score - a.score))
      .slice(0, 6)
      .sort((a, b) => a.roundNumber - b.roundNumber);
  }

  private queueDamageChartRender(): void {
    setTimeout(() => this.renderDamageChart(), 0);
  }

  private renderDamageChart(): void {
    const canvas = this.damageChartCanvasRef?.nativeElement;
    const player = this.selectedPlayer();
    const data = this.playerDamageByRound;

    if (!canvas || !player || data.length === 0) {
      this.destroyDamageChart();
      return;
    }

    const context = canvas.getContext('2d');
    if (!context) {
      return;
    }

    const gradient = context.createLinearGradient(0, 0, 0, 260);
    gradient.addColorStop(0, 'rgba(42, 151, 231, 0.35)');
    gradient.addColorStop(1, 'rgba(42, 151, 231, 0.03)');

    this.destroyDamageChart();
    this.damageChart = new Chart(context, {
      type: 'line',
      data: {
        labels: data.map((point) => `R${point.roundNumber}`),
        datasets: [
          {
            label: `${player.playerName} damage`,
            data: data.map((point) => point.damage),
            borderColor: '#1f84d1',
            backgroundColor: gradient,
            pointBackgroundColor: '#f7fbff',
            pointBorderColor: '#1f84d1',
            pointRadius: 3,
            pointHoverRadius: 5,
            borderWidth: 2,
            fill: true,
            tension: 0.4,
          },
        ],
      },
      options: {
        animation: {
          duration: 520,
          easing: 'easeOutQuart',
        },
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: {
            display: false,
          },
          tooltip: {
            backgroundColor: 'rgba(20, 55, 86, 0.94)',
            borderColor: 'rgba(112, 174, 224, 0.48)',
            borderWidth: 1,
            titleColor: '#eaf5ff',
            bodyColor: '#d6ecff',
            displayColors: false,
            callbacks: {
              label: (ctx) => `${ctx.raw as number} damage`,
            },
          },
        },
        scales: {
          x: {
            grid: {
              color: 'rgba(142, 185, 221, 0.22)',
            },
            ticks: {
              color: '#5f82a6',
              maxRotation: 0,
              autoSkip: true,
            },
          },
          y: {
            beginAtZero: true,
            max: this.matchDamageScaleMax,
            grid: {
              color: 'rgba(142, 185, 221, 0.22)',
            },
            ticks: {
              color: '#5f82a6',
            },
          },
        },
      },
    });
  }

  private destroyDamageChart(): void {
    if (this.damageChart) {
      this.damageChart.destroy();
      this.damageChart = undefined;
    }
  }

  private startPolling(jobId: string, demoId: string): void {
    this.fakeProgress.set(5);
    this.progressInterval = setInterval(() => {
      const current = this.fakeProgress();
      if (current < 85) {
        this.fakeProgress.update(p => Math.min(85, p + (Math.random() * 4 + 1)));
      }
    }, 800);

    this.pollingSubscription = interval(1200)
      .pipe(switchMap(() => this.getJobStatus.execute(jobId)))
      .subscribe({
        next: (job) => {
          this.job.set(job);
          if (job.status === 'COMPLETED') {
            this.pollingSubscription?.unsubscribe();
            clearInterval(this.progressInterval);
            this.fakeProgress.set(100);
            this.loadSummary(demoId);
          }
          if (job.status === 'FAILED') {
            this.pollingSubscription?.unsubscribe();
            clearInterval(this.progressInterval);
            this.errorMessage.set(job.error || 'Analysis failed.');
          }
        },
        error: () => {
          this.errorMessage.set('Failed to fetch analysis status.');
        },
      });
  }

  private loadSummary(demoId: string): void {
    this.getMatchSummary.execute(demoId).subscribe({
      next: (summary) => {
        this.summary.set(summary);
        this.selectPlayer(null);
        this.isPlayerDrawerOpen.set(false);
        this.destroyDamageChart();
        setTimeout(() => this.applyMapBackground());
      },
      error: () => {
        this.errorMessage.set('Failed to fetch match summary.');
      },
    });
  }
}
