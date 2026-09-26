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

// GenerationMode identifies the execution boundary for the optional image and
// model generation path.
type GenerationMode string

const (
	GenerationModeOff    GenerationMode = "off"
	GenerationModeLocal  GenerationMode = "local"
	GenerationModeRemote GenerationMode = "remote"
)

// GenerationConfig is the single process-level policy for the optional image
// and model generation path. Provider calls remain disabled until the
// documented GATE/POC/worker work is complete.
type GenerationConfig struct {
	Mode                  GenerationMode
	Enabled               bool
	ProviderCallsEnabled  bool
	LocalImageEndpoint    string
	Currency              string
	MaxConcurrentTasks    int
	MaxQuotaUnits         int
	MaxBudgetMinorUnits   int64
	MaxSubmissionAttempts int
	ProviderTimeout       time.Duration
	TaskTimeout           time.Duration
	Retention             time.Duration
}

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
	Generation              GenerationConfig
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
	generation, err := loadGenerationConfig(get)
	if err != nil {
		return Config{}, err
	}
	c.Generation = generation
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

func loadGenerationConfig(get func(string, string) string) (GenerationConfig, error) {
	enabled, err := parseBool("GENERATION_ENABLED", get("GENERATION_ENABLED", "false"))
	if err != nil {
		return GenerationConfig{}, err
	}
	mode, err := parseGenerationMode(get("GENERATION_MODE", ""), enabled)
	if err != nil {
		return GenerationConfig{}, err
	}
	providerCalls, err := parseBool("GENERATION_PROVIDER_CALLS_ENABLED", get("GENERATION_PROVIDER_CALLS_ENABLED", "false"))
	if err != nil {
		return GenerationConfig{}, err
	}
	maxConcurrentTasks, err := parseOptionalInt("GENERATION_MAX_CONCURRENT_TASKS", get("GENERATION_MAX_CONCURRENT_TASKS", "0"))
	if err != nil {
		return GenerationConfig{}, err
	}
	maxQuotaUnits, err := parseOptionalInt("GENERATION_MAX_QUOTA_UNITS", get("GENERATION_MAX_QUOTA_UNITS", "0"))
	if err != nil {
		return GenerationConfig{}, err
	}
	maxBudgetMinorUnits, err := parseOptionalInt64("GENERATION_MAX_BUDGET_MINOR_UNITS", get("GENERATION_MAX_BUDGET_MINOR_UNITS", "0"))
	if err != nil {
		return GenerationConfig{}, err
	}
	maxSubmissionAttempts, err := parseOptionalInt("GENERATION_MAX_SUBMISSION_ATTEMPTS", get("GENERATION_MAX_SUBMISSION_ATTEMPTS", "0"))
	if err != nil {
		return GenerationConfig{}, err
	}
	providerTimeout, err := parseOptionalDuration("GENERATION_PROVIDER_TIMEOUT", get("GENERATION_PROVIDER_TIMEOUT", "0s"))
	if err != nil {
		return GenerationConfig{}, err
	}
	taskTimeout, err := parseOptionalDuration("GENERATION_TASK_TIMEOUT", get("GENERATION_TASK_TIMEOUT", "0s"))
	if err != nil {
		return GenerationConfig{}, err
	}
	retention, err := parseOptionalDuration("GENERATION_RETENTION", get("GENERATION_RETENTION", "0s"))
	if err != nil {
		return GenerationConfig{}, err
	}
	configuration := GenerationConfig{
		Mode:                  mode,
		Enabled:               mode != GenerationModeOff,
		ProviderCallsEnabled:  providerCalls,
		LocalImageEndpoint:    get("GENERATION_LOCAL_IMAGE_ENDPOINT", ""),
		Currency:              get("GENERATION_CURRENCY", ""),
		MaxConcurrentTasks:    maxConcurrentTasks,
		MaxQuotaUnits:         maxQuotaUnits,
		MaxBudgetMinorUnits:   maxBudgetMinorUnits,
		MaxSubmissionAttempts: maxSubmissionAttempts,
		ProviderTimeout:       providerTimeout,
		TaskTimeout:           taskTimeout,
		Retention:             retention,
	}
	if err := validateGenerationConfig(configuration); err != nil {
		return GenerationConfig{}, err
	}
	return configuration, nil
}

func validateGenerationConfig(configuration GenerationConfig) error {
	if configuration.ProviderCallsEnabled {
		return fmt.Errorf("GENERATION_PROVIDER_CALLS_ENABLED: provider calls remain disabled until 14-01 GATE/POC/WORKER completion")
	}
	if configuration.Mode == GenerationModeOff {
		if configuration.Enabled || configuration.LocalImageEndpoint != "" {
			return fmt.Errorf("GENERATION_MODE: off cannot enable generation or configure a local image endpoint")
		}
		return nil
	}
	if configuration.Mode == GenerationModeLocal {
		if configuration.Currency != "" || configuration.MaxBudgetMinorUnits != 0 {
			return fmt.Errorf("GENERATION_*: local mode requires empty currency and zero budget")
		}
		if configuration.MaxConcurrentTasks < 1 || configuration.MaxConcurrentTasks > 1000 || configuration.MaxQuotaUnits < 1 || configuration.MaxQuotaUnits > 1_000_000 || configuration.MaxSubmissionAttempts < 1 || configuration.MaxSubmissionAttempts > 10 || configuration.ProviderTimeout <= 0 || configuration.ProviderTimeout > 10*time.Minute || configuration.TaskTimeout < time.Minute || configuration.TaskTimeout > 24*time.Hour || configuration.Retention <= 0 || configuration.Retention > 365*24*time.Hour {
			return fmt.Errorf("GENERATION_*: local mode requires bounded concurrency, quota, attempts, timeout and retention")
		}
		if configuration.LocalImageEndpoint != "" {
			if err := validateLocalImageEndpoint(configuration.LocalImageEndpoint); err != nil {
				return err
			}
		}
		return nil
	}
	if configuration.Mode != GenerationModeRemote || !validCurrency(configuration.Currency) || configuration.MaxConcurrentTasks < 1 || configuration.MaxConcurrentTasks > 1000 || configuration.MaxQuotaUnits < 1 || configuration.MaxQuotaUnits > 1_000_000 || configuration.MaxBudgetMinorUnits < 1 || configuration.MaxBudgetMinorUnits > 10_000_000_000 || configuration.MaxSubmissionAttempts < 1 || configuration.MaxSubmissionAttempts > 10 || configuration.ProviderTimeout <= 0 || configuration.ProviderTimeout > 10*time.Minute || configuration.TaskTimeout < time.Minute || configuration.TaskTimeout > 24*time.Hour || configuration.Retention <= 0 || configuration.Retention > 365*24*time.Hour {
		return fmt.Errorf("GENERATION_*: remote mode requires bounded currency, budget, quota, attempts, timeout and retention")
	}
	if configuration.LocalImageEndpoint != "" {
		return fmt.Errorf("GENERATION_LOCAL_IMAGE_ENDPOINT: only local mode may configure a local image endpoint")
	}
	return nil
}

func parseGenerationMode(value string, legacyEnabled bool) (GenerationMode, error) {
	if value == "" {
		if legacyEnabled {
			return GenerationModeRemote, nil
		}
		return GenerationModeOff, nil
	}
	switch GenerationMode(value) {
	case GenerationModeOff, GenerationModeLocal, GenerationModeRemote:
		return GenerationMode(value), nil
	default:
		return "", fmt.Errorf("GENERATION_MODE: expected off, local or remote")
	}
}

func validateLocalImageEndpoint(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || parsed.Scheme != "http" || parsed.Hostname() == "" || parsed.Port() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !isLoopbackHost(parsed.Hostname()) {
		return fmt.Errorf("GENERATION_LOCAL_IMAGE_ENDPOINT: expected loopback HTTP URL with port")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("GENERATION_LOCAL_IMAGE_ENDPOINT: invalid port")
	}
	return nil
}

func parseBool(key, value string) (bool, error) {
	switch value {
	case "false":
		return false, nil
	case "true":
		return true, nil
	default:
		return false, fmt.Errorf("%s: expected true or false", key)
	}
}

func parseOptionalInt(key, value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s: expected a non-negative integer", key)
	}
	return parsed, nil
}

func parseOptionalInt64(key, value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s: expected a non-negative integer", key)
	}
	return parsed, nil
}

func parseOptionalDuration(key, value string) (time.Duration, error) {
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s: expected a non-negative duration", key)
	}
	return parsed, nil
}

func validCurrency(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
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
