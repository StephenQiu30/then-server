//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/adapter/generationfixture"
	"github.com/StephenQiu30/then-server/backend/internal/adapter/objectstore"
	store "github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	"github.com/StephenQiu30/then-server/backend/tests/internal/testcontainer"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGenerationRetentionRevokesPublishedImageAndDependentModel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	password := rand.Text()
	postgresContainer := integrationContainer(t, ctx, testcontainers.ContainerRequest{
		Image:        "postgres:18.4@sha256:a02db8cac496f15b094798a38254f14d6e00741f709360e5e00bb6668ea31636",
		Env:          map[string]string{"POSTGRES_USER": "then_test", "POSTGRES_DB": "then_test", "POSTGRES_PASSWORD": password},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(time.Minute),
	})
	databaseURL := "postgres://then_test:" + url.QueryEscape(password) + "@" + mappedAddress(t, ctx, postgresContainer, "5432/tcp") + "/then_test?sslmode=disable"
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx, database); err != nil {
		t.Fatal(err)
	}
	objectStorePassword := rand.Text()
	minioContainer := integrationContainer(t, ctx, testcontainers.ContainerRequest{
		Image: testcontainer.MinIOImage(), Cmd: []string{"server", "/data"},
		Env:          map[string]string{"MINIO_ROOT_USER": "then_test", "MINIO_ROOT_PASSWORD": objectStorePassword},
		ExposedPorts: []string{"9000/tcp"},
		WaitingFor:   wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").WithStartupTimeout(time.Minute),
	})
	objects := openGenerationObjectStore(t, ctx, mappedAddress(t, ctx, minioContainer, "9000/tcp"), objectStorePassword)
	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	if err != nil {
		t.Fatal(err)
	}
	owner, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "generation-retention@example.test", DisplayName: "Retention", Password: "generation-retention-password"})
	if err != nil {
		t.Fatal(err)
	}
	repository := store.NewGenerationRepository(database)
	policy := generationapp.AdmissionPolicy{Enabled: true, ZeroCost: true, MaxConcurrentTasks: 2, MaxQuotaUnits: 2, OutputRetention: time.Hour}
	service, err := generationapp.NewService(accounts, repository, policy)
	if err != nil {
		t.Fatal(err)
	}
	lookID := uuid.NewString()
	inputs := generationapp.InputSnapshot{LookID: lookID, LookRevision: 1, References: []generationapp.InputReference{
		{MediaID: uuid.NewString(), Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: strings.Repeat("a", 64)},
		{MediaID: uuid.NewString(), Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 1, SHA256: strings.Repeat("b", 64)},
	}}
	seedGenerationInputMedia(t, ctx, database, owner.User.ID, inputs.References)
	imageRequest := generationapp.CreateServiceInput{
		IdempotencyKey: uuid.NewString(), LookID: lookID, LookRevision: 1,
		Purpose: generationapp.PurposeImage, Provider: generationfixture.ProviderName, Model: generationfixture.ImageModel,
		Parameters: []byte(`{"seed":"retention-image"}`), Inputs: inputs,
		Consent: generationapp.ConsentReceipt{ID: uuid.NewString(), Purpose: generationapp.PurposeImage, PolicyVersion: "local-image-v1", AcceptedAt: time.Now().UTC().Add(-time.Second)},
	}
	image, err := service.Create(ctx, owner.Token, imageRequest)
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := generationfixture.New(objects)
	if err != nil {
		t.Fatal(err)
	}
	submission, err := generationapp.NewSubmissionWorker(repository, fixture, generationapp.SubmissionWorkerPolicy{
		WorkerID: "retention-submission", Provider: generationfixture.ProviderName, LeaseTTL: time.Minute, ProviderTimeout: time.Second,
		Retry: generationapp.RetryPolicy{MaxAttempts: 2, BaseDelay: time.Second, MaxDelay: time.Minute},
	})
	if err != nil {
		t.Fatal(err)
	}
	observation, err := generationapp.NewObservationWorker(repository, fixture, generationapp.ObservationWorkerPolicy{
		WorkerID: "retention-observation", Provider: generationfixture.ProviderName, LeaseTTL: time.Minute, ProviderTimeout: time.Second, PollInterval: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := generationapp.NewResultWorker(repository, fixture, objects, generationapp.ResultWorkerPolicy{
		WorkerID: "retention-result", Provider: generationfixture.ProviderName, LeaseTTL: time.Minute, FetchTimeout: time.Second, RetryDelay: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	complete := func(taskID string) generationapp.OutputAsset {
		t.Helper()
		if found, _, err := submission.RunNext(ctx); err != nil || !found {
			t.Fatalf("submit %s: found=%v err=%v", taskID, found, err)
		}
		if found, _, err := observation.RunNext(ctx); err != nil || !found {
			t.Fatalf("observe %s: found=%v err=%v", taskID, found, err)
		}
		found, published, err := result.RunNext(ctx)
		if err != nil || !found || published.View.Task.ID != taskID || published.View.Asset == nil {
			t.Fatalf("publish %s: found=%v view=%+v err=%v", taskID, found, published.View, err)
		}
		return *published.View.Asset
	}
	imageAsset := complete(image.View.Task.ID)
	if _, err := service.ConfirmImage(ctx, owner.Token, image.View.Task.ID); err != nil {
		t.Fatal(err)
	}
	modelRequest := generationapp.CreateServiceInput{
		IdempotencyKey: uuid.NewString(), LookID: lookID, LookRevision: 1,
		Purpose: generationapp.PurposeModel, Provider: generationfixture.ProviderName, Model: generationfixture.ModelModel,
		Parameters: []byte(`{"seed":"retention-model"}`),
		Inputs: generationapp.InputSnapshot{LookID: lookID, LookRevision: 1, ImageAssetID: imageAsset.ID, ImageSHA256: imageAsset.SHA256,
			References: []generationapp.InputReference{{MediaID: imageAsset.ID, Role: generationapp.InputRoleLookImage, Ordinal: 0, Revision: 1, SHA256: imageAsset.SHA256}}},
		Consent: generationapp.ConsentReceipt{ID: uuid.NewString(), Purpose: generationapp.PurposeModel, PolicyVersion: "local-model-v1", AcceptedAt: time.Now().UTC().Add(-time.Second)},
	}
	model, err := service.Create(ctx, owner.Token, modelRequest)
	if err != nil {
		t.Fatal(err)
	}
	modelAsset := complete(model.View.Task.ID)
	if _, err := service.GetOutput(ctx, owner.Token, image.View.Task.ID); err != nil {
		t.Fatalf("unexpired image output unavailable: %v", err)
	}
	deadline := imageAsset.PublishedAt.Add(time.Hour)
	if _, err := repository.ConfirmImage(ctx, owner.User.ID, image.View.Task.ID, deadline, time.Hour); !errors.Is(err, generationapp.ErrImageNotConfirmable) {
		t.Fatalf("expired image accepted confirmation: %v", err)
	}
	shortPolicy := policy
	shortPolicy.OutputRetention = time.Nanosecond
	shortService, err := generationapp.NewService(accounts, repository, shortPolicy)
	if err != nil {
		t.Fatal(err)
	}
	modelRequest.IdempotencyKey = uuid.NewString()
	modelRequest.Parameters = []byte(`{"seed":"after-expiry"}`)
	if _, err := shortService.Create(ctx, owner.Token, modelRequest); !errors.Is(err, generationapp.ErrGenerationSourceUnavailable) {
		t.Fatalf("expired image accepted as a new model source: %v", err)
	}
	if _, err := shortService.GetOutput(ctx, owner.Token, image.View.Task.ID); !errors.Is(err, generationapp.ErrGenerationNotFound) {
		t.Fatalf("expired image remained readable before cleanup: %v", err)
	}
	replayed, err := shortService.Create(ctx, owner.Token, imageRequest)
	if err != nil || !replayed.Reused || replayed.View.Task.ID != image.View.Task.ID {
		t.Fatalf("expired request replay created a second task: result=%+v err=%v", replayed, err)
	}
	imageRequest.IdempotencyKey = uuid.NewString()
	replacement, err := shortService.Create(ctx, owner.Token, imageRequest)
	if err != nil || replacement.Reused || replacement.View.Task.ID == image.View.Task.ID {
		t.Fatalf("new intent reused an expired output: result=%+v err=%v", replacement, err)
	}
	beforeDeadline := deadline.Add(-time.Minute)
	if processed, err := repository.ExpirePublishedOutputs(ctx, beforeDeadline.Add(-time.Hour), beforeDeadline, 10); err != nil || processed != 0 {
		t.Fatalf("output expired before deadline: count=%d err=%v", processed, err)
	}
	processed, err := repository.ExpirePublishedOutputs(ctx, deadline.Add(-time.Hour), deadline, 10)
	if err != nil || processed < 1 {
		t.Fatalf("due output was not revoked: count=%d err=%v", processed, err)
	}
	for _, taskID := range []string{image.View.Task.ID, model.View.Task.ID} {
		view, err := repository.Get(ctx, owner.User.ID, taskID)
		if err != nil || view.Task.Status != generationapp.StatusSucceeded || view.Task.AccessRevokedAt == nil || view.Cleanup == nil || view.Cleanup.Status != generationapp.CleanupPending || view.Asset != nil {
			t.Fatalf("expired output did not preserve success and revoke access: task=%s view=%+v err=%v", taskID, view, err)
		}
		if _, err := service.GetOutput(ctx, owner.Token, taskID); !errors.Is(err, generationapp.ErrGenerationNotFound) {
			t.Fatalf("revoked output remained accessible: task=%s err=%v", taskID, err)
		}
	}
	baseCleanup, err := objectstore.NewGenerationCleanupExecutor(objects)
	if err != nil {
		t.Fatal(err)
	}
	fixtureCleanup, err := generationfixture.NewCleanupExecutor(baseCleanup)
	if err != nil {
		t.Fatal(err)
	}
	cleanup, err := generationapp.NewCleanupWorker(repository, fixtureCleanup, generationapp.CleanupRetryPolicy{LeaseTTL: time.Minute, BaseDelay: time.Second, MaxDelay: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		if found, err := cleanup.RunOnceAt(ctx, deadline.Add(time.Duration(i+1)*time.Second)); err != nil || !found {
			t.Fatalf("run expired output cleanup %d: found=%v err=%v", i, found, err)
		}
	}
	for _, asset := range []generationapp.OutputAsset{imageAsset, modelAsset} {
		if _, err := objects.ReadOutputVersion(ctx, asset.ObjectKey, asset.ObjectVersionID, asset.ByteSize); err == nil {
			t.Fatalf("expired object version remained readable: %s", asset.ID)
		}
	}
	if processed, err := repository.ExpirePublishedOutputs(ctx, deadline.Add(-time.Hour+time.Minute), deadline.Add(time.Minute), 10); err != nil || processed != 0 {
		t.Fatalf("completed expiration was not idempotent: count=%d err=%v", processed, err)
	}

	// A task deadline is independent of the successful output retention clock.
	queuedAt := replacement.View.Task.CreatedAt.Add(time.Second)
	_, queuedLease, err := repository.AcquireLease(ctx, replacement.View.Task.ID, "timeout-in-flight", queuedAt, 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if due, err := repository.ListDueTasks(ctx, generationfixture.ProviderName, replacement.View.Task.CreatedAt.Add(-time.Second), queuedAt, 10); err != nil || len(due) != 0 {
		t.Fatalf("task listed before deadline: due=%d err=%v", len(due), err)
	}
	if expired, err := repository.ExpireTaskIfDue(ctx, replacement.View.Task.ID, generationfixture.ProviderName, replacement.View.Task.CreatedAt, replacement.View.Task.CreatedAt.Add(time.Hour), nil); err != nil || expired {
		t.Fatalf("live lease was expired: expired=%v err=%v", expired, err)
	}
	queuedDeadline := queuedLease.ExpiresAt.Add(time.Second)
	if expired, err := repository.ExpireTaskIfDue(ctx, replacement.View.Task.ID, generationfixture.ProviderName, replacement.View.Task.CreatedAt, queuedDeadline, nil); err != nil || !expired {
		t.Fatalf("overdue task did not expire: expired=%v err=%v", expired, err)
	}
	queuedView, err := repository.Get(ctx, owner.User.ID, replacement.View.Task.ID)
	if err != nil || queuedView.Task.Status != generationapp.StatusExpired || queuedView.Task.AccessRevokedAt == nil || queuedView.Cleanup == nil || queuedView.Cleanup.Status != generationapp.CleanupPending {
		t.Fatalf("expired task did not release/revoke: view=%+v err=%v", queuedView, err)
	}
	if _, err := repository.ReleaseLease(ctx, queuedLease, queuedDeadline); !errors.Is(err, generationapp.ErrGenerationLeaseConflict) && !errors.Is(err, generationapp.ErrGenerationLeaseExpired) {
		t.Fatalf("stale worker mutated expired task: %v", err)
	}
	if expired, err := repository.ExpireTaskIfDue(ctx, replacement.View.Task.ID, generationfixture.ProviderName, replacement.View.Task.CreatedAt, queuedDeadline, nil); err != nil || expired {
		t.Fatalf("terminal timeout was not idempotent: expired=%v err=%v", expired, err)
	}

	imageRequest.IdempotencyKey = uuid.NewString()
	imageRequest.Parameters = []byte(`{"seed":"timeout-unpublished"}`)
	unpublished, err := service.Create(ctx, owner.Token, imageRequest)
	if err != nil {
		t.Fatal(err)
	}
	if found, _, err := submission.RunNext(ctx); err != nil || !found {
		t.Fatalf("submit timeout task: found=%v err=%v", found, err)
	}
	if found, _, err := observation.RunNext(ctx); err != nil || !found {
		t.Fatalf("observe timeout task: found=%v err=%v", found, err)
	}
	current, err := repository.Get(ctx, owner.User.ID, unpublished.View.Task.ID)
	if err != nil || current.Task.Status != generationapp.StatusValidating {
		t.Fatalf("task did not reach validating: view=%+v err=%v", current, err)
	}
	key, err := generationapp.OutputObjectKey(current.Task)
	if err != nil {
		t.Fatal(err)
	}
	version, err := objects.PutDerived(ctx, key, strings.NewReader("unpublished-output"), int64(len("unpublished-output")))
	if err != nil {
		t.Fatal(err)
	}
	versions, err := objects.ListOutputVersions(ctx, key)
	if err != nil || len(versions) != 1 || versions[0] != version {
		t.Fatalf("unpublished version inventory: versions=%v err=%v", versions, err)
	}
	resultDeadline := current.Task.CreatedAt.Add(time.Hour)
	if expired, err := repository.ExpireTaskIfDue(ctx, current.Task.ID, generationfixture.ProviderName, current.Task.CreatedAt, resultDeadline, versions); err != nil || !expired {
		t.Fatalf("validating task did not expire: expired=%v err=%v", expired, err)
	}
	expiredView, err := repository.Get(ctx, owner.User.ID, current.Task.ID)
	if err != nil || expiredView.Task.Status != generationapp.StatusExpired || expiredView.Cleanup == nil || len(expiredView.Cleanup.Targets) != 2 {
		t.Fatalf("timeout omitted provider/object cleanup: targets=%+v view=%+v err=%v", expiredView.Cleanup.Targets, expiredView, err)
	}
	if found, err := cleanup.RunOnceAt(ctx, resultDeadline.Add(time.Second)); err != nil || !found {
		t.Fatalf("timeout cleanup: found=%v err=%v", found, err)
	}
	if _, err := objects.ReadOutputVersion(ctx, key, version, int64(len("unpublished-output"))); err == nil {
		t.Fatal("timed out task left an unpublished output version")
	}
}
