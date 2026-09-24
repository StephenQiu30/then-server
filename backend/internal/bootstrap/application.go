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
	var broker *messagequeue.Broker
	if cfg.Role == "worker" || cfg.Role == "all" {
		broker, err = messagequeue.Open(startup, cfg.KafkaBrokers, cfg.KafkaTopicPrefix)
		if err != nil {
			return err
		}
		defer broker.Close()
		runner, err = eventworkerapp.New(postgres.NewMediaRepository(pool.ORM()), broker, objects)
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
	generationPolicy := generationapp.AdmissionPolicy{
		Enabled:             cfg.Generation.Enabled,
		ZeroCost:            cfg.Generation.Mode == config.GenerationModeLocal,
		Currency:            cfg.Generation.Currency,
		MaxConcurrentTasks:  cfg.Generation.MaxConcurrentTasks,
		MaxQuotaUnits:       cfg.Generation.MaxQuotaUnits,
		MaxBudgetMinorUnits: cfg.Generation.MaxBudgetMinorUnits,
	}
	generations, err := generationapp.NewService(accounts, postgres.NewGenerationRepository(pool.ORM()), generationPolicy)
	if err != nil {
		return err
	}
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
		exportHandler, httpapi.NewSyncHandler(syncChanges, cfg.SessionSecure), httpapi.NewGenerationHandler(generations, cfg.SessionSecure), cfg.HealthTimeout, log, mediaHandler,
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
