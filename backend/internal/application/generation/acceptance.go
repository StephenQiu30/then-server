package generation

import (
	"errors"
	"time"
)

var (
	ErrGenerationIdempotencyConflict = errors.New("generation idempotency key conflicts with existing request")
	ErrGenerationReservationRequired = errors.New("generation reservation id is required")
	ErrGenerationOutboxRequired      = errors.New("generation outbox id is required")
	ErrInvalidGenerationOutbox       = errors.New("invalid generation outbox event")
)

// GenerationRequestedEvent is the only event emitted when a new generation
// task is accepted. The event carries identifiers and a revision; workers
// reload the task from the database instead of receiving private media or
// parameters in Kafka.
const GenerationRequestedEvent = "generation.task_requested"

// OutboxEvent is the provider-independent dispatch fact created with a new
// task. Publishing state, attempts and leases belong to the persistence
// adapter; this immutable value only defines the event identity and payload
// needed to wake a worker.
type OutboxEvent struct {
	ID                string
	EventType         string
	AggregateID       string
	AggregateRevision int
	Purpose           Purpose
	CreatedAt         time.Time
}

// NewOutboxEvent creates the dispatch intent for a queued task. A task that
// was already canceled or submitted must not create a second dispatch event.
func NewOutboxEvent(id string, task Task, now time.Time) (OutboxEvent, error) {
	if !validID(id) || !validID(task.ID) || !task.Purpose.valid() ||
		task.Status != StatusQueued || task.SubmissionState != SubmissionNotStarted ||
		task.ExternalTaskID != "" || task.CancelRequestedAt != nil || task.StatusRevision < 1 || now.IsZero() {
		return OutboxEvent{}, ErrInvalidGenerationOutbox
	}
	if !task.UpdatedAt.IsZero() && now.Before(task.UpdatedAt) {
		return OutboxEvent{}, ErrInvalidGenerationOutbox
	}
	now = now.UTC()
	return OutboxEvent{
		ID:                id,
		EventType:         GenerationRequestedEvent,
		AggregateID:       task.ID,
		AggregateRevision: task.StatusRevision,
		Purpose:           task.Purpose,
		CreatedAt:         now,
	}, nil
}

// AcceptanceResult is the side-effect-free result of generation admission.
// A repository must persist Task, Reservation (when non-nil), and Event in
// one transaction. Reused results contain the existing task and no new
// reservation or event.
type AcceptanceResult struct {
	Task        Task
	Reservation *QuotaReservation
	Event       OutboxEvent
	Match       RequestMatch
	Reused      bool
}

// PrepareAcceptance composes the provider-neutral admission rules. It
// returns an existing task for an idempotent replay or eligible content
// duplicate before checking current policy limits, so a retry never creates a
// second hold or Outbox event. New work is checked against the caller's
// transaction snapshot and returns all facts needed for one atomic insert.
func PrepareAcceptance(policy AdmissionPolicy, usage AdmissionUsage, existing []Task, input CreateInput, reservationID, outboxID string, now time.Time) (AcceptanceResult, error) {
	candidate, err := NewTask(input, now)
	if err != nil {
		return AcceptanceResult{}, err
	}

	var replay *Task
	var duplicate *Task
	for _, prior := range existing {
		match, classifyErr := ClassifyRequest(prior, input)
		if classifyErr != nil {
			return AcceptanceResult{}, classifyErr
		}
		switch match {
		case RequestMatchIdempotencyConflict:
			return AcceptanceResult{}, ErrGenerationIdempotencyConflict
		case RequestMatchIdempotentReplay:
			copy := cloneTask(prior)
			replay = &copy
		case RequestMatchContentDedupe:
			if duplicate == nil {
				copy := cloneTask(prior)
				duplicate = &copy
			}
		}
	}
	if replay != nil {
		return AcceptanceResult{Task: *replay, Match: RequestMatchIdempotentReplay, Reused: true}, nil
	}
	if duplicate != nil {
		return AcceptanceResult{Task: *duplicate, Match: RequestMatchContentDedupe, Reused: true}, nil
	}

	if err := policy.Check(candidate.Cost, usage); err != nil {
		return AcceptanceResult{}, err
	}
	event, err := NewOutboxEvent(outboxID, candidate, now)
	if err != nil {
		if outboxID == "" {
			return AcceptanceResult{}, ErrGenerationOutboxRequired
		}
		return AcceptanceResult{}, err
	}

	var reservation *QuotaReservation
	if candidate.Cost.ReservedQuotaUnits != 0 || candidate.Cost.EstimatedMinorUnits != 0 {
		if reservationID == "" {
			return AcceptanceResult{}, ErrGenerationReservationRequired
		}
		hold, reservationErr := NewQuotaReservation(reservationID, candidate, now)
		if reservationErr != nil {
			return AcceptanceResult{}, reservationErr
		}
		reservation = &hold
	}

	return AcceptanceResult{Task: candidate, Reservation: reservation, Event: event, Match: RequestMatchNone}, nil
}

func cloneTask(task Task) Task {
	task.Parameters = append([]byte(nil), task.Parameters...)
	task.Inputs = cloneSnapshot(task.Inputs)
	task.SubmissionStartedAt = cloneTime(task.SubmissionStartedAt)
	task.SubmissionUnknownAt = cloneTime(task.SubmissionUnknownAt)
	task.CancelRequestedAt = cloneTime(task.CancelRequestedAt)
	task.LeaseUntil = cloneTime(task.LeaseUntil)
	return task
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
