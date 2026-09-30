//go:build integration

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/adapter/generationfixture"
	"github.com/StephenQiu30/then-server/backend/internal/adapter/objectstore"
	store "github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Advance only the publication boundary to simulate fetching across the deadline.
// All lease, settlement and cleanup operations still use real PostgreSQL.
type deadlinePublicationRepository struct {
	*store.GenerationRepository
	deadline time.Time
}

func (r deadlinePublicationRepository) PublishOutput(ctx context.Context, lease generationapp.Lease, asset generationapp.OutputAsset, _ time.Time) (generationapp.TaskView, error) {
	return r.GenerationRepository.PublishOutput(ctx, lease, asset, r.deadline)
}

func verifyGenerationProcessingDeadline(t *testing.T, ctx context.Context, database *gorm.DB, objects *objectstore.Store, accounts *accountapp.AccountService) {
	t.Helper()
	const timeout = 10 * time.Minute
	repository := store.NewGenerationRepository(database, timeout)
	owner, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "generation-deadline@example.test", DisplayName: "Deadline", Password: "generation-deadline-password"})
	if err != nil {
		t.Fatal(err)
	}
	service, err := generationapp.NewServiceWithCostEstimator(accounts, repository,
		generationapp.AdmissionPolicy{Enabled: true, Currency: "USD", MaxConcurrentTasks: 4, MaxQuotaUnits: 4, MaxBudgetMinorUnits: 100},
		generationapp.CostEstimatorFunc(func(generationapp.Purpose, string, string, []byte) (generationapp.CostEstimate, error) {
			return generationapp.CostEstimate{Currency: "USD", EstimatedMinorUnits: 25, ReservedQuotaUnits: 1}, nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	lookID := uuid.NewString()
	inputs := generationapp.InputSnapshot{LookID: lookID, LookRevision: 1, References: []generationapp.InputReference{
		{MediaID: uuid.NewString(), Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: strings.Repeat("a", 64)},
		{MediaID: uuid.NewString(), Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 1, SHA256: strings.Repeat("b", 64)},
	}}
	seedGenerationInputMedia(t, ctx, database, owner.User.ID, inputs.References)
	create := func(t *testing.T) generationapp.TaskView {
		t.Helper()
		created, err := service.Create(ctx, owner.Token, generationapp.CreateServiceInput{
			IdempotencyKey: uuid.NewString(), LookID: lookID, LookRevision: 1,
			Purpose: generationapp.PurposeImage, Provider: generationfixture.ProviderName, Model: generationfixture.ImageModel,
			Parameters: []byte(`{"seed":"` + uuid.NewString() + `"}`), Inputs: inputs,
			Consent: generationapp.ConsentReceipt{ID: uuid.NewString(), Purpose: generationapp.PurposeImage, PolicyVersion: "local-image-v1", AcceptedAt: time.Now().UTC().Add(-time.Second)},
		})
		if err != nil || created.Reused || created.View.Reservation == nil {
			t.Fatalf("create deadline task: result=%+v err=%v", created, err)
		}
		return created.View
	}
	prepare := func(t *testing.T, view generationapp.TaskView, stage string) {
		t.Helper()
		if stage == "submission" {
			return
		}
		at := view.Task.CreatedAt.Add(time.Second)
		_, lease, err := repository.AcquireLease(ctx, view.Task.ID, "deadline-prepare", at, 20*time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := repository.BeginSubmission(ctx, lease, at.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.RecordExternalTaskID(ctx, lease, "fixture:"+view.Task.ID, at.Add(2*time.Second)); err != nil {
			t.Fatal(err)
		}
		if stage == "result" {
			if _, err := repository.ApplyProviderState(ctx, lease, "fixture:"+view.Task.ID, generationapp.StatusValidating, "", at.Add(3*time.Second)); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := repository.ReleaseLease(ctx, lease, at.Add(4*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	type claimFunc func(context.Context, string, string, time.Time, time.Duration) (generationapp.TaskView, generationapp.Lease, bool, error)
	type acquireFunc func(context.Context, string, string, time.Time, time.Duration) (generationapp.TaskView, generationapp.Lease, error)
	for _, queue := range []struct {
		name    string
		claim   claimFunc
		acquire acquireFunc
	}{
		{"submission", repository.ClaimNextSubmissionLeaseForProvider, repository.AcquireLease},
		{"observation", repository.ClaimNextObservationLeaseForProvider, repository.AcquireObservationLease},
		{"result", repository.ClaimNextResultLeaseForProvider, repository.AcquireResultLease},
	} {
		t.Run(queue.name, func(t *testing.T) {
			old, fresh := create(t), create(t)
			prepare(t, old, queue.name)
			prepare(t, fresh, queue.name)
			deadline := old.Task.CreatedAt.Add(timeout)
			if _, _, err := queue.acquire(ctx, old.Task.ID, "expired-claim", deadline, time.Minute); !errors.Is(err, generationapp.ErrGenerationTaskTimedOut) {
				t.Fatalf("expired %s task acquired a lease: %v", queue.name, err)
			}
			claimed, lease, found, err := queue.claim(ctx, "deadline-claim", generationfixture.ProviderName, deadline, 20*time.Minute)
			if err != nil || !found || claimed.Task.ID != fresh.Task.ID {
				t.Fatalf("%s queue did not skip expired head: found=%v task=%s err=%v", queue.name, found, claimed.Task.ID, err)
			}
			freshDeadline := fresh.Task.CreatedAt.Add(timeout)
			if _, _, err := repository.RenewLease(ctx, lease, freshDeadline, 20*time.Minute); !errors.Is(err, generationapp.ErrGenerationTaskTimedOut) {
				t.Fatalf("overdue lease renewal = %v", err)
			}
			if queue.name == "submission" {
				if _, _, err := repository.BeginSubmission(ctx, lease, freshDeadline); !errors.Is(err, generationapp.ErrGenerationTaskTimedOut) {
					t.Fatalf("overdue task started submission: %v", err)
				}
			}
			unchanged, err := repository.Get(ctx, owner.User.ID, fresh.Task.ID)
			if err != nil || unchanged.Task.StatusRevision != claimed.Task.StatusRevision || unchanged.Task.LeaseUntil == nil || !unchanged.Task.LeaseUntil.Equal(lease.ExpiresAt) || unchanged.Task.SubmissionState != claimed.Task.SubmissionState {
				t.Fatalf("rejected deadline operation changed task: view=%+v err=%v", unchanged, err)
			}
			if _, err := repository.ReleaseLease(ctx, lease, freshDeadline); err != nil {
				t.Fatal(err)
			}
			if _, _, found, err := queue.claim(ctx, "overdue-only", generationfixture.ProviderName, freshDeadline, time.Minute); err != nil || found {
				t.Fatalf("%s queue reclaimed an overdue task: found=%v err=%v", queue.name, found, err)
			}
			for _, view := range []generationapp.TaskView{old, fresh} {
				if expired, err := repository.ExpireTaskIfDue(ctx, view.Task.ID, generationfixture.ProviderName, freshDeadline.Add(-timeout), freshDeadline, nil); err != nil || !expired {
					t.Fatalf("expire queue task: expired=%v err=%v", expired, err)
				}
			}
		})
	}

	t.Run("late-accepted-identity", func(t *testing.T) {
		inFlight := create(t)
		at := inFlight.Task.CreatedAt.Add(time.Minute)
		_, lease, err := repository.AcquireLease(ctx, inFlight.Task.ID, "late-receipt", at, 20*time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := repository.BeginSubmission(ctx, lease, at); err != nil {
			t.Fatal(err)
		}
		deadline := inFlight.Task.CreatedAt.Add(timeout)
		accepted, err := repository.RecordExternalTaskID(ctx, lease, "fixture:"+inFlight.Task.ID, deadline)
		if err != nil || accepted.Task.ExternalTaskID == "" || accepted.Task.SubmissionState != generationapp.SubmissionAccepted {
			t.Fatalf("late receipt lost its cleanup identity: view=%+v err=%v", accepted, err)
		}
		if _, err := repository.ReleaseLease(ctx, lease, deadline); err != nil {
			t.Fatal(err)
		}
		if expired, err := repository.ExpireTaskIfDue(ctx, inFlight.Task.ID, generationfixture.ProviderName, inFlight.Task.CreatedAt, deadline, nil); err != nil || !expired {
			t.Fatalf("late receipt task did not expire: expired=%v err=%v", expired, err)
		}
	})

	late := create(t)
	prepare(t, late, "result")
	deadline := late.Task.CreatedAt.Add(timeout)
	fixture, err := generationfixture.New(objects)
	if err != nil {
		t.Fatal(err)
	}
	resultWorker, err := generationapp.NewResultWorker(deadlinePublicationRepository{repository, deadline}, fixture, objects, generationapp.ResultWorkerPolicy{
		WorkerID: "late-result", Provider: generationfixture.ProviderName, LeaseTTL: 20 * time.Minute, FetchTimeout: time.Second, RetryDelay: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := resultWorker.RunOnceAt(ctx, late.Task.ID, late.Task.CreatedAt.Add(time.Minute))
	if !errors.Is(err, generationapp.ErrGenerationTaskTimedOut) || result.View.Task.Status != generationapp.StatusValidating || result.View.Asset != nil || result.View.Reservation == nil || result.View.Reservation.State != generationapp.ReservationReserved || result.View.Task.LeaseOwner != "" {
		t.Fatalf("late publication consumed quota or succeeded: result=%+v err=%v", result, err)
	}
	key, err := generationapp.OutputObjectKey(late.Task)
	if err != nil {
		t.Fatal(err)
	}
	versions, err := objects.ListOutputVersions(ctx, key)
	if err != nil || len(versions) != 1 {
		t.Fatalf("late output versions=%v err=%v", versions, err)
	}
	var orphanCount, outputCount int64
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("task_id = ? AND scope = ?", late.Task.ID, string(generationapp.CleanupScopeOrphanOutput)).Count(&orphanCount).Error; err != nil || orphanCount != 1 {
		t.Fatalf("late output cleanup not persisted: count=%d err=%v", orphanCount, err)
	}
	if err := database.WithContext(ctx).Table("generation_outputs").Where("task_id = ?", late.Task.ID).Count(&outputCount).Error; err != nil || outputCount != 0 {
		t.Fatalf("late output metadata persisted: count=%d err=%v", outputCount, err)
	}
	if expired, err := repository.ExpireTaskIfDue(ctx, late.Task.ID, generationfixture.ProviderName, late.Task.CreatedAt, deadline, versions); err != nil || !expired {
		t.Fatalf("expire late result: expired=%v err=%v", expired, err)
	}
	expired, err := repository.Get(ctx, owner.User.ID, late.Task.ID)
	if err != nil || expired.Task.Status != generationapp.StatusExpired || expired.Reservation == nil || expired.Reservation.State != generationapp.ReservationReleased || expired.Task.AccessRevokedAt == nil || expired.Cleanup == nil {
		t.Fatalf("late task did not expire and release quota: view=%+v err=%v", expired, err)
	}
	baseCleanup, err := objectstore.NewGenerationCleanupExecutor(objects)
	if err != nil {
		t.Fatal(err)
	}
	executor, err := generationfixture.NewCleanupExecutor(baseCleanup)
	if err != nil {
		t.Fatal(err)
	}
	cleanup, err := generationapp.NewCleanupWorker(repository, executor, generationapp.CleanupRetryPolicy{LeaseTTL: time.Minute, BaseDelay: time.Second, MaxDelay: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		found, err := cleanup.RunOnceAt(ctx, deadline.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			break
		}
	}
	cleaned, err := repository.Get(ctx, owner.User.ID, late.Task.ID)
	if err != nil || cleaned.Cleanup == nil || cleaned.Cleanup.Status != generationapp.CleanupComplete {
		t.Fatalf("late result cleanup did not converge: view=%+v err=%v", cleaned, err)
	}
	if _, err := objects.ReadOutputVersion(ctx, key, versions[0], generationapp.MaxGenerationImageOutputBytes); err == nil {
		t.Fatal("late result version remained readable after cleanup")
	}
}
