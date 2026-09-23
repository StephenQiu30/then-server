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

func mustTask(input CreateInput) Task {
	task, err := NewTask(input, generationTestNow)
	if err != nil {
		panic(err)
	}
	return task
}
