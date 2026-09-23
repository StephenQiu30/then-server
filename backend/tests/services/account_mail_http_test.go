//go:build services

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/adapter/httpapi"
	store "github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	"github.com/StephenQiu30/then-server/backend/internal/platform/ratelimit"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type accountMailHTTPProbe struct{}

func (accountMailHTTPProbe) Probe(context.Context) error { return nil }

func TestAccountMailHTTPWithPostgresAndRedis(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	base, err := gorm.Open(postgres.Open(environment.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open mail HTTP database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create mail HTTP schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := base.WithContext(cleanup).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("mail HTTP schema cleanup failed")
		}
	})
	databaseURL, err := url.Parse(environment.databaseURL)
	serviceOK(t, "parse mail HTTP database", err)
	query := databaseURL.Query()
	query.Set("search_path", schema)
	databaseURL.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open isolated mail HTTP schema", err)
	serviceOK(t, "migrate mail HTTP schema", store.Migrate(ctx, database))
	repository := store.NewAccountRepository(database)
	accounts, err := accountapp.NewAccountService(repository)
	serviceOK(t, "construct mail HTTP account service", err)
	sender := &accountMailSender{}
	mail, err := accountapp.NewMailService(accounts, repository, sender, []byte(strings.Repeat("k", 32)), "https://then.example/account/mail")
	serviceOK(t, "construct mail HTTP service", err)
	redisAddress := url.URL{Scheme: "redis", Host: environment.redisAddr, Path: "/" + strconv.Itoa(environment.redisDB)}
	if environment.redisPassword != "" {
		redisAddress.User = url.UserPassword("", environment.redisPassword)
	}
	limiter, err := ratelimit.Open(ctx, redisAddress.String())
	serviceOK(t, "open mail HTTP Redis limiter", err)
	defer limiter.Close()
	router, err := httpapi.NewRouter(ctx, false, accountMailHTTPProbe{}, httpapi.NewAccountHandlerWithMail(accounts, mail, true, limiter), nil, nil, nil, nil, time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	serviceOK(t, "construct mail HTTP router", err)
	addressBits, err := strconv.ParseUint(schema[len(schema)-6:], 16, 32)
	serviceOK(t, "derive isolated client address", err)
	remoteAddress := fmt.Sprintf("127.1.%d.%d:2000", byte(addressBits>>8), byte(addressBits))
	email := "flow-" + schema[len(schema)-12:] + "@example.test"
	call := func(path, body string, cookie *http.Cookie, expected int) *http.Response {
		t.Helper()
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, path, strings.NewReader(body))
		serviceOK(t, "build mail HTTP request", err)
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = remoteAddress
		if cookie != nil {
			request.AddCookie(cookie)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		response := recorder.Result()
		if response.StatusCode != expected {
			response.Body.Close()
			t.Fatalf("%s returned status %d, expected %d", path, response.StatusCode, expected)
		}
		return response
	}
	registration := call("/auth/registrations", `{"email":"`+email+`","display_name":"Flow","password":"initial-password-2026"}`, nil, http.StatusCreated)
	cookies := registration.Cookies()
	registration.Body.Close()
	if len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatal("registration did not create a protected session")
	}
	cookie := cookies[0]
	requested := call("/auth/email-verifications", "", cookie, http.StatusAccepted)
	requested.Body.Close()
	serviceOK(t, "deliver HTTP-requested verification", mail.DeliverOnce(ctx))
	if len(sender.messages) != 1 {
		t.Fatal("HTTP verification request did not persist mail")
	}
	verificationToken := accountMailToken(t, sender.messages[0])
	confirmed := call("/auth/email-verifications/confirm", `{"token":"`+verificationToken+`"}`, cookie, http.StatusNoContent)
	confirmed.Body.Close()
	get, err := http.NewRequestWithContext(ctx, http.MethodGet, "/users/me", nil)
	serviceOK(t, "build verified user request", err)
	get.AddCookie(cookie)
	get.RemoteAddr = remoteAddress
	currentRecorder := httptest.NewRecorder()
	router.ServeHTTP(currentRecorder, get)
	current := currentRecorder.Result()
	var user struct {
		EmailVerified bool `json:"email_verified"`
	}
	serviceOK(t, "decode verified user", json.NewDecoder(io.LimitReader(current.Body, 8192)).Decode(&user))
	current.Body.Close()
	if !user.EmailVerified {
		t.Fatal("HTTP user response did not expose the verified state")
	}
	for _, requestedEmail := range []string{"missing-" + schema[len(schema)-12:] + "@example.test", email} {
		response := call("/auth/password-resets", `{"email":"`+requestedEmail+`"}`, nil, http.StatusAccepted)
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 8192))
		serviceOK(t, "read generic reset response", readErr)
		if len(body) != 0 {
			t.Fatal("password reset request exposed account existence")
		}
		response.Body.Close()
	}
	serviceOK(t, "deliver HTTP-requested reset", mail.DeliverOnce(ctx))
	if len(sender.messages) != 2 || sender.messages[1].to != email {
		t.Fatal("unknown account produced mail or known account did not")
	}
	resetToken := accountMailToken(t, sender.messages[1])
	reset := call("/auth/password-resets/confirm", `{"token":"`+resetToken+`","new_password":"replacement-password-2026"}`, nil, http.StatusNoContent)
	reset.Body.Close()
	get, err = http.NewRequestWithContext(ctx, http.MethodGet, "/users/me", nil)
	serviceOK(t, "build revoked session request", err)
	get.AddCookie(cookie)
	get.RemoteAddr = remoteAddress
	staleRecorder := httptest.NewRecorder()
	router.ServeHTTP(staleRecorder, get)
	stale := staleRecorder.Result()
	stale.Body.Close()
	if stale.StatusCode != http.StatusUnauthorized {
		t.Fatal("password reset did not revoke the old HTTP session")
	}
}
