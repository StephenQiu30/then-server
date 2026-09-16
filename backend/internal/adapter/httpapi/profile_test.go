package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

func fixtureProfile() domain.PublicProfile {
	bio := "记录日常穿搭与轻量生活。"
	return domain.PublicProfile{
		Handle: "then_style", DisplayName: "于是用户", Bio: &bio, Revision: 1,
		CreatedAt: time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC),
	}
}

func TestProfileSuccessResponsesMatchOpenAPIAndStayPublic(t *testing.T) {
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
		auth                     bool
	}{
		{name: "current", method: http.MethodGet, path: "/users/me/profile", auth: true},
		{name: "put", method: http.MethodPut, path: "/users/me/profile", body: `{"handle":"Then_Style","bio":"记录日常穿搭与轻量生活。","expected_revision":0}`, auth: true},
		{name: "public", method: http.MethodGet, path: "/profiles/then_style"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &accountServiceStub{profile: fixtureProfile()}
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
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			body := response.Body.String()
			if strings.Contains(body, "person@example.test") || strings.Contains(body, "018f1f74") || strings.Contains(body, "then_session") {
				t.Fatalf("public profile exposed private account data: %s", body)
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
			if test.name == "put" && (service.profilePut.Handle != "Then_Style" || service.profilePut.ExpectedRevision != 0) {
				t.Fatal("profile request was not mapped to the service input")
			}
		})
	}
}

func TestProfileErrorsUseStableBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		status     int
		code       string
		clearToken bool
	}{
		{name: "not found", err: domain.ErrProfileNotFound, status: http.StatusNotFound, code: "NOT_FOUND"},
		{name: "stale", err: domain.ErrProfileConflict, status: http.StatusConflict, code: "REVISION_CONFLICT"},
		{name: "handle", err: domain.ErrHandleConflict, status: http.StatusConflict, code: "HANDLE_UNAVAILABLE"},
		{name: "session", err: domain.ErrAuthentication, status: http.StatusUnauthorized, code: "AUTHENTICATION_FAILED", clearToken: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &accountServiceStub{err: test.err}
			router := accountRouter(t, service, true)
			request := httptest.NewRequest(http.MethodGet, "/users/me/profile", nil)
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.status || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if test.clearToken && (len(response.Result().Cookies()) != 1 || response.Result().Cookies()[0].MaxAge != -1) {
				t.Fatal("rejected profile session did not clear its cookie")
			}
		})
	}
}

func TestProfileContractRejectsUnknownFields(t *testing.T) {
	service := &accountServiceStub{profile: fixtureProfile()}
	router := accountRouter(t, service, false)
	request := httptest.NewRequest(http.MethodPut, "/users/me/profile", strings.NewReader(`{"handle":"then_style","expected_revision":0,"role":"admin"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("a", 43)})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || service.profilePut.Handle != "" {
		t.Fatalf("unknown profile field reached service: status=%d body=%s", response.Code, response.Body.String())
	}
}
