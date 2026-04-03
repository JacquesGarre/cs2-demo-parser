import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { MatchSummary } from '../../domain/models/match-summary.model';
import { AnalysisApiPort } from '../../ports/analysis-api.port';

@Injectable({ providedIn: 'root' })
export class GetMatchSummaryUseCase {
  private readonly api = inject(AnalysisApiPort);

  execute(demoId: string): Observable<MatchSummary> {
    return this.api.getMatchSummary(demoId);
  }
}
