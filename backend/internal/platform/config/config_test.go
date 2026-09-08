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
