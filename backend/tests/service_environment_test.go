//go:build services

package tests

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
)

type serviceEnvironment struct {
	databaseURL    string
	minioEndpoint  string
	minioAccessKey string
	minioSecretKey string
	redisAddr      string
	redisPassword  string
	redisDB        int
	rabbitMQURL    string
}

func loadServiceEnvironment(t *testing.T) serviceEnvironment {
	t.Helper()
	environment, err := parseServiceEnvironment(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	return environment
}

func parseServiceEnvironment(lookup func(string) (string, bool)) (serviceEnvironment, error) {
	setting := func(name, fallback string) string {
		if value, ok := lookup(name); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
		return fallback
	}
	environment := serviceEnvironment{
		databaseURL:    setting("THEN_TEST_DATABASE_URL", "postgres://127.0.0.1/postgres?sslmode=disable"),
		minioEndpoint:  setting("THEN_TEST_MINIO_ENDPOINT", "127.0.0.1:9000"),
		minioAccessKey: setting("THEN_TEST_MINIO_ACCESS_KEY", "minioadmin"),
		minioSecretKey: setting("THEN_TEST_MINIO_SECRET_KEY", "minioadmin"),
		redisAddr:      setting("THEN_TEST_REDIS_ADDR", "127.0.0.1:6379"),
		redisPassword:  setting("THEN_TEST_REDIS_PASSWORD", ""),
		rabbitMQURL:    setting("THEN_TEST_RABBITMQ_URL", "amqp://guest:guest@127.0.0.1:5672/"),
	}
	redisDB, err := strconv.Atoi(setting("THEN_TEST_REDIS_DB", "15"))
	if err != nil || redisDB < 0 || redisDB > 15 {
		return serviceEnvironment{}, fmt.Errorf("THEN_TEST_REDIS_DB: expected an integer from 0 through 15")
	}
	environment.redisDB = redisDB
	if err := requireLoopbackURL("THEN_TEST_DATABASE_URL", environment.databaseURL, "postgres", "postgresql"); err != nil {
		return serviceEnvironment{}, err
	}
	if err := requireLoopbackEndpoint("THEN_TEST_MINIO_ENDPOINT", environment.minioEndpoint); err != nil {
		return serviceEnvironment{}, err
	}
	if err := requireLoopbackEndpoint("THEN_TEST_REDIS_ADDR", environment.redisAddr); err != nil {
		return serviceEnvironment{}, err
	}
	if err := requireLoopbackURL("THEN_TEST_RABBITMQ_URL", environment.rabbitMQURL, "amqp", "amqps"); err != nil {
		return serviceEnvironment{}, err
	}
	return environment, nil
}

func requireLoopbackURL(name, value string, schemes ...string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return fmt.Errorf("%s: expected a local service URL", name)
	}
	allowed := false
	for _, scheme := range schemes {
		allowed = allowed || parsed.Scheme == scheme
	}
	if !allowed || !isLoopbackHost(parsed.Hostname()) {
		return fmt.Errorf("%s: remote test services are not allowed", name)
	}
	return nil
}

func requireLoopbackEndpoint(name, value string) error {
	host, _, err := net.SplitHostPort(value)
	if err != nil || !isLoopbackHost(host) {
		return fmt.Errorf("%s: expected a loopback host and port", name)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func serviceID(t *testing.T) string {
	t.Helper()
	var id [12]byte
	if _, err := rand.Read(id[:]); err != nil {
		t.Fatal("cannot create isolated resource name")
	}
	return "then-test-" + hex.EncodeToString(id[:])
}

func serviceOK(t *testing.T, operation string, err error) {
	t.Helper()
	// SDK errors can contain signed URLs or credentials. Only report the operation.
	if err != nil {
		t.Fatalf("%s failed", operation)
	}
}

func TestServiceEnvironmentDefaultsStayLocal(t *testing.T) {
	environment, err := parseServiceEnvironment(func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	if environment.redisDB != 15 {
		t.Fatal("test cache must use the isolated default database")
	}
}

func TestServiceEnvironmentRejectsRemoteDependencies(t *testing.T) {
	for _, variable := range []string{"THEN_TEST_DATABASE_URL", "THEN_TEST_MINIO_ENDPOINT", "THEN_TEST_REDIS_ADDR", "THEN_TEST_RABBITMQ_URL"} {
		t.Run(variable, func(t *testing.T) {
			values := map[string]string{
				"THEN_TEST_DATABASE_URL":   "postgres://test@example.com/test?sslmode=verify-full",
				"THEN_TEST_MINIO_ENDPOINT": "example.com:9000",
				"THEN_TEST_REDIS_ADDR":     "example.com:6379",
				"THEN_TEST_RABBITMQ_URL":   "amqps://test@example.com:5671/",
			}
			_, err := parseServiceEnvironment(func(name string) (string, bool) {
				if name == variable {
					return values[name], true
				}
				return "", false
			})
			if err == nil || !strings.Contains(err.Error(), variable) {
				t.Fatal("remote dependency was not rejected safely")
			}
		})
	}
}
