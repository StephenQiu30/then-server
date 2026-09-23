// Package config validates process configuration without exposing secret values.
package config

import (
	"encoding/base64"
	"fmt"
	"net"
	mailaddr "net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Role                    string
	HTTPAddr                string
	DocsEnabled             bool
	SessionSecure           bool
	MailEnabled             bool
	MailSMTPAddr            string
	MailFrom                string
	MailAuthCode            string
	MailKey                 []byte
	MailLinkBase            string
	DatabaseURL             string
	RedisURL                string
	MediaDevelopmentEnabled bool
	MinIOEndpoint           string
	MinIOAccessKey          string
	MinIOSecretKey          string
	MinIOSecure             bool
	KafkaBrokers            []string
	KafkaTopicPrefix        string
	MaxOpenConns            int
	MaxIdleConns            int
	ConnMaxLifetime         time.Duration
	StartupTimeout          time.Duration
	HealthTimeout           time.Duration
	ShutdownTimeout         time.Duration
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
		Role:             get("APP_ROLE", "api"),
		HTTPAddr:         get("HTTP_ADDR", "127.0.0.1:8080"),
		DatabaseURL:      get("DATABASE_URL", ""),
		RedisURL:         get("REDIS_URL", "redis://127.0.0.1:6379/0"),
		MailSMTPAddr:     get("ACCOUNT_MAIL_SMTP_ADDR", "smtp.163.com:465"),
		MailFrom:         get("ACCOUNT_MAIL_FROM", ""),
		MailAuthCode:     get("ACCOUNT_MAIL_AUTH_CODE", ""),
		MailLinkBase:     get("ACCOUNT_MAIL_LINK_BASE", ""),
		MinIOEndpoint:    get("MINIO_ENDPOINT", "127.0.0.1:9000"),
		MinIOAccessKey:   get("MINIO_ACCESS_KEY", ""),
		MinIOSecretKey:   get("MINIO_SECRET_KEY", ""),
		KafkaBrokers:     strings.Split(get("KAFKA_BROKERS", "127.0.0.1:9092"), ","),
		KafkaTopicPrefix: get("KAFKA_TOPIC_PREFIX", "then"),
	}
	if c.Role != "api" && c.Role != "worker" && c.Role != "all" {
		return Config{}, fmt.Errorf("APP_ROLE: expected api, worker or all")
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
	switch get("MEDIA_DEVELOPMENT_ENABLED", "false") {
	case "false":
	case "true":
		c.MediaDevelopmentEnabled = true
	default:
		return Config{}, fmt.Errorf("MEDIA_DEVELOPMENT_ENABLED: expected true or false")
	}
	switch get("MINIO_SECURE", "false") {
	case "false":
	case "true":
		c.MinIOSecure = true
	default:
		return Config{}, fmt.Errorf("MINIO_SECURE: expected true or false")
	}
	if (c.Role == "worker" || c.Role == "all") && !c.MediaDevelopmentEnabled {
		return Config{}, fmt.Errorf("MEDIA_DEVELOPMENT_ENABLED: required for worker and all roles")
	}
	if c.MediaDevelopmentEnabled {
		if !net.ParseIP(host).IsLoopback() {
			return Config{}, fmt.Errorf("MEDIA_DEVELOPMENT_ENABLED: requires loopback HTTP_ADDR")
		}
		if c.MinIOAccessKey == "" || c.MinIOSecretKey == "" {
			return Config{}, fmt.Errorf("MINIO credentials: required for local media development")
		}
		minioHost, minioPort, splitErr := net.SplitHostPort(c.MinIOEndpoint)
		parsedPort, parseErr := strconv.Atoi(minioPort)
		if splitErr != nil || !isLoopbackHost(minioHost) || parseErr != nil || parsedPort < 1 || parsedPort > 65535 {
			return Config{}, fmt.Errorf("MINIO_ENDPOINT: local media development requires loopback IP:port")
		}
		if err := validateKafka(c.KafkaBrokers, c.KafkaTopicPrefix); err != nil {
			return Config{}, err
		}
	}
	if !c.SessionSecure && !net.ParseIP(host).IsLoopback() {
		return Config{}, fmt.Errorf("SESSION_COOKIE_SECURE: required for non-loopback HTTP_ADDR")
	}
	mailKey := get("ACCOUNT_MAIL_KEY", "")
	if c.MailFrom != "" || c.MailAuthCode != "" || mailKey != "" || c.MailLinkBase != "" {
		c.MailEnabled = true
		address, addressErr := mailaddr.ParseAddress(c.MailFrom)
		key, keyErr := base64.RawURLEncoding.DecodeString(mailKey)
		mailHost, mailPort, hostErr := net.SplitHostPort(c.MailSMTPAddr)
		link, linkErr := url.Parse(c.MailLinkBase)
		portNumber, portErr := strconv.Atoi(mailPort)
		if addressErr != nil || address.Address != c.MailFrom || c.MailAuthCode == "" || len(c.MailAuthCode) > 256 || strings.ContainsAny(c.MailAuthCode, "\r\n") || keyErr != nil || len(key) < 32 || hostErr != nil || (mailHost != "smtp.163.com" && !isLoopbackHost(mailHost)) || portErr != nil || portNumber < 1 || portNumber > 65535 || (mailHost == "smtp.163.com" && mailPort != "465") || linkErr != nil || link.Scheme != "https" || link.Host == "" || link.User != nil || link.Fragment != "" {
			return Config{}, fmt.Errorf("ACCOUNT_MAIL_*: complete TLS SMTP account, auth code, key and HTTPS link are required")
		}
		c.MailKey = key
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

func validateKafka(brokers []string, prefix string) error {
	if len(brokers) == 0 || len(brokers) > 8 {
		return fmt.Errorf("KAFKA_BROKERS: expected 1 through 8 loopback endpoints")
	}
	for _, address := range brokers {
		host, port, err := net.SplitHostPort(address)
		number, parseErr := strconv.Atoi(port)
		if err != nil || !isLoopbackHost(host) || parseErr != nil || number < 1 || number > 65535 {
			return fmt.Errorf("KAFKA_BROKERS: local development requires loopback host:port")
		}
	}
	if len(prefix) < 1 || len(prefix) > 80 || strings.Trim(prefix, "abcdefghijklmnopqrstuvwxyz0123456789-") != "" || prefix[0] == '-' || prefix[len(prefix)-1] == '-' {
		return fmt.Errorf("KAFKA_TOPIC_PREFIX: expected 1 through 80 lowercase letters, digits or internal hyphens")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
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
