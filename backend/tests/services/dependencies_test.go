//go:build services

package services

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/platform/ratelimit"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
)

func TestServicesPostgresRollback(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", environment.databaseURL)
	serviceOK(t, "open database", err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	var version int
	serviceOK(t, "database version", db.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::int").Scan(&version))
	if version/10000 != 18 {
		t.Fatal("expected PostgreSQL 18")
	}
	_, err = db.ExecContext(ctx, "CREATE TEMP TABLE service_probe (id integer PRIMARY KEY, metadata jsonb NOT NULL)")
	serviceOK(t, "create temporary probe", err)
	tx, err := db.BeginTx(ctx, nil)
	serviceOK(t, "begin transaction", err)
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO service_probe VALUES (1, '{"synthetic":true}')`)
	serviceOK(t, "transaction write", err)
	serviceOK(t, "rollback", tx.Rollback())
	var count int
	serviceOK(t, "read rollback result", db.QueryRowContext(ctx, "SELECT count(*) FROM service_probe").Scan(&count))
	if count != 0 {
		t.Fatal("rolled-back row remains")
	}
}

func TestServicesMinIOPrivateLifecycle(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := minio.New(environment.minioEndpoint, &minio.Options{
		Creds: credentials.NewStaticV4(
			environment.minioAccessKey,
			environment.minioSecretKey,
			"",
		),
		Secure: false,
	})
	serviceOK(t, "create object client", err)
	bucket := serviceID(t)
	serviceOK(t, "create private bucket", client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: "us-east-1"}))
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := client.RemoveObject(cleanup, bucket, "synthetic.txt", minio.RemoveObjectOptions{}); err != nil {
			t.Error("object cleanup failed")
		}
		if err := client.RemoveBucket(cleanup, bucket); err != nil {
			t.Error("bucket cleanup failed")
		}
	})
	data := []byte("synthetic object lifecycle probe")
	_, err = client.PutObject(ctx, bucket, "synthetic.txt", bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: "text/plain"})
	serviceOK(t, "put object", err)
	obj, err := client.GetObject(ctx, bucket, "synthetic.txt", minio.GetObjectOptions{})
	serviceOK(t, "get object", err)
	actual, readErr := io.ReadAll(io.LimitReader(obj, 1024))
	closeErr := obj.Close()
	serviceOK(t, "read object", readErr)
	serviceOK(t, "close object", closeErr)
	if !bytes.Equal(data, actual) {
		t.Fatal("object bytes differ")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+environment.minioEndpoint+"/"+bucket+"/synthetic.txt", nil)
	serviceOK(t, "build anonymous request", err)
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Do(req)
	serviceOK(t, "anonymous request", err)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatal("private object did not reject anonymous access")
	}
	serviceOK(t, "delete object", client.RemoveObject(ctx, bucket, "synthetic.txt", minio.RemoveObjectOptions{}))
	_, err = client.StatObject(ctx, bucket, "synthetic.txt", minio.StatObjectOptions{})
	if minio.ToErrorResponse(err).Code != "NoSuchKey" {
		t.Fatal("deleted object still accessible or deletion result unverified")
	}
}

func TestServicesRedisTTL(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	client := redis.NewClient(&redis.Options{Addr: environment.redisAddr, Password: environment.redisPassword, DB: environment.redisDB, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second})
	defer client.Close()
	key := serviceID(t)
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 2*time.Second)
		defer done()
		if err := client.Del(cleanup, key).Err(); err != nil {
			t.Error("cache cleanup failed")
		}
	}()
	serviceOK(t, "set expiring cache", client.Set(ctx, key, "synthetic", 500*time.Millisecond).Err())
	value, err := client.Get(ctx, key).Result()
	serviceOK(t, "read cache", err)
	if value != "synthetic" {
		t.Fatal("cache value differs")
	}
	for {
		_, err = client.Get(ctx, key).Result()
		if errors.Is(err, redis.Nil) {
			break
		}
		serviceOK(t, "wait for expiry", err)
		select {
		case <-ctx.Done():
			t.Fatal("cache did not expire")
		case <-time.After(50 * time.Millisecond):
		}
	}
	if environment.redisPassword != "" {
		unauth := redis.NewClient(&redis.Options{Addr: environment.redisAddr, DB: environment.redisDB, MaxRetries: -1})
		defer unauth.Close()
		if err := unauth.Ping(ctx).Err(); err == nil {
			t.Fatal("Redis allowed unauthenticated access")
		}
	}
}

func TestServicesRedisAuthenticationRateLimitIsAtomicAndPrivate(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	connection := url.URL{Scheme: "redis", Host: environment.redisAddr, Path: "/" + strconv.Itoa(environment.redisDB)}
	if environment.redisPassword != "" {
		connection.User = url.UserPassword("", environment.redisPassword)
	}
	limiter, err := ratelimit.Open(ctx, connection.String())
	serviceOK(t, "open authentication rate limiter", err)
	defer limiter.Close()
	scope := serviceID(t)
	prefix := "then:auth-rate:" + scope + ":"
	inspector := redis.NewClient(&redis.Options{Addr: environment.redisAddr, Password: environment.redisPassword, DB: environment.redisDB, DialTimeout: 2 * time.Second})
	t.Cleanup(func() {
		defer inspector.Close()
		cleanup, done := context.WithTimeout(context.Background(), 2*time.Second)
		defer done()
		keys, _, err := inspector.Scan(cleanup, 0, prefix+"*", 100).Result()
		if err != nil {
			t.Error("rate-limit key scan cleanup failed")
			return
		}
		if len(keys) > 0 && inspector.Del(cleanup, keys...).Err() != nil {
			t.Error("rate-limit key cleanup failed")
		}
	})
	const subject = "192.0.2.55"
	const attempts = 32
	const limit = 10
	results := make(chan bool, attempts)
	errorsFound := make(chan error, attempts)
	var group sync.WaitGroup
	for range attempts {
		group.Add(1)
		go func() {
			defer group.Done()
			allowed, retryAfter, err := limiter.Allow(ctx, scope, subject, limit, time.Minute)
			if err != nil {
				errorsFound <- err
				return
			}
			if retryAfter <= 0 || retryAfter > time.Minute {
				errorsFound <- errors.New("rate-limit TTL is outside the configured window")
				return
			}
			results <- allowed
		}()
	}
	group.Wait()
	close(results)
	close(errorsFound)
	if len(errorsFound) != 0 {
		t.Fatal("concurrent rate-limit operation failed")
	}
	allowed := 0
	for result := range results {
		if result {
			allowed++
		}
	}
	if allowed != limit {
		t.Fatalf("allowed attempts=%d expected=%d", allowed, limit)
	}
	keys, _, err := inspector.Scan(ctx, 0, prefix+"*", 100).Result()
	serviceOK(t, "inspect authentication rate-limit key", err)
	if len(keys) != 1 || strings.Contains(keys[0], subject) {
		t.Fatal("rate-limit state did not use exactly one non-identifying key")
	}
}
