package generation

import (
	"context"
	"regexp"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
)

type SubmissionDecision string

const (
	SubmissionDecisionAccepted    SubmissionDecision = "accepted"
	SubmissionDecisionNotAccepted SubmissionDecision = "not_accepted"
)

type SubmissionEvidenceType string

const (
	SubmissionEvidenceProviderConsole SubmissionEvidenceType = "provider_console"
	SubmissionEvidenceProviderQuery   SubmissionEvidenceType = "provider_query"
	SubmissionEvidenceSupportCase     SubmissionEvidenceType = "support_case"
)

var evidenceReferencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// UnknownSubmission contains only the fields required to identify and
// reconcile an indeterminate submission. Request snapshots and account data
// remain outside the operational listing.
type UnknownSubmission struct {
	ID                string
	Purpose           Purpose
	Provider          string
	Model             string
	StatusRevision    int
	SubmissionAttempt int
	UnknownAt         time.Time
	CancelRequestedAt *time.Time
	AccessRevokedAt   *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type UnknownSubmissionPage struct {
	Items       []UnknownSubmission
	NextAfterID *string
}

type ReconcileUnknownInput struct {
	TaskID            string
	ExpectedRevision  int
	Decision          SubmissionDecision
	ExternalTaskID    string
	EvidenceType      SubmissionEvidenceType
	EvidenceReference string
}

func (input ReconcileUnknownInput) Validate() error {
	if !validID(input.TaskID) || input.ExpectedRevision < 1 || !evidenceReferencePattern.MatchString(input.EvidenceReference) {
		return ErrInvalidGenerationInput
	}
	switch input.EvidenceType {
	case SubmissionEvidenceProviderConsole, SubmissionEvidenceProviderQuery, SubmissionEvidenceSupportCase:
	default:
		return ErrInvalidGenerationInput
	}
	switch input.Decision {
	case SubmissionDecisionAccepted:
		if !validToken(input.ExternalTaskID, 256) {
			return ErrInvalidGenerationInput
		}
	case SubmissionDecisionNotAccepted:
		if input.ExternalTaskID != "" {
			return ErrInvalidGenerationInput
		}
	default:
		return ErrInvalidGenerationInput
	}
	return nil
}

type SubmissionReconciliation struct {
	View       TaskView
	AuditID    string
	Decision   SubmissionDecision
	RecordedAt time.Time
}

// ListUnknown returns actionable unknown submissions to administrators.
func (s *Service) ListUnknown(ctx context.Context, token string, limit int, afterID *string) (UnknownSubmissionPage, error) {
	user, err := s.currentUser(ctx, token)
	if err != nil {
		return UnknownSubmissionPage{}, err
	}
	if user.Role != accountapp.AccountAdmin {
		return UnknownSubmissionPage{}, ErrGenerationForbidden
	}
	if limit < 1 || limit > 100 || afterID != nil && !validUUID(*afterID) {
		return UnknownSubmissionPage{}, ErrInvalidGenerationInput
	}
	return s.repository.ListUnknown(ctx, user.ID, limit, afterID)
}

// ReconcileUnknown applies an evidence-backed administrator decision without
// contacting a provider or resubmitting the task.
func (s *Service) ReconcileUnknown(ctx context.Context, token string, input ReconcileUnknownInput) (SubmissionReconciliation, error) {
	user, err := s.currentUser(ctx, token)
	if err != nil {
		return SubmissionReconciliation{}, err
	}
	if user.Role != accountapp.AccountAdmin {
		return SubmissionReconciliation{}, ErrGenerationForbidden
	}
	if err := input.Validate(); err != nil {
		return SubmissionReconciliation{}, err
	}
	return s.repository.ReconcileUnknown(ctx, user.ID, input, s.now().UTC())
}
