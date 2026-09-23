package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
)

type accountMailStub struct {
	verificationSession string
	verificationToken   string
	resetEmail          string
	resetToken          string
	resetPassword       string
	err                 error
}

func (s *accountMailStub) RequestVerification(_ context.Context, session string) error {
	s.verificationSession = session
	return s.err
}

func (s *accountMailStub) ConfirmVerification(_ context.Context, session, token string) error {
	s.verificationSession, s.verificationToken = session, token
	return s.err
}

func (s *accountMailStub) RequestPasswordReset(_ context.Context, email string) error {
	s.resetEmail = email
	return s.err
}

func (s *accountMailStub) ConfirmPasswordReset(_ context.Context, token, password string) error {
	s.resetToken, s.resetPassword = token, password
	return s.err
}

func accountMailRouter(t *testing.T, mail AccountMailService) *Router {
	return accountMailRouterWithLimiter(t, mail, &authRateLimiterStub{})
}

func accountMailRouterWithLimiter(t *testing.T, mail AccountMailService, limiter *authRateLimiterStub) *Router {
	t.Helper()
	accounts := &accountServiceStub{user: fixtureUser()}
	router, err := NewRouter(context.Background(), false, probeFunc(func(context.Context) error { return nil }),
		NewAccountHandlerWithMail(accounts, mail, true, limiter), nil, nil, nil, nil, time.Second,
		slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func TestAccountMailRequestRateLimitsAndRedisFailure(t *testing.T) {
	limiter := &authRateLimiterStub{}
	mail := &accountMailStub{}
	router := accountMailRouterWithLimiter(t, mail, limiter)
	for attempt := 1; attempt <= 4; attempt++ {
		request := authRequest(http.MethodPost, "/auth/password-resets", `{"email":"same@example.test"}`, "127.0.0.1:2000")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		expected := http.StatusAccepted
		if attempt == 4 {
			expected = http.StatusTooManyRequests
		}
		if response.Code != expected {
			t.Fatalf("request %d: status=%d body=%s", attempt, response.Code, response.Body.String())
		}
	}
	if limiter.counts["password-reset-email:same@example.test"] != 4 || limiter.counts["password-reset-ip:127.0.0.1"] != 4 {
		t.Fatal("password reset did not apply both email and IP limits")
	}
	limiter.err = errors.New("synthetic Redis outage")
	request := authRequest(http.MethodPost, "/auth/password-resets", `{"email":"other@example.test"}`, "127.0.0.1:2000")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || mail.resetEmail != "same@example.test" {
		t.Fatal("Redis failure did not stop password reset before account access")
	}
}

func TestAccountMailRoutesUseSingleChallengeContract(t *testing.T) {
	mail := &accountMailStub{}
	router := accountMailRouter(t, mail)
	token := strings.Repeat("a", 80)
	for _, step := range []struct {
		path, body string
		status     int
		session    bool
	}{
		{"/auth/email-verifications", "", http.StatusAccepted, true},
		{"/auth/email-verifications/confirm", `{"token":"` + token + `"}`, http.StatusNoContent, true},
		{"/auth/password-resets", `{"email":"unknown@example.test"}`, http.StatusAccepted, false},
		{"/auth/password-resets/confirm", `{"token":"` + token + `","new_password":"new-password-2026"}`, http.StatusNoContent, false},
	} {
		request := authRequest(http.MethodPost, step.path, step.body, "127.0.0.1:2000")
		if step.session {
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("s", 43)})
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != step.status || response.Header().Get("X-Request-ID") == "" || response.Body.Len() != 0 {
			t.Fatalf("%s: status=%d body=%s", step.path, response.Code, response.Body.String())
		}
	}
	if mail.verificationSession != strings.Repeat("s", 43) || mail.verificationToken != token || mail.resetEmail != "unknown@example.test" || mail.resetToken != token || mail.resetPassword != "new-password-2026" {
		t.Fatal("mail routes did not pass the intended session, token, email and password")
	}
}

func TestAccountMailRoutesHideChallengeAndFailClosed(t *testing.T) {
	mail := &accountMailStub{err: accountapp.ErrInvalidChallenge}
	router := accountMailRouter(t, mail)
	request := authRequest(http.MethodPost, "/auth/password-resets/confirm", `{"token":"`+strings.Repeat("a", 80)+`","new_password":"new-password-2026"}`, "127.0.0.1:2000")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "INVALID_CHALLENGE") || strings.Contains(response.Body.String(), strings.Repeat("a", 80)) {
		t.Fatal("invalid challenge response exposed a token or wrong status")
	}
	withoutMail := accountMailRouter(t, nil)
	request = authRequest(http.MethodPost, "/auth/password-resets", `{"email":"person@example.test"}`, "127.0.0.1:2000")
	response = httptest.NewRecorder()
	withoutMail.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatal("unconfigured mail endpoint did not fail closed")
	}
}
