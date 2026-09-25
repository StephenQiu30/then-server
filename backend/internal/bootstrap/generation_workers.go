package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/adapter/generationfixture"
	"github.com/StephenQiu30/then-server/backend/internal/adapter/objectstore"
	store "github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	"github.com/StephenQiu30/then-server/backend/internal/platform/config"
	"gorm.io/gorm"
)

const (
	generationWorkerLeaseTTL       = time.Minute
	generationWorkerPollInterval   = time.Second
	generationWorkerRetryDelay     = 2 * time.Second
	generationWorkerRetryMaxDelay  = time.Minute
	generationWorkerFailureBackoff = 5 * time.Second
)

type generationQueueStep func(context.Context) (bool, error)

func generationAdmissionPolicy(configuration config.GenerationConfig) (generationapp.AdmissionPolicy, generationapp.CostEstimator) {
	fixtureEnabled := configuration.Enabled && configuration.Mode == config.GenerationModeLocal && configuration.LocalImageEndpoint == ""
	policy := generationapp.AdmissionPolicy{
		Enabled:             fixtureEnabled,
		ZeroCost:            fixtureEnabled,
		Currency:            configuration.Currency,
		MaxConcurrentTasks:  configuration.MaxConcurrentTasks,
		MaxQuotaUnits:       configuration.MaxQuotaUnits,
		MaxBudgetMinorUnits: configuration.MaxBudgetMinorUnits,
	}
	if !fixtureEnabled {
		return policy, nil
	}
	estimator := generationapp.CostEstimatorFunc(func(purpose generationapp.Purpose, provider, model string, _ []byte) (generationapp.CostEstimate, error) {
		if purpose != generationapp.PurposeImage || provider != generationfixture.ProviderName || model != generationfixture.ImageModel {
			return generationapp.CostEstimate{}, generationapp.ErrInvalidGenerationInput
		}
		return generationapp.CostEstimate{}, nil
	})
	return policy, estimator
}

func newGenerationWorkerRunners(cfg config.Config, database *gorm.DB, objects *objectstore.Store, log *slog.Logger) ([]func(context.Context) error, error) {
	if cfg.Generation.Mode != config.GenerationModeLocal || cfg.Generation.LocalImageEndpoint != "" {
		return nil, nil
	}
	if !cfg.Generation.Enabled || database == nil || objects == nil {
		return nil, generationapp.ErrInvalidGenerationWorker
	}
	fixture, err := generationfixture.New(objects)
	if err != nil {
		return nil, err
	}
	repository := store.NewGenerationRepository(database)
	retry := generationapp.RetryPolicy{
		MaxAttempts: cfg.Generation.MaxSubmissionAttempts,
		BaseDelay:   time.Second,
		MaxDelay:    generationWorkerRetryMaxDelay,
	}
	submission, err := generationapp.NewSubmissionWorker(repository, fixture, generationapp.SubmissionWorkerPolicy{
		WorkerID:        "generation-submission-local",
		Provider:        generationfixture.ProviderName,
		LeaseTTL:        generationWorkerLeaseTTL,
		ProviderTimeout: cfg.Generation.ProviderTimeout,
		Retry:           retry,
	})
	if err != nil {
		return nil, err
	}
	observation, err := generationapp.NewObservationWorker(repository, fixture, generationapp.ObservationWorkerPolicy{
		WorkerID:        "generation-observation-local",
		Provider:        generationfixture.ProviderName,
		LeaseTTL:        generationWorkerLeaseTTL,
		ProviderTimeout: cfg.Generation.ProviderTimeout,
		PollInterval:    generationWorkerPollInterval,
	})
	if err != nil {
		return nil, err
	}
	result, err := generationapp.NewResultWorker(repository, fixture, objects, generationapp.ResultWorkerPolicy{
		WorkerID:     "generation-result-local",
		Provider:     generationfixture.ProviderName,
		LeaseTTL:     generationWorkerLeaseTTL,
		FetchTimeout: cfg.Generation.ProviderTimeout,
		RetryDelay:   generationWorkerRetryDelay,
	})
	if err != nil {
		return nil, err
	}
	return []func(context.Context) error{
		func(ctx context.Context) error {
			return runGenerationQueue(ctx, log, "submission", func(ctx context.Context) (bool, error) {
				found, _, err := submission.RunNext(ctx)
				return found, err
			})
		},
		func(ctx context.Context) error {
			return runGenerationQueue(ctx, log, "observation", func(ctx context.Context) (bool, error) {
				found, _, err := observation.RunNext(ctx)
				return found, err
			})
		},
		func(ctx context.Context) error {
			return runGenerationQueue(ctx, log, "result", func(ctx context.Context) (bool, error) {
				found, _, err := result.RunNext(ctx)
				return found, err
			})
		},
	}, nil
}

func runGenerationQueue(ctx context.Context, log *slog.Logger, stage string, next generationQueueStep) error {
	if log == nil || next == nil {
		return generationapp.ErrInvalidGenerationWorker
	}
	for {
		found, err := next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Warn("generation_worker_attempt_failed", "stage", stage, "error_type", fmt.Sprintf("%T", err))
			if !waitForGenerationWorker(ctx, generationWorkerFailureBackoff) {
				return ctx.Err()
			}
			continue
		}
		if found {
			continue
		}
		if !waitForGenerationWorker(ctx, generationWorkerPollInterval) {
			return ctx.Err()
		}
	}
}

func waitForGenerationWorker(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
