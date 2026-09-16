package transport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDocumentationRoutesAndContractOwnership(t *testing.T) {
	document, _, err := GeneratedOpenAPI(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	expected := bytes.Clone(document)
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("enabled=%v", enabled), func(t *testing.T) {
			router, err := NewRouter(context.Background(), enabled,
				probeFunc(func(context.Context) error { return nil }), nil, nil, nil, nil, nil, time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"/docs/", "/docs/swagger-initializer.js", "/docs/swagger-ui.css", "/docs/swagger-ui-bundle.js", "/docs/favicon.ico", "/docs/logo-mark.png", "/docs/LICENSE", "/docs/swagger-ui-bundle.js.LICENSE.txt", "/openapi.yaml", "/openapi.json"} {
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
				want := 404
				if enabled {
					want = 200
				}
				if w.Code != want {
					t.Fatalf("%s: got %d, want %d", path, w.Code, want)
				}
				if enabled {
					if w.Body.Len() == 0 || w.Header().Get("Content-Type") == "" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "connect-src 'self'") {
						t.Fatalf("%s: missing content or same-origin policy", path)
					}
					if path == "/openapi.yaml" && !bytes.Equal(w.Body.Bytes(), expected) {
						t.Fatal("served contract differs from generated runtime contract")
					}
					if path == "/openapi.json" {
						var contract struct {
							OpenAPI string                     `json:"openapi"`
							Paths   map[string]json.RawMessage `json:"paths"`
						}
						if err := json.Unmarshal(w.Body.Bytes(), &contract); err != nil {
							t.Fatalf("generated JSON contract is invalid: %v", err)
						}
						if contract.OpenAPI != "3.1.2" || len(contract.Paths) != 24 {
							t.Fatalf("generated JSON contract lost API content: version=%q paths=%d", contract.OpenAPI, len(contract.Paths))
						}
					}
					if path == "/docs/" && (!strings.Contains(w.Body.String(), "/docs/favicon.ico") || !strings.Contains(w.Body.String(), "/docs/logo-mark.png")) {
						t.Fatal("documentation page is missing project branding")
					}
				}
			}
			for _, path := range []string{"/docs", "/docs/unknown.js", "/docs/README.md", "/docs/SHA256SUMS", "/docs/../main.go", "/docs/%2e%2e/main.go", "/docs//swagger-ui.css", "/docs/swagger-ui-bundle.js.map"} {
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
				if w.Code != 404 {
					t.Fatalf("unlisted path %s: status=%d", path, w.Code)
				}
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("POST", "/openapi.yaml", nil))
			want := 404
			if enabled {
				want = 405
			}
			if w.Code != want {
				t.Fatalf("POST contract: got %d, want %d", w.Code, want)
			}
			w = httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("POST", "/openapi.json", nil))
			if w.Code != want {
				t.Fatalf("POST JSON contract: got %d, want %d", w.Code, want)
			}
			w = httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", "/health/live", nil))
			if w.Code != 200 {
				t.Fatal("documentation changed health behavior")
			}
		})
	}
}

func TestBundledSwaggerIntegrity(t *testing.T) {
	manifest, err := os.ReadFile("swaggerui/SHA256SUMS")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(manifest)), "\n")
	if len(lines) != 4 {
		t.Fatal("expected the two official assets, project license and bundled notices")
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatal("invalid integrity manifest")
		}
		data, err := docsFiles.ReadFile("swaggerui/" + fields[1])
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != fields[0] {
			t.Fatalf("official asset checksum mismatch: %s", fields[1])
		}
	}
}

func TestBundledSwaggerRuntimeConfiguration(t *testing.T) {
	data, err := docsFiles.ReadFile("swaggerui/swagger-initializer.js")
	if err != nil {
		t.Fatal(err)
	}
	configuration := string(data)
	for _, required := range []string{
		"validatorUrl: null",
		"queryConfigEnabled: false",
		"persistAuthorization: false",
		"withCredentials: true",
		`"get"`,
		`"post"`,
		`"put"`,
		`"delete"`,
		`"patch"`,
	} {
		if !strings.Contains(configuration, required) {
			t.Fatalf("Swagger runtime configuration is missing %q", required)
		}
	}
}
