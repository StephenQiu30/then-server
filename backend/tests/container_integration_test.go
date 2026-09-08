//go:build container

package tests

import (
	"context"
	"crypto/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	dockercontainer "github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestContainerRuntime(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	image := os.Getenv("THEN_BACKEND_TEST_IMAGE")
	if image == "" {
		t.Fatal("THEN_BACKEND_TEST_IMAGE must name an explicitly built local image")
	}
	docker, err := testcontainers.NewDockerClient()
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Close()
	info, err := docker.ImageInspect(ctx, image)
	if err != nil {
		t.Fatal("build the local image before this test")
	}
	// Resolve the mutable local tag once; every test container uses this immutable ID.
	image = info.ID
	password := rand.Text()
	db, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:18.4@sha256:a02db8cac496f15b094798a38254f14d6e00741f709360e5e00bb6668ea31636",
			Env:          map[string]string{"POSTGRES_DB": "then_test", "POSTGRES_USER": "then_test", "POSTGRES_PASSWORD": password},
			ExposedPorts: []string{"5432/tcp", "8080/tcp"},
			WaitingFor:   wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(time.Minute),
		}, Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func(c testcontainers.Container) {
		t.Helper()
		cleanupCtx, stop := context.WithTimeout(context.Background(), 20*time.Second)
		defer stop()
		if err := c.Terminate(cleanupCtx); err != nil {
			t.Error(err)
		}
	}
	defer cleanup(db)
	host, err := db.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := db.MappedPort(ctx, "8080/tcp")
	if err != nil {
		t.Fatal(err)
	}
	dsn := url.URL{Scheme: "postgres", User: url.UserPassword("then_test", password), Host: "127.0.0.1:5432", Path: "/then_test", RawQuery: "sslmode=disable"}
	api, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: image,
			Env:   map[string]string{"DATABASE_URL": dsn.String(), "HTTP_ADDR": "0.0.0.0:8080", "APP_ROLE": "api"},
			HostConfigModifier: func(h *dockercontainer.HostConfig) {
				h.NetworkMode = dockercontainer.NetworkMode("container:" + db.GetContainerID())
				h.ReadonlyRootfs = true
				h.CapDrop = []string{"ALL"}
				h.SecurityOpt = []string{"no-new-privileges:true"}
				limit := int64(64)
				h.Resources = dockercontainer.Resources{Memory: 256 * 1024 * 1024, NanoCPUs: 1000000000, PidsLimit: &limit}
			},
		}, Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup(api)
	client := &http.Client{Timeout: time.Second}
	base := "http://" + net.JoinHostPort(host, port.Port())
	deadline := time.Now().Add(15 * time.Second)
	for {
		response, e := client.Get(base + "/v1/health/ready")
		if e == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("restricted API container did not become ready")
		}
		time.Sleep(100 * time.Millisecond)
	}
	response, err := client.Get(base + "/v1/health/live")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("liveness failed")
	}
	inspected, err := api.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Config.User != "65532:65532" || !inspected.HostConfig.ReadonlyRootfs {
		t.Fatal("runtime security settings differ from contract")
	}
	timeout := 12 * time.Second
	if err := api.Stop(ctx, &timeout); err != nil {
		t.Fatal(err)
	}
	inspected, err = api.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.State.ExitCode != 0 {
		t.Fatalf("SIGTERM exit code: %d", inspected.State.ExitCode)
	}
	for _, role := range []string{"worker", "all"} {
		t.Run(role+" rejected", func(t *testing.T) {
			unsupported, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: image, Env: map[string]string{"APP_ROLE": role}}, Started: false})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup(unsupported)
			if err := unsupported.Start(ctx); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(5 * time.Second)
			for {
				state, err := unsupported.Inspect(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if !state.State.Running {
					if state.State.ExitCode == 0 {
						t.Fatal("unsupported role exited successfully")
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("unsupported role did not exit")
				}
				time.Sleep(50 * time.Millisecond)
			}
		})
	}
	t.Logf("image %s: ready/live 200, UID/GID 65532, readonly rootfs, SIGTERM exit 0", image)
}
