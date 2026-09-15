package transport

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
)

type privacyServiceStub struct {
	declaration model.SelfAdultDeclaration
	err         error
	token       string
	input       model.ConfirmSelfAdultDeclarationInput
}

func (s *privacyServiceStub) CurrentSelfAdultDeclaration(_ context.Context, token string) (model.SelfAdultDeclaration, error) {
	s.token = token
	return s.declaration, s.err
}

func (s *privacyServiceStub) ConfirmSelfAdultDeclaration(_ context.Context, token string, input model.ConfirmSelfAdultDeclarationInput) (model.SelfAdultDeclaration, error) {
	s.token, s.input = token, input
	return s.declaration, s.err
}

func (s *privacyServiceStub) WithdrawSelfAdultDeclaration(_ context.Context, token string) (model.SelfAdultDeclaration, error) {
	s.token = token
	return s.declaration, s.err
}

func TestPrivacyDeclarationHTTPContract(t *testing.T) {
	confirmedAt := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	service := &privacyServiceStub{declaration: model.SelfAdultDeclaration{
		PolicyVersion: model.CurrentSelfAdultPolicyVersion, Confirmed: true, ConfirmedAt: &confirmedAt,
	}}
	router := privacyRouter(t, service)
	request := httptest.NewRequest(http.MethodPut, "/v1/privacy/self-adult-declaration", strings.NewReader(`{"policy_version":"self-adult-v1","confirms_self_and_adult":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"confirmed":true`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if service.token != strings.Repeat("a", 43) || service.input.PolicyVersion != model.CurrentSelfAdultPolicyVersion || !service.input.ConfirmsSelfAndAdult {
		t.Fatal("privacy transport lost the authenticated declaration input")
	}
}

func TestPrivacyDeclarationRequiresSessionAndRejectsFalseConfirmation(t *testing.T) {
	service := &privacyServiceStub{}
	router := privacyRouter(t, service)
	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/v1/privacy/self-adult-declaration", nil))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing session status=%d", missing.Code)
	}

	request := httptest.NewRequest(http.MethodPut, "/v1/privacy/self-adult-declaration", strings.NewReader(`{"policy_version":"self-adult-v1","confirms_self_and_adult":false}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	rejected := httptest.NewRecorder()
	router.ServeHTTP(rejected, request)
	if rejected.Code != http.StatusBadRequest {
		t.Fatalf("false confirmation status=%d body=%s", rejected.Code, rejected.Body.String())
	}
}

func privacyRouter(t *testing.T, service PrivacyService) *Router {
	t.Helper()
	router, err := NewRouter(context.Background(), false, probeFunc(func(context.Context) error { return nil }), nil, NewPrivacyHandler(service, true), nil, time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return router
}
