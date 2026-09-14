//go:build services

package tests

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

func serviceSetting(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
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

func TestServicesPostgresRollback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", serviceSetting("THEN_TEST_DATABASE_URL", "postgres://127.0.0.1/postgres?sslmode=disable"))
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := minio.New(serviceSetting("THEN_TEST_MINIO_ENDPOINT", "127.0.0.1:9000"), &minio.Options{
		Creds: credentials.NewStaticV4(
			serviceSetting("THEN_TEST_MINIO_ACCESS_KEY", "minioadmin"),
			serviceSetting("THEN_TEST_MINIO_SECRET_KEY", "minioadmin"),
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+serviceSetting("THEN_TEST_MINIO_ENDPOINT", "127.0.0.1:9000")+"/"+bucket+"/synthetic.txt", nil)
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
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	password := serviceSetting("THEN_TEST_REDIS_PASSWORD", "")
	client := redis.NewClient(&redis.Options{Addr: serviceSetting("THEN_TEST_REDIS_ADDR", "127.0.0.1:6379"), Password: password, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second})
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
	if password != "" {
		unauth := redis.NewClient(&redis.Options{Addr: serviceSetting("THEN_TEST_REDIS_ADDR", "127.0.0.1:6379"), MaxRetries: -1})
		defer unauth.Close()
		if err := unauth.Ping(ctx).Err(); err == nil {
			t.Fatal("Redis allowed unauthenticated access")
		}
	}
}

func TestServicesRabbitConfirmationAndRedelivery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	connection, err := amqp.DialConfig(serviceSetting("THEN_TEST_RABBITMQ_URL", "amqp://guest:guest@127.0.0.1:5672/"), amqp.Config{
		Dial: func(network, address string) (net.Conn, error) {
			conn, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			if err := conn.SetDeadline(time.Now().Add(20 * time.Second)); err != nil {
				conn.Close()
				return nil, err
			}
			return conn, nil
		},
	})
	serviceOK(t, "connect RabbitMQ", err)
	defer connection.Close()
	channel, err := connection.Channel()
	serviceOK(t, "open channel", err)
	defer channel.Close()
	queue, err := channel.QueueDeclare(serviceID(t), true, false, false, false, amqp.Table{"x-queue-type": "quorum"})
	serviceOK(t, "declare quorum queue", err)
	defer func() {
		if _, err := channel.QueueDelete(queue.Name, false, false, false); err != nil {
			t.Error("queue cleanup failed")
		}
	}()
	serviceOK(t, "enable confirms", channel.Confirm(false))
	confirm, err := channel.PublishWithDeferredConfirmWithContext(ctx, "", queue.Name, true, false, amqp.Publishing{DeliveryMode: amqp.Persistent, ContentType: "text/plain", MessageId: queue.Name, Body: []byte("synthetic")})
	serviceOK(t, "publish persistent message", err)
	if confirm == nil {
		t.Fatal("publisher confirmation unavailable")
	}
	acked, err := confirm.WaitContext(ctx)
	serviceOK(t, "publisher confirmation", err)
	if !acked {
		t.Fatal("broker rejected publication")
	}
	receive := func() amqp.Delivery {
		t.Helper()
		for {
			message, found, err := channel.Get(queue.Name, false)
			serviceOK(t, "receive message", err)
			if found {
				return message
			}
			select {
			case <-ctx.Done():
				t.Fatal("message delivery timed out")
			case <-time.After(30 * time.Millisecond):
			}
		}
	}
	first := receive()
	if first.MessageId != queue.Name || string(first.Body) != "synthetic" {
		t.Fatal("message identity differs")
	}
	serviceOK(t, "requeue uncompleted delivery", first.Nack(false, true))
	second := receive()
	if !second.Redelivered || second.MessageId != first.MessageId {
		t.Fatal("redelivery identity not preserved")
	}
	serviceOK(t, "ack completed delivery", second.Ack(false))
	// A following synchronous RPC establishes ordering after the acknowledgement.
	remaining, err := channel.QueueInspect(queue.Name)
	serviceOK(t, "inspect completed queue", err)
	if remaining.Messages != 0 {
		t.Fatal("acknowledged message remains ready")
	}
}
