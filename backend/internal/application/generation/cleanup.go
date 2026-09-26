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
	ErrGenerationCleanupExhausted  = errors.New("generation cleanup retries exhausted")
)

const MaxCleanupAttempts = 100

// CleanupScope identifies the owner-facing operation that revoked access.
// Task is the first DELETE-01 slice; source and account cleanup use the same
// durable request shape when their cascades are wired in.
type CleanupScope string

const (
	CleanupScopeTask         CleanupScope = "task"
	CleanupScopeSource       CleanupScope = "source"
	CleanupScopeAccount      CleanupScope = "account"
	CleanupScopeOrphanOutput CleanupScope = "orphan_output"
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

const CleanupIdentityMismatchCode = "cleanup_identity_mismatch"

// CleanupTarget snapshots the private object identity or provider task that
// still needs removal. The manifest is internal and is never returned by the
// public task API. Empty object keys are retained only for legacy records.
type CleanupTarget struct {
	Kind            CleanupTargetKind `json:"kind"`
	ID              string            `json:"id"`
	Provider        string            `json:"provider,omitempty"`
	ObjectKey       string            `json:"object_key,omitempty"`
	ObjectVersionID string            `json:"object_version_id,omitempty"`
}

// SameCleanupTarget compares the immutable external side effect represented
// by two manifest entries. Object IDs are not sufficient on their own because
// versioned object storage can retain more than one physical version.
func SameCleanupTarget(left, right CleanupTarget) bool {
	return cleanupTargetKey(left) == cleanupTargetKey(right)
}

func cleanupTargetKey(target CleanupTarget) string {
	key := string(target.Kind) + "\x00" + target.ID
	if target.Kind == CleanupTargetObject {
		key += "\x00" + target.ObjectKey + "\x00" + target.ObjectVersionID
	}
	return key
}

func (target CleanupTarget) Validate() error {
	if !validToken(string(target.Kind), 32) || !validToken(target.ID, 256) {
		return ErrInvalidGenerationCleanup
	}
	switch target.Kind {
	case CleanupTargetObject:
		if target.Provider != "" || !validToken(target.ObjectVersionID, 160) || (target.ObjectKey != "" && !validGenerationOutputObjectKey(target.ObjectKey)) {
			return ErrInvalidGenerationCleanup
		}
	case CleanupTargetProvider:
		if target.ObjectKey != "" || target.ObjectVersionID != "" || (target.Provider != "" && !validToken(target.Provider, 96)) {
			return ErrInvalidGenerationCleanup
		}
	default:
		return ErrInvalidGenerationCleanup
	}
	return nil
}

func (target CleanupTarget) validateForTask(ownerID, taskID string) error {
	if err := target.Validate(); err != nil {
		return err
	}
	if target.Kind == CleanupTargetObject && target.ObjectKey != "" && !validGenerationOutputObjectKeyForTask(ownerID, taskID, target.ObjectKey) {
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
	AccessRevokedAt   *time.Time
	CompletedAt       *time.Time
	StableError       string
	Attempts          int
	NextAttemptAt     *time.Time
	Targets           []CleanupTarget
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func NewCleanupRequest(id string, task Task, scope CleanupScope, targets []CleanupTarget, at time.Time) (CleanupRequest, error) {
	if err := task.Validate(); err != nil || !validID(id) || !validID(task.OwnerID) || !validID(task.ID) || !validCleanupScope(scope) || scope == CleanupScopeOrphanOutput || task.AccessRevokedAt == nil || at.IsZero() {
		return CleanupRequest{}, ErrInvalidGenerationCleanup
	}
	at = at.UTC()
	request := CleanupRequest{
		ID:              id,
		OwnerID:         task.OwnerID,
		TaskID:          task.ID,
		Scope:           scope,
		Status:          CleanupPending,
		AccessRevokedAt: cloneTime(task.AccessRevokedAt),
		Targets:         cloneCleanupTargets(targets),
		CreatedAt:       at,
		UpdatedAt:       at,
	}
	if err := request.Validate(); err != nil {
		return CleanupRequest{}, err
	}
	return request, nil
}

// NewOrphanOutputCleanupRequest records an exact, unpublished output version.
// Unlike owner-requested cleanup, it does not revoke the task or its visibility.
func NewOrphanOutputCleanupRequest(id string, task Task, target CleanupTarget, at time.Time) (CleanupRequest, error) {
	if err := task.Validate(); err != nil || task.AccessRevokedAt != nil || !validID(id) || !validID(task.OwnerID) || !validID(task.ID) || target.Kind != CleanupTargetObject || target.ObjectKey == "" || at.IsZero() {
		return CleanupRequest{}, ErrInvalidGenerationCleanup
	}
	at = at.UTC()
	request := CleanupRequest{
		ID:        id,
		OwnerID:   task.OwnerID,
		TaskID:    task.ID,
		Scope:     CleanupScopeOrphanOutput,
		Status:    CleanupPending,
		Targets:   []CleanupTarget{target},
		CreatedAt: at,
		UpdatedAt: at,
	}
	if err := request.Validate(); err != nil {
		return CleanupRequest{}, err
	}
	return request, nil
}

func (r CleanupRequest) Validate() error {
	if !validID(r.ID) || !validID(r.OwnerID) || !validID(r.TaskID) || !validCleanupScope(r.Scope) || (r.SourceMediaID != "" && !validID(r.SourceMediaID)) || (r.AccountDeletionID != "" && !validID(r.AccountDeletionID)) || !validCleanupStatus(r.Status) || r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() || r.UpdatedAt.Before(r.CreatedAt) || r.Attempts < 0 || r.Attempts > MaxCleanupAttempts || !validToken(r.StableError, 96) && r.StableError != "" {
		return ErrInvalidGenerationCleanup
	}
	if r.Scope == CleanupScopeOrphanOutput {
		if r.AccessRevokedAt != nil || r.SourceMediaID != "" || len(r.Targets) == 0 {
			return ErrInvalidGenerationCleanup
		}
	} else if r.AccessRevokedAt == nil || r.AccessRevokedAt.IsZero() || r.AccessRevokedAt.After(r.UpdatedAt) {
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
		if err := target.validateForTask(r.OwnerID, r.TaskID); err != nil {
			return err
		}
		if r.Scope == CleanupScopeOrphanOutput && (target.Kind != CleanupTargetObject || target.ObjectKey == "") {
			return ErrInvalidGenerationCleanup
		}
		key := cleanupTargetKey(target)
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
	if r.Attempts >= MaxCleanupAttempts {
		return nil, ErrGenerationCleanupExhausted
	}
	if at.Before(r.UpdatedAt) {
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

// Exhaust records a terminally retryable cleanup failure after the bounded
// attempt budget is consumed. The request remains visible for audit and
// operator action but is no longer eligible for automatic claiming.
func (r *CleanupRequest) Exhaust(at time.Time) error {
	if r == nil || r.Validate() != nil || r.Status != CleanupRunning || r.Attempts < MaxCleanupAttempts || at.IsZero() || at.Before(r.UpdatedAt) {
		return ErrInvalidGenerationCleanup
	}
	at = at.UTC()
	r.Status = CleanupFailed
	r.CompletedAt = nil
	r.StableError = "cleanup_retry_exhausted"
	r.NextAttemptAt = nil
	r.UpdatedAt = at
	return r.Validate()
}

// RejectUnsafeTarget retains a revoked request for repair while removing it
// from automatic retries. No target is considered deleted.
func (r *CleanupRequest) RejectUnsafeTarget(at time.Time) error {
	if r == nil || r.Validate() != nil || r.Status == CleanupComplete || at.IsZero() || at.Before(r.UpdatedAt) {
		return ErrInvalidGenerationCleanup
	}
	r.Status = CleanupFailed
	r.CompletedAt = nil
	r.StableError = CleanupIdentityMismatchCode
	r.NextAttemptAt = nil
	r.UpdatedAt = at.UTC()
	return r.Validate()
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
	if r == nil || r.Validate() != nil || r.Status != CleanupRunning || at.IsZero() || at.Before(r.UpdatedAt) {
		return ErrInvalidGenerationCleanup
	}
	if r.Attempts >= MaxCleanupAttempts {
		return r.Exhaust(at)
	}
	if retryAt.IsZero() || !validToken(stableError, 96) || retryAt.Before(at) {
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
	if r == nil || r.Validate() != nil || at.IsZero() {
		return ErrInvalidGenerationCleanup
	}
	if err := target.validateForTask(r.OwnerID, r.TaskID); err != nil {
		return err
	}
	for _, existing := range r.Targets {
		if SameCleanupTarget(existing, target) {
			return nil
		}
	}
	if at.Before(r.UpdatedAt) {
		return ErrInvalidGenerationCleanup
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
	return scope == CleanupScopeTask || scope == CleanupScopeSource || scope == CleanupScopeAccount || scope == CleanupScopeOrphanOutput
}

func validCleanupStatus(status CleanupStatus) bool {
	return status == CleanupPending || status == CleanupRunning || status == CleanupComplete || status == CleanupFailed
}

func cloneCleanupTargets(targets []CleanupTarget) []CleanupTarget {
	return append([]CleanupTarget(nil), targets...)
}
