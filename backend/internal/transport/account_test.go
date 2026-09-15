package transport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

type accountServiceStub struct {
	registered model.RegisterAccountInput
	loggedIn   model.CreateSessionInput
	updated    model.UpdateCurrentUserInput
	user       model.User
	token      string
	err        error
	registers  int
	logins     int
}

func (s *accountServiceStub) Register(_ context.Context, input model.RegisterAccountInput) (model.AuthenticatedUser, error) {
	s.registered = input
	s.registers++
	return model.AuthenticatedUser{User: s.user, Token: s.token, ExpiresAt: time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)}, s.err
}

func (s *accountServiceStub) Login(_ context.Context, input model.CreateSessionInput) (model.AuthenticatedUser, error) {
	s.loggedIn = input
	s.logins++
	return model.AuthenticatedUser{User: s.user, Token: s.token, ExpiresAt: time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)}, s.err
}

type authRateLimiterStub struct {
	mu     sync.Mutex
	counts map[string]int
	err    error
}

func (s *authRateLimiterStub) Allow(_ context.Context, scope, subject string, limit int, window time.Duration) (bool, time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return false, 0, s.err
	}
	if s.counts == nil {
		s.counts = make(map[string]int)
	}
	key := scope + ":" + subject
	s.counts[key]++
	return s.counts[key] <= limit, window, nil
}

func (s *accountServiceStub) CurrentUser(_ context.Context, token string) (model.User, error) {
	s.token = token
	return s.user, s.err
}

func (s *accountServiceStub) UpdateCurrentUser(_ context.Context, token string, input model.UpdateCurrentUserInput) (model.User, error) {
	s.token, s.updated = token, input
	return s.user, s.err
}

func (s *accountServiceStub) Logout(_ context.Context, token string) error {
	s.token = token
	return s.err
}

func (s *accountServiceStub) DeleteCurrentUser(_ context.Context, token string) error {
	s.token = token
	return s.err
}

func accountRouter(t *testing.T, service AccountService, secure bool) *Router {
	t.Helper()
	return accountRouterWithLimiter(t, service, secure, &authRateLimiterStub{})
}

func accountRouterWithLimiter(t *testing.T, service AccountService, secure bool, limiter AuthenticationRateLimiter) *Router {
	t.Helper()
	router, err := NewRouter(context.Background(), false, probeFunc(func(context.Context) error { return nil }), NewAccountHandler(service, secure, limiter), nil, nil, time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func authRequest(method, path, body, remoteAddr string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = remoteAddr
	return request
}

func fixtureUser() model.User {
	return model.User{
		ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11", Email: "person@example.test", DisplayName: "示例用户",
		CreatedAt: time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC),
	}
}

func TestRegisterSetsProtectedSessionCookie(t *testing.T) {
	service := &accountServiceStub{user: fixtureUser(), token: strings.Repeat("a", 43)}
	router := accountRouter(t, service, true)
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/registrations", strings.NewReader(`{"email":"person@example.test","display_name":"示例用户","password":"correct-password"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/v1" {
		t.Fatal("registration cookie does not satisfy the security contract")
	}
	if service.registered.Password != "correct-password" || strings.Contains(response.Body.String(), service.registered.Password) {
		t.Fatal("registration transport lost or exposed credentials")
	}
}

func TestContractRejectsUnknownAccountFieldsBeforeService(t *testing.T) {
	service := &accountServiceStub{user: fixtureUser(), token: strings.Repeat("a", 43)}
	router := accountRouter(t, service, false)
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/registrations", strings.NewReader(`{"email":"person@example.test","display_name":"Name","password":"correct-password","role":"admin"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || service.registered.Email != "" {
		t.Fatal("contract-invalid registration reached the service")
	}
}

func TestCurrentUserRequiresSession(t *testing.T) {
	service := &accountServiceStub{user: fixtureUser()}
	router := accountRouter(t, service, false)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/users/me", nil))
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "AUTHENTICATION_FAILED") {
		t.Fatalf("missing session did not fail closed: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRejectedAuthenticatedSessionClearsStaleCookie(t *testing.T) {
	service := &accountServiceStub{user: fixtureUser(), err: model.ErrAuthentication}
	router := accountRouter(t, service, true)
	request := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName || cookies[0].Value != "" || cookies[0].MaxAge != -1 || cookies[0].Path != "/v1" || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("rejected authenticated session did not clear the stale cookie with matching security attributes")
	}
}

func TestFailedLoginDoesNotClearExistingSessionCookie(t *testing.T) {
	service := &accountServiceStub{user: fixtureUser(), err: model.ErrAuthentication}
	router := accountRouter(t, service, true)
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/sessions", strings.NewReader(`{"email":"person@example.test","password":"wrong-password"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatal("failed login unexpectedly changed the existing browser session")
	}
}

func TestRegistrationRateLimitStopsBeforeAccountService(t *testing.T) {
	service := &accountServiceStub{user: fixtureUser(), token: strings.Repeat("a", 43)}
	router := accountRouterWithLimiter(t, service, false, &authRateLimiterStub{})
	body := `{"email":"person@example.test","display_name":"示例用户","password":"correct-password"}`
	for attempt := 1; attempt <= 6; attempt++ {
		response := httptest.NewRecorder()
		request := authRequest(http.MethodPost, "/v1/auth/registrations", body, "192.0.2.10:4321")
		router.ServeHTTP(response, request)
		if attempt <= 5 && response.Code != http.StatusCreated {
			t.Fatalf("attempt %d status=%d body=%s", attempt, response.Code, response.Body.String())
		}
		if attempt == 6 {
			if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "3600" || !strings.Contains(response.Body.String(), `"code":"RATE_LIMITED"`) || !strings.Contains(response.Body.String(), `"retryable":true`) {
				t.Fatalf("rate limit response differs: status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
			}
			document, _, err := GeneratedOpenAPI(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			spec, err := openapi3.NewLoader().LoadFromData(document)
			if err != nil {
				t.Fatal(err)
			}
			contract, err := gorillamux.NewRouter(spec)
			if err != nil {
				t.Fatal(err)
			}
			route, params, err := contract.FindRoute(request)
			if err != nil {
				t.Fatal(err)
			}
			input := &openapi3filter.RequestValidationInput{Request: request, Route: route, PathParams: params}
			output := &openapi3filter.ResponseValidationInput{RequestValidationInput: input, Status: response.Code, Header: response.Header()}
			if err := openapi3filter.ValidateResponse(request.Context(), output.SetBodyBytes(response.Body.Bytes())); err != nil {
				t.Fatal(err)
			}
		}
	}
	if service.registers != 5 {
		t.Fatalf("registration service calls=%d expected=5", service.registers)
	}
}

func TestLoginRateLimitIsolatedByDirectSourceIP(t *testing.T) {
	service := &accountServiceStub{user: fixtureUser(), token: strings.Repeat("a", 43)}
	router := accountRouterWithLimiter(t, service, false, &authRateLimiterStub{})
	body := `{"email":"person@example.test","password":"correct-password"}`
	for attempt := 1; attempt <= 11; attempt++ {
		response := httptest.NewRecorder()
		request := authRequest(http.MethodPost, "/v1/auth/sessions", body, "192.0.2.20:4321")
		request.Header.Set("X-Forwarded-For", "198.51.100.77")
		router.ServeHTTP(response, request)
		expected := http.StatusOK
		if attempt == 11 {
			expected = http.StatusTooManyRequests
		}
		if response.Code != expected {
			t.Fatalf("attempt %d status=%d expected=%d body=%s", attempt, response.Code, expected, response.Body.String())
		}
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, authRequest(http.MethodPost, "/v1/auth/sessions", body, "198.51.100.77:4321"))
	if response.Code != http.StatusOK {
		t.Fatalf("different direct IP shared a rate-limit bucket: status=%d body=%s", response.Code, response.Body.String())
	}
	if service.logins != 11 {
		t.Fatalf("login service calls=%d expected=11", service.logins)
	}
}

func TestAuthenticationRateLimiterFailureStopsBeforeAccountService(t *testing.T) {
	service := &accountServiceStub{user: fixtureUser(), token: strings.Repeat("a", 43)}
	router := accountRouterWithLimiter(t, service, false, &authRateLimiterStub{err: errors.New("synthetic-secret")})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, authRequest(http.MethodPost, "/v1/auth/sessions", `{"email":"person@example.test","password":"correct-password"}`, "192.0.2.30:4321"))
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "1" || !strings.Contains(response.Body.String(), `"code":"NOT_READY"`) || service.logins != 0 {
		t.Fatalf("limiter failure did not fail closed: status=%d headers=%v body=%s calls=%d", response.Code, response.Header(), response.Body.String(), service.logins)
	}
}

func TestAuthenticationWithInvalidDirectSourceFailsClosed(t *testing.T) {
	service := &accountServiceStub{user: fixtureUser(), token: strings.Repeat("a", 43)}
	router := accountRouterWithLimiter(t, service, false, &authRateLimiterStub{})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, authRequest(http.MethodPost, "/v1/auth/sessions", `{"email":"person@example.test","password":"correct-password"}`, "not-an-address"))
	if response.Code != http.StatusServiceUnavailable || service.logins != 0 {
		t.Fatalf("invalid direct source did not fail closed: status=%d calls=%d", response.Code, service.logins)
	}
}

func TestLogoutClearsOnlyCurrentCookie(t *testing.T) {
	service := &accountServiceStub{user: fixtureUser()}
	router := accountRouter(t, service, false)
	request := httptest.NewRequest(http.MethodDelete, "/v1/auth/session", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || service.token == "" {
		t.Fatal("logout did not revoke the current session")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge != -1 || cookies[0].Value != "" || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("logout did not clear the protected cookie")
	}
}

func TestAccountDeletionRequiresPrivateMediaDeletionFirst(t *testing.T) {
	service := &accountServiceStub{user: fixtureUser(), err: model.ErrAccountMediaConflict}
	router := accountRouter(t, service, false)
	request := httptest.NewRequest(http.MethodDelete, "/v1/users/me", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"CONFLICT"`) {
		t.Fatalf("active private media did not block account deletion: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAccountSuccessResponsesMatchOpenAPI(t *testing.T) {
	document, _, err := GeneratedOpenAPI(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	spec, err := openapi3.NewLoader().LoadFromData(document)
	if err != nil {
		t.Fatal(err)
	}
	contract, err := gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 43)
	tests := []struct {
		name, method, path, body string
		status                   int
		auth                     bool
	}{
		{"register", http.MethodPost, "/v1/auth/registrations", `{"email":"person@example.test","display_name":"示例用户","password":"correct-password"}`, http.StatusCreated, false},
		{"login", http.MethodPost, "/v1/auth/sessions", `{"email":"person@example.test","password":"correct-password"}`, http.StatusOK, false},
		{"current", http.MethodGet, "/v1/users/me", "", http.StatusOK, true},
		{"update", http.MethodPatch, "/v1/users/me", `{"display_name":"新名称"}`, http.StatusOK, true},
		{"logout", http.MethodDelete, "/v1/auth/session", "", http.StatusNoContent, true},
		{"delete", http.MethodDelete, "/v1/users/me", "", http.StatusNoContent, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &accountServiceStub{user: fixtureUser(), token: token}
			router := accountRouter(t, service, true)
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			if test.auth {
				request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			route, params, err := contract.FindRoute(request)
			if err != nil {
				t.Fatal(err)
			}
			input := &openapi3filter.RequestValidationInput{Request: request, Route: route, PathParams: params}
			output := &openapi3filter.ResponseValidationInput{
				RequestValidationInput: input,
				Status:                 response.Code,
				Header:                 response.Header(),
			}
			if err := openapi3filter.ValidateResponse(request.Context(), output.SetBodyBytes(response.Body.Bytes())); err != nil {
				t.Fatal(err)
			}
		})
	}
}
