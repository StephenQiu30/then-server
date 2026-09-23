// Package generation contains the provider-independent contract for optional
// image and model generation jobs.
package generation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalidGenerationInput   = errors.New("invalid generation input")
	ErrInvalidGenerationState   = errors.New("invalid generation state")
	ErrGenerationNotCancellable = errors.New("generation job is not cancellable")
	ErrGenerationNotSubmittable = errors.New("generation job is not submittable")
	ErrSubmissionInProgress     = errors.New("generation submission is already in progress")
	ErrSubmissionOutcomeUnknown = errors.New("generation submission outcome is unknown")
	ErrExternalTaskConflict     = errors.New("external generation task conflict")
	ErrGenerationOutputRequired = errors.New("validated generation output is required before success")
)

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Purpose identifies the single output requested by a generation job.
type Purpose string

const (
	PurposeImage Purpose = "image"
	PurposeModel Purpose = "model"
)

func (p Purpose) valid() bool { return p == PurposeImage || p == PurposeModel }

// RequestMatch classifies an existing owner-scoped task against a new
// request. The repository uses this result inside its idempotent transaction;
// the domain does not expose whether a task or asset is readable.
type RequestMatch string

const (
	RequestMatchNone                RequestMatch = "none"
	RequestMatchIdempotentReplay    RequestMatch = "idempotent_replay"
	RequestMatchContentDedupe       RequestMatch = "content_dedupe"
	RequestMatchIdempotencyConflict RequestMatch = "idempotency_conflict"
)

// Status is the persisted, user-visible task state. Cancellation requests are
// tracked separately because an external provider may acknowledge them later.
type Status string

const (
	StatusQueued     Status = "queued"
	StatusRunning    Status = "running"
	StatusValidating Status = "validating"
	StatusSucceeded  Status = "succeeded"
	StatusFailed     Status = "failed"
	StatusCanceled   Status = "canceled"
	StatusExpired    Status = "expired"
)

func (s Status) terminal() bool {
	return s == StatusSucceeded || s == StatusFailed || s == StatusCanceled || s == StatusExpired
}

// SubmissionState records the provider submission boundary separately from
// the user-visible task state. In particular, unknown means that transport
// failed after the request may have been accepted and must be reconciled
// before another billable submit is allowed.
type SubmissionState string

const (
	SubmissionNotStarted SubmissionState = "not_started"
	SubmissionInFlight   SubmissionState = "in_flight"
	SubmissionUnknown    SubmissionState = "unknown"
	SubmissionAccepted   SubmissionState = "accepted"
)

// CanTransitionTo describes the only legal task state transitions.
func (s Status) CanTransitionTo(next Status) bool {
	switch s {
	case StatusQueued:
		return next == StatusRunning || next == StatusFailed || next == StatusCanceled || next == StatusExpired
	case StatusRunning:
		return next == StatusValidating || next == StatusFailed || next == StatusCanceled || next == StatusExpired
	case StatusValidating:
		return next == StatusSucceeded || next == StatusFailed || next == StatusCanceled || next == StatusExpired
	default:
		return false
	}
}

// InputRole identifies the purpose of one immutable input reference.
type InputRole string

const (
	InputRolePerson    InputRole = "person"
	InputRoleGarment   InputRole = "garment"
	InputRoleLookImage InputRole = "look_image"
)

func (r InputRole) valid() bool {
	return r == InputRolePerson || r == InputRoleGarment || r == InputRoleLookImage
}

// InputReference is metadata only. Binary media stays in the media/object
// store and is never embedded in a generation job.
type InputReference struct {
	MediaID  string
	Role     InputRole
	Ordinal  int
	Revision int
	SHA256   string
}

// InputSnapshot freezes the source facts used by a job. Later Look edits do
// not mutate an already accepted task.
type InputSnapshot struct {
	LookID       string
	LookRevision int
	References   []InputReference
	ImageAssetID string
	ImageSHA256  string
}

// ConsentReceipt records the purpose-specific consent captured at acceptance.
type ConsentReceipt struct {
	ID            string
	Purpose       Purpose
	PolicyVersion string
	AcceptedAt    time.Time
}

// CostEstimate is recorded with the task. It is not a user-facing price and
// does not authorize a provider charge by itself.
type CostEstimate struct {
	Currency            string
	EstimatedMinorUnits int64
	ReservedQuotaUnits  int
}

// CreateInput is the provider-neutral input to the generation domain.
type CreateInput struct {
	ID             string
	OwnerID        string
	IdempotencyKey string
	LookID         string
	LookRevision   int
	Purpose        Purpose
	Provider       string
	Model          string
	Parameters     []byte
	Inputs         InputSnapshot
	Consent        ConsentReceipt
	Cost           CostEstimate
}

// Task is the immutable-input, mutable-state generation fact. It is suitable
// for a future repository record but deliberately has no GORM or HTTP tags.
type Task struct {
	ID           string
	OwnerID      string
	LookID       string
	LookRevision int
	Purpose      Purpose
	Provider     string
	Model        string
	Parameters   []byte
	Inputs       InputSnapshot
	Consent      ConsentReceipt
	Cost         CostEstimate
	// IdempotencyKeyHash is scoped to owner and purpose so it is safe to pass
	// as a provider idempotency token without cross-account collisions.
	IdempotencyKeyHash  string
	DedupeKey           string
	Status              Status
	StatusRevision      int
	SubmissionState     SubmissionState
	SubmissionAttempt   int
	SubmissionStartedAt *time.Time
	SubmissionUnknownAt *time.Time
	CancelRequestedAt   *time.Time
	ExternalTaskID      string
	ResultAssetID       string
	FailureCode         string
	LeaseOwner          string
	FencingToken        uint64
	LeaseAttempt        int
	LeaseUntil          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// PrepareSubmission creates the provider-neutral payload for a first submit.
// Workers should call BeginSubmission before sending it. A task with an
// external ID or a non-idle submission state must be reconciled instead of
// being submitted again.
func (t Task) PrepareSubmission() (Submission, error) {
	if !submissionStatusAllowed(t.Status) || t.CancelRequestedAt != nil || t.ExternalTaskID != "" || t.SubmissionState != SubmissionNotStarted {
		return Submission{}, ErrGenerationNotSubmittable
	}
	return t.submission(), nil
}

func submissionStatusAllowed(status Status) bool {
	return status == StatusQueued || status == StatusRunning
}

func (t Task) submission() Submission {
	return Submission{
		TaskID:      t.ID,
		Purpose:     t.Purpose,
		Provider:    t.Provider,
		Model:       t.Model,
		Parameters:  append([]byte(nil), t.Parameters...),
		Inputs:      cloneSnapshot(t.Inputs),
		Idempotency: t.IdempotencyKeyHash,
		Attempt:     t.SubmissionAttempt,
	}
}

// BeginSubmission atomically marks the first provider submission as in flight
// and returns its immutable payload. The caller must hold an active lease and
// persist this mutation with its fencing token before making a network
// request. If the request outcome is lost, the task can then be marked unknown
// and cannot be submitted again until reconciliation.
func (t *Task) BeginSubmission(at time.Time) (Submission, error) {
	if t == nil || !submissionStatusAllowed(t.Status) || t.CancelRequestedAt != nil || t.ExternalTaskID != "" {
		return Submission{}, ErrGenerationNotSubmittable
	}
	if err := t.validateActiveLeaseAt(at); err != nil {
		return Submission{}, err
	}
	if at.IsZero() || (!t.UpdatedAt.IsZero() && at.Before(t.UpdatedAt)) {
		return Submission{}, ErrInvalidGenerationState
	}
	switch t.SubmissionState {
	case SubmissionInFlight:
		return Submission{}, ErrSubmissionInProgress
	case SubmissionUnknown:
		return Submission{}, ErrSubmissionOutcomeUnknown
	case SubmissionAccepted:
		return Submission{}, ErrGenerationNotSubmittable
	case SubmissionNotStarted:
	default:
		return Submission{}, ErrInvalidGenerationState
	}
	at = at.UTC()
	t.SubmissionState = SubmissionInFlight
	t.SubmissionAttempt++
	t.SubmissionStartedAt = timePtr(at)
	t.StatusRevision++
	t.UpdatedAt = at
	return t.submission(), nil
}

// MarkSubmissionUnknown records a transport outcome that cannot prove
// whether the provider accepted the request. The worker must still hold an
// active lease. It deliberately leaves the task queued while blocking blind
// resubmission until an explicit reconciliation.
func (t *Task) MarkSubmissionUnknown(at time.Time) error {
	if t == nil || t.SubmissionState != SubmissionInFlight || at.IsZero() {
		return ErrInvalidGenerationState
	}
	if err := t.validateActiveLeaseAt(at); err != nil {
		return err
	}
	if !t.UpdatedAt.IsZero() && at.Before(t.UpdatedAt) {
		return ErrInvalidGenerationState
	}
	at = at.UTC()
	t.SubmissionState = SubmissionUnknown
	t.SubmissionUnknownAt = timePtr(at)
	t.StatusRevision++
	t.UpdatedAt = at
	return nil
}

// ReconcileSubmissionNotAccepted clears an unknown submission only after a
// provider lookup or an operator decision proves that no external task was
// accepted. The worker must hold an active lease; a new attempt then uses the
// same idempotency identity.
func (t *Task) ReconcileSubmissionNotAccepted(at time.Time) error {
	if t == nil || t.SubmissionState != SubmissionUnknown || at.IsZero() {
		return ErrInvalidGenerationState
	}
	if err := t.validateActiveLeaseAt(at); err != nil {
		return err
	}
	if !t.UpdatedAt.IsZero() && at.Before(t.UpdatedAt) {
		return ErrInvalidGenerationState
	}
	at = at.UTC()
	t.SubmissionState = SubmissionNotStarted
	t.SubmissionUnknownAt = nil
	t.StatusRevision++
	t.UpdatedAt = at
	return nil
}

// NewTask validates and freezes a generation request in queued state. The
// caller supplies the ID so persistence and future idempotency can share one
// identity source.
func NewTask(input CreateInput, now time.Time) (Task, error) {
	canonicalParameters, idempotencyHash, dedupeKey, err := identity(input)
	if err != nil {
		return Task{}, err
	}
	if !validID(input.ID) {
		return Task{}, ErrInvalidGenerationInput
	}
	now = now.UTC()
	if now.IsZero() || input.Consent.AcceptedAt.IsZero() || input.Consent.AcceptedAt.After(now) {
		return Task{}, ErrInvalidGenerationInput
	}
	return Task{
		ID:                 input.ID,
		OwnerID:            input.OwnerID,
		LookID:             input.LookID,
		LookRevision:       input.LookRevision,
		Purpose:            input.Purpose,
		Provider:           input.Provider,
		Model:              input.Model,
		Parameters:         canonicalParameters,
		Inputs:             cloneSnapshot(input.Inputs),
		Consent:            input.Consent,
		Cost:               input.Cost,
		IdempotencyKeyHash: idempotencyHash,
		DedupeKey:          dedupeKey,
		Status:             StatusQueued,
		StatusRevision:     1,
		SubmissionState:    SubmissionNotStarted,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}

// Transition applies a legal status transition and increments the optimistic
// revision. A failure code is retained as a stable, non-sensitive reason.
func (t *Task) Transition(next Status, failureCode string, at time.Time) error {
	if t == nil || !t.Status.CanTransitionTo(next) || at.IsZero() {
		return ErrInvalidGenerationState
	}
	if !validFailureState(t.Status, t.FailureCode) || !validFailureState(next, failureCode) {
		return ErrInvalidGenerationState
	}
	if next == StatusSucceeded && t.CancelRequestedAt != nil {
		return ErrInvalidGenerationState
	}
	if next == StatusSucceeded && !validID(t.ResultAssetID) {
		return ErrGenerationOutputRequired
	}
	if next != StatusSucceeded && t.ResultAssetID != "" {
		return ErrInvalidGenerationState
	}
	if !t.UpdatedAt.IsZero() && at.Before(t.UpdatedAt) {
		return ErrInvalidGenerationState
	}
	at = at.UTC()
	t.Status = next
	t.StatusRevision++
	t.UpdatedAt = at
	t.FailureCode = ""
	if next != StatusSucceeded {
		t.FailureCode = failureCode
	}
	return nil
}

// RecordExternalTaskID attaches the provider's task identity without changing
// the user-visible state. Non-terminal tasks require an active worker lease.
// A late acceptance is still recorded after a local cancellation or expiry so
// cleanup can find the provider task; a different provider identity is never
// allowed to overwrite the first one.
func (t *Task) RecordExternalTaskID(externalID string, at time.Time) error {
	if t == nil || !validToken(externalID, 256) || at.IsZero() {
		return ErrInvalidGenerationState
	}
	if t.ExternalTaskID != "" {
		if t.ExternalTaskID != externalID {
			return ErrExternalTaskConflict
		}
		return nil
	}
	if !t.Status.terminal() {
		if err := t.validateActiveLeaseAt(at); err != nil {
			return err
		}
	}
	switch t.SubmissionState {
	case SubmissionNotStarted, SubmissionInFlight, SubmissionUnknown:
	case SubmissionAccepted:
		return ErrInvalidGenerationState
	default:
		return ErrInvalidGenerationState
	}
	at = at.UTC()
	t.ExternalTaskID = externalID
	t.SubmissionState = SubmissionAccepted
	t.SubmissionUnknownAt = nil
	if t.SubmissionAttempt == 0 {
		t.SubmissionAttempt = 1
	}
	t.StatusRevision++
	// The provider may have accepted the request before a response was
	// delivered. Keep the local clock monotonic while retaining that late
	// identity for cleanup.
	if t.UpdatedAt.IsZero() || at.After(t.UpdatedAt) {
		t.UpdatedAt = at
	}
	return nil
}

// ApplyProviderState reconciles an observation that has already been mapped
// by an adapter to the domain state machine. Non-terminal tasks require an
// active worker lease. A provider-success observation stops at validating;
// PublishOutput is the only path that can mark a task succeeded after object
// validation and lineage checks. Repeated observations are idempotent; stale
// or cross-task observations cannot move a task backwards or attach a result
// to another task.
func (t *Task) ApplyProviderState(externalID string, next Status, failureCode string, at time.Time) error {
	if t == nil || !validToken(externalID, 256) || t.ExternalTaskID == "" || t.ExternalTaskID != externalID {
		return ErrExternalTaskConflict
	}
	if at.IsZero() {
		return ErrInvalidGenerationState
	}
	if !validFailureState(t.Status, t.FailureCode) || !validFailureState(next, failureCode) {
		return ErrInvalidGenerationState
	}
	if !t.Status.terminal() {
		if err := t.validateActiveLeaseAt(at); err != nil {
			return err
		}
	}
	if next == StatusSucceeded {
		if t.Status.terminal() {
			return ErrInvalidGenerationState
		}
		return ErrGenerationOutputRequired
	}
	if next == t.Status {
		if failureCode != t.FailureCode {
			return ErrInvalidGenerationState
		}
		return nil
	}
	return t.Transition(next, failureCode, at)
}

// RequestCancel records a cancellation request without claiming that the
// provider has already stopped or that cleanup has completed.
func (t *Task) RequestCancel(at time.Time) error {
	if t == nil || t.Status.terminal() || at.IsZero() {
		return ErrGenerationNotCancellable
	}
	if t.CancelRequestedAt != nil {
		return nil
	}
	at = at.UTC()
	if !t.UpdatedAt.IsZero() && at.Before(t.UpdatedAt) {
		return ErrInvalidGenerationState
	}
	t.CancelRequestedAt = &at
	t.StatusRevision++
	t.UpdatedAt = at
	return nil
}

// ClassifyRequest compares a new request with one task already found in the
// same owner scope. A failed, canceled, or expired task does not block an
// explicitly new attempt through content deduplication, while replaying its
// original idempotency key still returns that task.
func ClassifyRequest(existing Task, input CreateInput) (RequestMatch, error) {
	_, idempotencyKeyHash, dedupeKey, err := identity(input)
	if err != nil {
		return RequestMatchNone, err
	}
	if existing.OwnerID != input.OwnerID {
		return RequestMatchNone, nil
	}
	if existing.IdempotencyKeyHash == idempotencyKeyHash {
		if existing.DedupeKey == dedupeKey {
			return RequestMatchIdempotentReplay, nil
		}
		return RequestMatchIdempotencyConflict, nil
	}
	if existing.DedupeKey == dedupeKey && dedupeEligible(existing.Status) {
		return RequestMatchContentDedupe, nil
	}
	return RequestMatchNone, nil
}

func dedupeEligible(status Status) bool {
	switch status {
	case StatusQueued, StatusRunning, StatusValidating, StatusSucceeded:
		return true
	default:
		return false
	}
}

// Identity returns owner/purpose-scoped idempotency and content deduplication
// hashes. The raw idempotency key is never retained.
func Identity(input CreateInput) (idempotencyKeyHash, dedupeKey string, err error) {
	_, idempotencyKeyHash, dedupeKey, err = identity(input)
	return idempotencyKeyHash, dedupeKey, err
}

func identity(input CreateInput) ([]byte, string, string, error) {
	if !validID(input.OwnerID) || !validID(input.LookID) || input.LookRevision < 1 || !input.Purpose.valid() || !validToken(input.Provider, 96) || !validToken(input.Model, 128) || !validToken(input.IdempotencyKey, 256) {
		return nil, "", "", ErrInvalidGenerationInput
	}
	if input.Cost.EstimatedMinorUnits < 0 || input.Cost.ReservedQuotaUnits < 0 || input.Cost.EstimatedMinorUnits > 0 && !validToken(input.Cost.Currency, 16) {
		return nil, "", "", ErrInvalidGenerationInput
	}
	if input.Cost.EstimatedMinorUnits == 0 && input.Cost.Currency != "" && !validToken(input.Cost.Currency, 16) {
		return nil, "", "", ErrInvalidGenerationInput
	}
	parameters, err := canonicalJSON(input.Parameters)
	if err != nil {
		return nil, "", "", ErrInvalidGenerationInput
	}
	if !validSnapshot(input.Purpose, input.LookID, input.LookRevision, input.Inputs) || !validConsent(input.Purpose, input.Consent) {
		return nil, "", "", ErrInvalidGenerationInput
	}
	keyHash := hashIdempotency(input.OwnerID, input.Purpose, input.IdempotencyKey)
	dedupePayload := struct {
		OwnerID      string
		LookID       string
		LookRevision int
		Purpose      Purpose
		Provider     string
		Model        string
		Parameters   json.RawMessage
		Inputs       InputSnapshot
	}{input.OwnerID, input.LookID, input.LookRevision, input.Purpose, input.Provider, input.Model, parameters, canonicalSnapshot(input.Inputs)}
	encoded, err := json.Marshal(dedupePayload)
	if err != nil {
		return nil, "", "", ErrInvalidGenerationInput
	}
	return parameters, keyHash, hashBytes(encoded), nil
}

func hashIdempotency(ownerID string, purpose Purpose, key string) string {
	return hashBytes([]byte(ownerID + "\x00" + string(purpose) + "\x00" + strings.TrimSpace(key)))
}

func validSnapshot(purpose Purpose, lookID string, lookRevision int, snapshot InputSnapshot) bool {
	if snapshot.LookID != lookID || snapshot.LookRevision != lookRevision || len(snapshot.References) == 0 || len(snapshot.References) > 4 {
		return false
	}
	seenIDs := make(map[string]struct{}, len(snapshot.References))
	seenOrdinals := make(map[int]struct{}, len(snapshot.References))
	personCount, garmentCount, lookImageCount := 0, 0, 0
	for _, reference := range snapshot.References {
		if !validID(reference.MediaID) || !reference.Role.valid() || reference.Revision < 1 || reference.Ordinal < 0 || !sha256Pattern.MatchString(reference.SHA256) {
			return false
		}
		if _, ok := seenIDs[reference.MediaID]; ok {
			return false
		}
		if _, ok := seenOrdinals[reference.Ordinal]; ok {
			return false
		}
		seenIDs[reference.MediaID] = struct{}{}
		seenOrdinals[reference.Ordinal] = struct{}{}
		switch reference.Role {
		case InputRolePerson:
			personCount++
		case InputRoleGarment:
			garmentCount++
		case InputRoleLookImage:
			lookImageCount++
		}
	}
	if purpose == PurposeModel {
		if !validID(snapshot.ImageAssetID) || !sha256Pattern.MatchString(snapshot.ImageSHA256) || lookImageCount != 1 || personCount != 0 || garmentCount != 0 {
			return false
		}
		for _, reference := range snapshot.References {
			if reference.Role == InputRoleLookImage {
				return reference.MediaID == snapshot.ImageAssetID && reference.SHA256 == snapshot.ImageSHA256
			}
		}
		return false
	}
	return snapshot.ImageAssetID == "" && snapshot.ImageSHA256 == "" && personCount == 1 && garmentCount <= 3 && lookImageCount == 0
}

func validConsent(purpose Purpose, receipt ConsentReceipt) bool {
	return validID(receipt.ID) && receipt.Purpose == purpose && validToken(receipt.PolicyVersion, 96) && !receipt.AcceptedAt.IsZero()
}

func canonicalJSON(value []byte) ([]byte, error) {
	value = bytes.TrimSpace(value)
	if len(value) == 0 {
		value = []byte("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("multiple JSON values")
	}
	if _, ok := decoded.(map[string]any); !ok {
		return nil, errors.New("parameters must be an object")
	}
	return json.Marshal(decoded)
}

func canonicalSnapshot(snapshot InputSnapshot) InputSnapshot {
	copySnapshot := cloneSnapshot(snapshot)
	sort.Slice(copySnapshot.References, func(i, j int) bool {
		left, right := copySnapshot.References[i], copySnapshot.References[j]
		if left.Ordinal != right.Ordinal {
			return left.Ordinal < right.Ordinal
		}
		return left.MediaID < right.MediaID
	})
	return copySnapshot
}

func cloneSnapshot(snapshot InputSnapshot) InputSnapshot {
	snapshot.References = append([]InputReference(nil), snapshot.References...)
	return snapshot
}

func timePtr(value time.Time) *time.Time {
	copy := value
	return &copy
}

func hashText(value string) string { return hashBytes([]byte(value)) }

func hashBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func validID(value string) bool { return validToken(value, 128) }

func validFailureState(status Status, failureCode string) bool {
	switch status {
	case StatusFailed:
		return validToken(failureCode, 96)
	case StatusQueued, StatusRunning, StatusValidating, StatusSucceeded, StatusCanceled, StatusExpired:
		return failureCode == ""
	default:
		return false
	}
}

func validToken(value string, maxBytes int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maxBytes || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
