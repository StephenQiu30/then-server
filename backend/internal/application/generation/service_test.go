package generation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
)

type generationServiceAuthStub struct {
	user accountapp.User
	err  error
}

func (s generationServiceAuthStub) CurrentUser(context.Context, string) (accountapp.User, error) {
	return s.user, s.err
}

type generationServiceRepositoryStub struct {
	acceptCalls         int
	getCalls            int
	listCalls           int
	cancelCalls         int
	input               CreateInput
	policy              AdmissionPolicy
	result              AcceptanceResult
	view                TaskView
	unknownCalls        int
	reconcileCalls      int
	unknownPage         UnknownSubmissionPage
	reconciliation      SubmissionReconciliation
	reconciliationActor string
	reconciliationInput ReconcileUnknownInput
	reconciliationErr   error
}

func (s *generationServiceRepositoryStub) Accept(_ context.Context, input CreateInput, policy AdmissionPolicy, reservationID, outboxID string, at time.Time) (AcceptanceResult, error) {
	s.acceptCalls++
	s.input = input
	s.policy = policy
	if _, err := NewOutboxEvent(outboxID, mustTaskForServiceTest(input, at), at); err != nil {
		return AcceptanceResult{}, err
	}
	result := s.result
	if result.Task.ID == "" {
		result.Task = mustTaskForServiceTest(input, at)
		result.Match = RequestMatchNone
	}
	return result, nil
}

func (s *generationServiceRepositoryStub) Get(context.Context, string, string) (TaskView, error) {
	s.getCalls++
	return s.view, nil
}

func (s *generationServiceRepositoryStub) List(context.Context, string, int, *string) (TaskPage, error) {
	s.listCalls++
	return TaskPage{}, nil
}

func (s *generationServiceRepositoryStub) RequestCancel(context.Context, string, string, time.Time) (TaskView, error) {
	s.cancelCalls++
	return s.view, nil
}

func (s *generationServiceRepositoryStub) RequestTaskCleanup(context.Context, string, string, time.Time) (TaskView, CleanupRequest, error) {
	return s.view, CleanupRequest{ID: "cleanup-1", OwnerID: s.view.Task.OwnerID, TaskID: s.view.Task.ID, Scope: CleanupScopeTask, Status: CleanupPending, AccessRevokedAt: generationTestNow, CreatedAt: generationTestNow, UpdatedAt: generationTestNow}, nil
}

func (s *generationServiceRepositoryStub) ListUnknown(context.Context, string, int, *string) (UnknownSubmissionPage, error) {
	s.unknownCalls++
	return s.unknownPage, nil
}

func (s *generationServiceRepositoryStub) ReconcileUnknown(_ context.Context, actorID string, input ReconcileUnknownInput, _ time.Time) (SubmissionReconciliation, error) {
	s.reconcileCalls++
	s.reconciliationActor = actorID
	s.reconciliationInput = input
	return s.reconciliation, s.reconciliationErr
}

func serviceTestInput() CreateServiceInput {
	return CreateServiceInput{
		IdempotencyKey: "request-1",
		LookID:         "11111111-1111-4111-8111-111111111111",
		LookRevision:   3,
		Purpose:        PurposeImage,
		Provider:       "local",
		Model:          "local-image",
		Parameters:     []byte(`{"quality":"draft"}`),
		Inputs: InputSnapshot{
			LookID:       "11111111-1111-4111-8111-111111111111",
			LookRevision: 3,
			References: []InputReference{
				{MediaID: "22222222-2222-4222-8222-222222222222", Role: InputRolePerson, Ordinal: 0, Revision: 1, SHA256: strings.Repeat("a", 64)},
			},
		},
		Consent: ConsentReceipt{ID: "33333333-3333-4333-8333-333333333333", Purpose: PurposeImage, PolicyVersion: "generation-v1", AcceptedAt: generationTestNow.Add(-time.Minute)},
	}
}

func mustTaskForServiceTest(input CreateInput, at time.Time) Task {
	task, err := NewTask(input, at)
	if err != nil {
		panic(err)
	}
	return task
}

func TestServiceCreatePassesAuthenticatedOwnerAndPolicyToRepository(t *testing.T) {
	ownerID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	auth := generationServiceAuthStub{user: accountapp.User{ID: ownerID, Status: accountapp.AccountActive}}
	repository := &generationServiceRepositoryStub{}
	service, err := NewService(auth, repository, AdmissionPolicy{Enabled: true, ZeroCost: true, MaxConcurrentTasks: 2, MaxQuotaUnits: 10})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return generationTestNow }
	result, err := service.Create(context.Background(), "session-token", serviceTestInput())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repository.acceptCalls != 1 || repository.input.OwnerID != ownerID || repository.policy.ZeroCost != true {
		t.Fatalf("repository received unexpected acceptance: calls=%d input=%+v policy=%+v", repository.acceptCalls, repository.input, repository.policy)
	}
	if result.View.Task.OwnerID != ownerID || result.View.Task.Status != StatusQueued || result.Reused {
		t.Fatalf("unexpected create result: %+v", result)
	}
	if repository.input.ID == "" || repository.input.ID == ownerID {
		t.Fatalf("service did not allocate a task id: %q", repository.input.ID)
	}
	if repository.input.Cost != (CostEstimate{}) {
		t.Fatalf("local development estimate was not zero cost: %+v", repository.input.Cost)
	}
}

func TestServiceCreateRequiresServerOwnedCostEstimate(t *testing.T) {
	auth := generationServiceAuthStub{user: accountapp.User{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Status: accountapp.AccountActive}}
	repository := &generationServiceRepositoryStub{}
	policy := AdmissionPolicy{Enabled: true, Currency: "USD", MaxConcurrentTasks: 2, MaxQuotaUnits: 10, MaxBudgetMinorUnits: 100}
	service, err := NewService(auth, repository, policy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), "session", serviceTestInput()); !errors.Is(err, ErrGenerationCostUnavailable) {
		t.Fatalf("missing server-side cost estimate error = %v", err)
	}
	if repository.acceptCalls != 0 {
		t.Fatalf("accepted %d tasks without a server-side cost estimate", repository.acceptCalls)
	}
}

func TestServiceCreateUsesCanonicalParametersAndServerOwnedEstimate(t *testing.T) {
	auth := generationServiceAuthStub{user: accountapp.User{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Status: accountapp.AccountActive}}
	repository := &generationServiceRepositoryStub{}
	estimate := CostEstimate{Currency: "USD", EstimatedMinorUnits: 60, ReservedQuotaUnits: 1}
	service, err := NewServiceWithCostEstimator(auth, repository, AdmissionPolicy{Enabled: true, Currency: "USD", MaxConcurrentTasks: 2, MaxQuotaUnits: 10, MaxBudgetMinorUnits: 100}, CostEstimatorFunc(func(purpose Purpose, provider, model string, parameters []byte) (CostEstimate, error) {
		if purpose != PurposeImage || provider != "local" || model != "local-image" || string(parameters) != `{"quality":"draft"}` {
			t.Fatalf("estimator received noncanonical request facts: purpose=%q provider=%q model=%q parameters=%s", purpose, provider, model, parameters)
		}
		return estimate, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	input := serviceTestInput()
	input.Parameters = []byte(`{ "quality" : "draft" }`)
	if _, err := service.Create(context.Background(), "session", input); err != nil {
		t.Fatal(err)
	}
	if repository.input.Cost != estimate {
		t.Fatalf("repository did not receive the server-side quote: got %+v want %+v", repository.input.Cost, estimate)
	}
	if string(repository.input.Parameters) != `{"quality":"draft"}` {
		t.Fatalf("repository parameters were not canonical: %s", repository.input.Parameters)
	}
}

func TestServiceCreateFailsClosedWhenDisabled(t *testing.T) {
	auth := generationServiceAuthStub{user: accountapp.User{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Status: accountapp.AccountActive}}
	repository := &generationServiceRepositoryStub{}
	service, err := NewService(auth, repository, AdmissionPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), "session-token", serviceTestInput()); !errors.Is(err, ErrGenerationDisabled) {
		t.Fatalf("disabled Create() error = %v", err)
	}
	if repository.acceptCalls != 0 {
		t.Fatal("disabled generation reached repository")
	}
}

func TestServiceValidatesResourceIDsBeforeRepositoryCalls(t *testing.T) {
	auth := generationServiceAuthStub{user: accountapp.User{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Status: accountapp.AccountActive}}
	repository := &generationServiceRepositoryStub{}
	service, err := NewService(auth, repository, AdmissionPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(context.Background(), "session-token", "bad-id"); !errors.Is(err, ErrInvalidGenerationInput) {
		t.Fatalf("Get() error = %v", err)
	}
	if _, err := service.List(context.Background(), "session-token", 20, stringPointer("bad-id")); !errors.Is(err, ErrInvalidGenerationInput) {
		t.Fatalf("List() error = %v", err)
	}
	if _, err := service.Cancel(context.Background(), "session-token", "bad-id"); !errors.Is(err, ErrInvalidGenerationInput) {
		t.Fatalf("Cancel() error = %v", err)
	}
	if repository.getCalls != 0 || repository.listCalls != 0 || repository.cancelCalls != 0 {
		t.Fatal("invalid resource id reached repository")
	}
}

func TestServiceUnknownSubmissionOperationsRequireAdminAndValidatedEvidence(t *testing.T) {
	adminID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	taskID := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	repository := &generationServiceRepositoryStub{}
	adminAuth := generationServiceAuthStub{user: accountapp.User{ID: adminID, Role: accountapp.AccountAdmin, Status: accountapp.AccountActive}}
	service, err := NewService(adminAuth, repository, AdmissionPolicy{Enabled: true, ZeroCost: true, MaxConcurrentTasks: 1, MaxQuotaUnits: 1})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return generationTestNow }
	if _, err := service.ListUnknown(context.Background(), "admin-session", 20, nil); err != nil || repository.unknownCalls != 1 {
		t.Fatalf("admin unknown listing error=%v calls=%d", err, repository.unknownCalls)
	}
	input := ReconcileUnknownInput{TaskID: taskID, ExpectedRevision: 4, Decision: SubmissionDecisionAccepted, ExternalTaskID: "provider-task-7", EvidenceType: SubmissionEvidenceProviderQuery, EvidenceReference: "query-2026-09-25-7"}
	if _, err := service.ReconcileUnknown(context.Background(), "admin-session", input); err != nil {
		t.Fatalf("admin reconciliation error=%v", err)
	}
	if repository.reconcileCalls != 1 || repository.reconciliationActor != adminID || repository.reconciliationInput != input {
		t.Fatalf("repository received unexpected reconciliation: calls=%d actor=%q input=%+v", repository.reconcileCalls, repository.reconciliationActor, repository.reconciliationInput)
	}
	invalid := input
	invalid.EvidenceReference = "https://provider.example/private?token=secret"
	if _, err := service.ReconcileUnknown(context.Background(), "admin-session", invalid); !errors.Is(err, ErrInvalidGenerationInput) {
		t.Fatalf("invalid evidence reference error=%v", err)
	}
	if repository.reconcileCalls != 1 {
		t.Fatal("invalid evidence reached repository")
	}

	moderator := generationServiceAuthStub{user: accountapp.User{ID: adminID, Role: accountapp.AccountModerator, Status: accountapp.AccountActive}}
	moderatorService, err := NewService(moderator, repository, AdmissionPolicy{Enabled: true, ZeroCost: true, MaxConcurrentTasks: 1, MaxQuotaUnits: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := moderatorService.ListUnknown(context.Background(), "moderator-session", 20, nil); !errors.Is(err, ErrGenerationForbidden) {
		t.Fatalf("moderator listing error=%v", err)
	}
	if _, err := moderatorService.ReconcileUnknown(context.Background(), "moderator-session", input); !errors.Is(err, ErrGenerationForbidden) {
		t.Fatalf("moderator reconciliation error=%v", err)
	}
	if repository.unknownCalls != 1 || repository.reconcileCalls != 1 {
		t.Fatal("moderator reached reconciliation repository")
	}
}

func stringPointer(value string) *string { return &value }
