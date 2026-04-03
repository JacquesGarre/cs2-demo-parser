import { Observable } from 'rxjs';
import { AnalysisJob } from '../domain/models/analysis-job.model';
import { MatchSummary } from '../domain/models/match-summary.model';

export abstract class AnalysisApiPort {
  abstract uploadDemo(file: File): Observable<AnalysisJob>;
  abstract getJobStatus(jobId: string): Observable<AnalysisJob>;
  abstract getMatchSummary(demoId: string): Observable<MatchSummary>;
}
