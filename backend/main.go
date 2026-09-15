package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/StephenQiu30/then-server/backend/internal/platform/config"
	"github.com/StephenQiu30/then-server/backend/internal/platform/database"
	"github.com/StephenQiu30/then-server/backend/internal/platform/httpserver"
	"github.com/StephenQiu30/then-server/backend/internal/platform/ratelimit"
	"github.com/StephenQiu30/then-server/backend/internal/repository"
	"github.com/StephenQiu30/then-server/backend/internal/service"
	"github.com/StephenQiu30/then-server/backend/internal/transport"
	"github.com/gin-gonic/gin"
)

type dependencyProbes []transport.DependencyProbe

func (probes dependencyProbes) Probe(ctx context.Context) error {
	for _, probe := range probes {
		if probe == nil || probe.Probe(ctx) != nil {
			return errors.New("runtime dependency unavailable")
		}
	}
	return nil
}

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
	if err := repository.Migrate(startup, pool.ORM()); err != nil {
		return errors.New("database schema migration failed")
	}
	limiter, err := ratelimit.Open(startup, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer limiter.Close()
	accounts, err := service.NewAccountService(repository.NewAccountRepository(pool.ORM()))
	if err != nil {
		return err
	}
	gin.SetMode(gin.ReleaseMode)
	privacy, err := service.NewPrivacyService(accounts, repository.NewPrivacyRepository(pool.ORM()))
	if err != nil {
		return err
	}
	router, err := transport.NewRouter(startup, cfg.DocsEnabled, dependencyProbes{pool, limiter}, transport.NewAccountHandler(accounts, cfg.SessionSecure, limiter), transport.NewPrivacyHandler(privacy, cfg.SessionSecure), cfg.HealthTimeout, log)
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
