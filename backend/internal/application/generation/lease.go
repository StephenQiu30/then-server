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
	if t.Status.terminal() {
		return Lease{}, ErrInvalidGenerationState
	}
	if !t.UpdatedAt.IsZero() && at.Before(t.UpdatedAt) {
		return Lease{}, ErrInvalidGenerationLease
	}
	if (t.LeaseUntil == nil) != (t.LeaseOwner == "") || t.LeaseAttempt < 0 {
		return Lease{}, ErrInvalidGenerationLease
	}
	if t.LeaseUntil != nil && (t.LeaseUntil.IsZero() || t.FencingToken == 0 || t.LeaseAttempt < 1) {
		return Lease{}, ErrInvalidGenerationLease
	}
	if t.LeaseUntil != nil && at.Before(*t.LeaseUntil) {
		return Lease{}, ErrGenerationLeaseHeld
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
	t.LeaseOwner = ""
	t.LeaseUntil = nil
	t.StatusRevision++
	t.UpdatedAt = at.UTC()
	return nil
}

// ValidateLease checks owner, attempt, fencing token and expiry at the time a
// worker wants to persist a side effect. Repositories should use the returned
// token in a conditional UPDATE so a stale worker affects zero rows.
func (t Task) ValidateLease(lease Lease, at time.Time) error {
	return t.validateLease(lease, at)
}

func (t Task) validateLease(lease Lease, at time.Time) error {
	if !validID(t.ID) || !validID(lease.TaskID) || lease.TaskID != t.ID || !validID(lease.Owner) || lease.FencingToken == 0 || lease.Attempt < 1 || lease.ExpiresAt.IsZero() {
		return ErrInvalidGenerationLease
	}
	if t.LeaseOwner != lease.Owner || t.FencingToken != lease.FencingToken || t.LeaseAttempt != lease.Attempt || t.LeaseUntil == nil || !t.LeaseUntil.Equal(lease.ExpiresAt) {
		return ErrGenerationLeaseConflict
	}
	if at.IsZero() {
		return ErrInvalidGenerationLease
	}
	if at.Before(t.UpdatedAt) {
		return ErrInvalidGenerationLease
	}
	if !at.Before(*t.LeaseUntil) {
		return ErrGenerationLeaseExpired
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
