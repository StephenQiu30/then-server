package generation

import (
	"errors"
	"time"
)

var (
	ErrGenerationLeaseHeld     = errors.New("generation task lease is held")
	ErrGenerationLeaseExpired  = errors.New("generation task lease expired")
	ErrGenerationLeaseConflict = errors.New("generation task lease conflicts with task")
	ErrInvalidGenerationLease  = errors.New("invalid generation task lease")
)

const maxWorkerRetryDelay = 30 * time.Minute

// Lease is the worker proof carried between short database transactions. The
// fencing token changes on every acquisition, so an expired worker cannot
// mutate a task after another worker has recovered it.
type Lease struct {
	TaskID       string
	Owner        string
	FencingToken uint64
	Attempt      int
	ExpiresAt    time.Time
}

// AcquireLease claims an active task for a bounded worker interval. A queued
// task enters running exactly once; recovering a running or validating task
// only advances the fencing token and worker attempt.
func (t *Task) AcquireLease(owner string, at time.Time, ttl time.Duration) (Lease, error) {
	if t == nil || !validID(owner) || at.IsZero() || ttl <= 0 {
		return Lease{}, ErrInvalidGenerationLease
	}
	if !activeStatus(t.Status) {
		return Lease{}, ErrInvalidGenerationState
	}
	if !t.validTaskCoreFacts() {
		return Lease{}, ErrInvalidGenerationState
	}
	if !t.validLeaseFacts() {
		return Lease{}, ErrInvalidGenerationLease
	}
	if !t.UpdatedAt.IsZero() && at.Before(t.UpdatedAt) {
		return Lease{}, ErrInvalidGenerationLease
	}
	if (t.LeaseUntil == nil) != (t.LeaseOwner == "") || t.LeaseAttempt < 0 {
		return Lease{}, ErrInvalidGenerationLease
	}
	if t.LeaseUntil != nil && (t.LeaseUntil.IsZero() || !validID(t.LeaseOwner) || t.FencingToken == 0 || t.LeaseAttempt < 1) {
		return Lease{}, ErrInvalidGenerationLease
	}
	if t.LeaseUntil != nil && at.Before(*t.LeaseUntil) {
		return Lease{}, ErrGenerationLeaseHeld
	}
	if t.NextAttemptAt != nil && at.Before(*t.NextAttemptAt) {
		return Lease{}, ErrGenerationRetryNotReady
	}
	if t.FencingToken == ^uint64(0) || t.LeaseAttempt == int(^uint(0)>>1) {
		return Lease{}, ErrInvalidGenerationLease
	}
	at = at.UTC()
	if t.Status == StatusQueued {
		if err := t.Transition(StatusRunning, "", at); err != nil {
			return Lease{}, err
		}
	} else {
		if !t.canAdvanceStatusRevision() {
			return Lease{}, ErrInvalidGenerationState
		}
		t.NextAttemptAt = nil
		t.StatusRevision++
		t.UpdatedAt = at
	}
	t.FencingToken++
	t.LeaseAttempt++
	t.LeaseOwner = owner
	expiresAt := at.Add(ttl)
	t.LeaseUntil = timePtr(expiresAt)
	return t.currentLease(), nil
}

// RenewLease extends a lease only while the current fencing proof is valid.
// Renewals must extend the deadline; shortening a lease is an accidental
// caller bug and is rejected.
func (t *Task) RenewLease(lease Lease, at time.Time, ttl time.Duration) (Lease, error) {
	if err := t.validateLease(lease, at); err != nil {
		return Lease{}, err
	}
	if ttl <= 0 {
		return Lease{}, ErrInvalidGenerationLease
	}
	expiresAt := at.UTC().Add(ttl)
	if !expiresAt.After(*t.LeaseUntil) {
		return Lease{}, ErrInvalidGenerationLease
	}
	if !t.canAdvanceStatusRevision() {
		return Lease{}, ErrInvalidGenerationLease
	}
	t.LeaseUntil = timePtr(expiresAt)
	t.StatusRevision++
	t.UpdatedAt = at.UTC()
	return t.currentLease(), nil
}

// ReleaseLease clears the active worker lease. An expired or stale worker
// cannot release a lease that may already belong to a recovery worker.
func (t *Task) ReleaseLease(lease Lease, at time.Time) error {
	if err := t.validateLease(lease, at); err != nil {
		return err
	}
	if !t.canAdvanceStatusRevision() {
		return ErrInvalidGenerationLease
	}
	t.LeaseOwner = ""
	t.LeaseUntil = nil
	t.StatusRevision++
	t.UpdatedAt = at.UTC()
	return nil
}

// ReleaseLeaseForRetry clears a current worker lease and durably schedules
// the next observation or result attempt. The same timestamp is also used for
// explicitly reconciled submission retries; accepted tasks can only schedule
// polling while still running or validating.
func (t *Task) ReleaseLeaseForRetry(lease Lease, at time.Time, delay time.Duration) error {
	if t == nil || delay <= 0 || delay > maxWorkerRetryDelay || !t.validTaskFacts() ||
		t.SubmissionState != SubmissionAccepted || (t.Status != StatusRunning && t.Status != StatusValidating) || t.AccessRevokedAt != nil {
		return ErrInvalidGenerationLease
	}
	if err := t.validateLease(lease, at); err != nil {
		return err
	}
	if !t.canAdvanceStatusRevision() {
		return ErrInvalidGenerationLease
	}
	at = at.UTC()
	next := at.Add(delay)
	if !next.After(at) {
		return ErrInvalidGenerationLease
	}
	t.LeaseOwner = ""
	t.LeaseUntil = nil
	t.NextAttemptAt = timePtr(next)
	t.StatusRevision++
	t.UpdatedAt = at
	return nil
}

// ValidateLease checks owner, attempt, fencing token and expiry at the time a
// worker wants to persist a side effect. Repositories should use the returned
// token in a conditional UPDATE so a stale worker affects zero rows.
func (t Task) ValidateLease(lease Lease, at time.Time) error {
	return t.validateLease(lease, at)
}

// CurrentLease returns the persisted lease proof for a task. Repositories use
// it after a write so callers receive the database-normalized expiry time and
// fencing token instead of an in-memory value that may differ in timestamp
// precision.
func (t Task) CurrentLease() (Lease, error) {
	if !t.validTaskFacts() || t.LeaseUntil == nil || t.LeaseOwner == "" {
		return Lease{}, ErrInvalidGenerationLease
	}
	return t.currentLease(), nil
}

func (t Task) validateActiveLeaseAt(at time.Time) error {
	if !t.validTaskFacts() || !t.validLeaseFacts() {
		return ErrInvalidGenerationLease
	}
	if (t.LeaseUntil == nil) != (t.LeaseOwner == "") || t.LeaseAttempt < 0 {
		return ErrInvalidGenerationLease
	}
	if t.LeaseUntil == nil {
		return ErrGenerationLeaseConflict
	}
	if t.LeaseUntil.IsZero() || !validID(t.LeaseOwner) || t.FencingToken == 0 || t.LeaseAttempt < 1 || at.IsZero() {
		return ErrInvalidGenerationLease
	}
	if !t.UpdatedAt.IsZero() && at.Before(t.UpdatedAt) {
		return ErrInvalidGenerationLease
	}
	if !at.Before(*t.LeaseUntil) {
		return ErrGenerationLeaseExpired
	}
	return nil
}

func (t Task) validateLease(lease Lease, at time.Time) error {
	if !t.validTaskFacts() || !t.validLeaseFacts() || !validID(lease.TaskID) || lease.TaskID != t.ID || !validID(lease.Owner) || lease.FencingToken == 0 || lease.Attempt < 1 || lease.ExpiresAt.IsZero() {
		return ErrInvalidGenerationLease
	}
	if t.LeaseOwner != lease.Owner || t.FencingToken != lease.FencingToken || t.LeaseAttempt != lease.Attempt || t.LeaseUntil == nil || !t.LeaseUntil.Equal(lease.ExpiresAt) {
		return ErrGenerationLeaseConflict
	}
	if err := t.validateActiveLeaseAt(at); err != nil {
		return err
	}
	return nil
}

func (t Task) currentLease() Lease {
	return Lease{
		TaskID:       t.ID,
		Owner:        t.LeaseOwner,
		FencingToken: t.FencingToken,
		Attempt:      t.LeaseAttempt,
		ExpiresAt:    *t.LeaseUntil,
	}
}
