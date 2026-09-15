//go:build integration

package tests

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestActualBinarySyntheticMediaLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	postgresPassword, minioPassword := rand.Text(), rand.Text()
	postgresContainer := integrationContainer(t, ctx, testcontainers.ContainerRequest{Image: "postgres:18.4@sha256:a02db8cac496f15b094798a38254f14d6e00741f709360e5e00bb6668ea31636", Env: map[string]string{"POSTGRES_USER": "then_test", "POSTGRES_DB": "then_test", "POSTGRES_PASSWORD": postgresPassword}, ExposedPorts: []string{"5432/tcp"}, WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(time.Minute)})
	redisContainer := integrationContainer(t, ctx, testcontainers.ContainerRequest{Image: "redis:8.10.0@sha256:344e3945a0b431c8ff1eecd58c5573538126bd756f02fc7e218ddf1fc2546366", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForLog("Ready to accept connections").WithStartupTimeout(time.Minute)})
	minioContainer := integrationContainer(t, ctx, testcontainers.ContainerRequest{Image: "quay.io/minio/minio:RELEASE.2025-09-07T16-13-09Z@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e", Cmd: []string{"server", "/data"}, Env: map[string]string{"MINIO_ROOT_USER": "then_test", "MINIO_ROOT_PASSWORD": minioPassword}, ExposedPorts: []string{"9000/tcp"}, WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").WithStartupTimeout(time.Minute)})
	rabbitContainer := integrationContainer(t, ctx, testcontainers.ContainerRequest{Image: "rabbitmq:4.3.5-management@sha256:57bddb6fbc3498b5d8b5a14dc6f4506073ebcf94c66ba2a7678c335faa8dd631", Env: map[string]string{"RABBITMQ_DEFAULT_USER": "then_test", "RABBITMQ_DEFAULT_PASS": "then_test", "RABBITMQ_DEFAULT_VHOST": "then_test"}, ExposedPorts: []string{"5672/tcp"}, WaitingFor: wait.ForLog("Server startup complete").WithStartupTimeout(time.Minute)})

	postgresAddress := mappedAddress(t, ctx, postgresContainer, "5432/tcp")
	redisAddress := mappedAddress(t, ctx, redisContainer, "6379/tcp")
	minioAddress := mappedAddress(t, ctx, minioContainer, "9000/tcp")
	rabbitAddress := mappedAddress(t, ctx, rabbitContainer, "5672/tcp")
	binary := filepath.Join(t.TempDir(), "then-backend")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build actual binary: %v %s", err, output)
	}
	process := exec.CommandContext(ctx, binary)
	process.Env = []string{
		"APP_ROLE=all", "HTTP_ADDR=127.0.0.1:0", "API_DOCS_ENABLED=true",
		"DATABASE_URL=postgres://then_test:" + url.QueryEscape(postgresPassword) + "@" + postgresAddress + "/then_test?sslmode=disable",
		"REDIS_URL=redis://" + redisAddress + "/0", "MEDIA_DEVELOPMENT_ENABLED=true",
		"MINIO_ENDPOINT=" + minioAddress, "MINIO_ACCESS_KEY=then_test", "MINIO_SECRET_KEY=" + minioPassword,
		"MINIO_SECURE=false", "RABBITMQ_URL=amqp://then_test:then_test@" + rabbitAddress + "/then_test",
	}
	stdout, err := process.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	defer process.Process.Kill()
	started := make(chan string, 1)
	logs := new(bytes.Buffer)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Bytes()
			logs.Write(line)
			var entry struct {
				Message string `json:"msg"`
				Address string `json:"address"`
			}
			if json.Unmarshal(line, &entry) == nil && entry.Message == "api_started" {
				started <- entry.Address
			}
		}
		close(started)
	}()
	var address string
	select {
	case address = <-started:
		if address == "" {
			t.Fatalf("actual all-role process did not start: %s", logs.String())
		}
	case <-ctx.Done():
		t.Fatal("actual all-role process startup timed out")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 10 * time.Second, Jar: jar}
	baseURL := "http://" + address
	postJSON(t, client, http.MethodPost, baseURL+"/v1/auth/registrations", map[string]any{"email": "integration@example.test", "display_name": "Integration", "password": "correct-password-integration"}, http.StatusCreated, nil)
	postJSON(t, client, http.MethodPut, baseURL+"/v1/privacy/self-adult-declaration", map[string]any{"policy_version": "self-adult-v1", "confirms_self_and_adult": true}, http.StatusOK, nil)
	var consent struct {
		ID string `json:"id"`
	}
	postJSON(t, client, http.MethodPost, baseURL+"/v1/consents", map[string]any{"purpose": "avatar_source_preparation", "category": "person_photo", "processor": "then", "region": "local-development", "policy_version": "person-photo-v1", "max_retention_hours": 24, "actively_agreed": true, "training_allowed": false}, http.StatusCreated, &consent)
	photo := runtimeSyntheticJPEG(t)
	digest := fmt.Sprintf("%x", sha256.Sum256(photo))
	var upload struct {
		Media struct {
			ID string `json:"id"`
		} `json:"media"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	postJSON(t, client, http.MethodPost, baseURL+"/v1/media/uploads", map[string]any{"consent_id": consent.ID, "purpose": "avatar_source_preparation", "content_type": "image/jpeg", "byte_size": len(photo), "sha256": digest}, http.StatusCreated, &upload)
	put, err := http.NewRequestWithContext(ctx, http.MethodPut, upload.URL, bytes.NewReader(photo))
	if err != nil {
		t.Fatal(err)
	}
	put.ContentLength = int64(len(photo))
	for key, value := range upload.Headers {
		if key != "Content-Length" {
			put.Header.Set(key, value)
		}
	}
	putResponse, err := client.Do(put)
	if err != nil {
		t.Fatal("signed PUT failed")
	}
	putResponse.Body.Close()
	versionID := putResponse.Header.Get("X-Amz-Version-Id")
	if putResponse.StatusCode != http.StatusOK || versionID == "" {
		t.Fatal("signed PUT did not produce a version")
	}
	postJSON(t, client, http.MethodPost, baseURL+"/v1/media/"+upload.Media.ID+"/complete", map[string]any{"version_id": versionID}, http.StatusOK, nil)
	waitForHTTPStatus(t, ctx, client, baseURL+"/v1/media/"+upload.Media.ID, "ready")
	var deletion struct {
		ID string `json:"id"`
	}
	postJSON(t, client, http.MethodDelete, baseURL+"/v1/media/"+upload.Media.ID, nil, http.StatusAccepted, &deletion)
	waitForHTTPStatus(t, ctx, client, baseURL+"/v1/deletion-requests/"+deletion.ID, "complete")
	if err := process.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := process.Wait(); err != nil {
		t.Fatalf("actual all-role process did not exit cleanly: %v", err)
	}
	if strings.Contains(logs.String(), postgresPassword) || strings.Contains(logs.String(), minioPassword) || strings.Contains(logs.String(), upload.URL) {
		t.Fatal("actual process logs exposed credentials or signed URL")
	}
}

func integrationContainer(t *testing.T, ctx context.Context, request testcontainers.ContainerRequest) testcontainers.Container {
	t.Helper()
	container, err := createTestContainer(t, ctx, testcontainers.GenericContainerRequest{ContainerRequest: request, Started: true})
	if err != nil {
		t.Fatal(err)
	}
	return container
}

func mappedAddress(t *testing.T, ctx context.Context, container testcontainers.Container, port string) string {
	t.Helper()
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mapped, err := container.MappedPort(ctx, port)
	if err != nil {
		t.Fatal(err)
	}
	return net.JoinHostPort(host, mapped.Port())
}

func postJSON(t *testing.T, client *http.Client, method, endpoint string, body any, expected int, output any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequest(method, endpoint, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("%s request failed", method)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != expected {
		t.Fatalf("%s %s status=%d body=%s", method, endpoint, response.StatusCode, data)
	}
	if output != nil && json.Unmarshal(data, output) != nil {
		t.Fatal("response JSON invalid")
	}
}

func waitForHTTPStatus(t *testing.T, ctx context.Context, client *http.Client, endpoint, expected string) {
	t.Helper()
	for {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		response, err := client.Do(request)
		if err == nil {
			var body struct{ Status, Reason string }
			decodeErr := json.NewDecoder(response.Body).Decode(&body)
			response.Body.Close()
			if response.StatusCode == http.StatusOK && decodeErr == nil && body.Status == expected {
				return
			}
			if body.Status == "rejected" || body.Status == "failed" {
				t.Fatalf("terminal failure: %s", body.Reason)
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal("HTTP resource did not reach expected status")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func runtimeSyntheticJPEG(t *testing.T) []byte {
	t.Helper()
	pixels := image.NewRGBA(image.Rect(0, 0, 24, 36))
	for y := range 36 {
		for x := range 24 {
			pixels.Set(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 5), B: 140, A: 255})
		}
	}
	var output bytes.Buffer
	if err := jpeg.Encode(&output, pixels, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
