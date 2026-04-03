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
};
type RadarSplit = {
  bounds?: { top: number; bottom: number };
  offset: { x: number; y: number };
  zRange?: { min: number; max: number };
};
type RadarCrop = { left: number; top: number; width: number; height: number };

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
  private readonly cropCache = new Map<string, RadarCrop | null>();
  private readonly pendingImageLoads = new Set<string>();
  private readonly pendingMetaLoads = new Set<string>();
  private readonly radarMetaBaseSize = 1024;

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
    const crop = this.cropCache.get(mapKey) ?? null;
    const bounds = MAP_BOUNDS[mapKey] ?? MAP_BOUNDS['de_mirage'];
    const maxCount = Math.max(1, ...this.points.map((point) => point.count));

    this.drawRadarBackground(ctx, width, height, mapKey);

    for (const point of this.points) {
      let x = 0;
      let y = 0;

      if (meta) {
        // Meta calibration is defined in a fixed 1024px radar space.
        const projected = this.projectWithRadarMeta(point, meta);
        x = projected.x;
        y = projected.y;

        if (crop) {
          x -= crop.left;
          y -= crop.top;
        }
      } else {
        const normalizedX = (point.x - bounds.minX) / (bounds.maxX - bounds.minX);
        const normalizedY = (point.y - bounds.minY) / (bounds.maxY - bounds.minY);
        x = Math.max(0, Math.min(1, normalizedX)) * width;
        y = (1 - Math.max(0, Math.min(1, normalizedY))) * height;
      }

      x = Math.max(0, Math.min(width, x));
      y = Math.max(0, Math.min(height, y));

      const intensity = point.count / maxCount;
      const baseRadius = Math.max(2, Math.min(width, height) * 0.0055);
      const radius = baseRadius + baseRadius * 3.2 * intensity;
      const color =
        this.mode === 'kills'
          ? `rgba(220, 53, 69, ${0.25 + intensity * 0.45})`
          : `rgba(46, 134, 222, ${0.25 + intensity * 0.45})`;

      ctx.beginPath();
      ctx.fillStyle = color;
      ctx.arc(x, y, radius, 0, Math.PI * 2);
      ctx.fill();
    }
  }

  private drawRadarBackground(ctx: CanvasRenderingContext2D, width: number, height: number, mapKey: string): void {
    const radarImage = this.imageCache.get(mapKey);
    const crop = this.cropCache.get(mapKey) ?? null;
    if (radarImage && radarImage.complete && radarImage.naturalWidth > 0) {
      if (crop) {
        const scaleX = radarImage.naturalWidth / this.radarMetaBaseSize;
        const scaleY = radarImage.naturalHeight / this.radarMetaBaseSize;
        ctx.drawImage(
          radarImage,
          crop.left * scaleX,
          crop.top * scaleY,
          crop.width * scaleX,
          crop.height * scaleY,
          0,
          0,
          width,
          height,
        );
      } else {
        ctx.drawImage(radarImage, 0, 0, width, height);
      }
      ctx.fillStyle = 'rgba(6, 15, 25, 0.22)';
      ctx.fillRect(0, 0, width, height);
      this.drawMapLabels(ctx, width, height, mapKey);
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

  private syncCanvasSize(canvas: HTMLCanvasElement, mapKey: string): void {
    const radarImage = this.imageCache.get(mapKey);
    const hasValidImage = !!radarImage && radarImage.complete && radarImage.naturalWidth > 0 && radarImage.naturalHeight > 0;
    const hasMeta = this.metaCache.get(mapKey) !== null && this.metaCache.has(mapKey);
    const crop = this.cropCache.get(mapKey) ?? null;

    const targetWidth = hasMeta ? Math.round(crop?.width ?? this.radarMetaBaseSize) : hasValidImage ? radarImage.naturalWidth : 1024;
    const targetHeight = hasMeta ? Math.round(crop?.height ?? this.radarMetaBaseSize) : hasValidImage ? radarImage.naturalHeight : 1024;

    if (canvas.width !== targetWidth || canvas.height !== targetHeight) {
      canvas.width = targetWidth;
      canvas.height = targetHeight;
    }
  }

  private loadRadarImage(mapKey: string): void {
    const fromRadars = new Image();
    fromRadars.onload = () => {
      this.imageCache.set(mapKey, fromRadars);
      this.cropCache.set(mapKey, this.computeRadarCrop(fromRadars));
      this.pendingImageLoads.delete(mapKey);
      this.draw();
    };
    fromRadars.onerror = () => {
      const fallback = new Image();
      fallback.onload = () => {
        this.imageCache.set(mapKey, fallback);
        this.cropCache.set(mapKey, null);
        this.pendingImageLoads.delete(mapKey);
        this.draw();
      };
      fallback.onerror = () => {
        this.imageCache.set(mapKey, null);
        this.cropCache.set(mapKey, null);
        this.pendingImageLoads.delete(mapKey);
        this.draw();
      };
      fallback.src = `/maps/${mapKey}.png`;
    };
    fromRadars.src = `/radars/${mapKey}/radar.png`;
  }

  private async loadRadarMeta(mapKey: string): Promise<void> {
    try {
      const response = await fetch(`/radars/${mapKey}/meta.json5`, { cache: 'force-cache' });
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

      this.metaCache.set(mapKey, {
        resolution: parsed.resolution,
        offset: {
          x: parsed.offset.x,
          y: parsed.offset.y,
        },
        splits: this.parseRadarSplits(parsed.splits),
      });
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

    const pointWithZ = point as HeatPoint & { z?: number };
    const hasPointZ = typeof pointWithZ.z === 'number';

    if (hasPointZ) {
      for (const split of splits) {
        if (this.matchesSplitZ(pointWithZ.z as number, split.zRange)) {
          return split;
        }
      }
    }

    for (const split of splits) {
      if (split.zRange) {
        // If split has an explicit zRange but point has no z, skip bounds fallback to avoid false floor matches.
        continue;
      }

      if (this.matchesSplitBounds(point.y, split.bounds)) {
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

  private computeRadarCrop(image: HTMLImageElement): RadarCrop | null {
    if (!image.naturalWidth || !image.naturalHeight) {
      return null;
    }

    const probe = document.createElement('canvas');
    probe.width = image.naturalWidth;
    probe.height = image.naturalHeight;

    const ctx = probe.getContext('2d', { willReadFrequently: true });
    if (!ctx) {
      return null;
    }

    ctx.drawImage(image, 0, 0);
    const { data, width, height } = ctx.getImageData(0, 0, probe.width, probe.height);

    let minX = width;
    let minY = height;
    let maxX = -1;
    let maxY = -1;

    for (let y = 0; y < height; y++) {
      for (let x = 0; x < width; x++) {
        const alpha = data[(y * width + x) * 4 + 3];
        if (alpha === 0) {
          continue;
        }
        if (x < minX) minX = x;
        if (y < minY) minY = y;
        if (x > maxX) maxX = x;
        if (y > maxY) maxY = y;
      }
    }

    if (maxX < minX || maxY < minY) {
      return null;
    }

    const scaleX = image.naturalWidth / this.radarMetaBaseSize;
    const scaleY = image.naturalHeight / this.radarMetaBaseSize;
    return {
      left: minX / scaleX,
      top: minY / scaleY,
      width: (maxX - minX + 1) / scaleX,
      height: (maxY - minY + 1) / scaleY,
    };
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

    this.drawMapLabels(ctx, width, height, mapKey);
  }

  private drawMapLabels(ctx: CanvasRenderingContext2D, width: number, height: number, mapKey: string): void {
    ctx.fillStyle = 'rgba(255, 255, 255, 0.85)';
    ctx.font = 'bold 16px Segoe UI';
    ctx.fillText('A', width - 34, 26);
    ctx.fillText('B', 18, height - 16);
    ctx.font = '12px Segoe UI';
    ctx.fillStyle = 'rgba(255, 255, 255, 0.7)';
    ctx.fillText(mapKey, 16, 22);
  }

  private normalizeMapKey(mapName: string): string {
    const key = mapName.trim().toLowerCase().replace(/\s+/g, '_');
    return key.startsWith('de_') ? key : `de_${key.replace(/^map-/, '')}`;
  }
}
