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

	exportapp "github.com/StephenQiu30/then-server/backend/internal/application/dataexport"
)

type dataExportStub struct {
	calls int
	mode  string
}

func (s *dataExportStub) Create(_ context.Context, _, _, mode string) (exportapp.Job, error) {
	s.calls++
	s.mode = mode
	return exportapp.Job{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11", Mode: mode, Status: exportapp.StatusPreparing, Counts: map[string]int{}, Omissions: []string{}, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour)}, nil
}

func TestDataExportHTTPAcceptsMediaMode(t *testing.T) {
	service := &dataExportStub{}
	router, err := NewRouterWithExport(context.Background(), false, probeFunc(func(context.Context) error { return nil }), nil, nil, nil, nil, nil, nil, nil, nil, NewDataExportHandler(service, &authRateLimiterStub{}), time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/exports", strings.NewReader(`{"password":"synthetic-password","mode":"with_media"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "synthetic-session"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || service.calls != 1 || service.mode != exportapp.ModeWithMedia || !strings.Contains(response.Body.String(), `"mode":"with_media"`) {
		t.Fatalf("media mode HTTP contract failed: status=%d body=%s", response.Code, response.Body.String())
	}
}
func (s *dataExportStub) Get(context.Context, string, string) (exportapp.Job, error) {
	return exportapp.Job{}, nil
}
func (s *dataExportStub) Open(context.Context, string, string) (io.ReadCloser, error) {
	return nil, exportapp.ErrNotReady
}
func (s *dataExportStub) Revoke(context.Context, string, string) error { return nil }

func TestDataExportLimiterFailsClosed(t *testing.T) {
	service := &dataExportStub{}
	limiter := &authRateLimiterStub{err: errors.New("synthetic redis failure")}
	router, err := NewRouterWithExport(context.Background(), false, probeFunc(func(context.Context) error { return nil }), nil, nil, nil, nil, nil, nil, nil, nil, NewDataExportHandler(service, limiter), time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/exports", strings.NewReader(`{"password":"synthetic-password","mode":"structured"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "synthetic-session"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || service.calls != 0 || strings.Contains(response.Body.String(), "synthetic redis failure") {
		t.Fatal("limiter failure did not stop export before reauthentication")
	}
}
