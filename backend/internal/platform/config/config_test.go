package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestAccountMailConfigurationIsCompleteAndSecretFree(t *testing.T) {
	base := map[string]string{
		"DATABASE_URL":           "postgres://fixture:fixture@127.0.0.1:5432/fixture?sslmode=disable",
		"ACCOUNT_MAIL_FROM":      "sender@163.com",
		"ACCOUNT_MAIL_AUTH_CODE": "synthetic-authorization-code",
		"ACCOUNT_MAIL_KEY":       base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("k", 32))),
		"ACCOUNT_MAIL_LINK_BASE": "https://then.example/account/mail",
	}
	for _, step := range []struct {
		name, key, value string
		valid            bool
	}{
		{"complete", "", "", true},
		{"missing authorization", "ACCOUNT_MAIL_AUTH_CODE", "", false},
		{"short key", "ACCOUNT_MAIL_KEY", "short-secret", false},
		{"plaintext link", "ACCOUNT_MAIL_LINK_BASE", "http://then.example/account/mail", false},
		{"wrong provider", "ACCOUNT_MAIL_SMTP_ADDR", "smtp.example.com:465", false},
		{"plaintext SMTP port", "ACCOUNT_MAIL_SMTP_ADDR", "smtp.163.com:25", false},
	} {
		t.Run(step.name, func(t *testing.T) {
			environment := make(map[string]string, len(base)+1)
			for key, value := range base {
				environment[key] = value
			}
			if step.key != "" {
				environment[step.key] = step.value
			}
			configuration, err := Load(func(key string) (string, bool) { value, exists := environment[key]; return value, exists })
			if (err == nil) != step.valid {
				t.Fatal("mail configuration validity differed from contract")
			}
			if err != nil && strings.Contains(err.Error(), "synthetic-authorization-code") {
				t.Fatal("mail configuration error exposed the authorization code")
			}
			if step.valid && (!configuration.MailEnabled || len(configuration.MailKey) != 32) {
				t.Fatal("complete mail configuration did not enable delivery")
			}
		})
	}
}

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
		{"local redis plaintext", "REDIS_URL", "redis://127.0.0.1:6379/0", true},
		{"remote redis plaintext", "REDIS_URL", "redis://:synthetic-secret@example.test:6379/0", false},
		{"remote redis TLS", "REDIS_URL", "rediss://:synthetic-secret@example.test:6380/0", true},
		{"remote redis TLS without password", "REDIS_URL", "rediss://example.test:6380/0", false},
		{"redis missing database", "REDIS_URL", "redis://127.0.0.1:6379", false},
		{"redis unsupported query", "REDIS_URL", "redis://127.0.0.1:6379/0?protocol=3", false},
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

func TestLocalMediaRolesRequireExplicitLocalConfiguration(t *testing.T) {
	base := map[string]string{
		"DATABASE_URL":              "postgres://fixture:fixture@127.0.0.1:5432/fixture?sslmode=disable",
		"MEDIA_DEVELOPMENT_ENABLED": "true",
		"MINIO_ENDPOINT":            "127.0.0.1:9000",
		"MINIO_ACCESS_KEY":          "fixture",
		"MINIO_SECRET_KEY":          "synthetic-secret",
		"KAFKA_BROKERS":             "127.0.0.1:9092",
	}
	for _, role := range []string{"api", "worker", "all"} {
		t.Run(role, func(t *testing.T) {
			environment := make(map[string]string, len(base)+1)
			for key, value := range base {
				environment[key] = value
			}
			environment["APP_ROLE"] = role
			configuration, err := Load(func(key string) (string, bool) { value, ok := environment[key]; return value, ok })
			if err != nil || configuration.Role != role || !configuration.MediaDevelopmentEnabled {
				t.Fatalf("local media role rejected: role=%s err=%v", role, err)
			}
		})
	}
	for name, value := range map[string]string{"HTTP_ADDR": "0.0.0.0:8080", "MINIO_ENDPOINT": "192.0.2.1:9000", "KAFKA_BROKERS": "192.0.2.1:9092"} {
		t.Run("reject "+name, func(t *testing.T) {
			environment := make(map[string]string, len(base)+1)
			for key, current := range base {
				environment[key] = current
			}
			environment[name] = value
			_, err := Load(func(key string) (string, bool) { current, ok := environment[key]; return current, ok })
			if err == nil || strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatal("remote or secret-bearing local media configuration was not rejected safely")
			}
		})
	}
}

func TestKafkaConfigurationBoundaries(t *testing.T) {
	for _, test := range []struct {
		name    string
		brokers []string
		prefix  string
		valid   bool
	}{
		{"local endpoints", []string{"127.0.0.1:9092", "[::1]:9092"}, "then-test", true},
		{"empty brokers", nil, "then", false},
		{"mixed remote", []string{"127.0.0.1:9092", "example.com:9092"}, "then", false},
		{"invalid port", []string{"127.0.0.1:65536"}, "then", false},
		{"URL not endpoint", []string{"kafka://synthetic-secret@127.0.0.1:9092"}, "then", false},
		{"empty namespace", []string{"127.0.0.1:9092"}, "", false},
		{"invalid namespace", []string{"127.0.0.1:9092"}, "then/other", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateKafka(test.brokers, test.prefix)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
			if err != nil && strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatal("configuration error exposes raw input")
			}
		})
	}
}
