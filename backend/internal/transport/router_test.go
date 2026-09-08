package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

type probeFunc func(context.Context) error

func (f probeFunc) Probe(ctx context.Context) error { return f(ctx) }

func newTestRouter(t *testing.T, probe probeFunc) (*Router, *bytes.Buffer) {
	t.Helper()
	doc, err := os.ReadFile("../../openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	log := new(bytes.Buffer)
	router, err := NewRouter(context.Background(), doc, probe, 20*time.Millisecond, slog.New(slog.NewJSONHandler(log, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return router, log
}

func TestHealthContractAndFailureIsolation(t *testing.T) {
	data, err := os.ReadFile("../../openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := openapi3.NewLoader().LoadFromData(data)
	if err != nil {
		t.Fatal(err)
	}
	if err = doc.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	contract, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, path string
		probe      probeFunc
		drain      bool
		status     int
	}{
		{"live ignores database", "/v1/health/live", func(context.Context) error { t.Fatal("liveness contacted database"); return nil }, false, 200},
		{"ready", "/v1/health/ready", func(context.Context) error { return nil }, false, 200},
		{"database unavailable", "/v1/health/ready", func(context.Context) error { return errors.New("synthetic-secret") }, false, 503},
		{"timeout", "/v1/health/ready", func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }, false, 503},
		{"draining", "/v1/health/ready", func(context.Context) error { t.Fatal("draining contacted database"); return nil }, true, 503},
		{"panic redaction", "/v1/health/ready", func(context.Context) error { panic("synthetic-secret") }, false, 500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, logs := newTestRouter(t, tt.probe)
			if tt.drain {
				r.Drain()
			}
			req := httptest.NewRequest("GET", "http://localhost"+tt.path, nil)
			req.Header.Set("X-Request-ID", "synthetic-secret")
			req.Header.Set("Authorization", "Bearer synthetic-secret")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Fatalf("status=%d expected=%d", w.Code, tt.status)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("health must not be cached")
			}
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["request_id"] != w.Header().Get("X-Request-ID") {
				t.Fatal("request IDs differ")
			}
			if strings.Contains(w.Body.String()+logs.String(), "synthetic-secret") {
				t.Fatal("sensitive input reached response/log")
			}
			route, params, err := contract.FindRoute(req)
			if err != nil {
				t.Fatal(err)
			}
			input := &openapi3filter.RequestValidationInput{Request: req, Route: route, PathParams: params}
			output := &openapi3filter.ResponseValidationInput{RequestValidationInput: input, Status: w.Code, Header: w.Header()}
			if err := openapi3filter.ValidateResponse(req.Context(), output.SetBodyBytes(w.Body.Bytes())); err != nil {
				t.Fatal(err)
			}
			// The response validator must actually reject a wrong status enum/unknown field.
			if tt.status == 200 {
				bad := []byte(`{"status":"invalid","request_id":"TESTREQUESTIDENTIFIER00000001","unexpected":true}`)
				if err := openapi3filter.ValidateResponse(req.Context(), output.SetBodyBytes(bad)); err == nil {
					t.Fatal("contract validator accepted invalid JSON")
				}
			}
		})
	}
}

func TestOnlyExplicitHealthRequestsAreAccepted(t *testing.T) {
	tests := []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/v1/health/live?token=synthetic-secret", "", 400},
		{"GET", "/v1/health/live", "synthetic-secret", 400},
		{"POST", "/v1/health/live", "", 405},
		{"GET", "/v1/v1/health/live", "", 404},
		{"GET", "/v1/health/live/", "", 404},
		{"GET", "/synthetic-secret", "", 404},
		{"POST", "/v1/jobs", "", 404},
	}
	for _, tt := range tests {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			r, logs := newTestRouter(t, func(context.Context) error { return nil })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
			if w.Code != tt.status {
				t.Fatalf("status=%d expected=%d", w.Code, tt.status)
			}
			if strings.Contains(logs.String(), "synthetic-secret") {
				t.Fatal("raw URL/body leaked into log")
			}
		})
	}
}

func TestDrainDuringProbeDoesNotReturnReady(t *testing.T) {
	var r *Router
	r, _ = newTestRouter(t, func(context.Context) error { r.Drain(); return nil })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/v1/health/ready", nil))
	if w.Code != 503 {
		t.Fatal("reported ready after drain")
	}
}
