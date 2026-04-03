package handlers

import (
	"net/http"
	"strings"

	"github.com/jcqsg/cs2-demos/backend/internal/application/usecases"
)

type JobHandler struct {
	getJobStatusUseCase usecases.GetJobStatusUseCase
}

func NewJobHandler(getJobStatusUseCase usecases.GetJobStatusUseCase) JobHandler {
	return JobHandler{getJobStatusUseCase: getJobStatusUseCase}
}

func (h JobHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs/")
	if jobID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing job id"})
		return
	}

	job, err := h.getJobStatusUseCase.Execute(jobID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":        job.ID,
		"demoId":    job.DemoID,
		"status":    job.Status,
		"error":     job.Error,
		"createdAt": job.CreatedAt,
		"updatedAt": job.UpdatedAt,
	})
}
