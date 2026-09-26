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
	generationWorkerLeaseMargin    = time.Minute
	generationWorkerPollInterval   = time.Second
	generationWorkerRetryDelay     = 2 * time.Second
	generationWorkerRetryMaxDelay  = time.Minute
	generationWorkerFailureBackoff = 5 * time.Second
)

type generationQueueStep func(context.Context) (bool, error)

func generationWorkerLeaseTTL(timeout time.Duration) time.Duration {
	return timeout + generationWorkerLeaseMargin
}

type generationWakeSignals struct {
	submission  chan struct{}
	observation chan struct{}
	result      chan struct{}
}

func newGenerationWakeSignals() *generationWakeSignals {
	return &generationWakeSignals{
		submission:  make(chan struct{}, 1),
		observation: make(chan struct{}, 1),
		result:      make(chan struct{}, 1),
	}
}

func (s *generationWakeSignals) WakeGeneration(ctx context.Context, _ string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.signal()
	return nil
}

func (s *generationWakeSignals) signal() {
	if s == nil {
		return
	}
	for _, wake := range []chan struct{}{s.submission, s.observation, s.result} {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
}

func (s *generationWakeSignals) channel(stage string) <-chan struct{} {
	if s == nil {
		return nil
	}
	switch stage {
	case "submission":
		return s.submission
	case "observation":
		return s.observation
	case "result":
		return s.result
	default:
		return nil
	}
}

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
		if provider != generationfixture.ProviderName ||
			!(purpose == generationapp.PurposeImage && model == generationfixture.ImageModel ||
				purpose == generationapp.PurposeModel && model == generationfixture.ModelModel) {
			return generationapp.CostEstimate{}, generationapp.ErrInvalidGenerationInput
		}
		return generationapp.CostEstimate{}, nil
	})
	return policy, estimator
}

func newGenerationWorkerRunners(cfg config.Config, database *gorm.DB, objects *objectstore.Store, log *slog.Logger, wake *generationWakeSignals) ([]func(context.Context) error, error) {
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
	leaseTTL := generationWorkerLeaseTTL(cfg.Generation.ProviderTimeout)
	retry := generationapp.RetryPolicy{
		MaxAttempts: cfg.Generation.MaxSubmissionAttempts,
		BaseDelay:   time.Second,
		MaxDelay:    generationWorkerRetryMaxDelay,
	}
	submission, err := generationapp.NewSubmissionWorker(repository, fixture, generationapp.SubmissionWorkerPolicy{
		WorkerID:        "generation-submission-local",
		Provider:        generationfixture.ProviderName,
		LeaseTTL:        leaseTTL,
		ProviderTimeout: cfg.Generation.ProviderTimeout,
		Retry:           retry,
	})
	if err != nil {
		return nil, err
	}
	observation, err := generationapp.NewObservationWorker(repository, fixture, generationapp.ObservationWorkerPolicy{
		WorkerID:        "generation-observation-local",
		Provider:        generationfixture.ProviderName,
		LeaseTTL:        leaseTTL,
		ProviderTimeout: cfg.Generation.ProviderTimeout,
		PollInterval:    generationWorkerPollInterval,
	})
	if err != nil {
		return nil, err
	}
	result, err := generationapp.NewResultWorker(repository, fixture, objects, generationapp.ResultWorkerPolicy{
		WorkerID:     "generation-result-local",
		Provider:     generationfixture.ProviderName,
		LeaseTTL:     leaseTTL,
		FetchTimeout: cfg.Generation.ProviderTimeout,
		RetryDelay:   generationWorkerRetryDelay,
	})
	if err != nil {
		return nil, err
	}
	return []func(context.Context) error{
		func(ctx context.Context) error {
			return runGenerationQueue(ctx, log, "submission", wake.channel("submission"), wake, func(ctx context.Context) (bool, error) {
				found, _, err := submission.RunNext(ctx)
				return found, err
			})
		},
		func(ctx context.Context) error {
			return runGenerationQueue(ctx, log, "observation", wake.channel("observation"), wake, func(ctx context.Context) (bool, error) {
				found, _, err := observation.RunNext(ctx)
				return found, err
			})
		},
		func(ctx context.Context) error {
			return runGenerationQueue(ctx, log, "result", wake.channel("result"), wake, func(ctx context.Context) (bool, error) {
				found, _, err := result.RunNext(ctx)
				return found, err
			})
		},
	}, nil
}

func runGenerationQueue(ctx context.Context, log *slog.Logger, stage string, notifications <-chan struct{}, wake *generationWakeSignals, next generationQueueStep) error {
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
			if !waitForGenerationWorker(ctx, generationWorkerFailureBackoff, nil) {
				return ctx.Err()
			}
			continue
		}
		if found {
			wake.signal()
			continue
		}
		if !waitForGenerationWorker(ctx, generationWorkerPollInterval, notifications) {
			return ctx.Err()
		}
	}
}

func waitForGenerationWorker(ctx context.Context, delay time.Duration, notifications <-chan struct{}) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-notifications:
		return true
	case <-timer.C:
		return true
	}
}
