package bootstrap

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	"github.com/StephenQiu30/then-server/backend/internal/platform/config"
)

func TestGenerationAdmissionPolicyEnablesOnlyZeroCostFixtureTasks(t *testing.T) {
	local := config.GenerationConfig{
		Mode:               config.GenerationModeLocal,
		Enabled:            true,
		MaxConcurrentTasks: 2,
		MaxQuotaUnits:      10,
	}
	policy, estimator := generationAdmissionPolicy(local)
	if err := policy.Validate(); err != nil || !policy.Enabled || !policy.ZeroCost || estimator == nil {
		t.Fatalf("valid local fixture admission was not enabled: policy=%+v err=%v", policy, err)
	}
	quote, err := estimator.Estimate(generationapp.PurposeImage, "fixture", "fixture-image-v1", []byte(`{}`))
	if err != nil || quote.EstimatedMinorUnits != 0 || quote.ReservedQuotaUnits != 0 || quote.Currency != "" {
		t.Fatalf("fixture quote was not zero-cost: quote=%+v err=%v", quote, err)
	}
	modelQuote, err := estimator.Estimate(generationapp.PurposeModel, "fixture", "fixture-model-v1", []byte(`{}`))
	if err != nil || modelQuote.EstimatedMinorUnits != 0 || modelQuote.ReservedQuotaUnits != 0 || modelQuote.Currency != "" {
		t.Fatalf("fixture model quote was not zero-cost: quote=%+v err=%v", modelQuote, err)
	}
	for _, request := range []struct {
		purpose  generationapp.Purpose
		provider string
		model    string
	}{
		{generationapp.PurposeImage, "remote-provider", "remote-model"},
		{generationapp.PurposeModel, "fixture", "fixture-image-v1"},
		{generationapp.PurposeImage, "fixture", "fixture-model-v1"},
	} {
		if _, err := estimator.Estimate(request.purpose, request.provider, request.model, []byte(`{}`)); err != generationapp.ErrInvalidGenerationInput {
			t.Fatalf("unsupported local fixture request was accepted: %+v err=%v", request, err)
		}
	}

	for _, configuration := range []config.GenerationConfig{
		{Mode: config.GenerationModeOff},
		{Mode: config.GenerationModeRemote, Enabled: true},
		{Mode: config.GenerationModeLocal, Enabled: true, LocalImageEndpoint: "http://127.0.0.1:9100/v1/generate"},
	} {
		policy, estimator := generationAdmissionPolicy(configuration)
		if policy.Enabled || policy.ZeroCost || estimator != nil {
			t.Fatalf("generation admission opened outside the implemented local fixture mode: cfg=%+v policy=%+v", configuration, policy)
		}
	}
}

func TestGenerationWorkerRunnersStayClosedOutsideLocalFixtureMode(t *testing.T) {
	for _, cfg := range []config.Config{
		{Generation: config.GenerationConfig{Mode: config.GenerationModeOff}},
		{Generation: config.GenerationConfig{Mode: config.GenerationModeRemote, Enabled: true}},
		{Generation: config.GenerationConfig{Mode: config.GenerationModeLocal, Enabled: true, LocalImageEndpoint: "http://127.0.0.1:9100/v1/generate"}},
	} {
		runners, err := newGenerationWorkerRunners(cfg, nil, nil, nil, nil)
		if err != nil || len(runners) != 0 {
			t.Fatalf("unsupported generation mode started local workers: cfg=%+v runners=%d err=%v", cfg.Generation, len(runners), err)
		}
	}
	localFixture := config.Config{Generation: config.GenerationConfig{
		Mode:                  config.GenerationModeLocal,
		Enabled:               true,
		MaxConcurrentTasks:    2,
		MaxQuotaUnits:         10,
		MaxSubmissionAttempts: 2,
		ProviderTimeout:       time.Second,
		Retention:             time.Hour,
	}}
	if runners, err := newGenerationWorkerRunners(localFixture, nil, nil, nil, nil); err == nil || len(runners) != 0 {
		t.Fatalf("local fixture workers started without durable dependencies: runners=%d err=%v", len(runners), err)
	}
}

func TestGenerationWorkerLeaseCoversConfiguredExternalBudget(t *testing.T) {
	for _, timeout := range []time.Duration{time.Second, 30 * time.Second, 10 * time.Minute} {
		leaseTTL := generationWorkerLeaseTTL(timeout)
		if leaseTTL < timeout+30*time.Second || leaseTTL > 30*time.Minute {
			t.Fatalf("unsafe lease duration for external timeout %s: %s", timeout, leaseTTL)
		}
		if err := (generationapp.SubmissionWorkerPolicy{WorkerID: "worker", LeaseTTL: leaseTTL, ProviderTimeout: timeout, Retry: generationapp.RetryPolicy{MaxAttempts: 2, BaseDelay: time.Second, MaxDelay: time.Minute}}).Validate(); err != nil {
			t.Fatalf("submission policy rejected runtime lease for %s: %v", timeout, err)
		}
		if err := (generationapp.ObservationWorkerPolicy{WorkerID: "worker", LeaseTTL: leaseTTL, ProviderTimeout: timeout, PollInterval: time.Second}).Validate(); err != nil {
			t.Fatalf("observation policy rejected runtime lease for %s: %v", timeout, err)
		}
		if err := (generationapp.ResultWorkerPolicy{WorkerID: "worker", LeaseTTL: leaseTTL, FetchTimeout: timeout, RetryDelay: time.Second}).Validate(); err != nil {
			t.Fatalf("result policy rejected runtime lease for %s: %v", timeout, err)
		}
	}
}

func TestRunGenerationQueueProcessesReadyWorkAndStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := runGenerationQueue(ctx, log, "test", nil, nil, func(context.Context) (bool, error) {
		calls++
		if calls == 1 {
			return true, nil
		}
		cancel()
		return false, nil
	})
	if !errors.Is(err, context.Canceled) || calls != 2 {
		t.Fatalf("queue runner did not drain work then stop on context: calls=%d err=%v", calls, err)
	}
}

func TestRunGenerationQueueDelaysAfterFailedAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := runGenerationQueue(ctx, log, "test", nil, nil, func(context.Context) (bool, error) {
		calls++
		cancel()
		return false, errors.New("synthetic failure")
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("queue runner retried immediately after a failed attempt: calls=%d err=%v", calls, err)
	}
}

func TestGenerationWakeSignalsAreCoalescedPerStage(t *testing.T) {
	wake := newGenerationWakeSignals()
	if err := wake.WakeGeneration(context.Background(), "task"); err != nil {
		t.Fatal(err)
	}
	if err := wake.WakeGeneration(context.Background(), "task"); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"submission", "observation", "result"} {
		select {
		case <-wake.channel(stage):
		default:
			t.Fatalf("generation task did not wake %s stage", stage)
		}
		select {
		case <-wake.channel(stage):
			t.Fatalf("duplicate wake was not coalesced for %s stage", stage)
		default:
		}
	}

	wake.signal()
	if !waitForGenerationWorker(context.Background(), time.Hour, wake.channel("submission")) {
		t.Fatal("generation wake did not release a waiting worker")
	}
}
