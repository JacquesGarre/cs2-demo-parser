import { CommonModule } from '@angular/common';
import {
  AfterViewInit,
  Component,
  ElementRef,
  Input,
  OnChanges,
  SimpleChanges,
  ViewChild,
} from '@angular/core';
import { HeatPoint } from '../../../../core/domain/models/match-summary.model';

type Bounds = { minX: number; maxX: number; minY: number; maxY: number };
type RadarMeta = {
  resolution: number;
  offset: { x: number; y: number };
  splits: RadarSplit[];
  advisoryPosition?: { x: number; y: number };
  zRange?: { min: number; max: number };
};
type RadarSplit = {
  bounds?: { top: number; bottom: number };
  offset: { x: number; y: number };
  zRange?: { min: number; max: number };
};
type RenderedHeatPoint = {
  point: HeatPoint;
  x: number;
  y: number;
  radius: number;
};

const MAP_BOUNDS: Record<string, Bounds> = {
  de_mirage: { minX: -3230, maxX: 1913, minY: -3400, maxY: 1713 },
  de_inferno: { minX: -2087, maxX: 3876, minY: -3870, maxY: 3100 },
  de_dust2: { minX: -2476, maxX: 3238, minY: -1150, maxY: 3236 },
};

@Component({
  selector: 'app-radar-minimap',
  imports: [CommonModule],
  templateUrl: './radar-minimap.component.html',
  styleUrl: './radar-minimap.component.scss',
})
export class RadarMinimapComponent implements AfterViewInit, OnChanges {
  @Input({ required: true }) points: HeatPoint[] = [];
  @Input({ required: true }) mapName = 'de_mirage';
  @Input({ required: true }) mode: 'kills' | 'deaths' = 'kills';

  @ViewChild('canvas')
  private canvasRef?: ElementRef<HTMLCanvasElement>;

  private readonly imageCache = new Map<string, HTMLImageElement | null>();
  private readonly metaCache = new Map<string, RadarMeta | null>();
  private readonly pendingImageLoads = new Set<string>();
  private readonly pendingMetaLoads = new Set<string>();
  private readonly radarMetaBaseSize = 1024;
  private readonly renderedPoints: RenderedHeatPoint[] = [];

  hoveredPoint: HeatPoint | null = null;
  tooltipX = 0;
  tooltipY = 0;

  ngAfterViewInit(): void {
    this.draw();
  }

  ngOnChanges(_changes: SimpleChanges): void {
    this.draw();
  }

  private draw(): void {
    const canvas = this.canvasRef?.nativeElement;
    if (!canvas) {
      return;
    }

    const ctx = canvas.getContext('2d');
    if (!ctx) {
      return;
    }

    const mapKey = this.normalizeMapKey(this.mapName);
    this.ensureMapAssets(mapKey);

    this.syncCanvasSize(canvas, mapKey);

    const width = canvas.width;
    const height = canvas.height;

    ctx.clearRect(0, 0, width, height);

    const meta = this.metaCache.get(mapKey) ?? null;
    const bounds = MAP_BOUNDS[mapKey] ?? MAP_BOUNDS['de_mirage'];
    this.renderedPoints.length = 0;

    this.drawRadarBackground(ctx, width, height, mapKey);

    for (const point of this.points) {
      let x = 0;
      let y = 0;

      if (meta) {
        const projected = this.projectWithRadarMeta(point, meta);
        x = projected.x;
        y = projected.y;
      } else {
        const normalizedX = (point.x - bounds.minX) / (bounds.maxX - bounds.minX);
        const normalizedY = (point.y - bounds.minY) / (bounds.maxY - bounds.minY);
        x = Math.max(0, Math.min(1, normalizedX)) * width;
        y = (1 - Math.max(0, Math.min(1, normalizedY))) * height;
      }

      x = Math.max(0, Math.min(width, x));
      y = Math.max(0, Math.min(height, y));

      const radius = 15;
      const alpha = Math.min(0.82, 0.58 + Math.log(point.count + 1) * 0.06);
      const fillColor =
        this.mode === 'kills'
          ? `rgba(255, 99, 132, ${alpha})`
          : `rgba(77, 171, 247, ${alpha})`;
      const strokeColor =
        this.mode === 'kills'
          ? 'rgba(255, 189, 201, 0.95)'
          : 'rgba(196, 230, 255, 0.95)';

      ctx.beginPath();
      ctx.fillStyle = fillColor;
      ctx.arc(x, y, radius, 0, Math.PI * 2);
      ctx.fill();
      ctx.lineWidth = 1.4;
      ctx.strokeStyle = strokeColor;
      ctx.stroke();
      this.renderedPoints.push({ point, x, y, radius });
    }
  }

  private drawRadarBackground(ctx: CanvasRenderingContext2D, width: number, height: number, mapKey: string): void {
    const radarImage = this.imageCache.get(mapKey);
    if (radarImage && radarImage.complete && radarImage.naturalWidth > 0) {
      ctx.drawImage(radarImage, 0, 0, width, height);
      ctx.fillStyle = 'rgba(6, 15, 25, 0.22)';
      ctx.fillRect(0, 0, width, height);
      return;
    }

    if (radarImage === null) {
      this.drawFallbackBackground(ctx, width, height, mapKey);
      return;
    }

    this.drawFallbackBackground(ctx, width, height, mapKey);
  }

  private ensureMapAssets(mapKey: string): void {
    if (!this.imageCache.has(mapKey) && !this.pendingImageLoads.has(mapKey)) {
      this.pendingImageLoads.add(mapKey);
      this.loadRadarImage(mapKey);
    }

    if (!this.metaCache.has(mapKey) && !this.pendingMetaLoads.has(mapKey)) {
      this.pendingMetaLoads.add(mapKey);
      this.loadRadarMeta(mapKey);
    }
  }

  private syncCanvasSize(canvas: HTMLCanvasElement, _mapKey: string): void {
    if (canvas.width !== this.radarMetaBaseSize || canvas.height !== this.radarMetaBaseSize) {
      canvas.width = this.radarMetaBaseSize;
      canvas.height = this.radarMetaBaseSize;
    }
  }

  onCanvasPointerMove(event: MouseEvent): void {
    const canvas = this.canvasRef?.nativeElement;
    if (!canvas || this.renderedPoints.length === 0) {
      this.hoveredPoint = null;
      return;
    }

    const rect = canvas.getBoundingClientRect();
    const pointerX = (event.clientX - rect.left) * (canvas.width / rect.width);
    const pointerY = (event.clientY - rect.top) * (canvas.height / rect.height);

    let nearest: RenderedHeatPoint | null = null;
    let nearestDistance = Number.POSITIVE_INFINITY;

    for (const rendered of this.renderedPoints) {
      const hitRadius = rendered.radius + 8;
      const dx = pointerX - rendered.x;
      const dy = pointerY - rendered.y;
      const distance = dx * dx + dy * dy;

      if (distance <= hitRadius * hitRadius && distance < nearestDistance) {
        nearest = rendered;
        nearestDistance = distance;
      }
    }

    if (!nearest) {
      this.hoveredPoint = null;
      return;
    }

    this.hoveredPoint = nearest.point;
    this.tooltipX = Math.min(Math.max(12, event.clientX - rect.left + 14), Math.max(12, rect.width - 270));
    this.tooltipY = Math.min(Math.max(12, event.clientY - rect.top + 14), Math.max(12, rect.height - 130));
  }

  clearHoveredPoint(): void {
    this.hoveredPoint = null;
  }

  private loadRadarImage(mapKey: string): void {
    const fromRadars = new Image();
    fromRadars.onload = () => {
      this.imageCache.set(mapKey, fromRadars);
      this.pendingImageLoads.delete(mapKey);
      this.draw();
    };
    fromRadars.onerror = () => {
      const fallback = new Image();
      fallback.onload = () => {
        this.imageCache.set(mapKey, fallback);
        this.pendingImageLoads.delete(mapKey);
        this.draw();
      };
      fallback.onerror = () => {
        this.imageCache.set(mapKey, null);
        this.pendingImageLoads.delete(mapKey);
        this.draw();
      };
      fallback.src = `/maps/${mapKey}.png`;
    };
    fromRadars.src = `/radars/${mapKey}/radar.png`;
  }

  private async loadRadarMeta(mapKey: string): Promise<void> {
    try {
      const response = await fetch(`/radars/${mapKey}/meta.json5`, { cache: 'no-cache' });
      if (!response.ok) {
        this.metaCache.set(mapKey, null);
        return;
      }

      const text = await response.text();
      const parsed = this.parseJson5Like(text) as Partial<RadarMeta>;
      if (
        !parsed ||
        typeof parsed.resolution !== 'number' ||
        !parsed.offset ||
        typeof parsed.offset.x !== 'number' ||
        typeof parsed.offset.y !== 'number'
      ) {
        this.metaCache.set(mapKey, null);
        return;
      }

      const normalizedMeta: RadarMeta = {
        resolution: parsed.resolution,
        offset: {
          x: parsed.offset.x,
          y: parsed.offset.y,
        },
        splits: this.parseRadarSplits(parsed.splits),
        advisoryPosition:
          parsed.advisoryPosition && typeof parsed.advisoryPosition.x === 'number' && typeof parsed.advisoryPosition.y === 'number'
            ? { x: parsed.advisoryPosition.x, y: parsed.advisoryPosition.y }
            : undefined,
        zRange:
          parsed.zRange && typeof parsed.zRange.min === 'number' && typeof parsed.zRange.max === 'number'
            ? { min: parsed.zRange.min, max: parsed.zRange.max }
            : undefined,
      };

      this.metaCache.set(mapKey, normalizedMeta);
    } catch {
      this.metaCache.set(mapKey, null);
    } finally {
      this.pendingMetaLoads.delete(mapKey);
      this.draw();
    }
  }

  private parseJson5Like(content: string): unknown {
    const withoutBlockComments = content.replace(/\/\*[\s\S]*?\*\//g, '');
    const withoutLineComments = withoutBlockComments.replace(/(^|\s)\/\/.*$/gm, '$1');
    const withoutTrailingCommas = withoutLineComments.replace(/,\s*([}\]])/g, '$1');
    return JSON.parse(withoutTrailingCommas);
  }

  private projectWithRadarMeta(point: HeatPoint, meta: RadarMeta): { x: number; y: number } {
    let x = (point.x + meta.offset.x) / meta.resolution;
    let y = this.radarMetaBaseSize - (point.y + meta.offset.y) / meta.resolution;

    const split = this.findMatchingSplit(point, meta.splits);
    if (split) {
      const offsetX = (split.offset.x / 100) * this.radarMetaBaseSize;
      // Split offsets are expressed from bottom-left coordinates, while canvas uses top-left.
      const offsetY = (split.offset.y / 100) * this.radarMetaBaseSize;
      x += offsetX;
      y -= offsetY;
    }

    return { x, y };
  }

  private findMatchingSplit(point: HeatPoint, splits: RadarSplit[]): RadarSplit | null {
    if (!splits.length) {
      return null;
    }

    for (const split of splits) {
      const matchesBounds = split.bounds ? this.matchesSplitBounds(point.y, split.bounds) : true;
      const matchesZ = split.zRange ? typeof point.z === 'number' && this.matchesSplitZ(point.z, split.zRange) : true;

      if (matchesBounds && matchesZ && (split.bounds || split.zRange)) {
        return split;
      }
    }

    return null;
  }

  private matchesSplitBounds(y: number, bounds?: { top: number; bottom: number }): boolean {
    if (!bounds) {
      return false;
    }

    const minY = Math.min(bounds.top, bounds.bottom);
    const maxY = Math.max(bounds.top, bounds.bottom);
    return y >= minY && y <= maxY;
  }

  private matchesSplitZ(z: number, zRange?: { min: number; max: number }): boolean {
    if (!zRange) {
      return false;
    }

    const minZ = Math.min(zRange.min, zRange.max);
    const maxZ = Math.max(zRange.min, zRange.max);
    return z >= minZ && z <= maxZ;
  }

  private parseRadarSplits(raw: unknown): RadarSplit[] {
    if (!Array.isArray(raw)) {
      return [];
    }

    const splits: RadarSplit[] = [];

    for (const candidate of raw) {
      if (!candidate || typeof candidate !== 'object') {
        continue;
      }

      const value = candidate as {
        bounds?: { top?: unknown; bottom?: unknown };
        offset?: { x?: unknown; y?: unknown };
        zRange?: { min?: unknown; max?: unknown };
      };

      if (!value.offset || typeof value.offset.x !== 'number' || typeof value.offset.y !== 'number') {
        continue;
      }

      const split: RadarSplit = {
        offset: {
          x: value.offset.x,
          y: value.offset.y,
        },
      };

      if (value.bounds && typeof value.bounds.top === 'number' && typeof value.bounds.bottom === 'number') {
        split.bounds = {
          top: value.bounds.top,
          bottom: value.bounds.bottom,
        };
      }

      if (value.zRange && typeof value.zRange.min === 'number' && typeof value.zRange.max === 'number') {
        split.zRange = {
          min: value.zRange.min,
          max: value.zRange.max,
        };
      }

      splits.push(split);
    }

    return splits;
  }

  private drawFallbackBackground(ctx: CanvasRenderingContext2D, width: number, height: number, mapKey: string): void {
    const gradient = ctx.createLinearGradient(0, 0, width, height);
    gradient.addColorStop(0, '#1f2b37');
    gradient.addColorStop(1, '#2d3a46');
    ctx.fillStyle = gradient;
    ctx.fillRect(0, 0, width, height);

    ctx.strokeStyle = 'rgba(255, 255, 255, 0.08)';
    ctx.lineWidth = 1;
    for (let step = 1; step < 8; step++) {
      const x = (width / 8) * step;
      const y = (height / 8) * step;
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x, height);
      ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(width, y);
      ctx.stroke();
    }

  }

  private normalizeMapKey(mapName: string): string {
    const key = mapName.trim().toLowerCase().replace(/\s+/g, '_');
    return key.startsWith('de_') ? key : `de_${key.replace(/^map-/, '')}`;
  }
}
