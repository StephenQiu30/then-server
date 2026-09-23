package generation

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func reservationTask() Task {
	input := validCreateInput()
	input.Cost = CostEstimate{Currency: "USD", EstimatedMinorUnits: 25, ReservedQuotaUnits: 1}
	task, err := NewTask(input, generationTestNow)
	if err != nil {
		panic(err)
	}
	return task
}

func TestQuotaReservationBindsTaskOwnerAndPurpose(t *testing.T) {
	reservation, err := NewQuotaReservation("reservation-1", reservationTask(), generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatalf("NewQuotaReservation() error = %v", err)
	}
	if reservation.TaskID != "job-1" || reservation.OwnerID != "owner-1" || reservation.Purpose != PurposeImage || reservation.State != ReservationReserved || reservation.StateRevision != 1 {
		t.Fatalf("unexpected reservation: %+v", reservation)
	}
}

func TestQuotaReservationReleaseIsIdempotentAndBlocksConsume(t *testing.T) {
	reservation, err := NewQuotaReservation("reservation-1", reservationTask(), generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := reservation.Release(generationTestNow.Add(2 * time.Minute)); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	revision := reservation.StateRevision
	if err := reservation.Release(generationTestNow.Add(3 * time.Minute)); err != nil {
		t.Fatalf("repeated Release() error = %v", err)
	}
	if reservation.StateRevision != revision || reservation.State != ReservationReleased {
		t.Fatalf("repeated release was not idempotent: %+v", reservation)
	}
	if err := reservation.Consume(generationTestNow.Add(4 * time.Minute)); !errors.Is(err, ErrQuotaReservationClosed) {
		t.Fatalf("released reservation was consumed: %v", err)
	}
}

func TestQuotaReservationConsumeIsIdempotentAndBlocksRelease(t *testing.T) {
	reservation, err := NewQuotaReservation("reservation-1", reservationTask(), generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := reservation.Consume(generationTestNow.Add(2 * time.Minute)); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	revision := reservation.StateRevision
	if err := reservation.Consume(generationTestNow.Add(3 * time.Minute)); err != nil {
		t.Fatalf("repeated Consume() error = %v", err)
	}
	if reservation.StateRevision != revision || reservation.State != ReservationConsumed {
		t.Fatalf("repeated consume was not idempotent: %+v", reservation)
	}
	if err := reservation.Release(generationTestNow.Add(4 * time.Minute)); !errors.Is(err, ErrQuotaReservationClosed) {
		t.Fatalf("consumed reservation was released: %v", err)
	}
}

func TestQuotaReservationRejectsEmptyHoldInvalidCostAndStaleTransition(t *testing.T) {
	zeroCost := validCreateInput()
	if _, err := NewQuotaReservation("reservation-1", mustTask(zeroCost), generationTestNow); !errors.Is(err, ErrInvalidQuotaReservation) {
		t.Fatalf("zero-cost reservation error = %v", err)
	}
	invalidTask := reservationTask()
	invalidTask.Cost = CostEstimate{EstimatedMinorUnits: 1, ReservedQuotaUnits: 1}
	if _, err := NewQuotaReservation("reservation-1", invalidTask, generationTestNow); !errors.Is(err, ErrInvalidQuotaReservation) {
		t.Fatalf("invalid currency reservation error = %v", err)
	}
	reservation, err := NewQuotaReservation("reservation-1", reservationTask(), generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := reservation.Release(generationTestNow); !errors.Is(err, ErrInvalidQuotaReservation) {
		t.Fatalf("stale release error = %v", err)
	}
	if reservation.Currency != "USD" || !strings.HasPrefix(reservation.ID, "reservation-") {
		t.Fatalf("reservation fields were not retained: %+v", reservation)
	}
}

func TestQuotaReservationRejectsNonQueuedOrSubmittedTask(t *testing.T) {
	cases := []struct {
		name   string
		at     time.Time
		mutate func(*Task)
	}{
		{
			name: "terminal task",
			at:   generationTestNow.Add(time.Minute),
			mutate: func(task *Task) {
				task.Status = StatusFailed
				task.FailureCode = "provider_error"
				task.StatusRevision++
				task.UpdatedAt = generationTestNow.Add(time.Minute)
			},
		},
		{
			name: "submitted task",
			at:   generationTestNow.Add(2 * time.Minute),
			mutate: func(task *Task) {
				task.SubmissionState = SubmissionInFlight
				task.SubmissionAttempt = 1
				task.SubmissionStartedAt = timePtr(generationTestNow.Add(time.Minute))
				task.UpdatedAt = generationTestNow.Add(time.Minute)
			},
		},
		{
			name: "canceled request",
			at:   generationTestNow.Add(time.Minute),
			mutate: func(task *Task) {
				if err := task.RequestCancel(generationTestNow.Add(time.Minute)); err != nil {
					panic(err)
				}
			},
		},
		{
			name:   "stale timestamp",
			at:     generationTestNow.Add(-time.Second),
			mutate: func(task *Task) {},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := reservationTask()
			tc.mutate(&task)
			if _, err := NewQuotaReservation("reservation-1", task, tc.at); !errors.Is(err, ErrInvalidQuotaReservation) {
				t.Fatalf("invalid reservation task was accepted: %v", err)
			}
		})
	}
}

func TestQuotaReservationRejectsMalformedPersistedFacts(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*QuotaReservation)
	}{
		{
			name: "missing state revision",
			mutate: func(reservation *QuotaReservation) {
				reservation.StateRevision = 0
			},
		},
		{
			name: "missing created time",
			mutate: func(reservation *QuotaReservation) {
				reservation.CreatedAt = time.Time{}
			},
		},
		{
			name: "updated before created",
			mutate: func(reservation *QuotaReservation) {
				reservation.UpdatedAt = reservation.CreatedAt.Add(-time.Second)
			},
		},
		{
			name: "unknown state",
			mutate: func(reservation *QuotaReservation) {
				reservation.State = ReservationState("unknown")
			},
		},
		{
			name: "finalized state has initial revision",
			mutate: func(reservation *QuotaReservation) {
				reservation.State = ReservationReleased
			},
		},
		{
			name: "empty hold",
			mutate: func(reservation *QuotaReservation) {
				reservation.ReservedQuotaUnits = 0
				reservation.EstimatedMinorUnits = 0
				reservation.Currency = ""
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reservation, err := NewQuotaReservation("reservation-1", reservationTask(), generationTestNow.Add(time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(&reservation)
			if err := reservation.Release(generationTestNow.Add(2 * time.Minute)); !errors.Is(err, ErrInvalidQuotaReservation) {
				t.Fatalf("malformed reservation was finalized: %v", err)
			}
		})
	}
}

func TestQuotaReservationRejectsStateRevisionOverflowBeforeMutation(t *testing.T) {
	reservation, err := NewQuotaReservation("reservation-1", reservationTask(), generationTestNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	reservation.StateRevision = maxInt()
	before := reservation
	if err := reservation.Release(generationTestNow.Add(2 * time.Minute)); !errors.Is(err, ErrInvalidQuotaReservation) {
		t.Fatalf("overflowing reservation release error = %v", err)
	}
	if reservation.State != before.State || reservation.StateRevision != before.StateRevision || !reservation.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("overflowing reservation release mutated reservation: %+v", reservation)
	}
}

func mustTask(input CreateInput) Task {
	task, err := NewTask(input, generationTestNow)
	if err != nil {
		panic(err)
	}
	return task
}
