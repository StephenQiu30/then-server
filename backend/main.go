package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/StephenQiu30/then-server/backend/internal/messagequeue"
	"github.com/StephenQiu30/then-server/backend/internal/objectstore"
	"github.com/StephenQiu30/then-server/backend/internal/platform/config"
	"github.com/StephenQiu30/then-server/backend/internal/platform/database"
	"github.com/StephenQiu30/then-server/backend/internal/platform/httpserver"
	"github.com/StephenQiu30/then-server/backend/internal/platform/ratelimit"
	"github.com/StephenQiu30/then-server/backend/internal/repository"
	"github.com/StephenQiu30/then-server/backend/internal/service"
	"github.com/StephenQiu30/then-server/backend/internal/transport"
	"github.com/StephenQiu30/then-server/backend/internal/worker"
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
	var objects *objectstore.Store
	if cfg.MediaDevelopmentEnabled {
		objects, err = objectstore.Open(startup, cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOSecure)
		if err != nil {
			return err
		}
	}
	var runner *worker.Runner
	var broker *messagequeue.Broker
	if cfg.Role == "worker" || cfg.Role == "all" {
		broker, err = messagequeue.Open(cfg.RabbitMQURL)
		if err != nil {
			return err
		}
		defer broker.Close()
		runner, err = worker.New(repository.NewMediaRepository(pool.ORM()), broker, objects)
		if err != nil {
			return err
		}
	}
	if cfg.Role == "worker" {
		log.Info("worker_started", "role", cfg.Role)
		err = runner.Run(ctx)
		log.Info("worker_stopped")
		return err
	}
	if cfg.Role == "all" {
		combined, cancelCombined := context.WithCancel(ctx)
		defer cancelCombined()
		results := make(chan error, 2)
		go func() { results <- runner.Run(combined) }()
		go func() { results <- runAPI(combined, startup, cfg, pool, objects, log) }()
		err = <-results
		cancelCombined()
		<-results
		return err
	}
	return runAPI(ctx, startup, cfg, pool, objects, log)
}

func runAPI(ctx, startup context.Context, cfg config.Config, pool *database.Pool, objects *objectstore.Store, log *slog.Logger) error {
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
	probes := dependencyProbes{pool, limiter}
	wardrobe, err := service.NewWardrobeService(accounts, repository.NewWardrobeRepository(pool.ORM()))
	if err != nil {
		return err
	}
	outfits, err := service.NewOutfitPlanService(accounts, repository.NewOutfitPlanRepository(pool.ORM()))
	if err != nil {
		return err
	}
	wearEvents, err := service.NewWearEventService(accounts, repository.NewWearEventRepository(pool.ORM()))
	if err != nil {
		return err
	}
	var mediaHandler *transport.MediaHandler
	if objects != nil {
		media, serviceErr := service.NewMediaService(accounts, repository.NewMediaRepository(pool.ORM()), objects)
		if serviceErr != nil {
			return serviceErr
		}
		mediaHandler = transport.NewMediaHandler(media, cfg.SessionSecure)
		probes = append(probes, objects)
	}
	router, err := transport.NewRouter(startup, cfg.DocsEnabled, probes, transport.NewAccountHandler(accounts, cfg.SessionSecure, limiter), transport.NewPrivacyHandler(privacy, cfg.SessionSecure), transport.NewWardrobeHandler(wardrobe, cfg.SessionSecure), transport.NewOutfitPlanHandler(outfits, cfg.SessionSecure), transport.NewWearEventHandler(wearEvents, cfg.SessionSecure), cfg.HealthTimeout, log, mediaHandler)
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
