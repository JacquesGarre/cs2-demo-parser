package ports

type AnalysisQueue interface {
	Enqueue(jobID string) error
}
