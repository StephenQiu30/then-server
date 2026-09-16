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

type wearEventTransportStub struct {
	token    string
	eventID  string
	input    model.WearEventInput
	expected int
	err      error
}

func (s *wearEventTransportStub) CreateWearEvent(_ context.Context, token, eventID string, input model.WearEventInput) (model.WearEvent, error) {
	s.token, s.eventID, s.input = token, eventID, input
	return wearEventFixture(), s.err
}
func (s *wearEventTransportStub) ListWearEvents(_ context.Context, token string, _ int, _, _ *string) (model.WearEventPage, error) {
	s.token = token
	return model.WearEventPage{Events: []model.WearEvent{wearEventFixture()}}, s.err
}
func (s *wearEventTransportStub) GetWearEvent(_ context.Context, token, eventID string) (model.WearEvent, error) {
	s.token, s.eventID = token, eventID
	return wearEventFixture(), s.err
}
func (s *wearEventTransportStub) UpdateWearEvent(_ context.Context, token, eventID string, expected int, input model.WearEventInput) (model.WearEvent, error) {
	s.token, s.eventID, s.expected, s.input = token, eventID, expected, input
	event := wearEventFixture()
	event.Revision = expected + 1
	return event, s.err
}
func (s *wearEventTransportStub) DeleteWearEvent(_ context.Context, token, eventID string, expected int) error {
	s.token, s.eventID, s.expected = token, eventID, expected
	return s.err
}

func wearEventFixture() model.WearEvent {
	now := time.Date(2026, 9, 16, 3, 0, 0, 0, time.UTC)
	return model.WearEvent{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400e11", LocalDate: "2026-09-16", TimeZone: "Asia/Shanghai", Completeness: model.WearEventComplete, SourceKind: model.WearEventUnplanned, Revision: 1, CreatedAt: now, UpdatedAt: now, Items: []model.OutfitPlanItemSnapshot{{Ordinal: 0, Content: &model.OutfitItemContent{ItemID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", ItemRevision: 2, Name: "Blue Shirt", Category: model.WardrobeTop, Availability: model.WardrobeWearable}}}}
}

func wearEventRouter(t *testing.T, service WearEventHTTPService) *Router {
	t.Helper()
	router, err := NewRouter(context.Background(), false, probeFunc(func(context.Context) error { return nil }), nil, nil, nil, nil, NewWearEventHandler(service, true), time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func TestWearEventHTTPContractRejectsInjectedFacts(t *testing.T) {
	service := new(wearEventTransportStub)
	router := wearEventRouter(t, service)
	valid := `{"id":"018f1f74-a2d0-7c6d-9c17-4a0ea2400e11","local_date":"2026-09-16","time_zone":"Asia/Shanghai","completeness":"complete","context_summary":null,"items":[{"item_id":"018f1f74-a2d0-7c6d-9c17-4a0ea2400a12","revision":2}],"laundry_item_ids":[],"confirmed_unavailable_ids":[],"source_plan_id":null,"source_plan_revision":null,"source_kind":"unplanned","duplicate_confirmations":[]}`
	request := httptest.NewRequest(http.MethodPost, "/wear-events", strings.NewReader(valid))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.token == "" || !strings.Contains(response.Body.String(), `"name":"Blue Shirt"`) {
		t.Fatalf("valid wear event create failed: status=%d body=%s", response.Code, response.Body.String())
	}
	for name, body := range map[string]string{
		"owner":    strings.TrimSuffix(valid, "}") + `,"owner_id":"other"}`,
		"snapshot": strings.Replace(valid, `"revision":2`, `"revision":2,"name":"Injected"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/wear-events", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("injected wear event fact was accepted: status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestWearEventHTTPCommandsAndDuplicateCandidates(t *testing.T) {
	service := new(wearEventTransportStub)
	router := wearEventRouter(t, service)
	eventID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400e11"
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/wear-events?limit=20&local_date=2026-09-16", nil),
		httptest.NewRequest(http.MethodGet, "/wear-events/"+eventID, nil),
		httptest.NewRequest(http.MethodPut, "/wear-events/"+eventID, strings.NewReader(`{"expected_revision":1,"local_date":"2026-09-16","time_zone":"Asia/Shanghai","completeness":"partial","context_summary":null,"items":[{"item_id":"018f1f74-a2d0-7c6d-9c17-4a0ea2400a12","revision":2}],"laundry_item_ids":[],"confirmed_unavailable_ids":[],"source_plan_id":null,"source_plan_revision":null,"source_kind":"unplanned","duplicate_confirmations":[]}`)),
		httptest.NewRequest(http.MethodDelete, "/wear-events/"+eventID+"?expected_revision=2", nil),
	} {
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
		if request.Body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		expected := http.StatusOK
		if request.Method == http.MethodDelete {
			expected = http.StatusNoContent
		}
		if response.Code != expected {
			t.Fatalf("%s %s status=%d body=%s", request.Method, request.URL.Path, response.Code, response.Body.String())
		}
	}
	candidate := model.WearEventCandidate{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400e22", Revision: 4}
	service.err = &model.WearEventDuplicateError{Candidates: []model.WearEventCandidate{candidate}}
	request := httptest.NewRequest(http.MethodGet, "/wear-events/"+eventID, nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), candidate.ID) || !strings.Contains(response.Body.String(), `"revision":4`) {
		t.Fatalf("duplicate candidates were not returned: status=%d body=%s", response.Code, response.Body.String())
	}
}
