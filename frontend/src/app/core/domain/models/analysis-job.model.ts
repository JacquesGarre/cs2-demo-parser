export type JobStatus = 'QUEUED' | 'PROCESSING' | 'COMPLETED' | 'FAILED';

export interface AnalysisJob {
  id: string;
  demoId: string;
  status: JobStatus;
  error?: string;
  createdAt?: string;
  updatedAt?: string;
}
