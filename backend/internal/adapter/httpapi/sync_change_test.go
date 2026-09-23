package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	syncapp "github.com/StephenQiu30/then-server/backend/internal/application/syncchange"
)

type syncHTTPStub struct {
	calls int
	after string
	limit int
	err   error
}

func (s *syncHTTPStub) List(_ context.Context, _, after string, limit int) (syncapp.Page, error) {
	s.calls++
	s.after, s.limit = after, limit
	return syncapp.Page{Changes: []syncapp.Change{{Seq: 1, Kind: "wardrobe_item", EntityID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400b01", Action: "upsert"}}, NextCursor: "cursor", HasMore: true}, s.err
}

func TestSyncChangesHTTPContract(t *testing.T) {
	service := &syncHTTPStub{}
	router, err := NewRouterWithSync(context.Background(), false, probeFunc(func(context.Context) error { return nil }), nil, nil, nil, nil, nil, nil, nil, nil, nil, NewSyncHandler(service, true), time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/sync/changes?after=prior&limit=1", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "synthetic-session"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || service.calls != 1 || service.after != "prior" || service.limit != 1 {
		t.Fatalf("sync read failed: status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Changes []map[string]json.RawMessage `json:"changes"`
		Next    string                       `json:"next_cursor"`
		More    bool                         `json:"has_more"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Changes) != 1 || body.Next != "cursor" || !body.More || len(body.Changes[0]) != 5 || body.Changes[0]["revision"] == nil {
		t.Fatalf("sync response leaked or omitted fields: %s", response.Body.String())
	}
	for _, path := range []string{"/sync/changes", "/sync/changes?limit=0", "/sync/changes?limit=101", "/sync/changes?unexpected=1"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if path != "/sync/changes" {
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "synthetic-session"})
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if path == "/sync/changes" && response.Code != http.StatusUnauthorized || path != "/sync/changes" && response.Code != http.StatusBadRequest {
			t.Fatalf("invalid sync request %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
	service.err = syncapp.ErrInvalidCursor
	request = httptest.NewRequest(http.MethodGet, "/sync/changes?after=invalid", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "synthetic-session"})
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), "cursor") {
		t.Fatalf("invalid cursor was exposed: status=%d body=%s", response.Code, response.Body.String())
	}
}
