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
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/adapter/generationfixture"
	"github.com/StephenQiu30/then-server/backend/internal/adapter/httpapi"
	"github.com/StephenQiu30/then-server/backend/internal/adapter/messagequeue"
	"github.com/StephenQiu30/then-server/backend/internal/adapter/objectstore"
	"github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"
	exportapp "github.com/StephenQiu30/then-server/backend/internal/application/dataexport"
	diaryapp "github.com/StephenQiu30/then-server/backend/internal/application/diary"
	eventworkerapp "github.com/StephenQiu30/then-server/backend/internal/application/eventworker"
	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
	feedbackapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitfeedback"
	outfitplanapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitplan"
	privacyapp "github.com/StephenQiu30/then-server/backend/internal/application/privacy"
	syncapp "github.com/StephenQiu30/then-server/backend/internal/application/syncchange"
	wardrobeapp "github.com/StephenQiu30/then-server/backend/internal/application/wardrobe"
	weareventapp "github.com/StephenQiu30/then-server/backend/internal/application/wearevent"
	"github.com/StephenQiu30/then-server/backend/internal/platform/config"
	"github.com/StephenQiu30/then-server/backend/internal/platform/database"
	"github.com/StephenQiu30/then-server/backend/internal/platform/httpserver"
	"github.com/StephenQiu30/then-server/backend/internal/platform/mail"
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
	var runner *eventworkerapp.Runner
	var cleanupWorker *generationapp.CleanupWorker
	var retentionWorker *generationapp.RetentionWorker
	var generationWorkers []func(context.Context) error
	var broker *messagequeue.Broker
	generationWake := newGenerationWakeSignals()
	if cfg.Role == "worker" || cfg.Role == "all" {
		broker, err = messagequeue.Open(startup, cfg.KafkaBrokers, cfg.KafkaTopicPrefix)
		if err != nil {
			return err
		}
		defer broker.Close()
		runner, err = eventworkerapp.NewWithGenerationWake(postgres.NewMediaRepository(pool.ORM()), broker, objects, generationWake)
		if err != nil {
			return err
		}
		cleanupExecutor, executorErr := objectstore.NewGenerationCleanupExecutor(objects)
		if executorErr != nil {
			return executorErr
		}
		fixtureCleanup, executorErr := generationfixture.NewCleanupExecutor(cleanupExecutor)
		if executorErr != nil {
			return executorErr
		}
		cleanupWorker, err = generationapp.NewCleanupWorker(
			postgres.NewGenerationRepository(pool.ORM()),
			fixtureCleanup,
			generationapp.CleanupRetryPolicy{LeaseTTL: time.Minute, BaseDelay: 5 * time.Second, MaxDelay: 5 * time.Minute},
		)
		if err != nil {
			return err
		}
		if cfg.Generation.Retention > 0 {
			retentionWorker, err = generationapp.NewRetentionWorker(postgres.NewGenerationRepository(pool.ORM()), cfg.Generation.Retention)
			if err != nil {
				return err
			}
		}
		generationWorkers, err = newGenerationWorkerRunners(cfg, pool.ORM(), objects, log, generationWake)
		if err != nil {
			return err
		}
	}
	if cfg.Role == "worker" {
		log.Info("worker_started", "role", cfg.Role)
		workers := []func(context.Context) error{runner.Run, func(ctx context.Context) error { return cleanupWorker.Run(ctx, time.Second) }}
		if retentionWorker != nil {
			workers = append(workers, func(ctx context.Context) error { return retentionWorker.Run(ctx, time.Minute) })
		}
		workers = append(workers, generationWorkers...)
		err = runWorkers(ctx, workers...)
		log.Info("worker_stopped")
		return err
	}
	if cfg.Role == "all" {
		workers := []func(context.Context) error{runner.Run, func(ctx context.Context) error { return cleanupWorker.Run(ctx, time.Second) }, func(ctx context.Context) error { return runAPI(ctx, startup, cfg, pool, objects, log) }}
		if retentionWorker != nil {
			workers = append(workers, func(ctx context.Context) error { return retentionWorker.Run(ctx, time.Minute) })
		}
		workers = append(workers, generationWorkers...)
		err = runWorkers(ctx, workers...)
		return err
	}
	return runAPI(ctx, startup, cfg, pool, objects, log)
}

func runWorkers(ctx context.Context, workers ...func(context.Context) error) error {
	if len(workers) == 0 {
		return nil
	}
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan error, len(workers))
	for _, worker := range workers {
		go func(run func(context.Context) error) { results <- run(work) }(worker)
	}
	first := <-results
	cancel()
	for range len(workers) - 1 {
		result := <-results
		if first == nil || errors.Is(first, context.Canceled) {
			if result != nil && !errors.Is(result, context.Canceled) {
				first = result
			}
		}
	}
	if ctx.Err() != nil || errors.Is(first, context.Canceled) {
		return nil
	}
	return first
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
	var accountMail *accountapp.MailService
	if cfg.MailEnabled {
		sender, senderErr := mail.NewSender(cfg.MailSMTPAddr, cfg.MailFrom, cfg.MailAuthCode)
		if senderErr != nil {
			return senderErr
		}
		accountMail, err = accountapp.NewMailService(accounts, postgres.NewAccountRepository(pool.ORM()), sender, cfg.MailKey, cfg.MailLinkBase)
		if err != nil {
			return err
		}
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
	feedback, err := feedbackapp.NewService(accounts, postgres.NewOutfitFeedbackRepository(pool.ORM()))
	if err != nil {
		return err
	}
	diaries, err := diaryapp.NewService(accounts, postgres.NewDiaryRepository(pool.ORM()))
	if err != nil {
		return err
	}
	syncChanges, err := syncapp.New(accounts, postgres.NewSyncRepository(pool.ORM()))
	if err != nil {
		return err
	}
	generationPolicy, generationCostEstimator := generationAdmissionPolicy(cfg.Generation)
	generations, err := generationapp.NewServiceWithCostEstimator(accounts, postgres.NewGenerationRepository(pool.ORM()), generationPolicy, generationCostEstimator)
	if err != nil {
		return err
	}
	generationHandler := httpapi.NewGenerationHandler(generations, cfg.SessionSecure).WithOutputSigner(objects).WithOutputRetention(cfg.Generation.Retention)
	var mediaHandler *httpapi.MediaHandler
	var communityHandler *httpapi.CommunityHandler
	var exportHandler *httpapi.DataExportHandler
	var exports *exportapp.Service
	if objects != nil {
		exports, err = exportapp.New(accounts, postgres.NewDataExportRepository(pool.ORM()), objects)
		if err != nil {
			return err
		}
		exportHandler = httpapi.NewDataExportHandler(exports, limiter)
		media, serviceErr := mediaapp.NewMediaService(accounts, postgres.NewMediaRepository(pool.ORM()), objects)
		if serviceErr != nil {
			return serviceErr
		}
		mediaHandler = httpapi.NewMediaHandler(media, cfg.SessionSecure)
		community, serviceErr := communityapp.NewService(accounts, postgres.NewCommunityRepository(pool.ORM()))
		if serviceErr != nil {
			return serviceErr
		}
		communityHandler = httpapi.NewCommunityHandler(community, objects, cfg.SessionSecure)
		probes = append(probes, objects)
	}
	accountHandler := httpapi.NewAccountHandlerWithMail(accounts, accountMail, cfg.SessionSecure, limiter)
	if objects != nil {
		accountHandler.WithAvatarObjects(objects)
	}
	router, err := httpapi.NewRouterWithGeneration(
		startup, cfg.DocsEnabled, probes,
		accountHandler,
		httpapi.NewPrivacyHandler(privacy, cfg.SessionSecure),
		httpapi.NewWardrobeHandler(wardrobe, cfg.SessionSecure),
		httpapi.NewOutfitPlanHandler(outfits, cfg.SessionSecure),
		httpapi.NewWearEventHandler(wearEvents, cfg.SessionSecure),
		httpapi.NewDiaryHandler(diaries, cfg.SessionSecure),
		communityHandler, httpapi.NewFeedbackHandler(feedback, cfg.SessionSecure),
		exportHandler, httpapi.NewSyncHandler(syncChanges, cfg.SessionSecure), generationHandler, cfg.HealthTimeout, log, mediaHandler,
	)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return errors.New("HTTP listen failed")
	}
	log.Info("api_started", "address", listener.Addr().String(), "role", cfg.Role)
	background := []func(context.Context) error{func(ctx context.Context) error { return accounts.RunReceiptCleanup(ctx, log) }}
	if accountMail != nil {
		background = append(background, accountMail.Run)
	}
	if exports != nil {
		background = append(background, func(ctx context.Context) error { return exports.Run(ctx, log) })
	}
	if len(background) > 0 {
		work, cancelWork := context.WithCancel(ctx)
		defer cancelWork()
		results := make(chan error, len(background)+1)
		for _, run := range background {
			go func() { results <- run(work) }()
		}
		go func() { results <- httpserver.Serve(work, listener, router, cfg.ShutdownTimeout, router.Drain) }()
		err = <-results
		cancelWork()
		for range len(background) {
			<-results
		}
		return err
	}
	if err = httpserver.Serve(ctx, listener, router, cfg.ShutdownTimeout, router.Drain); err != nil {
		return err
	}
	log.Info("api_stopped")
	return nil
}
