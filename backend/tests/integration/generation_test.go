//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"errors"
	"net/url"
	"testing"
	"time"

	store "github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGenerationPersistenceLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	password := rand.Text()
	container := integrationContainer(t, ctx, testcontainers.ContainerRequest{
		Image:        "postgres:18.4@sha256:a02db8cac496f15b094798a38254f14d6e00741f709360e5e00bb6668ea31636",
		Env:          map[string]string{"POSTGRES_USER": "then_test", "POSTGRES_DB": "then_test", "POSTGRES_PASSWORD": password},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(time.Minute),
	})
	databaseAddress := mappedAddress(t, ctx, container, "5432/tcp")
	databaseURL := "postgres://then_test:" + url.QueryEscape(password) + "@" + databaseAddress + "/then_test?sslmode=disable"
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open generation database: %v", err)
	}
	if err := store.Migrate(ctx, database); err != nil {
		t.Fatalf("migrate generation database: %v", err)
	}

	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	if err != nil {
		t.Fatalf("construct account service: %v", err)
	}
	first, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "generation-first@example.test", DisplayName: "Generation First", Password: "correct-password-generation-first"})
	if err != nil {
		t.Fatalf("register first generation owner: %v", err)
	}
	second, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "generation-second@example.test", DisplayName: "Generation Second", Password: "correct-password-generation-second"})
	if err != nil {
		t.Fatalf("register second generation owner: %v", err)
	}

	policy := generationapp.AdmissionPolicy{Enabled: true, ZeroCost: true, MaxConcurrentTasks: 4, MaxQuotaUnits: 4}
	generations, err := generationapp.NewService(accounts, store.NewGenerationRepository(database), policy)
	if err != nil {
		t.Fatalf("construct generation service: %v", err)
	}
	acceptedAt := time.Now().UTC().Add(-time.Second)
	input := generationapp.CreateServiceInput{
		IdempotencyKey: "generation-request-one",
		LookID:         "00000000-0000-4000-8000-000000000101",
		LookRevision:   2,
		Purpose:        generationapp.PurposeImage,
		Provider:       "fixture",
		Model:          "look-image-v1",
		Parameters:     []byte(`{"seed":"one"}`),
		Inputs: generationapp.InputSnapshot{
			LookID:       "00000000-0000-4000-8000-000000000101",
			LookRevision: 2,
			References: []generationapp.InputReference{
				{MediaID: "00000000-0000-4000-8000-000000000201", Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				{MediaID: "00000000-0000-4000-8000-000000000202", Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 3, SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
			},
		},
		Consent: generationapp.ConsentReceipt{ID: "00000000-0000-4000-8000-000000000301", Purpose: generationapp.PurposeImage, PolicyVersion: "local-image-v1", AcceptedAt: acceptedAt},
	}

	created, err := generations.Create(ctx, first.Token, input)
	if err != nil {
		t.Fatalf("accept generation task: %v", err)
	}
	if created.Reused || created.Match != generationapp.RequestMatchNone || created.View.Task.Status != generationapp.StatusQueued || created.View.Reservation != nil {
		t.Fatalf("unexpected new generation task: %+v", created)
	}

	var outbox struct {
		EventType   string
		AggregateID string
		Payload     []byte
	}
	if err := database.WithContext(ctx).Table("outbox_events").Select("event_type, aggregate_id, payload").Where("aggregate_id = ?", created.View.Task.ID).First(&outbox).Error; err != nil {
		t.Fatalf("read generation outbox event: %v", err)
	}
	if outbox.EventType != generationapp.GenerationRequestedEvent || outbox.AggregateID != created.View.Task.ID || len(outbox.Payload) == 0 {
		t.Fatalf("invalid generation outbox event: %+v", outbox)
	}

	replayed, err := generations.Create(ctx, first.Token, input)
	if err != nil {
		t.Fatalf("replay generation task: %v", err)
	}
	if !replayed.Reused || replayed.Match != generationapp.RequestMatchIdempotentReplay || replayed.View.Task.ID != created.View.Task.ID {
		t.Fatalf("idempotent replay did not return the original task: %+v", replayed)
	}
	var outboxCount int64
	if err := database.WithContext(ctx).Table("outbox_events").Where("aggregate_id = ?", created.View.Task.ID).Count(&outboxCount).Error; err != nil {
		t.Fatalf("count generation outbox events: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("idempotent replay created %d outbox events", outboxCount)
	}

	deduplicatedInput := input
	deduplicatedInput.IdempotencyKey = "generation-request-two"
	deduplicated, err := generations.Create(ctx, first.Token, deduplicatedInput)
	if err != nil {
		t.Fatalf("content-deduplicate generation task: %v", err)
	}
	if !deduplicated.Reused || deduplicated.Match != generationapp.RequestMatchContentDedupe || deduplicated.View.Task.ID != created.View.Task.ID {
		t.Fatalf("content dedupe did not return the original task: %+v", deduplicated)
	}

	conflict := input
	conflict.Parameters = []byte(`{"seed":"two"}`)
	if _, err := generations.Create(ctx, first.Token, conflict); !errors.Is(err, generationapp.ErrGenerationIdempotencyConflict) {
		t.Fatalf("changed request with the same idempotency key returned %v", err)
	}

	if _, err := generations.Get(ctx, second.Token, created.View.Task.ID); !errors.Is(err, generationapp.ErrGenerationNotFound) {
		t.Fatalf("cross-owner generation lookup returned %v", err)
	}
	secondPage, err := generations.List(ctx, second.Token, 20, nil)
	if err != nil {
		t.Fatalf("list second owner's generation tasks: %v", err)
	}
	if len(secondPage.Items) != 0 || secondPage.NextAfterID != nil {
		t.Fatalf("cross-owner generation list leaked tasks: %+v", secondPage)
	}

	paidPolicy := generationapp.AdmissionPolicy{Enabled: true, Currency: "USD", MaxConcurrentTasks: 4, MaxQuotaUnits: 4, MaxBudgetMinorUnits: 100}
	paidGenerations, err := generationapp.NewService(accounts, store.NewGenerationRepository(database), paidPolicy)
	if err != nil {
		t.Fatalf("construct paid generation service: %v", err)
	}
	paidInput := input
	paidInput.IdempotencyKey = "generation-paid-request"
	paidInput.Parameters = []byte(`{"seed":"paid"}`)
	paidInput.Cost = generationapp.CostEstimate{Currency: "USD", EstimatedMinorUnits: 10, ReservedQuotaUnits: 2}
	paidCreated, err := paidGenerations.Create(ctx, second.Token, paidInput)
	if err != nil {
		t.Fatalf("accept quota-backed generation task: %v", err)
	}
	if paidCreated.Reused || paidCreated.View.Reservation == nil || paidCreated.View.Reservation.State != generationapp.ReservationReserved || paidCreated.View.Reservation.ReservedQuotaUnits != 2 || paidCreated.View.Reservation.EstimatedMinorUnits != 10 {
		t.Fatalf("quota-backed generation task did not persist its reservation: %+v", paidCreated)
	}
	var reservationCount int64
	if err := database.WithContext(ctx).Table("generation_quota_reservations").Where("task_id = ? AND owner_id = ? AND state = ?", paidCreated.View.Task.ID, second.User.ID, string(generationapp.ReservationReserved)).Count(&reservationCount).Error; err != nil {
		t.Fatalf("count generation quota reservations: %v", err)
	}
	if reservationCount != 1 {
		t.Fatalf("expected one active generation quota reservation, got %d", reservationCount)
	}

	canceled, err := generations.Cancel(ctx, first.Token, created.View.Task.ID)
	if err != nil {
		t.Fatalf("request generation cancellation: %v", err)
	}
	if canceled.Task.CancelRequestedAt == nil || canceled.Task.Status != generationapp.StatusQueued || canceled.Task.StatusRevision != 2 {
		t.Fatalf("cancellation did not persist the request: %+v", canceled.Task)
	}
	canceledAgain, err := generations.Cancel(ctx, first.Token, created.View.Task.ID)
	if err != nil {
		t.Fatalf("repeat generation cancellation: %v", err)
	}
	if canceledAgain.Task.StatusRevision != canceled.Task.StatusRevision || !canceledAgain.Task.CancelRequestedAt.Equal(*canceled.Task.CancelRequestedAt) {
		t.Fatalf("repeat cancellation was not idempotent: %+v", canceledAgain.Task)
	}

	page, err := generations.List(ctx, first.Token, 20, nil)
	if err != nil {
		t.Fatalf("list first owner's generation tasks: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Task.ID != created.View.Task.ID || page.NextAfterID != nil {
		t.Fatalf("owner-scoped generation list was incorrect: %+v", page)
	}
	loaded, err := generations.Get(ctx, first.Token, created.View.Task.ID)
	if err != nil {
		t.Fatalf("reload canceled generation task: %v", err)
	}
	if loaded.Task.CancelRequestedAt == nil || loaded.Task.StatusRevision != 2 {
		t.Fatalf("canceled generation task was not durable: %+v", loaded.Task)
	}
}
