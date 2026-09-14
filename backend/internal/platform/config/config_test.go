package config

import (
	"strings"
	"testing"
)

func TestConfigurationBoundaries(t *testing.T) {
	const url = "postgres://fixture:synthetic-secret@127.0.0.1:5432/fixture?sslmode=disable"
	tests := []struct {
		name, key, value string
		valid            bool
	}{
		{"defaults", "DATABASE_URL", url, true},
		{"ephemeral port", "HTTP_ADDR", "127.0.0.1:0", true},
		{"unimplemented worker", "APP_ROLE", "worker", false},
		{"unimplemented all", "APP_ROLE", "all", false},
		{"empty role", "APP_ROLE", "", false},
		{"empty database", "DATABASE_URL", "", false},
		{"bad URL secret", "DATABASE_URL", "postgres://fixture:synthetic-secret@%zz/fixture", false},
		{"remote plaintext", "DATABASE_URL", "postgres://fixture:synthetic-secret@example.test/fixture?sslmode=disable", false},
		{"remote verified TLS", "DATABASE_URL", "postgres://fixture:synthetic-secret@example.test/fixture?sslmode=verify-full", true},
		{"host override", "DATABASE_URL", url + "&host=example.test", false},
		{"duplicate TLS", "DATABASE_URL", url + "&sslmode=verify-full", false},
		{"wildcard hostname", "HTTP_ADDR", ":8080", false},
		{"public listener without secure cookie", "HTTP_ADDR", "0.0.0.0:8080", false},
		{"bad port", "HTTP_ADDR", "127.0.0.1:70000", false},
		{"unbounded pool", "DB_MAX_OPEN_CONNS", "0", false},
		{"pool over budget", "DB_MAX_OPEN_CONNS", "101", false},
		{"idle over open", "DB_MAX_IDLE_CONNS", "11", false},
		{"negative health", "HEALTH_TIMEOUT", "-1s", false},
		{"health exceeds budget", "HEALTH_TIMEOUT", "6s", false},
		{"unbounded shutdown", "SHUTDOWN_TIMEOUT", "0s", false},
		{"unbounded lifetime", "DB_CONN_MAX_LIFETIME", "0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{"DATABASE_URL": url, tt.key: tt.value}
			_, err := Load(func(k string) (string, bool) { v, ok := env[k]; return v, ok })
			if (err == nil) != tt.valid {
				t.Fatalf("configuration valid=%v, expected %v", err == nil, tt.valid)
			}
			if err != nil && strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatal("configuration error exposed a secret")
			}
		})
	}
}

func TestSessionCookieConfiguration(t *testing.T) {
	const databaseURL = "postgres://fixture:fixture@127.0.0.1:5432/fixture?sslmode=disable"
	for _, test := range []struct {
		name, address, secure string
		valid                 bool
	}{
		{"loopback development", "127.0.0.1:8080", "false", true},
		{"public secure", "0.0.0.0:8080", "true", true},
		{"public insecure", "0.0.0.0:8080", "false", false},
		{"invalid value", "127.0.0.1:8080", "synthetic-secret", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			environment := map[string]string{
				"DATABASE_URL": databaseURL, "HTTP_ADDR": test.address,
				"SESSION_COOKIE_SECURE": test.secure,
			}
			_, err := Load(func(key string) (string, bool) { value, ok := environment[key]; return value, ok })
			if (err == nil) != test.valid {
				t.Fatalf("configuration valid=%v expected=%v", err == nil, test.valid)
			}
			if err != nil && strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatal("session configuration exposed raw input")
			}
		})
	}
}

func TestDocumentationConfiguration(t *testing.T) {
	tests := []struct {
		name, enabled, address string
		valid                  bool
	}{
		{"disabled on all interfaces", "false", "0.0.0.0:8080", true},
		{"local IPv4", "true", "127.0.0.1:8080", true},
		{"local IPv6", "true", "[::1]:8080", true},
		{"public IPv4", "true", "0.0.0.0:8080", false},
		{"public IPv6", "true", "[::]:8080", false},
		{"invalid boolean", "synthetic-secret", "127.0.0.1:8080", false},
		{"empty boolean", "", "127.0.0.1:8080", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{
				"DATABASE_URL":     "postgres://fixture:fixture@127.0.0.1:5432/fixture?sslmode=disable",
				"API_DOCS_ENABLED": tt.enabled, "HTTP_ADDR": tt.address, "SESSION_COOKIE_SECURE": "true",
			}
			_, err := Load(func(k string) (string, bool) { v, ok := env[k]; return v, ok })
			if (err == nil) != tt.valid {
				t.Fatalf("configuration valid=%v, expected %v", err == nil, tt.valid)
			}
			if err != nil && strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatal("documentation configuration leaked raw input")
			}
		})
	}
}
