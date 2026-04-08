package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	infrastructurepostgres "example.com/taskservice/internal/infrastructure/postgres"
	postgresrepo "example.com/taskservice/internal/repository/postgres"
	transporthttp "example.com/taskservice/internal/transport/http"
	swaggerdocs "example.com/taskservice/internal/transport/http/docs"
	httphandlers "example.com/taskservice/internal/transport/http/handlers"
	"example.com/taskservice/internal/usecase/task"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := loadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := infrastructurepostgres.Open(ctx, cfg.DatabaseDSN)
	if err != nil {
		logger.Error("open postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	taskRepo := postgresrepo.New(pool)

	StartRecurrenceWorker(taskRepo)

	taskUsecase := task.NewService(taskRepo)
	taskHandler := httphandlers.NewTaskHandler(taskUsecase)
	docsHandler := swaggerdocs.NewHandler()
	router := transporthttp.NewRouter(taskHandler, docsHandler)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown http server", "error", err)
		}
	}()

	logger.Info("http server started", "addr", cfg.HTTPAddr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("listen and serve", "error", err)
		os.Exit(1)
	}
}

type config struct {
	HTTPAddr    string
	DatabaseDSN string
}

func loadConfig() config {
	cfg := config{
		HTTPAddr:    envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseDSN: envOrDefault("DATABASE_DSN", "postgres://postgres:postgres@localhost:5432/taskservice?sslmode=disable"),
	}

	if cfg.DatabaseDSN == "" {
		panic(fmt.Errorf("DATABASE_DSN is required"))
	}

	return cfg
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func StartRecurrenceWorker(repo *postgresrepo.Repository) {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		for range ticker.C {
			ctx := context.Background() 
			now := time.Now()

			tasks, err := repo.List(ctx)
			if err != nil {
				continue
			}

			for _, t := range tasks {
				if t.IsTemplate && t.Recurrence != nil && t.Recurrence.ShouldCreateToday(t.CreatedAt, now) {
					
					alreadyExists := false
					for _, check := range tasks {
						if check.ParentId != nil && *check.ParentId == t.ID && 
						   check.CreatedAt.YearDay() == now.YearDay() && 
						   check.CreatedAt.Year() == now.Year() {
							alreadyExists = true
							break
						}
					}

					if alreadyExists {
						continue
					}

					newTask := t
					parentId := t.ID 
					newTask.ID = 0
					newTask.ParentId = &parentId
					newTask.IsTemplate = false
					newTask.Title = fmt.Sprintf("[%s] %s", now.Format("02.01"), t.Title)
					newTask.CreatedAt = now
					newTask.UpdatedAt = now

					_, _ = repo.Create(ctx, &newTask)
				}
			}
		}
	}()
}
