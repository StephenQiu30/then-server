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
	acceptCalls int
	getCalls    int
	listCalls   int
	cancelCalls int
	input       CreateInput
	policy      AdmissionPolicy
	result      AcceptanceResult
	view        TaskView
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

func stringPointer(value string) *string { return &value }
