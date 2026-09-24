package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
)

type generationHTTPStub struct {
	createToken    string
	createInput    generationapp.CreateServiceInput
	create         generationapp.CreateResult
	page           generationapp.TaskPage
	view           generationapp.TaskView
	unknownPage    generationapp.UnknownSubmissionPage
	unknownToken   string
	reconcileToken string
	reconcileInput generationapp.ReconcileUnknownInput
	err            error
}

func (s *generationHTTPStub) Create(_ context.Context, token string, input generationapp.CreateServiceInput) (generationapp.CreateResult, error) {
	s.createToken = token
	s.createInput = input
	return s.create, s.err
}

func (s *generationHTTPStub) Get(context.Context, string, string) (generationapp.TaskView, error) {
	return s.view, s.err
}

func (s *generationHTTPStub) List(context.Context, string, int, *string) (generationapp.TaskPage, error) {
	return s.page, s.err
}

func (s *generationHTTPStub) Cancel(context.Context, string, string) (generationapp.TaskView, error) {
	return s.view, s.err
}

func (s *generationHTTPStub) Delete(context.Context, string, string) (generationapp.DeleteResult, error) {
	return generationapp.DeleteResult{View: s.view, Cleanup: generationapp.CleanupRequest{ID: "66666666-6666-4666-8666-666666666666", Status: generationapp.CleanupPending, AccessRevokedAt: s.view.Task.CreatedAt, CreatedAt: s.view.Task.CreatedAt, UpdatedAt: s.view.Task.CreatedAt}}, s.err
}

func (s *generationHTTPStub) ListUnknown(_ context.Context, token string, _ int, _ *string) (generationapp.UnknownSubmissionPage, error) {
	s.unknownToken = token
	return s.unknownPage, s.err
}

func (s *generationHTTPStub) ReconcileUnknown(_ context.Context, token string, input generationapp.ReconcileUnknownInput) (generationapp.SubmissionReconciliation, error) {
	s.reconcileToken = token
	s.reconcileInput = input
	return generationapp.SubmissionReconciliation{View: s.view, AuditID: "77777777-7777-4777-8777-777777777777", Decision: input.Decision, RecordedAt: s.view.Task.CreatedAt}, s.err
}

func generationHTTPFixture() generationapp.TaskView {
	created := time.Date(2026, 9, 24, 1, 0, 0, 0, time.UTC)
	return generationapp.TaskView{Task: generationapp.Task{
		ID:                "11111111-1111-4111-8111-111111111111",
		OwnerID:           "22222222-2222-4222-8222-222222222222",
		LookID:            "33333333-3333-4333-8333-333333333333",
		LookRevision:      2,
		Purpose:           generationapp.PurposeImage,
		Provider:          "fixture",
		Model:             "fixture-image-v1",
		Status:            generationapp.StatusQueued,
		StatusRevision:    1,
		SubmissionState:   generationapp.SubmissionNotStarted,
		SubmissionAttempt: 0,
		CreatedAt:         created,
		UpdatedAt:         created,
	}}
}

func generationRouter(t *testing.T, service GenerationHTTPService) *Router {
	t.Helper()
	router, err := NewRouterWithGeneration(
		context.Background(),
		false,
		probeFunc(func(context.Context) error { return nil }),
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		NewGenerationHandler(service, true),
		time.Second,
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
	)
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func generationSessionRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "synthetic-session"})
	return request
}

func generationHTTPCreateBody() string {
	return `{"idempotency_key":"request-1","look_id":"33333333-3333-4333-8333-333333333333","look_revision":2,"purpose":"image","provider":"fixture","model":"fixture-image-v1","parameters":{"seed":1,"prompt":"look"},"inputs":[{"media_id":"44444444-4444-4444-8444-444444444444","role":"person","ordinal":0,"revision":1,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}],"consent":{"id":"55555555-5555-4555-8555-555555555555","purpose":"image","policy_version":"local-image-v1","accepted_at":"2026-09-24T00:59:00Z"}}`
}

func TestGenerationHTTPContractUsesSessionAndMapsTaskOperations(t *testing.T) {
	service := &generationHTTPStub{}
	service.view = generationHTTPFixture()
	service.create.View = service.view
	service.page = generationapp.TaskPage{Items: []generationapp.TaskView{service.view}, NextAfterID: stringPointer("11111111-1111-4111-8111-111111111111")}
	router := generationRouter(t, service)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, generationSessionRequest(http.MethodPost, "/generation-jobs", generationHTTPCreateBody()))
	if response.Code != http.StatusAccepted || service.createToken != "synthetic-session" {
		t.Fatalf("create status=%d token=%q body=%s", response.Code, service.createToken, response.Body.String())
	}
	if service.createInput.Purpose != generationapp.PurposeImage || len(service.createInput.Inputs.References) != 1 || service.createInput.Inputs.References[0].Role != generationapp.InputRolePerson {
		t.Fatalf("create input was not mapped to the domain: %+v", service.createInput)
	}
	if string(service.createInput.Parameters) != `{"prompt":"look","seed":1}` {
		t.Fatalf("parameters were not canonical JSON: %s", service.createInput.Parameters)
	}
	if strings.Contains(response.Body.String(), "owner_id") || !strings.Contains(response.Body.String(), service.view.Task.ID) {
		t.Fatalf("create response leaked private fields or omitted task id: %s", response.Body.String())
	}
	cleanupAt := service.view.Task.CreatedAt.Add(time.Minute)
	service.view.Cleanup = &generationapp.CleanupRequest{ID: "66666666-6666-4666-8666-666666666666", Status: generationapp.CleanupPending, AccessRevokedAt: cleanupAt, CreatedAt: cleanupAt, UpdatedAt: cleanupAt}

	for _, operation := range []struct {
		name   string
		method string
		path   string
		status int
	}{
		{name: "list", method: http.MethodGet, path: "/generation-jobs?limit=20", status: http.StatusOK},
		{name: "get", method: http.MethodGet, path: "/generation-jobs/11111111-1111-4111-8111-111111111111", status: http.StatusOK},
		{name: "cancel", method: http.MethodPost, path: "/generation-jobs/11111111-1111-4111-8111-111111111111/cancel", status: http.StatusAccepted},
		{name: "delete", method: http.MethodDelete, path: "/generation-jobs/11111111-1111-4111-8111-111111111111", status: http.StatusAccepted},
	} {
		t.Run(operation.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, generationSessionRequest(operation.method, operation.path, ""))
			if response.Code != operation.status {
				t.Fatalf("status=%d expected=%d body=%s", response.Code, operation.status, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "owner_id") {
				t.Fatal("generation response exposed owner id")
			}
			if operation.name == "get" && !strings.Contains(response.Body.String(), `"cleanup"`) {
				t.Fatalf("generation get response omitted cleanup state: %s", response.Body.String())
			}
		})
	}

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/generation-jobs", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("missing session status=%d", unauthorized.Code)
	}
}

func TestGenerationHTTPContractMapsDisabledAndConflictErrors(t *testing.T) {
	service := &generationHTTPStub{err: generationapp.ErrGenerationDisabled}
	router := generationRouter(t, service)
	request := generationSessionRequest(http.MethodPost, "/generation-jobs", generationHTTPCreateBody())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "1" {
		t.Fatalf("disabled generation status=%d retry-after=%q body=%s", response.Code, response.Header().Get("Retry-After"), response.Body.String())
	}

	service.err = generationapp.ErrGenerationNotCancellable
	request = generationSessionRequest(http.MethodPost, "/generation-jobs/11111111-1111-4111-8111-111111111111/cancel", "")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"CONFLICT"`) {
		t.Fatalf("cancel conflict status=%d body=%s", response.Code, response.Body.String())
	}

	service.err = generationapp.ErrGenerationSourceUnavailable
	request = generationSessionRequest(http.MethodPost, "/generation-jobs", generationHTTPCreateBody())
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"CONFLICT"`) {
		t.Fatalf("source conflict status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestGenerationHTTPContractMapsAdminSubmissionReconciliation(t *testing.T) {
	service := &generationHTTPStub{view: generationHTTPFixture()}
	service.unknownPage = generationapp.UnknownSubmissionPage{Items: []generationapp.UnknownSubmission{{
		ID: service.view.Task.ID, Purpose: generationapp.PurposeImage, Provider: "fixture", Model: "fixture-image-v1",
		StatusRevision: 5, SubmissionAttempt: 1, UnknownAt: service.view.Task.CreatedAt,
		CreatedAt: service.view.Task.CreatedAt, UpdatedAt: service.view.Task.CreatedAt,
	}}}
	router := generationRouter(t, service)

	list := httptest.NewRecorder()
	router.ServeHTTP(list, generationSessionRequest(http.MethodGet, "/admin/generation/submission-reconciliations?limit=20", ""))
	if list.Code != http.StatusOK || service.unknownToken != "synthetic-session" || !strings.Contains(list.Body.String(), service.view.Task.ID) {
		t.Fatalf("unknown submission listing status=%d token=%q body=%s", list.Code, service.unknownToken, list.Body.String())
	}
	for _, privateField := range []string{"owner_id", "inputs", "parameters", "email"} {
		if strings.Contains(list.Body.String(), privateField) {
			t.Fatalf("unknown submission listing exposed %s: %s", privateField, list.Body.String())
		}
	}

	body := `{"expected_revision":5,"decision":"accepted","external_task_id":"fixture-task-1","evidence_type":"provider_query","evidence_reference":"query-2026-09-25-1"}`
	request := generationSessionRequest(http.MethodPost, "/admin/generation-jobs/11111111-1111-4111-8111-111111111111/submission-reconciliation", body)
	reconciled := httptest.NewRecorder()
	router.ServeHTTP(reconciled, request)
	if reconciled.Code != http.StatusOK || service.reconcileToken != "synthetic-session" || service.reconcileInput.ExpectedRevision != 5 || service.reconcileInput.Decision != generationapp.SubmissionDecisionAccepted || service.reconcileInput.ExternalTaskID != "fixture-task-1" {
		t.Fatalf("reconciliation status=%d token=%q input=%+v body=%s", reconciled.Code, service.reconcileToken, service.reconcileInput, reconciled.Body.String())
	}
	if !strings.Contains(reconciled.Body.String(), `"audit_id":"77777777-7777-4777-8777-777777777777"`) || strings.Contains(reconciled.Body.String(), "evidence_reference") {
		t.Fatalf("reconciliation response did not return receipt-only confirmation: %s", reconciled.Body.String())
	}

	service.err = generationapp.ErrGenerationForbidden
	for _, operation := range []struct{ method, path, body string }{
		{http.MethodGet, "/admin/generation/submission-reconciliations", ""},
		{http.MethodPost, "/admin/generation-jobs/11111111-1111-4111-8111-111111111111/submission-reconciliation", body},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, generationSessionRequest(operation.method, operation.path, operation.body))
		if response.Code != http.StatusForbidden {
			t.Fatalf("admin operation status=%d expected=403 body=%s", response.Code, response.Body.String())
		}
	}
}

func stringPointer(value string) *string { return &value }
