package generation

import (
	"errors"
	"time"
)

var (
	ErrInvalidGenerationCleanup    = errors.New("invalid generation cleanup")
	ErrGenerationCleanupNotReady   = errors.New("generation cleanup is not ready")
	ErrGenerationCleanupInProgress = errors.New("generation cleanup is already in progress")
	ErrGenerationCleanupClaim      = errors.New("generation cleanup claim is stale")
)

// CleanupScope identifies the owner-facing operation that revoked access.
// Task is the first DELETE-01 slice; source and account cleanup use the same
// durable request shape when their cascades are wired in.
type CleanupScope string

const (
	CleanupScopeTask    CleanupScope = "task"
	CleanupScopeSource  CleanupScope = "source"
	CleanupScopeAccount CleanupScope = "account"
)

type CleanupStatus string

const (
	CleanupPending  CleanupStatus = "pending"
	CleanupRunning  CleanupStatus = "running"
	CleanupComplete CleanupStatus = "complete"
	CleanupFailed   CleanupStatus = "failed"
)

type CleanupTargetKind string

const (
	CleanupTargetObject   CleanupTargetKind = "object"
	CleanupTargetProvider CleanupTargetKind = "provider_task"
)

// CleanupTarget is a non-sensitive snapshot of an external side effect that
// still needs to be removed. Object version IDs and provider task IDs are
// intentionally copied into the request so deleting the task row cannot erase
// the evidence required by a later cleanup worker.
type CleanupTarget struct {
	Kind            CleanupTargetKind `json:"kind"`
	ID              string            `json:"id"`
	ObjectVersionID string            `json:"object_version_id,omitempty"`
}

func (target CleanupTarget) Validate() error {
	if !validToken(string(target.Kind), 32) || !validToken(target.ID, 256) {
		return ErrInvalidGenerationCleanup
	}
	switch target.Kind {
	case CleanupTargetObject:
		if !validToken(target.ObjectVersionID, 160) {
			return ErrInvalidGenerationCleanup
		}
	case CleanupTargetProvider:
		if target.ObjectVersionID != "" {
			return ErrInvalidGenerationCleanup
		}
	default:
		return ErrInvalidGenerationCleanup
	}
	return nil
}

// CleanupRequest keeps deletion intent and its retry state independent from
// the generation task. It remains readable while pending/running/failed so a
// crashed worker can resume without recreating provider or object-store work.
type CleanupRequest struct {
	ID                string
	OwnerID           string
	TaskID            string
	Scope             CleanupScope
	SourceMediaID     string
	AccountDeletionID string
	Status            CleanupStatus
	AccessRevokedAt   time.Time
	CompletedAt       *time.Time
	StableError       string
	Attempts          int
	NextAttemptAt     *time.Time
	Targets           []CleanupTarget
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func NewCleanupRequest(id string, task Task, scope CleanupScope, targets []CleanupTarget, at time.Time) (CleanupRequest, error) {
	if err := task.Validate(); err != nil || !validID(id) || !validID(task.OwnerID) || !validID(task.ID) || !validCleanupScope(scope) || task.AccessRevokedAt == nil || at.IsZero() {
		return CleanupRequest{}, ErrInvalidGenerationCleanup
	}
	at = at.UTC()
	request := CleanupRequest{
		ID:              id,
		OwnerID:         task.OwnerID,
		TaskID:          task.ID,
		Scope:           scope,
		Status:          CleanupPending,
		AccessRevokedAt: task.AccessRevokedAt.UTC(),
		Targets:         cloneCleanupTargets(targets),
		CreatedAt:       at,
		UpdatedAt:       at,
	}
	if err := request.Validate(); err != nil {
		return CleanupRequest{}, err
	}
	return request, nil
}

func (r CleanupRequest) Validate() error {
	if !validID(r.ID) || !validID(r.OwnerID) || !validID(r.TaskID) || !validCleanupScope(r.Scope) || (r.SourceMediaID != "" && !validID(r.SourceMediaID)) || (r.AccountDeletionID != "" && !validID(r.AccountDeletionID)) || !validCleanupStatus(r.Status) || r.AccessRevokedAt.IsZero() || r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() || r.UpdatedAt.Before(r.CreatedAt) || r.AccessRevokedAt.Before(r.CreatedAt) || r.AccessRevokedAt.After(r.UpdatedAt) || r.Attempts < 0 || r.Attempts > 100 || !validToken(r.StableError, 96) && r.StableError != "" {
		return ErrInvalidGenerationCleanup
	}
	if r.CompletedAt != nil {
		if r.Status != CleanupComplete || r.CompletedAt.IsZero() || r.CompletedAt.Before(r.CreatedAt) || r.CompletedAt.After(r.UpdatedAt) {
			return ErrInvalidGenerationCleanup
		}
	} else if r.Status == CleanupComplete {
		return ErrInvalidGenerationCleanup
	}
	if r.NextAttemptAt != nil && (r.NextAttemptAt.IsZero() || r.NextAttemptAt.Before(r.CreatedAt) || r.Status == CleanupComplete || r.Status == CleanupRunning) {
		return ErrInvalidGenerationCleanup
	}
	if r.Status == CleanupFailed && r.StableError == "" {
		return ErrInvalidGenerationCleanup
	}
	if r.Status != CleanupFailed && r.StableError != "" {
		return ErrInvalidGenerationCleanup
	}
	seen := make(map[string]struct{}, len(r.Targets))
	for _, target := range r.Targets {
		if err := target.Validate(); err != nil {
			return err
		}
		key := string(target.Kind) + "\x00" + target.ID
		if _, exists := seen[key]; exists {
			return ErrInvalidGenerationCleanup
		}
		seen[key] = struct{}{}
	}
	return nil
}

// Begin claims one request for a cleanup worker and returns an immutable copy
// of its targets. A complete request is idempotently returned with no state
// change; a running request requires the previous worker to release or expire
// it before another worker can claim it.
func (r *CleanupRequest) Begin(at time.Time) ([]CleanupTarget, error) {
	if r == nil || r.Validate() != nil || at.IsZero() {
		return nil, ErrInvalidGenerationCleanup
	}
	if r.Status == CleanupComplete {
		return cloneCleanupTargets(r.Targets), nil
	}
	if r.Status == CleanupRunning {
		return nil, ErrGenerationCleanupInProgress
	}
	if r.NextAttemptAt != nil && at.Before(*r.NextAttemptAt) {
		return nil, ErrGenerationCleanupNotReady
	}
	if r.Attempts >= 100 || at.Before(r.UpdatedAt) {
		return nil, ErrInvalidGenerationCleanup
	}
	at = at.UTC()
	r.Status = CleanupRunning
	r.Attempts++
	r.NextAttemptAt = nil
	r.StableError = ""
	r.UpdatedAt = at
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return cloneCleanupTargets(r.Targets), nil
}

func (r *CleanupRequest) Complete(at time.Time) error {
	if r == nil || r.Validate() != nil || at.IsZero() {
		return ErrInvalidGenerationCleanup
	}
	if r.Status == CleanupComplete {
		return nil
	}
	if r.Status != CleanupRunning || at.Before(r.UpdatedAt) {
		return ErrInvalidGenerationCleanup
	}
	at = at.UTC()
	r.Status = CleanupComplete
	r.CompletedAt = timePtr(at)
	r.NextAttemptAt = nil
	r.StableError = ""
	r.UpdatedAt = at
	return r.Validate()
}

func (r *CleanupRequest) Fail(at time.Time, stableError string, retryAt time.Time) error {
	if r == nil || r.Validate() != nil || r.Status != CleanupRunning || at.IsZero() || retryAt.IsZero() || !validToken(stableError, 96) || retryAt.Before(at) || at.Before(r.UpdatedAt) {
		return ErrInvalidGenerationCleanup
	}
	at = at.UTC()
	retryAt = retryAt.UTC()
	r.Status = CleanupFailed
	r.StableError = stableError
	r.NextAttemptAt = timePtr(retryAt)
	r.UpdatedAt = at
	return r.Validate()
}

// AddTarget extends a cleanup manifest when a late external side effect is
// discovered after the original delete request was created. Adding a new
// target invalidates any running or completed claim and puts the request back
// into the durable pending state so the new target cannot be skipped.
func (r *CleanupRequest) AddTarget(target CleanupTarget, at time.Time) error {
	if r == nil || r.Validate() != nil || at.IsZero() || at.Before(r.UpdatedAt) {
		return ErrInvalidGenerationCleanup
	}
	if err := target.Validate(); err != nil {
		return err
	}
	for _, existing := range r.Targets {
		if existing.Kind == target.Kind && existing.ID == target.ID {
			return nil
		}
	}
	at = at.UTC()
	r.Targets = append(r.Targets, target)
	r.Status = CleanupPending
	r.CompletedAt = nil
	r.StableError = ""
	r.NextAttemptAt = nil
	r.UpdatedAt = at
	return r.Validate()
}

func validCleanupScope(scope CleanupScope) bool {
	return scope == CleanupScopeTask || scope == CleanupScopeSource || scope == CleanupScopeAccount
}

func validCleanupStatus(status CleanupStatus) bool {
	return status == CleanupPending || status == CleanupRunning || status == CleanupComplete || status == CleanupFailed
}

func cloneCleanupTargets(targets []CleanupTarget) []CleanupTarget {
	return append([]CleanupTarget(nil), targets...)
}
