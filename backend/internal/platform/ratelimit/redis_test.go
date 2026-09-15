package ratelimit

import (
	"strings"
	"testing"
)

func TestRateLimitKeyIsStableAndDoesNotContainSubject(t *testing.T) {
	const subject = "192.0.2.42"
	first := rateLimitKey("login", subject)
	second := rateLimitKey("login", subject)
	registration := rateLimitKey("registration", subject)
	if first != second || first == registration {
		t.Fatal("rate-limit key identity is not stable and scope-specific")
	}
	if strings.Contains(first, subject) || !strings.HasPrefix(first, "then:auth-rate:login:") {
		t.Fatal("rate-limit key exposed the raw subject or used an unexpected namespace")
	}
}

func TestRedisIntegerRejectsUnexpectedResultTypes(t *testing.T) {
	if value, ok := redisInteger(int64(7)); !ok || value != 7 {
		t.Fatal("valid Redis integer was rejected")
	}
	if _, ok := redisInteger("7"); ok {
		t.Fatal("unexpected Redis result type was accepted")
	}
}
