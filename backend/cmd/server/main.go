package main

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"smartreplenish/internal/app"
	"smartreplenish/internal/config"
	"smartreplenish/internal/decision"
	"smartreplenish/internal/httpapi"
	"smartreplenish/internal/postgres"
	"smartreplenish/migrations"
	"syscall"
	"time"
)

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(startup, config.Env("DATABASE_URL", config.DefaultDatabaseURL))
	if err != nil {
		return err
	}
	defer pool.Close()
	if err = pool.Ping(startup); err != nil {
		return err
	}
	if err = migrations.Apply(startup, pool); err != nil {
		return err
	}
	service := &app.Service{Store: &postgres.Store{Pool: pool}, Production: decision.RuleBasedDecisionEngine{}}
	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		service.Shadow = decision.JevDecisionEngine{
			APIKey:   key,
			Model:    config.Env("JEV_MODEL", "typesafe/jev-1.13"),
			Endpoint: config.Env("JEV_ENDPOINT", "https://openrouter.ai/api/alpha/decisions"),
		}
	} else if key := os.Getenv("TYPESAFE_API_KEY"); key != "" {
		service.Shadow = decision.JevDecisionEngine{APIKey: key, Model: config.Env("TYPESAFE_MODEL", "jev-latest")}
	}
	server := &http.Server{Addr: config.Env("HTTP_ADDR", "127.0.0.1:8080"), Handler: httpapi.API{Service: service}.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan error, 1)
	go func() {
		slog.Info("Smart Replenishment listening", "address", server.Addr, "jev_shadow", service.Shadow != nil)
		done <- server.ListenAndServe()
	}()
	select {
	case err = <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
