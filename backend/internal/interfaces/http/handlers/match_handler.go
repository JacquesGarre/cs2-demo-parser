package handlers

import (
	"net/http"
	"strings"

	"github.com/jcqsg/cs2-demos/backend/internal/application/usecases"
)

type MatchHandler struct {
	getMatchSummaryUseCase usecases.GetMatchSummaryUseCase
}

func NewMatchHandler(getMatchSummaryUseCase usecases.GetMatchSummaryUseCase) MatchHandler {
	return MatchHandler{getMatchSummaryUseCase: getMatchSummaryUseCase}
}

func (h MatchHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	demoID := strings.TrimPrefix(r.URL.Path, "/api/v1/matches/")
	demoID = strings.TrimSuffix(demoID, "/summary")
	if demoID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing demo id"})
		return
	}

	summary, err := h.getMatchSummaryUseCase.Execute(demoID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, summary)
}
