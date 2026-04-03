import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { AnalysisJob } from '../../domain/models/analysis-job.model';
import { AnalysisApiPort } from '../../ports/analysis-api.port';

@Injectable({ providedIn: 'root' })
export class GetJobStatusUseCase {
  private readonly api = inject(AnalysisApiPort);

  execute(jobId: string): Observable<AnalysisJob> {
    return this.api.getJobStatus(jobId);
  }
}
