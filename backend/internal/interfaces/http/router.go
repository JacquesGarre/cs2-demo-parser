package http

import (
	"net/http"
	"strings"

	"github.com/jcqsg/cs2-demos/backend/internal/interfaces/http/handlers"
)

type Dependencies struct {
	DemoHandler  handlers.DemoHandler
	JobHandler   handlers.JobHandler
	MatchHandler handlers.MatchHandler
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", handlers.Health)
	mux.HandleFunc("POST /api/v1/demos", deps.DemoHandler.Upload)
	mux.HandleFunc("GET /api/v1/jobs/{id}", deps.JobHandler.GetStatus)
	mux.HandleFunc("GET /api/v1/matches/{id}/summary", deps.MatchHandler.GetSummary)

	return withCORS(mux)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if strings.EqualFold(r.Method, http.MethodOptions) {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
