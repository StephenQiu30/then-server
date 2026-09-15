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

type outfitPlanTransportStub struct {
	token    string
	planID   string
	created  model.OutfitPlanInput
	updated  model.OutfitPlanInput
	expected int
	err      error
}

func (s *outfitPlanTransportStub) CreateOutfitPlan(_ context.Context, token, planID string, input model.OutfitPlanInput) (model.OutfitPlan, error) {
	s.token, s.planID, s.created = token, planID, input
	return outfitPlanFixture(), s.err
}
func (s *outfitPlanTransportStub) ListOutfitPlans(_ context.Context, token string, _ int, _, _ *string) (model.OutfitPlanPage, error) {
	s.token = token
	return model.OutfitPlanPage{Plans: []model.OutfitPlan{outfitPlanFixture()}}, s.err
}
func (s *outfitPlanTransportStub) GetOutfitPlan(_ context.Context, token, planID string) (model.OutfitPlan, error) {
	s.token, s.planID = token, planID
	return outfitPlanFixture(), s.err
}
func (s *outfitPlanTransportStub) UpdateOutfitPlan(_ context.Context, token, planID string, expected int, input model.OutfitPlanInput) (model.OutfitPlan, error) {
	s.token, s.planID, s.expected, s.updated = token, planID, expected, input
	return outfitPlanFixture(), s.err
}
func (s *outfitPlanTransportStub) CancelOutfitPlan(_ context.Context, token, planID string, expected int) (model.OutfitPlan, error) {
	s.token, s.planID, s.expected = token, planID, expected
	plan := outfitPlanFixture()
	plan.Status, plan.Revision = model.OutfitPlanCancelled, expected+1
	return plan, s.err
}
func (s *outfitPlanTransportStub) DeleteOutfitPlan(_ context.Context, token, planID string, expected int) error {
	s.token, s.planID, s.expected = token, planID, expected
	return s.err
}

func outfitPlanFixture() model.OutfitPlan {
	return model.OutfitPlan{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400d11", OwnerID: fixtureUser().ID, LocalDate: "2026-09-17", TimeZone: "Asia/Shanghai", Status: model.OutfitPlanActive, Revision: 1, CreatedAt: time.Date(2026, 9, 16, 3, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 9, 16, 3, 0, 0, 0, time.UTC), Items: []model.OutfitPlanItemSnapshot{{Ordinal: 0, Content: &model.OutfitItemContent{ItemID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", ItemRevision: 2, Name: "Blue Shirt", Category: model.WardrobeTop, Availability: model.WardrobeWearable, Attributes: model.WardrobeAttributes{FormalityBand: wardrobeTransportValue(model.WardrobeFormalitySmartCasual)}}}}}
}

func outfitPlanRouter(t *testing.T, service OutfitPlanHTTPService) *Router {
	t.Helper()
	router, err := NewRouter(context.Background(), false, probeFunc(func(context.Context) error { return nil }), nil, nil, nil, NewOutfitPlanHandler(service, true), time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func TestOutfitPlanCreateRejectsClientSnapshotAndReturnsServerSnapshot(t *testing.T) {
	service := new(outfitPlanTransportStub)
	router := outfitPlanRouter(t, service)
	valid := `{"id":"018f1f74-a2d0-7c6d-9c17-4a0ea2400d11","local_date":"2026-09-17","time_zone":"Asia/Shanghai","context_summary":"Lunch","items":[{"item_id":"018f1f74-a2d0-7c6d-9c17-4a0ea2400a12","revision":2}],"confirmed_unavailable_ids":[]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/outfit-plans", strings.NewReader(valid))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.token == "" || strings.Contains(response.Body.String(), "owner") || !strings.Contains(response.Body.String(), `"source":"user_confirmed"`) {
		t.Fatalf("valid plan create failed: status=%d body=%s", response.Code, response.Body.String())
	}
	if len(service.created.Items) != 1 || service.created.Items[0].Revision != 2 {
		t.Fatal("plan create transport lost ordered selections")
	}
	for name, body := range map[string]string{
		"owner":    strings.TrimSuffix(valid, "}") + `,"owner_id":"other"}`,
		"snapshot": strings.Replace(valid, `"revision":2`, `"revision":2,"name":"Injected"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/outfit-plans", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("client plan fact injection was accepted: status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestOutfitPlanListUpdateCancelDeleteHTTPContract(t *testing.T) {
	service := new(outfitPlanTransportStub)
	router := outfitPlanRouter(t, service)
	planID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400d11"
	requests := []*http.Request{
		httptest.NewRequest(http.MethodGet, "/v1/outfit-plans?limit=20&local_date=2026-09-17", nil),
		httptest.NewRequest(http.MethodPut, "/v1/outfit-plans/"+planID, strings.NewReader(`{"expected_revision":1,"local_date":"2026-09-18","time_zone":"Asia/Shanghai","context_summary":null,"items":[{"item_id":"018f1f74-a2d0-7c6d-9c17-4a0ea2400a12","revision":2}],"confirmed_unavailable_ids":[]}`)),
		httptest.NewRequest(http.MethodPost, "/v1/outfit-plans/"+planID+"/cancel", strings.NewReader(`{"expected_revision":2}`)),
		httptest.NewRequest(http.MethodDelete, "/v1/outfit-plans/"+planID+"?expected_revision=3", nil),
	}
	for _, request := range requests {
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
	if service.updated.LocalDate != "2026-09-18" || service.expected != 3 {
		t.Fatal("plan HTTP commands lost fields or expected revision")
	}
}

func TestOutfitPlanMapsDomainFailures(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
	}{{model.ErrInvalidOutfitPlanInput, http.StatusBadRequest}, {model.ErrOutfitPlanNotFound, http.StatusNotFound}, {model.ErrOutfitPlanConflict, http.StatusConflict}, {model.ErrOutfitItemsUnavailable, http.StatusConflict}, {model.ErrAuthentication, http.StatusUnauthorized}} {
		router := outfitPlanRouter(t, &outfitPlanTransportStub{err: test.err})
		request := httptest.NewRequest(http.MethodGet, "/v1/outfit-plans/018f1f74-a2d0-7c6d-9c17-4a0ea2400d11", nil)
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("error=%v status=%d body=%s", test.err, response.Code, response.Body.String())
		}
	}
}
