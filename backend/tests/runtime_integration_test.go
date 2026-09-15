//go:build integration

package tests

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/platform/config"
	"github.com/StephenQiu30/then-server/backend/internal/platform/database"
	"github.com/StephenQiu30/then-server/backend/internal/transport"
	dockerclient "github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresDisconnectRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	password := rand.Text()
	container, err := createTestContainer(t, ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:18.4@sha256:a02db8cac496f15b094798a38254f14d6e00741f709360e5e00bb6668ea31636",
			Env:          map[string]string{"POSTGRES_USER": "then_test", "POSTGRES_DB": "then_test", "POSTGRES_PASSWORD": password},
			ExposedPorts: []string{"5432/tcp"},
			WaitingFor:   wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(time.Minute),
		}, Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}
	u := url.URL{Scheme: "postgres", User: url.UserPassword("then_test", password), Host: net.JoinHostPort(host, port.Port()), Path: "/then_test", RawQuery: "sslmode=disable"}
	redisContainer, err := createTestContainer(t, ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:8.10.0@sha256:344e3945a0b431c8ff1eecd58c5573538126bd756f02fc7e218ddf1fc2546366",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(time.Minute),
		}, Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	redisHost, err := redisContainer.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	redisPort, err := redisContainer.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatal(err)
	}
	redisURL := "redis://" + net.JoinHostPort(redisHost, redisPort.Port()) + "/0"
	cfg, err := config.Load(func(key string) (string, bool) {
		if key == "DATABASE_URL" {
			return u.String(), true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	admin, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	migrationGuardID := strings.ToLower(rand.Text()[:12])
	migrationGuardSchema := "migration_guard_" + migrationGuardID
	migrationGuardRole := "migration_guard_" + migrationGuardID
	migrationGuardPassword := rand.Text()
	if err := admin.ORM().WithContext(ctx).Exec("CREATE SCHEMA " + migrationGuardSchema).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.ORM().WithContext(ctx).Exec("CREATE ROLE " + migrationGuardRole + " LOGIN PASSWORD '" + migrationGuardPassword + "'").Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := admin.ORM().WithContext(cleanup).Exec("DROP SCHEMA " + migrationGuardSchema + " CASCADE").Error; err != nil {
			t.Error("migration guard schema cleanup failed")
		}
		if err := admin.ORM().WithContext(cleanup).Exec("DROP ROLE " + migrationGuardRole).Error; err != nil {
			t.Error("migration guard role cleanup failed")
		}
	}()
	if err := admin.ORM().WithContext(ctx).Exec("GRANT USAGE ON SCHEMA " + migrationGuardSchema + " TO " + migrationGuardRole).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.ORM().WithContext(ctx).Exec("ALTER ROLE " + migrationGuardRole + " SET search_path TO " + migrationGuardSchema).Error; err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "then-backend")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	wrongPassword := u
	wrongPassword.User = url.UserPassword("then_test", "invalid-"+password)
	readOnlySchema := u
	readOnlySchema.User = url.UserPassword(migrationGuardRole, migrationGuardPassword)
	for _, failure := range []struct {
		name   string
		env    []string
		reason string
	}{
		{"missing database", []string{"APP_ROLE=api"}, "DATABASE_URL"},
		{"worker without media config", []string{"APP_ROLE=worker"}, "MEDIA_DEVELOPMENT_ENABLED"},
		{"all without media config", []string{"APP_ROLE=all"}, "MEDIA_DEVELOPMENT_ENABLED"},
		{"invalid credentials", []string{"DATABASE_URL=" + wrongPassword.String()}, "database unavailable"},
		{"schema migration", []string{"DATABASE_URL=" + readOnlySchema.String()}, "database schema migration failed"},
		{"redis unavailable", []string{"DATABASE_URL=" + u.String(), "REDIS_URL=redis://127.0.0.1:1/0"}, "rate limiter unavailable"},
		{"occupied listener", []string{"DATABASE_URL=" + u.String(), "HTTP_ADDR=" + occupied.Addr().String()}, "HTTP listen failed"},
	} {
		t.Run(failure.name, func(t *testing.T) {
			attempt, stop := context.WithTimeout(ctx, 10*time.Second)
			defer stop()
			command := exec.CommandContext(attempt, binary)
			command.Env = append([]string{}, failure.env...)
			hasRedisURL := false
			for _, variable := range command.Env {
				hasRedisURL = hasRedisURL || strings.HasPrefix(variable, "REDIS_URL=")
			}
			if !hasRedisURL {
				command.Env = append(command.Env, "REDIS_URL="+redisURL)
			}
			output, err := command.CombinedOutput()
			if attempt.Err() != nil {
				t.Fatal("startup failure did not exit within deadline")
			}
			if err == nil {
				t.Fatal("invalid startup returned success")
			}
			if strings.Contains(string(output), password) || strings.Contains(string(output), migrationGuardPassword) || strings.Contains(string(output), "postgres://") {
				t.Fatal("startup log exposed database credentials")
			}
			if strings.Contains(string(output), "api_started") {
				t.Fatal("failed startup reported started")
			}
			if !strings.Contains(string(output), failure.reason) {
				t.Fatalf("startup did not return expected safe reason: %s", output)
			}
		})
	}
	process := exec.CommandContext(ctx, binary)
	process.Env = []string{"DATABASE_URL=" + u.String(), "REDIS_URL=" + redisURL, "HTTP_ADDR=127.0.0.1:0", "APP_ROLE=api", "API_DOCS_ENABLED=true"}
	stdout, err := process.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	defer process.Process.Kill()
	started := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			var entry struct {
				Message string `json:"msg"`
				Address string `json:"address"`
			}
			if json.Unmarshal(scanner.Bytes(), &entry) == nil && entry.Message == "api_started" {
				started <- entry.Address
			}
		}
		close(started)
	}()
	select {
	case address := <-started:
		if address == "" {
			t.Fatal("process did not start")
		}
		client := &http.Client{Timeout: 2 * time.Second}
		for _, path := range []string{"/docs/", "/docs/swagger-ui-bundle.js", "/openapi.yaml", "/openapi.json", "/v1/health/ready"} {
			response, err := client.Get("http://" + address + path)
			if err != nil {
				t.Fatal(err)
			}
			body, readErr := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
			response.Body.Close()
			if readErr != nil || response.StatusCode != 200 || len(body) == 0 {
				t.Fatalf("built process route %s did not serve successfully", path)
			}
			if path == "/openapi.yaml" && !strings.Contains(string(body), "openapi: 3.1.2") {
				t.Fatal("binary did not serve its runtime-generated YAML contract")
			}
			if path == "/openapi.json" {
				var contract struct {
					OpenAPI string `json:"openapi"`
					Info    struct {
						Version string `json:"version"`
					} `json:"info"`
				}
				if json.Unmarshal(body, &contract) != nil || contract.OpenAPI != "3.1.2" || contract.Info.Version != "0.8.0" || !strings.Contains(string(body), `"operationId":"createWardrobeItem"`) {
					t.Fatal("binary did not serve a valid JSON representation of its compiled contract")
				}
			}
		}
		exerciseAccountHTTPLifecycle(t, ctx, client, "http://"+address, admin)
		docker, err := testcontainers.NewDockerClient()
		if err != nil {
			t.Fatal(err)
		}
		defer docker.Close()
		if _, err := docker.ContainerPause(ctx, redisContainer.GetContainerID(), dockerclient.ContainerPauseOptions{}); err != nil {
			t.Fatal(err)
		}
		redisPaused := true
		defer func() {
			if !redisPaused {
				return
			}
			cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
			defer done()
			_, _ = docker.ContainerUnpause(cleanup, redisContainer.GetContainerID(), dockerclient.ContainerUnpauseOptions{})
		}()
		status := func(path string) int {
			t.Helper()
			response, err := client.Get("http://" + address + path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			return response.StatusCode
		}
		if got := status("/v1/health/ready"); got != http.StatusServiceUnavailable {
			t.Fatalf("Redis-disconnected readiness %d", got)
		}
		if got := status("/v1/health/live"); got != http.StatusOK {
			t.Fatalf("Redis-disconnected liveness %d", got)
		}
		if _, err := docker.ContainerUnpause(ctx, redisContainer.GetContainerID(), dockerclient.ContainerUnpauseOptions{}); err != nil {
			t.Fatal(err)
		}
		redisPaused = false
		deadline := time.Now().Add(30 * time.Second)
		for status("/v1/health/ready") != http.StatusOK {
			if time.Now().After(deadline) {
				t.Fatal("readiness did not recover after Redis resumed")
			}
			time.Sleep(100 * time.Millisecond)
		}
	case <-ctx.Done():
		t.Fatal("startup timeout")
	}
	if err := process.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan error, 1)
	go func() { stopped <- process.Wait() }()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("SIGTERM did not exit")
	}
	t.Log("built API process starts with PostgreSQL 18 and exits successfully on SIGTERM")
	pool, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	router, err := transport.NewRouter(ctx, false, pool, nil, nil, nil, time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	defer server.Close()
	client := &http.Client{Timeout: 2 * time.Second}
	status := func(path string) int {
		t.Helper()
		response, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		return response.StatusCode
	}
	if got := status("/v1/health/ready"); got != 200 {
		t.Fatalf("initial readiness %d", got)
	}
	docker, err := testcontainers.NewDockerClient()
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Close()
	if _, err := docker.ContainerPause(ctx, container.GetContainerID(), dockerclient.ContainerPauseOptions{}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = docker.ContainerUnpause(cleanup, container.GetContainerID(), dockerclient.ContainerUnpauseOptions{})
	}()
	if got := status("/v1/health/ready"); got != 503 {
		t.Fatalf("disconnected readiness %d", got)
	}
	if got := status("/v1/health/live"); got != 200 {
		t.Fatalf("disconnected liveness %d", got)
	}
	if _, err := docker.ContainerUnpause(ctx, container.GetContainerID(), dockerclient.ContainerUnpauseOptions{}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for status("/v1/health/ready") != 200 {
		if time.Now().After(deadline) {
			t.Fatal("readiness did not recover")
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Log("PostgreSQL 18: ready 200 -> disconnected 503; live 200; recovered ready 200")
}

func exerciseAccountHTTPLifecycle(t *testing.T, ctx context.Context, client *http.Client, baseURL string, pool *database.Pool) {
	t.Helper()
	email := "http-" + strings.ToLower(rand.Text()[:16]) + "@example.test"
	password := "correct-http-password"
	registration, registrationBody := accountRequest(t, ctx, client, http.MethodPost, baseURL+"/v1/auth/registrations", `{"email":"`+email+`","display_name":"HTTP User","password":"`+password+`"}`, nil, http.StatusCreated)
	registeredUserID := extractUserID(t, registrationBody)
	registrationCookies := registration.Cookies()
	if len(registrationCookies) != 1 || registrationCookies[0].Name != "then_session" || registrationCookies[0].Value == "" || registrationCookies[0].Path != "/v1" || !registrationCookies[0].HttpOnly || registrationCookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("actual registration response did not set the protected session cookie")
	}
	firstSession := registrationCookies[0]
	accountRequest(t, ctx, client, http.MethodGet, baseURL+"/v1/users/me", "", firstSession, http.StatusOK)
	_, body := accountRequest(t, ctx, client, http.MethodGet, baseURL+"/v1/privacy/self-adult-declaration", "", firstSession, http.StatusOK)
	if !strings.Contains(string(body), `"policy_version":"self-adult-v1"`) || !strings.Contains(string(body), `"confirmed":false`) {
		t.Fatal("actual API process did not expose the initial declaration state")
	}
	accountRequest(t, ctx, client, http.MethodPut, baseURL+"/v1/privacy/self-adult-declaration", `{"policy_version":"self-adult-v1","confirms_self_and_adult":false}`, firstSession, http.StatusBadRequest)
	_, body = accountRequest(t, ctx, client, http.MethodPut, baseURL+"/v1/privacy/self-adult-declaration", `{"policy_version":"self-adult-v1","confirms_self_and_adult":true}`, firstSession, http.StatusOK)
	if !strings.Contains(string(body), `"confirmed":true`) || !strings.Contains(string(body), `"confirmed_at":`) {
		t.Fatal("actual API process did not persist the declaration")
	}
	_, body = accountRequest(t, ctx, client, http.MethodDelete, baseURL+"/v1/privacy/self-adult-declaration", "", firstSession, http.StatusOK)
	if !strings.Contains(string(body), `"confirmed":false`) || !strings.Contains(string(body), `"withdrawn_at":`) {
		t.Fatal("actual API process did not withdraw the declaration")
	}
	accountRequest(t, ctx, client, http.MethodPut, baseURL+"/v1/privacy/self-adult-declaration", `{"policy_version":"self-adult-v1","confirms_self_and_adult":true}`, firstSession, http.StatusOK)
	exerciseWardrobeHTTPLifecycle(t, ctx, client, baseURL, firstSession)

	logout, _ := accountRequest(t, ctx, client, http.MethodDelete, baseURL+"/v1/auth/session", "", firstSession, http.StatusNoContent)
	assertExpiredSessionCookie(t, logout)
	privacyReplay, body := accountRequest(t, ctx, client, http.MethodGet, baseURL+"/v1/privacy/self-adult-declaration", "", firstSession, http.StatusUnauthorized)
	assertAuthenticationFailure(t, body)
	assertExpiredSessionCookie(t, privacyReplay)
	replay, body := accountRequest(t, ctx, client, http.MethodGet, baseURL+"/v1/users/me", "", firstSession, http.StatusUnauthorized)
	assertAuthenticationFailure(t, body)
	assertExpiredSessionCookie(t, replay)

	login, _ := accountRequest(t, ctx, client, http.MethodPost, baseURL+"/v1/auth/sessions", `{"email":"`+email+`","password":"`+password+`"}`, nil, http.StatusOK)
	loginCookies := login.Cookies()
	if len(loginCookies) != 1 || loginCookies[0].Name != "then_session" || loginCookies[0].Value == "" {
		t.Fatal("actual login response did not establish a session")
	}
	secondSession := loginCookies[0]
	deletion, _ := accountRequest(t, ctx, client, http.MethodDelete, baseURL+"/v1/users/me", "", secondSession, http.StatusNoContent)
	assertExpiredSessionCookie(t, deletion)
	deletedReplay, body := accountRequest(t, ctx, client, http.MethodGet, baseURL+"/v1/users/me", "", secondSession, http.StatusUnauthorized)
	assertAuthenticationFailure(t, body)
	assertExpiredSessionCookie(t, deletedReplay)
	failedLogin, body := accountRequest(t, ctx, client, http.MethodPost, baseURL+"/v1/auth/sessions", `{"email":"`+email+`","password":"`+password+`"}`, nil, http.StatusUnauthorized)
	assertAuthenticationFailure(t, body)
	if len(failedLogin.Cookies()) != 0 {
		t.Fatal("invalid credentials unexpectedly changed browser cookies")
	}
	for range 8 {
		accountRequest(t, ctx, client, http.MethodPost, baseURL+"/v1/auth/sessions", `{"email":"`+email+`","password":"`+password+`"}`, nil, http.StatusUnauthorized)
	}
	limited, body := accountRequest(t, ctx, client, http.MethodPost, baseURL+"/v1/auth/sessions", `{"email":"`+email+`","password":"`+password+`"}`, nil, http.StatusTooManyRequests)
	if limited.Header.Get("Retry-After") == "" || !strings.Contains(string(body), `"code":"RATE_LIMITED"`) {
		t.Fatal("actual API process did not expose the authentication rate-limit recovery contract")
	}

	var remaining int64
	if err := pool.ORM().WithContext(ctx).Raw("SELECT count(*) FROM user_credentials WHERE email = ?", email).Scan(&remaining).Error; err != nil || remaining != 0 {
		t.Fatal("deleted HTTP account still has persisted credentials")
	}
	if err := pool.ORM().WithContext(ctx).Raw("SELECT count(*) FROM self_adult_declarations WHERE user_id = ?", registeredUserID).Scan(&remaining).Error; err != nil || remaining != 0 {
		t.Fatal("deleted HTTP account still has a declaration")
	}
	t.Log("actual API process: account, declaration, wardrobe CRUD, session revocation, cascade deletion and login rate-limit 429 verified")
}

func exerciseWardrobeHTTPLifecycle(t *testing.T, ctx context.Context, client *http.Client, baseURL string, session *http.Cookie) {
	t.Helper()
	const itemID = "018f1f74-a2d0-7c6d-9c17-4a0ea2400c11"
	createBody := `{"id":"` + itemID + `","name":"HTTP Shirt","category":"top","availability":"wearable","source":"quick_add"}`
	_, body := accountRequest(t, ctx, client, http.MethodPost, baseURL+"/v1/wardrobe/items", createBody, session, http.StatusCreated)
	if !strings.Contains(string(body), `"revision":1`) || strings.Contains(string(body), "owner_id") {
		t.Fatal("actual wardrobe create lost revision or exposed owner")
	}
	accountRequest(t, ctx, client, http.MethodPost, baseURL+"/v1/wardrobe/items", createBody, session, http.StatusCreated)
	_, body = accountRequest(t, ctx, client, http.MethodGet, baseURL+"/v1/wardrobe/items?limit=1", "", session, http.StatusOK)
	if !strings.Contains(string(body), itemID) {
		t.Fatal("actual wardrobe list omitted the created item")
	}
	updateBody := `{"expected_revision":1,"name":"HTTP Blue Shirt","category":"top","availability":"laundry"}`
	_, body = accountRequest(t, ctx, client, http.MethodPut, baseURL+"/v1/wardrobe/items/"+itemID, updateBody, session, http.StatusOK)
	if !strings.Contains(string(body), `"revision":2`) || !strings.Contains(string(body), `"source":"quick_add"`) {
		t.Fatal("actual wardrobe update lost revision or immutable source")
	}
	accountRequest(t, ctx, client, http.MethodPut, baseURL+"/v1/wardrobe/items/"+itemID, updateBody, session, http.StatusConflict)
	accountRequest(t, ctx, client, http.MethodDelete, baseURL+"/v1/wardrobe/items/"+itemID+"?expected_revision=1", "", session, http.StatusConflict)
	accountRequest(t, ctx, client, http.MethodDelete, baseURL+"/v1/wardrobe/items/"+itemID+"?expected_revision=2", "", session, http.StatusNoContent)
	accountRequest(t, ctx, client, http.MethodGet, baseURL+"/v1/wardrobe/items/"+itemID, "", session, http.StatusNotFound)
}

func extractUserID(t *testing.T, data []byte) string {
	t.Helper()
	var payload struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.User.ID == "" {
		t.Fatal("cannot extract registered user ID")
	}
	return payload.User.ID
}

func accountRequest(t *testing.T, ctx context.Context, client *http.Client, method, endpoint, body string, cookie *http.Cookie, expectedStatus int) (*http.Response, []byte) {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatal("cannot build account request")
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal("account request failed")
	}
	payload, readErr := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	response.Body.Close()
	if readErr != nil {
		t.Fatal("account response could not be read")
	}
	if response.StatusCode != expectedStatus {
		t.Fatalf("account response status=%d expected=%d", response.StatusCode, expectedStatus)
	}
	if response.Header.Get("X-Request-ID") == "" {
		t.Fatal("account response omitted request ID")
	}
	return response, payload
}

func assertExpiredSessionCookie(t *testing.T, response *http.Response) {
	t.Helper()
	cookies := response.Cookies()
	if len(cookies) != 1 || cookies[0].Name != "then_session" || cookies[0].Value != "" || cookies[0].MaxAge != -1 || cookies[0].Path != "/v1" || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("account response did not clear the session cookie")
	}
}

func assertAuthenticationFailure(t *testing.T, body []byte) {
	t.Helper()
	var failure struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(body, &failure) != nil || failure.Code != "AUTHENTICATION_FAILED" {
		t.Fatal("account response did not use the safe authentication error")
	}
}
