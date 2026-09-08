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
	DatabaseURL     string
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
	c := Config{Role: get("APP_ROLE", "api"), HTTPAddr: get("HTTP_ADDR", "127.0.0.1:8080"), DatabaseURL: get("DATABASE_URL", "")}
	if c.Role != "api" {
		return Config{}, fmt.Errorf("APP_ROLE: only api is implemented in this release")
	}
	host, port, err := net.SplitHostPort(c.HTTPAddr)
	p, portErr := strconv.Atoi(port)
	if err != nil || net.ParseIP(host) == nil || portErr != nil || p < 0 || p > 65535 {
		return Config{}, fmt.Errorf("HTTP_ADDR: expected IP:port")
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
