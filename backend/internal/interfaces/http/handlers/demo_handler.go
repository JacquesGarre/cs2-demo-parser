package handlers

import (
	"errors"
	"net/http"

	"github.com/jcqsg/cs2-demos/backend/internal/application/usecases"
)

type DemoHandler struct {
	submitDemoUseCase usecases.SubmitDemoUseCase
	maxUploadBytes    int64
}

func NewDemoHandler(submitDemoUseCase usecases.SubmitDemoUseCase, maxUploadBytes int64) DemoHandler {
	return DemoHandler{submitDemoUseCase: submitDemoUseCase, maxUploadBytes: maxUploadBytes}
}

func (h DemoHandler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if errors.Is(err, http.ErrBodyReadAfterClose) || err.Error() == "http: request body too large" {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "demo file is too large for current upload limit"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart request"})
		return
	}

	file, fileHeader, err := r.FormFile("demo")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing form field: demo"})
		return
	}
	defer file.Close()

	job, err := h.submitDemoUseCase.Execute(file, fileHeader.Filename)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"jobId":  job.ID,
		"demoId": job.DemoID,
		"status": string(job.Status),
	})
}
