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

type wardrobeTransportStub struct {
	created model.CreateWardrobeItemInput
	updated model.UpdateWardrobeItemInput
	token   string
	err     error
}

func (s *wardrobeTransportStub) CreateWardrobeItem(_ context.Context, token string, input model.CreateWardrobeItemInput) (model.WardrobeItem, error) {
	s.token, s.created = token, input
	return wardrobeFixture(), s.err
}
func (s *wardrobeTransportStub) ListWardrobeItems(_ context.Context, token string, _ int, _ *string) (model.WardrobePage, error) {
	s.token = token
	return model.WardrobePage{Items: []model.WardrobeItem{wardrobeFixture()}}, s.err
}
func (s *wardrobeTransportStub) GetWardrobeItem(_ context.Context, token, _ string) (model.WardrobeItem, error) {
	s.token = token
	return wardrobeFixture(), s.err
}
func (s *wardrobeTransportStub) UpdateWardrobeItem(_ context.Context, token, _ string, _ int, input model.UpdateWardrobeItemInput) (model.WardrobeItem, error) {
	s.token, s.updated = token, input
	return wardrobeFixture(), s.err
}
func (s *wardrobeTransportStub) DeleteWardrobeItem(_ context.Context, token, _ string, _ int) error {
	s.token = token
	return s.err
}

func wardrobeFixture() model.WardrobeItem {
	return model.WardrobeItem{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", OwnerID: fixtureUser().ID, Name: "Blue Shirt", Category: model.WardrobeTop, Availability: model.WardrobeWearable, Source: model.WardrobeSourceWardrobe, Attributes: model.WardrobeAttributes{FormalityBand: wardrobeTransportValue(model.WardrobeFormalitySmartCasual), WalkingUse: wardrobeTransportValue(model.WardrobeUseSuitable)}, Revision: 1, CreatedAt: time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)}
}

func wardrobeRouter(t *testing.T, service WardrobeHTTPService) *Router {
	t.Helper()
	router, err := NewRouter(context.Background(), false, probeFunc(func(context.Context) error { return nil }), nil, nil, NewWardrobeHandler(service, true), time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func TestWardrobeCreateUsesSessionAndRejectsOwnerField(t *testing.T) {
	service := new(wardrobeTransportStub)
	router := wardrobeRouter(t, service)
	valid := `{"id":"018f1f74-a2d0-7c6d-9c17-4a0ea2400a12","name":"Blue Shirt","category":"top","availability":"wearable","source":"wardrobe","attributes":{"formality_band":"smart_casual","walking_use":"suitable"}}`
	request := httptest.NewRequest(http.MethodPost, "/v1/wardrobe/items", strings.NewReader(valid))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.token == "" || strings.Contains(response.Body.String(), "owner") || !strings.Contains(response.Body.String(), `"formality_band":{"value":"smart_casual","source":"user_confirmed"}`) {
		t.Fatalf("valid wardrobe create failed: status=%d body=%s", response.Code, response.Body.String())
	}
	if service.created.Attributes.FormalityBand == nil || *service.created.Attributes.FormalityBand != model.WardrobeFormalitySmartCasual {
		t.Fatal("wardrobe create transport lost confirmed attributes")
	}

	request = httptest.NewRequest(http.MethodPost, "/v1/wardrobe/items", strings.NewReader(strings.TrimSuffix(valid, "}")+`,"owner_id":"other"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("owner injection was accepted: status=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/v1/wardrobe/items", strings.NewReader(strings.Replace(valid, `"formality_band":"smart_casual"`, `"formality_band":"smart_casual","source":"user_confirmed"`, 1)))
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
			request := httptest.NewRequest(http.MethodPost, "/v1/wardrobe/items", strings.NewReader(body))
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
		{model.ErrWardrobeNotFound, http.StatusNotFound},
		{model.ErrWardrobeConflict, http.StatusConflict},
		{model.ErrAuthentication, http.StatusUnauthorized},
	} {
		service := &wardrobeTransportStub{err: test.err}
		router := wardrobeRouter(t, service)
		request := httptest.NewRequest(http.MethodGet, "/v1/wardrobe/items/018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", nil)
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
		httptest.NewRequest(http.MethodGet, "/v1/wardrobe/items?limit=25", nil),
		httptest.NewRequest(http.MethodPut, "/v1/wardrobe/items/018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", strings.NewReader(`{"expected_revision":1,"name":"Updated Shirt","category":"top","availability":"laundry","attributes":{"warmth_band":"warm","rain_use":null}}`)),
		httptest.NewRequest(http.MethodDelete, "/v1/wardrobe/items/018f1f74-a2d0-7c6d-9c17-4a0ea2400a12?expected_revision=1", nil),
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
	if service.updated.Name != "Updated Shirt" || service.updated.Availability != model.WardrobeLaundry || service.updated.Attributes.WarmthBand == nil || *service.updated.Attributes.WarmthBand != model.WardrobeWarmthWarm || service.updated.Attributes.RainUse != nil {
		t.Fatal("wardrobe update transport lost confirmed fields")
	}
}

func wardrobeTransportValue[T any](value T) *T { return &value }
