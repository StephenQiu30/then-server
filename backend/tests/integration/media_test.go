//go:build integration

package integration

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

	"github.com/StephenQiu30/then-server/backend/tests/internal/testcontainer"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestActualBinarySyntheticMediaLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	postgresPassword, minioPassword := rand.Text(), rand.Text()
	postgresContainer := integrationContainer(t, ctx, testcontainers.ContainerRequest{Image: "postgres:18.4@sha256:a02db8cac496f15b094798a38254f14d6e00741f709360e5e00bb6668ea31636", Env: map[string]string{"POSTGRES_USER": "then_test", "POSTGRES_DB": "then_test", "POSTGRES_PASSWORD": postgresPassword}, ExposedPorts: []string{"5432/tcp"}, WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(time.Minute)})
	redisContainer := integrationContainer(t, ctx, testcontainers.ContainerRequest{Image: "redis:8.10.0@sha256:344e3945a0b431c8ff1eecd58c5573538126bd756f02fc7e218ddf1fc2546366", ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForLog("Ready to accept connections").WithStartupTimeout(time.Minute)})
	minioContainer := integrationContainer(t, ctx, testcontainers.ContainerRequest{Image: "quay.io/minio/minio:RELEASE.2025-09-07T16-13-09Z@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e", Cmd: []string{"server", "/data"}, Env: map[string]string{"MINIO_ROOT_USER": "then_test", "MINIO_ROOT_PASSWORD": minioPassword}, ExposedPorts: []string{"9000/tcp"}, WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").WithStartupTimeout(time.Minute)})
	_, kafkaAddress, err := testcontainer.Kafka(t, ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	postgresAddress := mappedAddress(t, ctx, postgresContainer, "5432/tcp")
	redisAddress := mappedAddress(t, ctx, redisContainer, "6379/tcp")
	minioAddress := mappedAddress(t, ctx, minioContainer, "9000/tcp")
	binary := filepath.Join(t.TempDir(), "then-backend")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "../..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build actual binary: %v %s", err, output)
	}
	process := exec.CommandContext(ctx, binary)
	process.Env = []string{
		"APP_ROLE=all", "HTTP_ADDR=127.0.0.1:0", "API_DOCS_ENABLED=true",
		"DATABASE_URL=postgres://then_test:" + url.QueryEscape(postgresPassword) + "@" + postgresAddress + "/then_test?sslmode=disable",
		"REDIS_URL=redis://" + redisAddress + "/0", "MEDIA_DEVELOPMENT_ENABLED=true",
		"MINIO_ENDPOINT=" + minioAddress, "MINIO_ACCESS_KEY=then_test", "MINIO_SECRET_KEY=" + minioPassword,
		"MINIO_SECURE=false", "KAFKA_BROKERS=" + kafkaAddress,
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
	var authorRegistration struct {
		User struct {
			ID       string `json:"id"`
			Revision int    `json:"revision"`
		} `json:"user"`
	}
	postJSON(t, client, http.MethodPost, baseURL+"/auth/registrations", map[string]any{"email": "integration@example.test", "display_name": "Integration", "password": "correct-password-integration"}, http.StatusCreated, &authorRegistration)
	postJSON(t, client, http.MethodPut, baseURL+"/privacy/self-adult-declaration", map[string]any{"policy_version": "self-adult-v1", "confirms_self_and_adult": true}, http.StatusOK, nil)
	var consent struct {
		ID string `json:"id"`
	}
	postJSON(t, client, http.MethodPost, baseURL+"/consents", map[string]any{"purpose": "avatar_source_preparation", "category": "person_photo", "processor": "then", "region": "local-development", "policy_version": "person-photo-v1", "max_retention_hours": 24, "actively_agreed": true, "training_allowed": false}, http.StatusCreated, &consent)
	photo := runtimeSyntheticJPEG(t)
	digest := fmt.Sprintf("%x", sha256.Sum256(photo))
	var upload struct {
		Media struct {
			ID string `json:"id"`
		} `json:"media"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	postJSON(t, client, http.MethodPost, baseURL+"/media/uploads", map[string]any{"consent_id": consent.ID, "purpose": "avatar_source_preparation", "content_type": "image/jpeg", "byte_size": len(photo), "sha256": digest}, http.StatusCreated, &upload)
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
	postJSON(t, client, http.MethodPost, baseURL+"/media/"+upload.Media.ID+"/complete", map[string]any{"version_id": versionID}, http.StatusOK, nil)
	waitForHTTPStatus(t, ctx, client, baseURL+"/media/"+upload.Media.ID, "ready")
	var deletion struct {
		ID string `json:"id"`
	}
	postJSON(t, client, http.MethodDelete, baseURL+"/media/"+upload.Media.ID, nil, http.StatusAccepted, &deletion)
	waitForHTTPStatus(t, ctx, client, baseURL+"/deletion-requests/"+deletion.ID, "complete")

	var diaryUpload struct {
		Media struct {
			ID string `json:"id"`
		} `json:"media"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	postJSON(t, client, http.MethodPost, baseURL+"/media/uploads", map[string]any{"purpose": "diary_image", "content_type": "image/jpeg", "byte_size": len(photo), "sha256": digest}, http.StatusCreated, &diaryUpload)
	put, err = http.NewRequestWithContext(ctx, http.MethodPut, diaryUpload.URL, bytes.NewReader(photo))
	if err != nil {
		t.Fatal(err)
	}
	put.ContentLength = int64(len(photo))
	for key, value := range diaryUpload.Headers {
		if key != "Content-Length" {
			put.Header.Set(key, value)
		}
	}
	putResponse, err = client.Do(put)
	if err != nil {
		t.Fatal("diary signed PUT failed")
	}
	putResponse.Body.Close()
	versionID = putResponse.Header.Get("X-Amz-Version-Id")
	if putResponse.StatusCode != http.StatusOK || versionID == "" {
		t.Fatal("diary signed PUT did not produce a version")
	}
	postJSON(t, client, http.MethodPost, baseURL+"/media/"+diaryUpload.Media.ID+"/complete", map[string]any{"version_id": versionID}, http.StatusOK, nil)
	waitForHTTPStatus(t, ctx, client, baseURL+"/media/"+diaryUpload.Media.ID, "ready")
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(location).Format("2006-01-02")
	entryID := "20000000-0000-4000-8000-000000000001"
	var diary struct {
		Revision int      `json:"revision"`
		MediaIDs []string `json:"media_ids"`
	}
	postJSON(t, client, http.MethodPost, baseURL+"/diary-entries", map[string]any{"id": entryID, "local_date": today, "time_zone": "Asia/Shanghai", "body": "合成图片日记", "media_ids": []string{diaryUpload.Media.ID}}, http.StatusCreated, &diary)
	if diary.Revision != 1 || len(diary.MediaIDs) != 1 {
		t.Fatal("actual diary create lost revision or media")
	}
	var calendar struct {
		Days []struct {
			LocalDate  string `json:"local_date"`
			DiaryCount int    `json:"diary_count"`
		} `json:"days"`
	}
	postJSON(t, client, http.MethodGet, baseURL+"/calendar?month="+today[:7], nil, http.StatusOK, &calendar)
	found := false
	for _, day := range calendar.Days {
		if day.LocalDate == today && day.DiaryCount == 1 {
			found = true
		}
	}
	if !found {
		t.Fatal("actual calendar did not count the private diary")
	}
	postJSON(t, client, http.MethodDelete, baseURL+"/diary-entries/"+entryID+"?expected_revision=1", nil, http.StatusNoContent, nil)
	postJSON(t, client, http.MethodDelete, baseURL+"/media/"+diaryUpload.Media.ID, nil, http.StatusAccepted, &deletion)
	waitForHTTPStatus(t, ctx, client, baseURL+"/deletion-requests/"+deletion.ID, "complete")

	postJSON(t, client, http.MethodPut, baseURL+"/users/me/profile", map[string]any{"handle": "integration_author", "expected_revision": 0}, http.StatusOK, nil)
	moderatorJar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	moderatorClient := &http.Client{Timeout: 10 * time.Second, Jar: moderatorJar}
	var moderatorRegistration struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	postJSON(t, moderatorClient, http.MethodPost, baseURL+"/auth/registrations", map[string]any{"email": "moderator@example.test", "display_name": "Moderator", "password": "correct-password-moderator"}, http.StatusCreated, &moderatorRegistration)
	postJSON(t, moderatorClient, http.MethodPut, baseURL+"/users/me/profile", map[string]any{"handle": "integration_moderator", "expected_revision": 0}, http.StatusOK, nil)
	databaseURL := "postgres://then_test:" + url.QueryEscape(postgresPassword) + "@" + postgresAddress + "/then_test?sslmode=disable"
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal("open integration governance database")
	}
	if err := database.WithContext(ctx).Exec("UPDATE users SET role = 'moderator' WHERE id = ?", moderatorRegistration.User.ID).Error; err != nil {
		t.Fatal("grant integration moderator role")
	}

	var communityUpload struct {
		Media struct {
			ID string `json:"id"`
		} `json:"media"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	postJSON(t, client, http.MethodPost, baseURL+"/media/uploads", map[string]any{"purpose": "community_publish", "content_type": "image/jpeg", "byte_size": len(photo), "sha256": digest}, http.StatusCreated, &communityUpload)
	put, err = http.NewRequestWithContext(ctx, http.MethodPut, communityUpload.URL, bytes.NewReader(photo))
	if err != nil {
		t.Fatal(err)
	}
	put.ContentLength = int64(len(photo))
	for key, value := range communityUpload.Headers {
		if key != "Content-Length" {
			put.Header.Set(key, value)
		}
	}
	putResponse, err = client.Do(put)
	if err != nil {
		t.Fatal("community signed PUT failed")
	}
	putResponse.Body.Close()
	versionID = putResponse.Header.Get("X-Amz-Version-Id")
	if putResponse.StatusCode != http.StatusOK || versionID == "" {
		t.Fatal("community signed PUT did not produce a version")
	}
	postJSON(t, client, http.MethodPost, baseURL+"/media/"+communityUpload.Media.ID+"/complete", map[string]any{"version_id": versionID}, http.StatusOK, nil)
	waitForHTTPStatus(t, ctx, client, baseURL+"/media/"+communityUpload.Media.ID, "ready")

	postID := "30000000-0000-4000-8000-000000000001"
	var draft struct {
		Revision int `json:"revision"`
	}
	postJSON(t, client, http.MethodPost, baseURL+"/posts", map[string]any{"id": postID, "title": "Integration Look", "body": "Synthetic public outfit note.", "media_ids": []string{communityUpload.Media.ID}, "tags": []string{"ootd"}}, http.StatusCreated, &draft)
	var pending struct {
		Revision       int  `json:"revision"`
		PendingVersion *int `json:"pending_version"`
		ReviewRound    int  `json:"review_round"`
	}
	postJSON(t, client, http.MethodPost, baseURL+"/posts/"+postID+"/submit", map[string]any{"expected_revision": draft.Revision}, http.StatusAccepted, &pending)
	if pending.PendingVersion == nil || pending.ReviewRound != 1 {
		t.Fatal("actual post did not enter moderation")
	}
	var queue struct {
		Candidates []struct {
			PostID string `json:"post_id"`
		} `json:"candidates"`
	}
	postJSON(t, moderatorClient, http.MethodGet, baseURL+"/admin/moderation/posts?limit=20", nil, http.StatusOK, &queue)
	if len(queue.Candidates) != 1 || queue.Candidates[0].PostID != postID {
		t.Fatal("actual moderation queue did not include submitted post")
	}
	var approved struct {
		Revision int    `json:"revision"`
		State    string `json:"state"`
	}
	postJSON(t, moderatorClient, http.MethodPost, baseURL+"/admin/moderation/posts/"+postID+"/decisions", map[string]any{"version": *pending.PendingVersion, "review_round": pending.ReviewRound, "expected_revision": pending.Revision, "decision": "approve", "reason_code": "content_approved"}, http.StatusOK, &approved)
	if approved.State != "published" {
		t.Fatal("actual moderation decision did not publish post")
	}
	var publicPost struct {
		AuthorHandle string `json:"author_handle"`
	}
	postJSON(t, &http.Client{Timeout: 10 * time.Second}, http.MethodGet, baseURL+"/posts/"+postID, nil, http.StatusOK, &publicPost)
	if publicPost.AuthorHandle != "integration_author" {
		t.Fatal("actual public post lost author handle")
	}
	imageResponse, err := http.Get(baseURL + "/posts/" + postID + "/images/0")
	if err != nil {
		t.Fatal("read actual public community image")
	}
	imageBytes, imageErr := io.ReadAll(io.LimitReader(imageResponse.Body, 1<<20))
	imageResponse.Body.Close()
	if imageErr != nil || imageResponse.StatusCode != http.StatusOK || imageResponse.Header.Get("Content-Type") != "image/jpeg" || len(imageBytes) == 0 {
		t.Fatal("actual public community image was not served from the fixed derived version")
	}
	var feed struct {
		Posts []struct {
			ID string `json:"id"`
		} `json:"posts"`
	}
	postJSON(t, moderatorClient, http.MethodGet, baseURL+"/feed?type=discover&limit=20", nil, http.StatusOK, &feed)
	if len(feed.Posts) != 1 || feed.Posts[0].ID != postID {
		t.Fatal("actual discovery feed did not include approved post")
	}
	postJSON(t, moderatorClient, http.MethodPut, baseURL+"/posts/"+postID+"/like", nil, http.StatusNoContent, nil)
	postJSON(t, moderatorClient, http.MethodPut, baseURL+"/posts/"+postID+"/bookmark", nil, http.StatusNoContent, nil)
	postJSON(t, moderatorClient, http.MethodPut, baseURL+"/profiles/integration_author/follow", nil, http.StatusNoContent, nil)
	commentID := "30000000-0000-4000-8000-000000000003"
	var pendingComment struct {
		Revision int `json:"revision"`
	}
	postJSON(t, moderatorClient, http.MethodPost, baseURL+"/posts/"+postID+"/comments", map[string]any{"id": commentID, "body": "Integration comment"}, http.StatusAccepted, &pendingComment)
	postJSON(t, moderatorClient, http.MethodPost, baseURL+"/admin/moderation/comments/"+commentID+"/decisions", map[string]any{"expected_revision": pendingComment.Revision, "decision": "approve", "reason_code": "content_approved"}, http.StatusOK, nil)
	var comments struct {
		Comments []struct {
			ID string `json:"id"`
		} `json:"comments"`
	}
	postJSON(t, &http.Client{Timeout: 10 * time.Second}, http.MethodGet, baseURL+"/posts/"+postID+"/comments?limit=20", nil, http.StatusOK, &comments)
	if len(comments.Comments) != 1 || comments.Comments[0].ID != commentID {
		t.Fatal("actual approved comment was not public")
	}
	notificationDeadline := time.Now().Add(5 * time.Second)
	for {
		var notifications struct {
			Notifications []json.RawMessage `json:"notifications"`
		}
		postJSON(t, client, http.MethodGet, baseURL+"/notifications?limit=20", nil, http.StatusOK, &notifications)
		if len(notifications.Notifications) >= 3 {
			break
		}
		if time.Now().After(notificationDeadline) {
			t.Fatal("actual notification worker did not deliver community events")
		}
		time.Sleep(100 * time.Millisecond)
	}
	postJSON(t, moderatorClient, http.MethodPut, baseURL+"/users/me/blocks/"+authorRegistration.User.ID, nil, http.StatusNoContent, nil)
	postJSON(t, moderatorClient, http.MethodGet, baseURL+"/feed?type=discover&limit=20", nil, http.StatusOK, &feed)
	if len(feed.Posts) != 0 {
		t.Fatal("actual block did not filter discovery")
	}
	postJSON(t, moderatorClient, http.MethodDelete, baseURL+"/users/me/blocks/"+authorRegistration.User.ID, nil, http.StatusNoContent, nil)

	reportID := "30000000-0000-4000-8000-000000000002"
	postJSON(t, moderatorClient, http.MethodPost, baseURL+"/reports", map[string]any{"id": reportID, "post_id": postID, "target_type": "post", "reason_code": "other"}, http.StatusCreated, nil)
	postJSON(t, moderatorClient, http.MethodPost, baseURL+"/admin/reports/"+reportID+"/resolve", map[string]any{"status": "resolved", "resolution_code": "reviewed_no_action"}, http.StatusOK, nil)
	postJSON(t, moderatorClient, http.MethodPost, baseURL+"/admin/posts/"+postID+"/remove", map[string]any{"expected_revision": approved.Revision, "reason_code": "policy_violation"}, http.StatusNoContent, nil)
	postJSON(t, &http.Client{Timeout: 10 * time.Second}, http.MethodGet, baseURL+"/posts/"+postID, nil, http.StatusNotFound, nil)
	postJSON(t, client, http.MethodDelete, baseURL+"/media/"+communityUpload.Media.ID, nil, http.StatusConflict, nil)
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
	container, err := testcontainer.Create(t, ctx, testcontainers.GenericContainerRequest{ContainerRequest: request, Started: true})
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
