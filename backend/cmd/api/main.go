package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/jcqsg/cs2-demos/backend/internal/application/usecases"
	postgresrepo "github.com/jcqsg/cs2-demos/backend/internal/infrastructure/persistence/postgres"
	"github.com/jcqsg/cs2-demos/backend/internal/infrastructure/processing"
	localstorage "github.com/jcqsg/cs2-demos/backend/internal/infrastructure/storage/local"
	httpapi "github.com/jcqsg/cs2-demos/backend/internal/interfaces/http"
	"github.com/jcqsg/cs2-demos/backend/internal/interfaces/http/handlers"
	"github.com/jcqsg/cs2-demos/backend/internal/platform/config"
	_ "github.com/lib/pq"
)

func main() {
	cfg := config.Load()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	if err := postgresrepo.EnsureSchema(db); err != nil {
		log.Fatalf("failed to initialize schema: %v", err)
	}

	demoRepository := postgresrepo.NewDemoRepository(db)
	jobRepository := postgresrepo.NewJobRepository(db)
	summaryRepository := postgresrepo.NewSummaryRepository(db)

	storage := localstorage.NewDemoStorage(cfg.UploadsDir)
	queue := processing.NewFakeAnalysisQueue(demoRepository, jobRepository, summaryRepository)

	submitDemoUseCase := usecases.NewSubmitDemoUseCase(demoRepository, jobRepository, storage, queue)
	getJobStatusUseCase := usecases.NewGetJobStatusUseCase(jobRepository)
	getMatchSummaryUseCase := usecases.NewGetMatchSummaryUseCase(summaryRepository)

	router := httpapi.NewRouter(httpapi.Dependencies{
		DemoHandler:  handlers.NewDemoHandler(submitDemoUseCase, cfg.MaxUploadBytes),
		JobHandler:   handlers.NewJobHandler(getJobStatusUseCase),
		MatchHandler: handlers.NewMatchHandler(getMatchSummaryUseCase),
	})

	addr := ":" + cfg.Port
	log.Printf("backend listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
