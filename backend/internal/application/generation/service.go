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
}

type TaskView struct {
	Task        Task
	Reservation *QuotaReservation
	Asset       *OutputAsset
}

type TaskPage struct {
	Items       []TaskView
	NextAfterID *string
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
	Cost           CostEstimate
}

type Service struct {
	auth       Authenticator
	repository Repository
	policy     AdmissionPolicy
	now        func() time.Time
}

func NewService(auth Authenticator, repository Repository, policy AdmissionPolicy) (*Service, error) {
	if auth == nil || repository == nil {
		return nil, ErrGenerationUnavailable
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &Service{auth: auth, repository: repository, policy: policy, now: time.Now}, nil
}

func (s *Service) Create(ctx context.Context, token string, input CreateServiceInput) (CreateResult, error) {
	user, err := s.currentUser(ctx, token)
	if err != nil {
		return CreateResult{}, err
	}
	if err := s.policy.Check(input.Cost, AdmissionUsage{}); err != nil && !errors.Is(err, ErrGenerationConcurrency) && !errors.Is(err, ErrGenerationQuotaExceeded) && !errors.Is(err, ErrGenerationBudgetExceeded) && !errors.Is(err, ErrGenerationCurrency) {
		// The repository repeats the check against a locked usage snapshot. This
		// early check only avoids accepting obviously malformed or disabled work.
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
		Parameters:     append([]byte(nil), input.Parameters...),
		Inputs:         input.Inputs,
		Consent:        input.Consent,
		Cost:           input.Cost,
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
