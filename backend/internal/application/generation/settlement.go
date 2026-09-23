package generation

import (
	"errors"
	"time"
)

var (
	ErrInvalidGenerationSettlement  = errors.New("invalid generation settlement")
	ErrGenerationSettlementConflict = errors.New("generation settlement conflicts with task")
)

// Settlement is the provider-independent write set for a terminal task. A
// repository persists the returned task, reservation and (for success) asset
// in one transaction; the input values are never mutated by these helpers.
type Settlement struct {
	Task        Task
	Reservation *QuotaReservation
	Asset       *OutputAsset
	// FencingToken is the token a repository must include in its conditional
	// update. A stale worker therefore cannot publish or finalize after lease
	// recovery, even if it still holds an old task snapshot.
	FencingToken uint64
	Reused       bool
}

// PublishOutput composes a validated output with the task's succeeded state
// and consumes its temporary quota hold. Repeating the same success command is
// idempotent when the task already records the same asset and consumed hold.
func PublishOutput(task Task, reservation *QuotaReservation, asset OutputAsset, at time.Time) (Settlement, error) {
	if !task.validSubmissionFacts() || !validSettlementTime(task, at) || !validFailureState(task.Status, task.FailureCode) || !validOutputAsset(task, asset) || asset.PublishedAt.After(at) || !reservationMatchesTask(task, reservation) {
		return Settlement{}, ErrInvalidGenerationSettlement
	}
	if task.Status == StatusSucceeded {
		if !validSettlementLease(task, at) {
			return Settlement{}, ErrInvalidGenerationSettlement
		}
		if task.ResultAssetID != asset.ID || !reservationFinalizedAs(reservation, ReservationConsumed) {
			return Settlement{}, ErrGenerationSettlementConflict
		}
		return Settlement{Task: cloneTask(task), Reservation: cloneReservation(reservation), Asset: cloneAsset(asset), FencingToken: task.FencingToken, Reused: true}, nil
	}
	if task.Status != StatusValidating || task.CancelRequestedAt != nil || task.ResultAssetID != "" || !validActiveSettlementLease(task, at) || !reservationAvailable(reservation) {
		return Settlement{}, ErrInvalidGenerationSettlement
	}

	nextTask := cloneTask(task)
	nextTask.ResultAssetID = asset.ID
	nextTask.LeaseOwner = ""
	nextTask.LeaseUntil = nil
	if err := nextTask.Transition(StatusSucceeded, "", at); err != nil {
		return Settlement{}, ErrInvalidGenerationSettlement
	}
	nextReservation := cloneReservation(reservation)
	if nextReservation != nil {
		if err := nextReservation.Consume(at); err != nil {
			return Settlement{}, ErrInvalidGenerationSettlement
		}
	}
	return Settlement{Task: nextTask, Reservation: nextReservation, Asset: cloneAsset(asset), FencingToken: task.FencingToken}, nil
}

// FinalizeWithoutOutput moves an active task to failed, canceled or expired
// and releases its temporary hold. Repeating the same terminal command is
// idempotent when the task already carries the same failure and released hold.
func FinalizeWithoutOutput(task Task, reservation *QuotaReservation, next Status, failureCode string, at time.Time) (Settlement, error) {
	if !task.validSubmissionFacts() || !validSettlementTime(task, at) || !validSettlementLease(task, at) || !terminalWithoutOutput(next) || !reservationMatchesTask(task, reservation) {
		return Settlement{}, ErrInvalidGenerationSettlement
	}
	if next == StatusFailed && !validToken(failureCode, 96) {
		return Settlement{}, ErrInvalidGenerationSettlement
	}
	if next != StatusFailed && failureCode != "" {
		return Settlement{}, ErrInvalidGenerationSettlement
	}
	if task.Status == next {
		if task.ResultAssetID != "" || task.FailureCode != failureCode || !reservationFinalizedAs(reservation, ReservationReleased) {
			return Settlement{}, ErrGenerationSettlementConflict
		}
		return Settlement{Task: cloneTask(task), Reservation: cloneReservation(reservation), FencingToken: task.FencingToken, Reused: true}, nil
	}
	if task.Status.terminal() || !reservationAvailable(reservation) {
		return Settlement{}, ErrInvalidGenerationSettlement
	}

	nextTask := cloneTask(task)
	nextTask.LeaseOwner = ""
	nextTask.LeaseUntil = nil
	if err := nextTask.Transition(next, failureCode, at); err != nil {
		return Settlement{}, ErrInvalidGenerationSettlement
	}
	nextReservation := cloneReservation(reservation)
	if nextReservation != nil {
		if err := nextReservation.Release(at); err != nil {
			return Settlement{}, ErrInvalidGenerationSettlement
		}
	}
	return Settlement{Task: nextTask, Reservation: nextReservation, FencingToken: task.FencingToken}, nil
}

func validSettlementTime(task Task, at time.Time) bool {
	return !at.IsZero() && (task.UpdatedAt.IsZero() || !at.Before(task.UpdatedAt))
}

func validSettlementLease(task Task, at time.Time) bool {
	if task.Status.terminal() {
		return task.LeaseOwner == "" && task.LeaseUntil == nil && task.LeaseAttempt >= 0
	}
	if task.LeaseUntil == nil {
		// A released task may be settled by a user-facing cancellation path,
		// but its persisted lease shape must still be internally consistent.
		return task.LeaseOwner == "" && task.LeaseAttempt >= 0
	}
	return task.validateActiveLeaseAt(at) == nil
}

func validActiveSettlementLease(task Task, at time.Time) bool {
	return task.validateActiveLeaseAt(at) == nil
}

func terminalWithoutOutput(status Status) bool {
	return status == StatusFailed || status == StatusCanceled || status == StatusExpired
}

func reservationMatchesTask(task Task, reservation *QuotaReservation) bool {
	needsReservation := task.Cost.ReservedQuotaUnits != 0 || task.Cost.EstimatedMinorUnits != 0
	if reservation == nil {
		return !needsReservation
	}
	return needsReservation && reservation.validFacts() && !reservation.CreatedAt.Before(task.CreatedAt) && reservation.TaskID == task.ID && reservation.OwnerID == task.OwnerID && reservation.Purpose == task.Purpose && reservation.Currency == task.Cost.Currency && reservation.ReservedQuotaUnits == task.Cost.ReservedQuotaUnits && reservation.EstimatedMinorUnits == task.Cost.EstimatedMinorUnits
}

func reservationAvailable(reservation *QuotaReservation) bool {
	return reservation == nil || reservation.State == ReservationReserved
}

func reservationFinalizedAs(reservation *QuotaReservation, state ReservationState) bool {
	return reservation == nil || reservation.State == state
}

func cloneReservation(reservation *QuotaReservation) *QuotaReservation {
	if reservation == nil {
		return nil
	}
	copy := *reservation
	return &copy
}

func cloneAsset(asset OutputAsset) *OutputAsset {
	copy := asset
	return &copy
}

func validOutputAsset(task Task, asset OutputAsset) bool {
	if !validID(asset.ID) || asset.Lineage.TaskID != task.ID || asset.Lineage.OwnerID != task.OwnerID || asset.Lineage.LookID != task.LookID || asset.Lineage.LookRevision != task.LookRevision || asset.Lineage.Purpose != task.Purpose || asset.PublishedAt.IsZero() || !validOutputFact(task.Purpose, OutputFact{ContentType: asset.ContentType, ByteSize: asset.ByteSize, SHA256: asset.SHA256, ObjectVersionID: asset.ObjectVersionID}) {
		return false
	}
	if task.Purpose == PurposeModel {
		return asset.Lineage.SourceImageAssetID == task.Inputs.ImageAssetID && asset.Lineage.SourceImageSHA256 == task.Inputs.ImageSHA256
	}
	return asset.Lineage.SourceImageAssetID == "" && asset.Lineage.SourceImageSHA256 == ""
}
