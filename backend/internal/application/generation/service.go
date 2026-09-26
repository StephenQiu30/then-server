package generation

import (
	"context"
	"errors"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	"github.com/google/uuid"
)

var (
	ErrGenerationUnavailable = errors.New("generation service unavailable")
	ErrGenerationNotFound    = errors.New("generation job not found")
	ErrImageNotConfirmable   = errors.New("generation image is not confirmable")
)

// Authenticator is the small account boundary needed by generation use cases.
// The generation package does not depend on sessions or persistence details.
type Authenticator interface {
	CurrentUser(context.Context, string) (accountapp.User, error)
}

// Repository owns the transaction that writes a task, quota hold and outbox
// intent together. Provider execution is deliberately outside this boundary.
type Repository interface {
	Accept(context.Context, CreateInput, AdmissionPolicy, string, string, time.Time) (AcceptanceResult, error)
	Get(context.Context, string, string) (TaskView, error)
	List(context.Context, string, int, *string) (TaskPage, error)
	RequestCancel(context.Context, string, string, time.Time) (TaskView, error)
	ConfirmImage(context.Context, string, string, time.Time, time.Duration) (TaskView, error)
	RequestTaskCleanup(context.Context, string, string, time.Time) (TaskView, CleanupRequest, error)
	ListUnknown(context.Context, string, int, *string) (UnknownSubmissionPage, error)
	ReconcileUnknown(context.Context, string, ReconcileUnknownInput, time.Time) (SubmissionReconciliation, error)
}

type TaskView struct {
	Task             Task
	Reservation      *QuotaReservation
	Asset            *OutputAsset
	ImageConfirmedAt *time.Time
	Cleanup          *CleanupRequest
}

type TaskPage struct {
	Items       []TaskView
	NextAfterID *string
}

type DeleteResult struct {
	View    TaskView
	Cleanup CleanupRequest
}

type CreateResult struct {
	View   TaskView
	Match  RequestMatch
	Reused bool
}

type CreateServiceInput struct {
	IdempotencyKey string
	LookID         string
	LookRevision   int
	Purpose        Purpose
	Provider       string
	Model          string
	Parameters     []byte
	Inputs         InputSnapshot
	Consent        ConsentReceipt
}

type Service struct {
	auth          Authenticator
	repository    Repository
	policy        AdmissionPolicy
	costEstimator CostEstimator
	now           func() time.Time
}

func NewService(auth Authenticator, repository Repository, policy AdmissionPolicy) (*Service, error) {
	var costEstimator CostEstimator
	if policy.ZeroCost {
		costEstimator = CostEstimatorFunc(func(Purpose, string, string, []byte) (CostEstimate, error) {
			return CostEstimate{}, nil
		})
	}
	return NewServiceWithCostEstimator(auth, repository, policy, costEstimator)
}

func NewServiceWithCostEstimator(auth Authenticator, repository Repository, policy AdmissionPolicy, costEstimator CostEstimator) (*Service, error) {
	if auth == nil || repository == nil {
		return nil, ErrGenerationUnavailable
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &Service{auth: auth, repository: repository, policy: policy, costEstimator: costEstimator, now: time.Now}, nil
}

func (s *Service) Create(ctx context.Context, token string, input CreateServiceInput) (CreateResult, error) {
	user, err := s.currentUser(ctx, token)
	if err != nil {
		return CreateResult{}, err
	}
	if !s.policy.Enabled {
		return CreateResult{}, ErrGenerationDisabled
	}
	if s.costEstimator == nil {
		return CreateResult{}, ErrGenerationCostUnavailable
	}
	parameters, err := CanonicalParameters(input.Parameters)
	if err != nil {
		return CreateResult{}, ErrInvalidGenerationInput
	}
	cost, err := s.costEstimator.Estimate(input.Purpose, input.Provider, input.Model, append([]byte(nil), parameters...))
	if err != nil {
		return CreateResult{}, err
	}
	if err := s.policy.Check(cost, AdmissionUsage{}); err != nil {
		return CreateResult{}, err
	}
	create := CreateInput{
		ID:             uuid.NewString(),
		OwnerID:        user.ID,
		IdempotencyKey: input.IdempotencyKey,
		LookID:         input.LookID,
		LookRevision:   input.LookRevision,
		Purpose:        input.Purpose,
		Provider:       input.Provider,
		Model:          input.Model,
		Parameters:     parameters,
		Inputs:         input.Inputs,
		Consent:        input.Consent,
		Cost:           cost,
	}
	now := s.now().UTC()
	result, err := s.repository.Accept(ctx, create, s.policy, uuid.NewString(), uuid.NewString(), now)
	if err != nil {
		return CreateResult{}, err
	}
	return CreateResult{View: TaskView{Task: result.Task, Reservation: result.Reservation}, Match: result.Match, Reused: result.Reused}, nil
}

func (s *Service) Get(ctx context.Context, token, taskID string) (TaskView, error) {
	user, err := s.currentUser(ctx, token)
	if err != nil {
		return TaskView{}, err
	}
	if !validUUID(taskID) {
		return TaskView{}, ErrInvalidGenerationInput
	}
	return s.repository.Get(ctx, user.ID, taskID)
}

func (s *Service) GetOutput(ctx context.Context, token, taskID string) (OutputAsset, error) {
	view, err := s.Get(ctx, token, taskID)
	if err != nil {
		return OutputAsset{}, err
	}
	if view.Task.AccessRevokedAt != nil || view.Task.Status != StatusSucceeded || view.Asset == nil ||
		view.Asset.ObjectKey == "" || view.Asset.ObjectVersionID == "" {
		return OutputAsset{}, ErrGenerationNotFound
	}
	if OutputExpired(view.Asset.PublishedAt, s.policy.OutputRetention, s.now().UTC()) {
		return OutputAsset{}, ErrGenerationNotFound
	}
	if err := view.Asset.ValidatePersistedFor(view.Task); err != nil {
		return OutputAsset{}, ErrGenerationUnavailable
	}
	return *view.Asset, nil
}

func (s *Service) List(ctx context.Context, token string, limit int, afterID *string) (TaskPage, error) {
	user, err := s.currentUser(ctx, token)
	if err != nil {
		return TaskPage{}, err
	}
	if limit < 1 || limit > 100 {
		return TaskPage{}, ErrInvalidGenerationInput
	}
	if afterID != nil && !validUUID(*afterID) {
		return TaskPage{}, ErrInvalidGenerationInput
	}
	return s.repository.List(ctx, user.ID, limit, afterID)
}

func (s *Service) Cancel(ctx context.Context, token, taskID string) (TaskView, error) {
	user, err := s.currentUser(ctx, token)
	if err != nil {
		return TaskView{}, err
	}
	if !validUUID(taskID) {
		return TaskView{}, ErrInvalidGenerationInput
	}
	return s.repository.RequestCancel(ctx, user.ID, taskID, s.now().UTC())
}

func (s *Service) ConfirmImage(ctx context.Context, token, taskID string) (TaskView, error) {
	user, err := s.currentUser(ctx, token)
	if err != nil {
		return TaskView{}, err
	}
	if !validUUID(taskID) {
		return TaskView{}, ErrInvalidGenerationInput
	}
	return s.repository.ConfirmImage(ctx, user.ID, taskID, s.now().UTC(), s.policy.OutputRetention)
}

func (s *Service) Delete(ctx context.Context, token, taskID string) (DeleteResult, error) {
	user, err := s.currentUser(ctx, token)
	if err != nil {
		return DeleteResult{}, err
	}
	if !validUUID(taskID) {
		return DeleteResult{}, ErrInvalidGenerationInput
	}
	view, cleanup, err := s.repository.RequestTaskCleanup(ctx, user.ID, taskID, s.now().UTC())
	if err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{View: view, Cleanup: cleanup}, nil
}

func (s *Service) currentUser(ctx context.Context, token string) (accountapp.User, error) {
	if token == "" {
		return accountapp.User{}, accountapp.ErrAuthentication
	}
	return s.auth.CurrentUser(ctx, token)
}

func validUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}
