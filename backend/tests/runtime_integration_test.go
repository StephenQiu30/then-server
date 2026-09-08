//go:build integration

package tests

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/StephenQiu30/then/backend/internal/platform/config"
	"github.com/StephenQiu30/then/backend/internal/platform/database"
	"github.com/StephenQiu30/then/backend/internal/transport"
	dockerclient "github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresDisconnectRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	password := rand.Text()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:18.4@sha256:a02db8cac496f15b094798a38254f14d6e00741f709360e5e00bb6668ea31636",
			Env:          map[string]string{"POSTGRES_USER": "then_test", "POSTGRES_DB": "then_test", "POSTGRES_PASSWORD": password},
			ExposedPorts: []string{"5432/tcp"},
			WaitingFor:   wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(time.Minute),
		}, Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		if err := container.Terminate(cleanup); err != nil {
			t.Error(err)
		}
	}()
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}
	u := url.URL{Scheme: "postgres", User: url.UserPassword("then_test", password), Host: net.JoinHostPort(host, port.Port()), Path: "/then_test", RawQuery: "sslmode=disable"}
	cfg, err := config.Load(func(key string) (string, bool) {
		if key == "DATABASE_URL" {
			return u.String(), true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "then-backend")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	wrongPassword := u
	wrongPassword.User = url.UserPassword("then_test", "invalid-"+password)
	for _, failure := range []struct {
		name   string
		env    []string
		reason string
	}{
		{"missing database", []string{"APP_ROLE=api"}, "DATABASE_URL"},
		{"unsupported worker", []string{"APP_ROLE=worker"}, "APP_ROLE"},
		{"unsupported all", []string{"APP_ROLE=all"}, "APP_ROLE"},
		{"invalid credentials", []string{"DATABASE_URL=" + wrongPassword.String()}, "database unavailable"},
		{"occupied listener", []string{"DATABASE_URL=" + u.String(), "HTTP_ADDR=" + occupied.Addr().String()}, "HTTP listen failed"},
	} {
		t.Run(failure.name, func(t *testing.T) {
			attempt, stop := context.WithTimeout(ctx, 10*time.Second)
			defer stop()
			command := exec.CommandContext(attempt, binary)
			command.Env = failure.env
			output, err := command.CombinedOutput()
			if attempt.Err() != nil {
				t.Fatal("startup failure did not exit within deadline")
			}
			if err == nil {
				t.Fatal("invalid startup returned success")
			}
			if strings.Contains(string(output), password) || strings.Contains(string(output), "postgres://") {
				t.Fatal("startup log exposed database credentials")
			}
			if strings.Contains(string(output), "api_started") {
				t.Fatal("failed startup reported started")
			}
			if !strings.Contains(string(output), failure.reason) {
				t.Fatal("startup did not return expected safe reason")
			}
		})
	}
	process := exec.CommandContext(ctx, binary)
	process.Env = []string{"DATABASE_URL=" + u.String(), "HTTP_ADDR=127.0.0.1:0", "APP_ROLE=api"}
	stdout, err := process.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	defer process.Process.Kill()
	started := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			var entry struct {
				Message string `json:"msg"`
				Address string `json:"address"`
			}
			if json.Unmarshal(scanner.Bytes(), &entry) == nil && entry.Message == "api_started" {
				started <- entry.Address
			}
		}
		close(started)
	}()
	select {
	case address := <-started:
		if address == "" {
			t.Fatal("process did not start")
		}
	case <-ctx.Done():
		t.Fatal("startup timeout")
	}
	if err := process.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan error, 1)
	go func() { stopped <- process.Wait() }()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(12 * time.Second):
		t.Fatal("SIGTERM did not exit")
	}
	t.Log("built API process starts with PostgreSQL 18 and exits successfully on SIGTERM")
	pool, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	spec, err := os.ReadFile("../openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	router, err := transport.NewRouter(ctx, spec, pool, time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router)
	defer server.Close()
	client := &http.Client{Timeout: 2 * time.Second}
	status := func(path string) int {
		t.Helper()
		response, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		return response.StatusCode
	}
	if got := status("/v1/health/ready"); got != 200 {
		t.Fatalf("initial readiness %d", got)
	}
	docker, err := testcontainers.NewDockerClient()
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Close()
	if _, err := docker.ContainerPause(ctx, container.GetContainerID(), dockerclient.ContainerPauseOptions{}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = docker.ContainerUnpause(cleanup, container.GetContainerID(), dockerclient.ContainerUnpauseOptions{})
	}()
	if got := status("/v1/health/ready"); got != 503 {
		t.Fatalf("disconnected readiness %d", got)
	}
	if got := status("/v1/health/live"); got != 200 {
		t.Fatalf("disconnected liveness %d", got)
	}
	if _, err := docker.ContainerUnpause(ctx, container.GetContainerID(), dockerclient.ContainerUnpauseOptions{}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for status("/v1/health/ready") != 200 {
		if time.Now().After(deadline) {
			t.Fatal("readiness did not recover")
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Log("PostgreSQL 18: ready 200 -> disconnected 503; live 200; recovered ready 200")
}
