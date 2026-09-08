package main

import (
	"context"
	_ "embed"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/StephenQiu30/then/backend/internal/platform/config"
	"github.com/StephenQiu30/then/backend/internal/platform/database"
	"github.com/StephenQiu30/then/backend/internal/platform/httpserver"
	"github.com/StephenQiu30/then/backend/internal/transport"
	"github.com/gin-gonic/gin"
)

//go:embed openapi.yaml
var apiDocument []byte

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("startup_or_runtime_failed", "reason", err.Error())
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, cfg.StartupTimeout)
	defer cancel()
	pool, err := database.Open(startup, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	gin.SetMode(gin.ReleaseMode)
	router, err := transport.NewRouter(startup, apiDocument, pool, cfg.HealthTimeout, log)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return errors.New("HTTP listen failed")
	}
	log.Info("api_started", "address", listener.Addr().String(), "role", cfg.Role)
	if err = httpserver.Serve(ctx, listener, router, cfg.ShutdownTimeout, router.Drain); err != nil {
		return err
	}
	log.Info("api_stopped")
	return nil
}
