import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { map, Observable } from 'rxjs';
import { AnalysisApiPort } from '../../core/ports/analysis-api.port';
import { AnalysisJob } from '../../core/domain/models/analysis-job.model';
import { MatchSummary } from '../../core/domain/models/match-summary.model';

interface UploadResponse {
  jobId: string;
  demoId: string;
  status: AnalysisJob['status'];
}

@Injectable()
export class AnalysisApiHttpService extends AnalysisApiPort {
  private readonly apiBaseUrl = 'http://localhost:8080/api/v1';

  constructor(private readonly http: HttpClient) {
    super();
  }

  uploadDemo(file: File): Observable<AnalysisJob> {
    const formData = new FormData();
    formData.append('demo', file, file.name);

    return this.http
      .post<UploadResponse>(`${this.apiBaseUrl}/demos`, formData)
      .pipe(
        map((response) => ({
          id: response.jobId,
          demoId: response.demoId,
          status: response.status,
        })),
      );
  }

  getJobStatus(jobId: string): Observable<AnalysisJob> {
    return this.http.get<AnalysisJob>(`${this.apiBaseUrl}/jobs/${jobId}`);
  }

  getMatchSummary(demoId: string): Observable<MatchSummary> {
    return this.http.get<MatchSummary>(`${this.apiBaseUrl}/matches/${demoId}/summary`);
  }
}
