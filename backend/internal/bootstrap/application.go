// Package bootstrap owns process lifecycle and concrete dependency wiring.
package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/StephenQiu30/then-server/backend/internal/adapter/httpapi"
	"github.com/StephenQiu30/then-server/backend/internal/adapter/messagequeue"
	"github.com/StephenQiu30/then-server/backend/internal/adapter/objectstore"
	"github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
	"github.com/StephenQiu30/then-server/backend/internal/application/mediaworker"
	outfitplanapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitplan"
	privacyapp "github.com/StephenQiu30/then-server/backend/internal/application/privacy"
	wardrobeapp "github.com/StephenQiu30/then-server/backend/internal/application/wardrobe"
	weareventapp "github.com/StephenQiu30/then-server/backend/internal/application/wearevent"
	"github.com/StephenQiu30/then-server/backend/internal/platform/config"
	"github.com/StephenQiu30/then-server/backend/internal/platform/database"
	"github.com/StephenQiu30/then-server/backend/internal/platform/httpserver"
	"github.com/StephenQiu30/then-server/backend/internal/platform/ratelimit"
	"github.com/gin-gonic/gin"
)

type dependencyProbes []httpapi.DependencyProbe

func (probes dependencyProbes) Probe(ctx context.Context) error {
	for _, probe := range probes {
		if probe == nil || probe.Probe(ctx) != nil {
			return errors.New("runtime dependency unavailable")
		}
	}
	return nil
}

func Run(log *slog.Logger) error {
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
	if err := postgres.Migrate(startup, pool.ORM()); err != nil {
		return errors.New("database schema migration failed")
	}
	var objects *objectstore.Store
	if cfg.MediaDevelopmentEnabled {
		objects, err = objectstore.Open(startup, cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOSecure)
		if err != nil {
			return err
		}
	}
	var runner *mediaworker.Runner
	var broker *messagequeue.Broker
	if cfg.Role == "worker" || cfg.Role == "all" {
		broker, err = messagequeue.Open(cfg.RabbitMQURL)
		if err != nil {
			return err
		}
		defer broker.Close()
		runner, err = mediaworker.New(postgres.NewMediaRepository(pool.ORM()), broker, objects)
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
	accounts, err := accountapp.NewAccountService(postgres.NewAccountRepository(pool.ORM()))
	if err != nil {
		return err
	}
	gin.SetMode(gin.ReleaseMode)
	privacy, err := privacyapp.NewPrivacyService(accounts, postgres.NewPrivacyRepository(pool.ORM()))
	if err != nil {
		return err
	}
	probes := dependencyProbes{pool, limiter}
	wardrobe, err := wardrobeapp.NewWardrobeService(accounts, postgres.NewWardrobeRepository(pool.ORM()))
	if err != nil {
		return err
	}
	outfits, err := outfitplanapp.NewOutfitPlanService(accounts, postgres.NewOutfitPlanRepository(pool.ORM()))
	if err != nil {
		return err
	}
	wearEvents, err := weareventapp.NewWearEventService(accounts, postgres.NewWearEventRepository(pool.ORM()))
	if err != nil {
		return err
	}
	var mediaHandler *httpapi.MediaHandler
	if objects != nil {
		media, serviceErr := mediaapp.NewMediaService(accounts, postgres.NewMediaRepository(pool.ORM()), objects)
		if serviceErr != nil {
			return serviceErr
		}
		mediaHandler = httpapi.NewMediaHandler(media, cfg.SessionSecure)
		probes = append(probes, objects)
	}
	router, err := httpapi.NewRouter(startup, cfg.DocsEnabled, probes, httpapi.NewAccountHandler(accounts, cfg.SessionSecure, limiter), httpapi.NewPrivacyHandler(privacy, cfg.SessionSecure), httpapi.NewWardrobeHandler(wardrobe, cfg.SessionSecure), httpapi.NewOutfitPlanHandler(outfits, cfg.SessionSecure), httpapi.NewWearEventHandler(wearEvents, cfg.SessionSecure), cfg.HealthTimeout, log, mediaHandler)
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
