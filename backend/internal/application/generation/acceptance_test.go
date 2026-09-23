package generation

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func generationPolicy() AdmissionPolicy {
	return AdmissionPolicy{
		Enabled:             true,
		Currency:            "USD",
		MaxConcurrentTasks:  2,
		MaxQuotaUnits:       10,
		MaxBudgetMinorUnits: 100,
	}
}

func TestPrepareAcceptanceBuildsTaskReservationAndOutboxTogether(t *testing.T) {
	input := validCreateInput()
	input.Cost = CostEstimate{Currency: "USD", EstimatedMinorUnits: 25, ReservedQuotaUnits: 2}

	result, err := PrepareAcceptance(generationPolicy(), AdmissionUsage{}, nil, input, "reservation-1", "outbox-1", generationTestNow)
	if err != nil {
		t.Fatalf("PrepareAcceptance() error = %v", err)
	}
	if result.Reused || result.Match != RequestMatchNone || result.Task.Status != StatusQueued {
		t.Fatalf("unexpected new acceptance result: %+v", result)
	}
	if result.Reservation == nil || result.Reservation.TaskID != result.Task.ID || result.Reservation.OwnerID != result.Task.OwnerID || result.Reservation.Purpose != result.Task.Purpose {
		t.Fatalf("reservation was not bound to task: %+v", result.Reservation)
	}
	if result.Event.ID != "outbox-1" || result.Event.EventType != GenerationRequestedEvent || result.Event.AggregateID != result.Task.ID || result.Event.AggregateRevision != result.Task.StatusRevision {
		t.Fatalf("unexpected outbox event: %+v", result.Event)
	}
}

func TestPrepareAcceptanceReusesReplayBeforePolicyAndCopiesTask(t *testing.T) {
	existing, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	result, err := PrepareAcceptance(AdmissionPolicy{}, AdmissionUsage{}, []Task{existing}, validCreateInput(), "", "", generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatalf("replay acceptance error = %v", err)
	}
	if !result.Reused || result.Match != RequestMatchIdempotentReplay || result.Reservation != nil || result.Event.ID != "" {
		t.Fatalf("replay created new side effects: %+v", result)
	}
	result.Task.Parameters[0] = 'x'
	result.Task.Inputs.References[0].MediaID = "mutated"
	if existing.Parameters[0] == 'x' || existing.Inputs.References[0].MediaID == "mutated" {
		t.Fatal("replayed task exposed mutable existing state")
	}
}

func TestPrepareAcceptanceRejectsMalformedExistingTask(t *testing.T) {
	existing, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	existing.UpdatedAt = existing.CreatedAt.Add(-time.Second)
	if _, err := PrepareAcceptance(AdmissionPolicy{}, AdmissionUsage{}, []Task{existing}, validCreateInput(), "", "", generationTestNow.Add(time.Minute)); !errors.Is(err, ErrInvalidGenerationState) {
		t.Fatalf("malformed existing task was reused: %v", err)
	}
}

func TestPrepareAcceptanceContentDedupeReturnsExistingTask(t *testing.T) {
	existingInput := validCreateInput()
	existing, err := NewTask(existingInput, generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	newInput := existingInput
	newInput.ID = "job-2"
	newInput.IdempotencyKey = "request-2"
	result, err := PrepareAcceptance(AdmissionPolicy{}, AdmissionUsage{}, []Task{existing}, newInput, "", "", generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatalf("content dedupe acceptance error = %v", err)
	}
	if !result.Reused || result.Match != RequestMatchContentDedupe || result.Task.ID != existing.ID {
		t.Fatalf("content duplicate was not reused: %+v", result)
	}
}

func TestPrepareAcceptanceRejectsIdempotencyConflictBeforeDedupe(t *testing.T) {
	conflictInput := validCreateInput()
	conflictInput.ID = "job-conflict"
	conflictInput.Parameters = []byte(`{"seed":"different"}`)
	conflict, err := NewTask(conflictInput, generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareAcceptance(generationPolicy(), AdmissionUsage{}, []Task{conflict}, validCreateInput(), "reservation-1", "outbox-1", generationTestNow); !errors.Is(err, ErrGenerationIdempotencyConflict) {
		t.Fatalf("idempotency conflict error = %v", err)
	}
}

func TestPrepareAcceptanceFailsClosedWithoutNewSideEffects(t *testing.T) {
	input := validCreateInput()
	input.Cost = CostEstimate{Currency: "USD", EstimatedMinorUnits: 25, ReservedQuotaUnits: 2}
	policy := generationPolicy()
	policy.Enabled = false
	if _, err := PrepareAcceptance(policy, AdmissionUsage{}, nil, input, "reservation-1", "outbox-1", generationTestNow); !errors.Is(err, ErrGenerationDisabled) {
		t.Fatalf("disabled policy error = %v", err)
	}
	policy.Enabled = true
	if _, err := PrepareAcceptance(policy, AdmissionUsage{ReservedMinorUnits: 100}, nil, input, "reservation-1", "outbox-1", generationTestNow); !errors.Is(err, ErrGenerationBudgetExceeded) {
		t.Fatalf("over-budget policy error = %v", err)
	}
}

func TestPrepareAcceptanceDoesNotCreateReservationForZeroCostTask(t *testing.T) {
	result, err := PrepareAcceptance(generationPolicy(), AdmissionUsage{}, nil, validCreateInput(), "", "outbox-1", generationTestNow)
	if err != nil {
		t.Fatalf("zero-cost acceptance error = %v", err)
	}
	if result.Reservation != nil || result.Event.ID != "outbox-1" {
		t.Fatalf("zero-cost acceptance created an unexpected hold/event: %+v", result)
	}
}

func TestPrepareAcceptanceRequiresPersistenceIdentitiesForNewWork(t *testing.T) {
	input := validCreateInput()
	input.Cost = CostEstimate{Currency: "USD", EstimatedMinorUnits: 1, ReservedQuotaUnits: 1}
	if _, err := PrepareAcceptance(generationPolicy(), AdmissionUsage{}, nil, input, "reservation-1", "", generationTestNow); !errors.Is(err, ErrGenerationOutboxRequired) {
		t.Fatalf("missing outbox ID error = %v", err)
	}
	if _, err := PrepareAcceptance(generationPolicy(), AdmissionUsage{}, nil, input, "", "outbox-1", generationTestNow); !errors.Is(err, ErrGenerationReservationRequired) {
		t.Fatalf("missing reservation ID error = %v", err)
	}
}

func TestNewOutboxEventRejectsSubmittedOrCanceledTask(t *testing.T) {
	task, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := task.RequestCancel(generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := NewOutboxEvent("outbox-1", task, generationTestNow.Add(time.Minute)); !errors.Is(err, ErrInvalidGenerationOutbox) {
		t.Fatalf("canceled task outbox error = %v", err)
	}

	task, err = NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	_, err = task.AcquireLease("worker-a", generationTestNow.Add(time.Minute), 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := task.BeginSubmission(generationTestNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := NewOutboxEvent("outbox-1", task, generationTestNow.Add(time.Minute)); !errors.Is(err, ErrInvalidGenerationOutbox) {
		t.Fatalf("submitted task outbox error = %v", err)
	}

	if _, err := NewOutboxEvent("outbox-1", task, generationTestNow.Add(2*time.Minute)); err == nil || !strings.Contains(err.Error(), "outbox") {
		t.Fatalf("invalid outbox event did not retain stable error: %v", err)
	}
}

func TestOutboxEventRejectsMalformedPersistedFacts(t *testing.T) {
	task, err := NewTask(validCreateInput(), generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	event, err := NewOutboxEvent("outbox-1", task, generationTestNow)
	if err != nil {
		t.Fatal(err)
	}
	if !event.validFacts() {
		t.Fatal("new outbox event was not valid")
	}

	tests := []struct {
		name   string
		mutate func(*OutboxEvent)
	}{
		{name: "missing id", mutate: func(event *OutboxEvent) { event.ID = "" }},
		{name: "wrong event type", mutate: func(event *OutboxEvent) { event.EventType = "generation.other" }},
		{name: "missing aggregate", mutate: func(event *OutboxEvent) { event.AggregateID = "" }},
		{name: "invalid aggregate revision", mutate: func(event *OutboxEvent) { event.AggregateRevision = 0 }},
		{name: "invalid purpose", mutate: func(event *OutboxEvent) { event.Purpose = Purpose("video") }},
		{name: "missing creation time", mutate: func(event *OutboxEvent) { event.CreatedAt = time.Time{} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := event
			test.mutate(&candidate)
			if candidate.validFacts() {
				t.Fatalf("malformed outbox event was accepted: %+v", candidate)
			}
		})
	}
}
