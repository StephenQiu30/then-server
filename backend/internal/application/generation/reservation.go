package generation

import (
	"errors"
	"time"
)

var (
	ErrInvalidQuotaReservation = errors.New("invalid quota reservation")
	ErrQuotaReservationClosed  = errors.New("quota reservation is already finalized")
)

// ReservationState describes the temporary user-quota hold associated with a
// generation task. Provider billing is recorded separately; this state only
// answers whether the product hold was released or consumed.
type ReservationState string

const (
	ReservationReserved ReservationState = "reserved"
	ReservationReleased ReservationState = "released"
	ReservationConsumed ReservationState = "consumed"
)

// QuotaReservation is the provider-independent quota fact created in the
// same transaction as its task. It intentionally contains no provider or
// database types; a repository will persist and transition it atomically.
type QuotaReservation struct {
	ID                  string
	TaskID              string
	OwnerID             string
	Purpose             Purpose
	Currency            string
	ReservedQuotaUnits  int
	EstimatedMinorUnits int64
	State               ReservationState
	StateRevision       int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (r QuotaReservation) validFacts() bool {
	if !validID(r.ID) || !validID(r.TaskID) || !validID(r.OwnerID) || !r.Purpose.valid() ||
		r.StateRevision < 1 || r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() || r.UpdatedAt.Before(r.CreatedAt) {
		return false
	}
	if r.ReservedQuotaUnits < 0 || r.EstimatedMinorUnits < 0 ||
		(r.EstimatedMinorUnits > 0 && !validToken(r.Currency, 16)) ||
		(r.EstimatedMinorUnits == 0 && r.Currency != "" && !validToken(r.Currency, 16)) ||
		(r.ReservedQuotaUnits == 0 && r.EstimatedMinorUnits == 0) {
		return false
	}
	switch r.State {
	case ReservationReserved:
		return r.StateRevision == 1
	case ReservationReleased, ReservationConsumed:
		return r.StateRevision == 2
	default:
		return false
	}
}

// NewQuotaReservation creates a temporary hold for a task after admission has
// passed. A zero-cost task has no hold and must not create a fake reservation.
func NewQuotaReservation(id string, task Task, now time.Time) (QuotaReservation, error) {
	if !validID(id) || !validID(task.ID) || !validID(task.OwnerID) || !task.Purpose.valid() || now.IsZero() ||
		task.Status != StatusQueued || task.SubmissionState != SubmissionNotStarted || task.ExternalTaskID != "" ||
		task.CancelRequestedAt != nil || task.StatusRevision < 1 || task.CreatedAt.IsZero() || !task.validTaskFacts() {
		return QuotaReservation{}, ErrInvalidQuotaReservation
	}
	if now.Before(task.CreatedAt) || (!task.UpdatedAt.IsZero() && now.Before(task.UpdatedAt)) {
		return QuotaReservation{}, ErrInvalidQuotaReservation
	}
	if !validCostEstimate(task.Cost) ||
		(task.Cost.ReservedQuotaUnits == 0 && task.Cost.EstimatedMinorUnits == 0) {
		return QuotaReservation{}, ErrInvalidQuotaReservation
	}
	now = now.UTC()
	return QuotaReservation{
		ID:                  id,
		TaskID:              task.ID,
		OwnerID:             task.OwnerID,
		Purpose:             task.Purpose,
		Currency:            task.Cost.Currency,
		ReservedQuotaUnits:  task.Cost.ReservedQuotaUnits,
		EstimatedMinorUnits: task.Cost.EstimatedMinorUnits,
		State:               ReservationReserved,
		StateRevision:       1,
		CreatedAt:           now,
		UpdatedAt:           now,
	}, nil
}

// Release returns the temporary hold after a task cannot produce a usable
// result. Repeating the same terminal command is idempotent.
func (r *QuotaReservation) Release(at time.Time) error {
	return r.finalize(ReservationReleased, at)
}

// Consume settles the product hold after the task produces a usable result.
// Provider billing and any later reconciliation remain separate facts.
func (r *QuotaReservation) Consume(at time.Time) error {
	return r.finalize(ReservationConsumed, at)
}

func (r *QuotaReservation) finalize(next ReservationState, at time.Time) error {
	if r == nil || at.IsZero() || !r.validFacts() {
		return ErrInvalidQuotaReservation
	}
	if r.State == next {
		return nil
	}
	if r.State != ReservationReserved {
		return ErrQuotaReservationClosed
	}
	if r.StateRevision == maxInt() {
		return ErrInvalidQuotaReservation
	}
	if !r.UpdatedAt.IsZero() && at.Before(r.UpdatedAt) {
		return ErrInvalidQuotaReservation
	}
	at = at.UTC()
	r.State = next
	r.StateRevision++
	r.UpdatedAt = at
	return nil
}
