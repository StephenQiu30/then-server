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

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	wardrobeapp "github.com/StephenQiu30/then-server/backend/internal/application/wardrobe"
)

type wardrobeTransportStub struct {
	created     wardrobeapp.CreateWardrobeItemInput
	recommended wardrobeapp.RecommendationContext
	updated     wardrobeapp.UpdateWardrobeItemInput
	filter      wardrobeapp.WardrobeListFilter
	token       string
	err         error
}

func (s *wardrobeTransportStub) RecommendWardrobe(_ context.Context, token string, input wardrobeapp.RecommendationContext) (wardrobeapp.RecommendationResult, error) {
	s.token, s.recommended = token, input
	return wardrobeapp.RecommendationResult{PolicyVersion: wardrobeapp.RecommendationPolicyVersion, Candidates: []wardrobeapp.RecommendationCandidate{}}, s.err
}

func TestWardrobeRecommendationRequiresSessionAndMapsConfirmedContext(t *testing.T) {
	service := new(wardrobeTransportStub)
	router := wardrobeRouter(t, service)
	requestBody := `{"local_date":"2026-09-26","time_zone":"Asia/Shanghai","formality_band":"smart_casual","requires_rain_suitability":true,"requires_walking_suitability":false,"include_packed_items":false}`
	request := httptest.NewRequest(http.MethodPost, "/wardrobe/recommendations", strings.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || service.token != "" {
		t.Fatalf("anonymous recommendation was not rejected: %d %s", response.Code, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodPost, "/wardrobe/recommendations", strings.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.token == "" || service.recommended.FormalityBand == nil || *service.recommended.FormalityBand != wardrobeapp.WardrobeFormalitySmartCasual || !service.recommended.RequiresRainSuitability || !strings.Contains(response.Body.String(), `"policy_version":"wardrobe-hard-constraints-v1"`) {
		t.Fatalf("confirmed recommendation context was lost: %d %s %+v", response.Code, response.Body.String(), service.recommended)
	}
}

func (s *wardrobeTransportStub) CreateWardrobeItem(_ context.Context, token string, input wardrobeapp.CreateWardrobeItemInput) (wardrobeapp.WardrobeItem, error) {
	s.token, s.created = token, input
	return wardrobeFixture(), s.err
}
func (s *wardrobeTransportStub) ListWardrobeItems(_ context.Context, token string, _ int, _ *string, filter wardrobeapp.WardrobeListFilter) (wardrobeapp.WardrobePage, error) {
	s.token, s.filter = token, filter
	return wardrobeapp.WardrobePage{Items: []wardrobeapp.WardrobeItem{wardrobeFixture()}}, s.err
}
func (s *wardrobeTransportStub) GetWardrobeItem(_ context.Context, token, _ string) (wardrobeapp.WardrobeItem, error) {
	s.token = token
	return wardrobeFixture(), s.err
}
func (s *wardrobeTransportStub) UpdateWardrobeItem(_ context.Context, token, _ string, _ int, input wardrobeapp.UpdateWardrobeItemInput) (wardrobeapp.WardrobeItem, error) {
	s.token, s.updated = token, input
	return wardrobeFixture(), s.err
}
func (s *wardrobeTransportStub) ArchiveWardrobeItem(_ context.Context, token, _ string, _ int) (wardrobeapp.WardrobeItem, error) {
	s.token = token
	item := wardrobeFixture()
	archivedAt := time.Date(2026, 9, 15, 9, 0, 0, 0, time.UTC)
	item.ArchivedAt = &archivedAt
	return item, s.err
}
func (s *wardrobeTransportStub) RestoreWardrobeItem(_ context.Context, token, _ string, _ int) (wardrobeapp.WardrobeItem, error) {
	s.token = token
	return wardrobeFixture(), s.err
}
func (s *wardrobeTransportStub) GetWardrobeDeletionImpact(_ context.Context, token, _ string) (wardrobeapp.WardrobeDeletionImpact, error) {
	s.token = token
	return wardrobeapp.WardrobeDeletionImpact{ExpectedImpact: emptyTransportImpact}, s.err
}
func (s *wardrobeTransportStub) DeleteWardrobeItem(_ context.Context, token, _ string, _ int, _ wardrobeapp.WardrobeHistoryPolicy, _ string) error {
	s.token = token
	return s.err
}

func wardrobeFixture() wardrobeapp.WardrobeItem {
	return wardrobeapp.WardrobeItem{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", OwnerID: fixtureUser().ID, Name: "Blue Shirt", Category: wardrobeapp.WardrobeTop, Availability: wardrobeapp.WardrobeWearable, Source: wardrobeapp.WardrobeSourceWardrobe, Attributes: wardrobeapp.WardrobeAttributes{FormalityBand: wardrobeTransportValue(wardrobeapp.WardrobeFormalitySmartCasual), WalkingUse: wardrobeTransportValue(wardrobeapp.WardrobeUseSuitable)}, Revision: 1, CreatedAt: time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)}
}

func wardrobeRouter(t *testing.T, service WardrobeHTTPService) *Router {
	t.Helper()
	router, err := NewRouter(context.Background(), false, probeFunc(func(context.Context) error { return nil }), nil, nil, NewWardrobeHandler(service, true), nil, nil, time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func TestWardrobeCreateUsesSessionAndRejectsOwnerField(t *testing.T) {
	service := new(wardrobeTransportStub)
	router := wardrobeRouter(t, service)
	valid := `{"id":"018f1f74-a2d0-7c6d-9c17-4a0ea2400a12","name":"Blue Shirt","category":"top","availability":"wearable","source":"wardrobe","attributes":{"formality_band":"smart_casual","walking_use":"suitable"}}`
	request := httptest.NewRequest(http.MethodPost, "/wardrobe/items", strings.NewReader(valid))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.token == "" || strings.Contains(response.Body.String(), "owner") || !strings.Contains(response.Body.String(), `"formality_band":{"value":"smart_casual","source":"user_confirmed"}`) {
		t.Fatalf("valid wardrobe create failed: status=%d body=%s", response.Code, response.Body.String())
	}
	if service.created.Attributes.FormalityBand == nil || *service.created.Attributes.FormalityBand != wardrobeapp.WardrobeFormalitySmartCasual {
		t.Fatal("wardrobe create transport lost confirmed attributes")
	}

	request = httptest.NewRequest(http.MethodPost, "/wardrobe/items", strings.NewReader(strings.TrimSuffix(valid, "}")+`,"owner_id":"other"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("owner injection was accepted: status=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/wardrobe/items", strings.NewReader(strings.Replace(valid, `"formality_band":"smart_casual"`, `"formality_band":"smart_casual","source":"user_confirmed"`, 1)))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("attribute source injection was accepted: status=%d body=%s", response.Code, response.Body.String())
	}

	for name, body := range map[string]string{
		"missing attributes": strings.Replace(valid, `,"attributes":{"formality_band":"smart_casual","walking_use":"suitable"}`, "", 1),
		"invalid attribute":  strings.Replace(valid, `"smart_casual"`, `"business"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/wardrobe/items", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid attribute contract was accepted: status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestWardrobeMapsNotFoundConflictAndAuthentication(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
	}{
		{wardrobeapp.ErrWardrobeNotFound, http.StatusNotFound},
		{wardrobeapp.ErrWardrobeConflict, http.StatusConflict},
		{accountapp.ErrAuthentication, http.StatusUnauthorized},
	} {
		service := &wardrobeTransportStub{err: test.err}
		router := wardrobeRouter(t, service)
		request := httptest.NewRequest(http.MethodGet, "/wardrobe/items/018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", nil)
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("error=%v status=%d body=%s", test.err, response.Code, response.Body.String())
		}
		if test.status == http.StatusUnauthorized && len(response.Result().Cookies()) != 1 {
			t.Fatal("wardrobe authentication rejection did not clear stale cookie")
		}
	}
}

func TestWardrobeListUpdateAndDeleteHTTPContract(t *testing.T) {
	service := new(wardrobeTransportStub)
	router := wardrobeRouter(t, service)
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/wardrobe/items?limit=25", nil),
		httptest.NewRequest(http.MethodPut, "/wardrobe/items/018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", strings.NewReader(`{"expected_revision":1,"name":"Updated Shirt","category":"top","availability":"laundry","attributes":{"warmth_band":"warm","rain_use":null}}`)),
		httptest.NewRequest(http.MethodDelete, "/wardrobe/items/018f1f74-a2d0-7c6d-9c17-4a0ea2400a12?expected_revision=1&history_policy=redact_snapshots&expected_impact="+emptyTransportImpact, nil),
	} {
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
		if request.Method == http.MethodPut {
			request.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		expected := http.StatusOK
		if request.Method == http.MethodDelete {
			expected = http.StatusNoContent
		}
		if response.Code != expected {
			t.Fatalf("%s status=%d body=%s", request.Method, response.Code, response.Body.String())
		}
	}
	if service.updated.Name != "Updated Shirt" || service.updated.Availability != wardrobeapp.WardrobeLaundry || service.updated.Attributes.WarmthBand == nil || *service.updated.Attributes.WarmthBand != wardrobeapp.WardrobeWarmthWarm || service.updated.Attributes.RainUse != nil {
		t.Fatal("wardrobe update transport lost confirmed fields")
	}
	if service.filter.Lifecycle != wardrobeapp.WardrobeActive {
		t.Fatalf("wardrobe list did not default to active lifecycle: %+v", service.filter)
	}
	request := httptest.NewRequest(http.MethodGet, "/wardrobe/items?lifecycle=archived&availability=laundry", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.filter.Lifecycle != wardrobeapp.WardrobeArchived || service.filter.Availability == nil || *service.filter.Availability != wardrobeapp.WardrobeLaundry {
		t.Fatalf("wardrobe list filters were not forwarded: status=%d filter=%+v", response.Code, service.filter)
	}
}

func TestWardrobeArchiveAndRestoreHTTPContract(t *testing.T) {
	service := new(wardrobeTransportStub)
	router := wardrobeRouter(t, service)
	for _, test := range []struct {
		path      string
		lifecycle string
	}{
		{path: "/wardrobe/items/018f1f74-a2d0-7c6d-9c17-4a0ea2400a12/archive", lifecycle: "archived"},
		{path: "/wardrobe/items/018f1f74-a2d0-7c6d-9c17-4a0ea2400a12/restore", lifecycle: "active"},
	} {
		request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(`{"expected_revision":1}`))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"lifecycle":"`+test.lifecycle+`"`) {
			t.Fatalf("lifecycle transition failed for %s: status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
}

func wardrobeTransportValue[T any](value T) *T { return &value }

const emptyTransportImpact = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
