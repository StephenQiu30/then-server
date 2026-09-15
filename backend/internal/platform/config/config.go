// Package config validates process configuration without exposing secret values.
package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Role            string
	HTTPAddr        string
	DocsEnabled     bool
	SessionSecure   bool
	DatabaseURL     string
	RedisURL        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	StartupTimeout  time.Duration
	HealthTimeout   time.Duration
	ShutdownTimeout time.Duration
}

// Load never includes raw environment values in returned errors.
func Load(lookup func(string) (string, bool)) (Config, error) {
	get := func(key, fallback string) string {
		if value, ok := lookup(key); ok {
			return value
		}
		return fallback
	}
	c := Config{
		Role:        get("APP_ROLE", "api"),
		HTTPAddr:    get("HTTP_ADDR", "127.0.0.1:8080"),
		DatabaseURL: get("DATABASE_URL", ""),
		RedisURL:    get("REDIS_URL", "redis://127.0.0.1:6379/0"),
	}
	if c.Role != "api" {
		return Config{}, fmt.Errorf("APP_ROLE: only api is implemented in this release")
	}
	host, port, err := net.SplitHostPort(c.HTTPAddr)
	p, portErr := strconv.Atoi(port)
	if err != nil || net.ParseIP(host) == nil || portErr != nil || p < 0 || p > 65535 {
		return Config{}, fmt.Errorf("HTTP_ADDR: expected IP:port")
	}
	switch get("API_DOCS_ENABLED", "false") {
	case "false":
	case "true":
		c.DocsEnabled = true
	default:
		return Config{}, fmt.Errorf("API_DOCS_ENABLED: expected true or false")
	}
	switch get("SESSION_COOKIE_SECURE", "false") {
	case "false":
	case "true":
		c.SessionSecure = true
	default:
		return Config{}, fmt.Errorf("SESSION_COOKIE_SECURE: expected true or false")
	}
	if !c.SessionSecure && !net.ParseIP(host).IsLoopback() {
		return Config{}, fmt.Errorf("SESSION_COOKIE_SECURE: required for non-loopback HTTP_ADDR")
	}
	if c.DocsEnabled && !net.ParseIP(host).IsLoopback() {
		return Config{}, fmt.Errorf("API_DOCS_ENABLED: documentation requires a loopback HTTP_ADDR")
	}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil || u == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || u.User == nil || u.User.Username() == "" || strings.Trim(u.Path, "/") == "" || u.Fragment != "" {
		return Config{}, fmt.Errorf("DATABASE_URL: expected PostgreSQL URL with host, user and database")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return Config{}, fmt.Errorf("DATABASE_URL: invalid connection options")
	}
	mode := query.Get("sslmode")
	for key, values := range query {
		switch key {
		case "sslmode", "sslrootcert", "sslcert", "sslkey", "connect_timeout":
		default:
			return Config{}, fmt.Errorf("DATABASE_URL: unsupported connection option")
		}
		if len(values) != 1 {
			return Config{}, fmt.Errorf("DATABASE_URL: duplicate connection option")
		}
	}
	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if mode != "verify-full" && !(mode == "disable" && local) {
		return Config{}, fmt.Errorf("DATABASE_URL: require sslmode=verify-full; disable is only allowed on loopback")
	}
	if err := validateRedisURL(c.RedisURL); err != nil {
		return Config{}, err
	}
	integers := []struct {
		key, fallback string
		min, max      int
		dest          *int
	}{
		{"DB_MAX_OPEN_CONNS", "10", 1, 100, &c.MaxOpenConns},
		{"DB_MAX_IDLE_CONNS", "2", 0, 100, &c.MaxIdleConns},
	}
	for _, field := range integers {
		v, e := strconv.Atoi(get(field.key, field.fallback))
		if e != nil || v < field.min || v > field.max {
			return Config{}, fmt.Errorf("%s: outside allowed integer range", field.key)
		}
		*field.dest = v
	}
	if c.MaxIdleConns > c.MaxOpenConns {
		return Config{}, fmt.Errorf("DB_MAX_IDLE_CONNS: cannot exceed DB_MAX_OPEN_CONNS")
	}
	durations := []struct {
		key, fallback string
		max           time.Duration
		dest          *time.Duration
	}{
		{"DB_CONN_MAX_LIFETIME", "30m", 24 * time.Hour, &c.ConnMaxLifetime},
		{"STARTUP_TIMEOUT", "5s", time.Minute, &c.StartupTimeout},
		{"HEALTH_TIMEOUT", "1s", 5 * time.Second, &c.HealthTimeout},
		{"SHUTDOWN_TIMEOUT", "10s", time.Minute, &c.ShutdownTimeout},
	}
	for _, field := range durations {
		v, e := time.ParseDuration(get(field.key, field.fallback))
		if e != nil || v <= 0 || v > field.max {
			return Config{}, fmt.Errorf("%s: outside allowed duration range", field.key)
		}
		*field.dest = v
	}
	return c, nil
}

func validateRedisURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || u == nil || (u.Scheme != "redis" && u.Scheme != "rediss") || u.Hostname() == "" || u.Port() == "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("REDIS_URL: expected redis URL with host, port and database")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("REDIS_URL: invalid port")
	}
	database, err := strconv.Atoi(strings.TrimPrefix(u.EscapedPath(), "/"))
	if err != nil || u.EscapedPath() != "/"+strconv.Itoa(database) || database < 0 || database > 15 {
		return fmt.Errorf("REDIS_URL: database must be an integer from 0 through 15")
	}
	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme == "redis" && !local {
		return fmt.Errorf("REDIS_URL: plaintext is only allowed on loopback")
	}
	if !local {
		if u.User == nil {
			return fmt.Errorf("REDIS_URL: remote TLS connection requires authentication")
		}
		if password, ok := u.User.Password(); !ok || password == "" {
			return fmt.Errorf("REDIS_URL: remote TLS connection requires authentication")
		}
	}
	return nil
}
