//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/adapter/httpapi"
	"github.com/StephenQiu30/then-server/backend/internal/adapter/objectstore"
	store "github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type generationRecoveryProviderStub struct{ submitCalls int }

type generationIntegrationProbe struct{}

func (generationIntegrationProbe) Probe(context.Context) error { return nil }

func (p *generationRecoveryProviderStub) Submit(context.Context, generationapp.Submission) (generationapp.Receipt, error) {
	p.submitCalls++
	return generationapp.Receipt{ExternalTaskID: "unexpected-provider-task"}, nil
}

func (*generationRecoveryProviderStub) Query(context.Context, string) (generationapp.RemoteTask, error) {
	return generationapp.RemoteTask{}, errors.New("query is not part of submission recovery")
}

func (*generationRecoveryProviderStub) Cancel(context.Context, string) error {
	return errors.New("cancel is not part of submission recovery")
}

type generationCleanupRepositoryStub struct {
	request       generationapp.CleanupRequest
	targets       []generationapp.CleanupTarget
	completeCalls int
	failureCode   string
}

func (r *generationCleanupRepositoryStub) ClaimNextCleanup(_ context.Context, at time.Time, _ time.Duration) (generationapp.CleanupRequest, []generationapp.CleanupTarget, bool, error) {
	targets, err := r.request.Begin(at)
	if err != nil {
		return generationapp.CleanupRequest{}, nil, false, err
	}
	return r.request, targets, true, nil
}

func (r *generationCleanupRepositoryStub) CompleteTaskCleanup(_ context.Context, request generationapp.CleanupRequest, at time.Time) (generationapp.CleanupRequest, error) {
	if request.ID != r.request.ID {
		return generationapp.CleanupRequest{}, generationapp.ErrGenerationCleanupClaim
	}
	if err := r.request.Complete(at); err != nil {
		return generationapp.CleanupRequest{}, err
	}
	r.completeCalls++
	return r.request, nil
}

func (r *generationCleanupRepositoryStub) FailTaskCleanup(_ context.Context, request generationapp.CleanupRequest, at time.Time, code string, retryAt time.Time) (generationapp.CleanupRequest, error) {
	if request.ID != r.request.ID {
		return generationapp.CleanupRequest{}, generationapp.ErrGenerationCleanupClaim
	}
	if err := r.request.Fail(at, code, retryAt); err != nil {
		return generationapp.CleanupRequest{}, err
	}
	r.failureCode = code
	return r.request, nil
}

func TestGenerationCleanupWorkerDeletesExactMinIOVersionAndBlocksProviderCalls(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	password := rand.Text()
	container := integrationContainer(t, ctx, testcontainers.ContainerRequest{
		Image:        "quay.io/minio/minio:RELEASE.2025-09-07T16-13-09Z@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e",
		Cmd:          []string{"server", "/data"},
		Env:          map[string]string{"MINIO_ROOT_USER": "then_test", "MINIO_ROOT_PASSWORD": password},
		ExposedPorts: []string{"9000/tcp"},
		WaitingFor:   wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").WithStartupTimeout(time.Minute),
	})
	endpoint := mappedAddress(t, ctx, container, "9000/tcp")
	objects, err := objectstore.Open(ctx, endpoint, "then_test", password, false)
	if err != nil {
		t.Fatalf("open isolated MinIO: %v", err)
	}

	ownerID := "00000000-0000-4000-8000-000000000911"
	taskID := "00000000-0000-4000-8000-000000000912"
	key := "owners/" + ownerID + "/generation/" + taskID + "/output.jpg"
	firstVersion, err := objects.PutDerived(ctx, key, strings.NewReader("first-version"), int64(len("first-version")))
	if err != nil || firstVersion == "" {
		t.Fatalf("write first generation output version: version=%q err=%v", firstVersion, err)
	}
	latestBytes := "latest-version"
	latestVersion, err := objects.PutDerived(ctx, key, strings.NewReader(latestBytes), int64(len(latestBytes)))
	if err != nil || latestVersion == "" || latestVersion == firstVersion {
		t.Fatalf("write later generation output version: first=%q latest=%q err=%v", firstVersion, latestVersion, err)
	}

	at := time.Now().UTC()
	repository := &generationCleanupRepositoryStub{
		request: generationapp.CleanupRequest{
			ID:              "00000000-0000-4000-8000-000000000913",
			OwnerID:         ownerID,
			TaskID:          taskID,
			Scope:           generationapp.CleanupScopeTask,
			Status:          generationapp.CleanupPending,
			AccessRevokedAt: at,
			Targets: []generationapp.CleanupTarget{
				{Kind: generationapp.CleanupTargetObject, ID: "00000000-0000-4000-8000-000000000914", ObjectKey: key, ObjectVersionID: firstVersion},
				{Kind: generationapp.CleanupTargetProvider, ID: "fixture-external-task"},
			},
			CreatedAt: at,
			UpdatedAt: at,
		},
	}
	executor, err := objectstore.NewGenerationCleanupExecutor(objects)
	if err != nil {
		t.Fatalf("construct local generation cleanup executor: %v", err)
	}
	worker, err := generationapp.NewCleanupWorker(repository, executor, generationapp.CleanupRetryPolicy{LeaseTTL: time.Minute, BaseDelay: time.Second, MaxDelay: time.Minute})
	if err != nil {
		t.Fatalf("construct generation cleanup worker: %v", err)
	}
	found, err := worker.RunOnce(ctx)
	if err != nil || !found {
		t.Fatalf("run generation cleanup: found=%v err=%v", found, err)
	}
	if repository.failureCode != "provider_cleanup_unavailable" || repository.request.Status != generationapp.CleanupFailed || repository.completeCalls != 0 {
		t.Fatalf("cleanup did not preserve the unavailable provider target: code=%q request=%+v completeCalls=%d", repository.failureCode, repository.request, repository.completeCalls)
	}
	if _, err := objects.OpenDerivedVersion(ctx, key, firstVersion); err == nil {
		t.Fatal("cleanup left the targeted MinIO version in place")
	}
	if err := objects.DeleteVersion(ctx, objectstore.DerivedBucket, key, firstVersion); err != nil {
		t.Fatalf("repeat targeted MinIO version deletion was not idempotent: %v", err)
	}
	latest, err := objects.OpenDerivedVersion(ctx, key, latestVersion)
	if err != nil {
		t.Fatalf("cleanup deleted an untargeted MinIO version: %v", err)
	}
	defer latest.Close()
	latestData, err := io.ReadAll(latest)
	if err != nil || string(latestData) != latestBytes {
		t.Fatalf("read untargeted MinIO version: bytes=%q err=%v", latestData, err)
	}
}

func verifyGenerationHTTPPersistence(t *testing.T, ctx context.Context, database *gorm.DB, service *generationapp.Service, ownerID, ownerToken, otherToken string) {
	t.Helper()

	router, err := httpapi.NewRouterWithGeneration(
		ctx,
		false,
		generationIntegrationProbe{},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		httpapi.NewGenerationHandler(service, false),
		time.Second,
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
	)
	if err != nil {
		t.Fatalf("construct generation HTTP router: %v", err)
	}

	body := fmt.Sprintf(`{"idempotency_key":"generation-http-request","look_id":"00000000-0000-4000-8000-000000000901","look_revision":1,"purpose":"image","provider":"fixture","model":"fixture-image-v1","parameters":{"seed":901},"inputs":[{"media_id":"00000000-0000-4000-8000-000000000902","role":"person","ordinal":0,"revision":1,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},{"media_id":"00000000-0000-4000-8000-000000000903","role":"garment","ordinal":1,"revision":1,"sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}],"consent":{"id":"00000000-0000-4000-8000-000000000904","purpose":"image","policy_version":"local-image-v1","accepted_at":"%s"}}`, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339))
	request := func(method, target, session, requestBody string) *http.Request {
		req := httptest.NewRequest(method, target, strings.NewReader(requestBody))
		if requestBody != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if session != "" {
			req.AddCookie(&http.Cookie{Name: "then_session", Value: session})
		}
		return req
	}
	decodeJob := func(response *httptest.ResponseRecorder) httpapi.GenerationJobResponse {
		t.Helper()
		var job httpapi.GenerationJobResponse
		if err := json.Unmarshal(response.Body.Bytes(), &job); err != nil {
			t.Fatalf("decode generation response %q: %v", response.Body.String(), err)
		}
		return job
	}

	createdResponse := httptest.NewRecorder()
	router.ServeHTTP(createdResponse, request(http.MethodPost, "/generation-jobs", ownerToken, body))
	if createdResponse.Code != http.StatusAccepted {
		t.Fatalf("create generation over HTTP status=%d body=%s", createdResponse.Code, createdResponse.Body.String())
	}
	created := decodeJob(createdResponse)
	if created.ID == "" || created.Status != generationapp.StatusQueued || created.Reused || created.Match != generationapp.RequestMatchNone {
		t.Fatalf("unexpected created HTTP generation job: %+v", created)
	}
	if strings.Contains(createdResponse.Body.String(), "owner_id") || strings.Contains(createdResponse.Body.String(), "object_key") || strings.Contains(createdResponse.Body.String(), "owners/") {
		t.Fatalf("generation HTTP response exposed private persistence fields: %s", createdResponse.Body.String())
	}
	var persisted struct {
		OwnerID      string
		LookID       string
		LookRevision int
		Inputs       []byte
		Consent      []byte
	}
	if err := database.WithContext(ctx).Table("generation_jobs").Select("owner_id, look_id, look_revision, inputs, consent").Where("id = ? AND status = ?", created.ID, string(generationapp.StatusQueued)).Scan(&persisted).Error; err != nil {
		t.Fatalf("read HTTP-created generation job: %v", err)
	}
	var inputSnapshot generationapp.InputSnapshot
	if err := json.Unmarshal(persisted.Inputs, &inputSnapshot); err != nil {
		t.Fatalf("decode persisted HTTP input snapshot %q: %v", persisted.Inputs, err)
	}
	var consent generationapp.ConsentReceipt
	if err := json.Unmarshal(persisted.Consent, &consent); err != nil {
		t.Fatalf("decode persisted HTTP consent receipt %q: %v", persisted.Consent, err)
	}
	wantReferences := []generationapp.InputReference{
		{MediaID: "00000000-0000-4000-8000-000000000902", Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{MediaID: "00000000-0000-4000-8000-000000000903", Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 1, SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	snapshotValid := inputSnapshot.LookID == created.LookID && inputSnapshot.LookRevision == created.LookRevision && len(inputSnapshot.References) == len(wantReferences)
	if snapshotValid {
		for index, reference := range wantReferences {
			if inputSnapshot.References[index] != reference {
				snapshotValid = false
				break
			}
		}
	}
	consentValid := consent.ID == "00000000-0000-4000-8000-000000000904" && consent.Purpose == generationapp.PurposeImage && consent.PolicyVersion == "local-image-v1" && !consent.AcceptedAt.IsZero()
	if persisted.OwnerID != ownerID || persisted.LookID != created.LookID || persisted.LookRevision != created.LookRevision || !snapshotValid || !consentValid {
		t.Fatalf("HTTP request facts were not persisted faithfully: owner=%q look=%q revision=%d inputs=%+v consent=%+v", persisted.OwnerID, persisted.LookID, persisted.LookRevision, inputSnapshot, consent)
	}
	var persistedCount int64
	if err := database.WithContext(ctx).Table("generation_jobs").Where("id = ? AND status = ?", created.ID, string(generationapp.StatusQueued)).Count(&persistedCount).Error; err != nil || persistedCount != 1 {
		t.Fatalf("HTTP-created generation job was not persisted: count=%d err=%v", persistedCount, err)
	}

	ownerRead := httptest.NewRecorder()
	router.ServeHTTP(ownerRead, request(http.MethodGet, "/generation-jobs/"+created.ID, ownerToken, ""))
	if ownerRead.Code != http.StatusOK || decodeJob(ownerRead).ID != created.ID {
		t.Fatalf("owner could not read HTTP-created task: status=%d body=%s", ownerRead.Code, ownerRead.Body.String())
	}
	otherOwnerRead := httptest.NewRecorder()
	router.ServeHTTP(otherOwnerRead, request(http.MethodGet, "/generation-jobs/"+created.ID, otherToken, ""))
	if otherOwnerRead.Code != http.StatusNotFound {
		t.Fatalf("cross-owner HTTP read status=%d body=%s", otherOwnerRead.Code, otherOwnerRead.Body.String())
	}
	otherOwnerList := httptest.NewRecorder()
	router.ServeHTTP(otherOwnerList, request(http.MethodGet, "/generation-jobs?limit=20", otherToken, ""))
	if otherOwnerList.Code != http.StatusOK {
		t.Fatalf("list another owner's generation tasks over HTTP status=%d body=%s", otherOwnerList.Code, otherOwnerList.Body.String())
	}
	var otherPage httpapi.GenerationJobPageResponse
	if err := json.Unmarshal(otherOwnerList.Body.Bytes(), &otherPage); err != nil {
		t.Fatalf("decode another owner's HTTP task list %q: %v", otherOwnerList.Body.String(), err)
	}
	for _, job := range otherPage.Jobs {
		if job.ID == created.ID {
			t.Fatalf("cross-owner HTTP list exposed task %q: %+v", created.ID, otherPage)
		}
	}

	changedRequest := strings.ReplaceAll(body, `"seed":901`, `"seed":902`)
	conflictResponse := httptest.NewRecorder()
	router.ServeHTTP(conflictResponse, request(http.MethodPost, "/generation-jobs", ownerToken, changedRequest))
	if conflictResponse.Code != http.StatusConflict || !strings.Contains(conflictResponse.Body.String(), `"code":"CONFLICT"`) {
		t.Fatalf("changed HTTP request with the same idempotency key status=%d body=%s", conflictResponse.Code, conflictResponse.Body.String())
	}

	replayResponse := httptest.NewRecorder()
	router.ServeHTTP(replayResponse, request(http.MethodPost, "/generation-jobs", ownerToken, body))
	if replayResponse.Code != http.StatusAccepted {
		t.Fatalf("idempotent HTTP replay status=%d body=%s", replayResponse.Code, replayResponse.Body.String())
	}
	replayed := decodeJob(replayResponse)
	if replayed.ID != created.ID || !replayed.Reused || replayed.Match != generationapp.RequestMatchIdempotentReplay {
		t.Fatalf("HTTP idempotent replay created or returned another job: created=%+v replayed=%+v", created, replayed)
	}
	contentDuplicateBody := strings.ReplaceAll(body, "generation-http-request", "generation-http-content-duplicate")
	contentDuplicateResponse := httptest.NewRecorder()
	router.ServeHTTP(contentDuplicateResponse, request(http.MethodPost, "/generation-jobs", ownerToken, contentDuplicateBody))
	if contentDuplicateResponse.Code != http.StatusAccepted {
		t.Fatalf("HTTP content duplicate status=%d body=%s", contentDuplicateResponse.Code, contentDuplicateResponse.Body.String())
	}
	contentDuplicate := decodeJob(contentDuplicateResponse)
	if contentDuplicate.ID != created.ID || !contentDuplicate.Reused || contentDuplicate.Match != generationapp.RequestMatchContentDedupe {
		t.Fatalf("HTTP content dedupe returned another task: created=%+v duplicate=%+v", created, contentDuplicate)
	}

	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, request(http.MethodGet, "/generation-jobs?limit=20", ownerToken, ""))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list generation over HTTP status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}
	var page httpapi.GenerationJobPageResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &page); err != nil || len(page.Jobs) != 1 || page.Jobs[0].ID != created.ID {
		t.Fatalf("owner HTTP list mismatch: page=%+v err=%v body=%s", page, err, listResponse.Body.String())
	}

	cancelResponse := httptest.NewRecorder()
	router.ServeHTTP(cancelResponse, request(http.MethodPost, "/generation-jobs/"+created.ID+"/cancel", ownerToken, ""))
	if cancelResponse.Code != http.StatusAccepted {
		t.Fatalf("cancel generation over HTTP status=%d body=%s", cancelResponse.Code, cancelResponse.Body.String())
	}
	canceled := decodeJob(cancelResponse)
	if canceled.ID != created.ID || canceled.Status != generationapp.StatusQueued || canceled.CancelRequestedAt == nil {
		t.Fatalf("HTTP cancellation claimed an unverified provider stop: %+v", canceled)
	}
	readAfterCancel := httptest.NewRecorder()
	router.ServeHTTP(readAfterCancel, request(http.MethodGet, "/generation-jobs/"+created.ID, ownerToken, ""))
	if readAfterCancel.Code != http.StatusOK || decodeJob(readAfterCancel).CancelRequestedAt == nil {
		t.Fatalf("HTTP cancellation request was not persisted: status=%d body=%s", readAfterCancel.Code, readAfterCancel.Body.String())
	}

	deleteResponse := httptest.NewRecorder()
	router.ServeHTTP(deleteResponse, request(http.MethodDelete, "/generation-jobs/"+created.ID, ownerToken, ""))
	if deleteResponse.Code != http.StatusAccepted {
		t.Fatalf("delete generation over HTTP status=%d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	deleted := decodeJob(deleteResponse)
	if deleted.ID != created.ID || deleted.AccessRevokedAt == nil || deleted.Cleanup == nil {
		t.Fatalf("HTTP deletion did not revoke access and return cleanup state: %+v", deleted)
	}
	readAfterDelete := httptest.NewRecorder()
	router.ServeHTTP(readAfterDelete, request(http.MethodGet, "/generation-jobs/"+created.ID, ownerToken, ""))
	if readAfterDelete.Code != http.StatusOK {
		t.Fatalf("owner could not read deletion progress over HTTP: status=%d body=%s", readAfterDelete.Code, readAfterDelete.Body.String())
	}
	deletionProgress := decodeJob(readAfterDelete)
	if deletionProgress.AccessRevokedAt == nil || deletionProgress.Cleanup == nil {
		t.Fatalf("HTTP deletion progress was not persisted: %+v", deletionProgress)
	}
}

func verifyGenerationHTTPQuotaPersistence(t *testing.T, ctx context.Context, database *gorm.DB, service *generationapp.Service, ownerID, ownerToken string) {
	t.Helper()

	router, err := httpapi.NewRouterWithGeneration(
		ctx,
		false,
		generationIntegrationProbe{},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		httpapi.NewGenerationHandler(service, false),
		time.Second,
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
	)
	if err != nil {
		t.Fatalf("construct quota generation HTTP router: %v", err)
	}

	body := fmt.Sprintf(`{"idempotency_key":"generation-http-quota-request","look_id":"00000000-0000-4000-8000-000000000911","look_revision":1,"purpose":"image","provider":"fixture","model":"fixture-image-v1","parameters":{"seed":911},"inputs":[{"media_id":"00000000-0000-4000-8000-000000000912","role":"person","ordinal":0,"revision":1,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},{"media_id":"00000000-0000-4000-8000-000000000913","role":"garment","ordinal":1,"revision":1,"sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}],"consent":{"id":"00000000-0000-4000-8000-000000000914","purpose":"image","policy_version":"local-image-v1","accepted_at":"%s"}}`, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339))
	call := func(requestBody string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, "/generation-jobs", strings.NewReader(requestBody))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(&http.Cookie{Name: "then_session", Value: ownerToken})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	decodeJob := func(response *httptest.ResponseRecorder) httpapi.GenerationJobResponse {
		t.Helper()
		var job httpapi.GenerationJobResponse
		if err := json.Unmarshal(response.Body.Bytes(), &job); err != nil {
			t.Fatalf("decode quota generation response %q: %v", response.Body.String(), err)
		}
		return job
	}

	createdResponse := call(body)
	if createdResponse.Code != http.StatusAccepted {
		t.Fatalf("create quota-backed generation over HTTP status=%d body=%s", createdResponse.Code, createdResponse.Body.String())
	}
	created := decodeJob(createdResponse)
	if created.Reservation == nil || created.Reservation.State != generationapp.ReservationReserved || created.Reservation.EstimatedMinorUnits != 60 || created.Reservation.ReservedQuotaUnits != 1 || created.Reservation.Currency != "USD" {
		t.Fatalf("HTTP admission did not return its server-estimated quota reservation: %+v", created)
	}

	replayResponse := call(body)
	if replayResponse.Code != http.StatusAccepted {
		t.Fatalf("replay quota-backed generation over HTTP status=%d body=%s", replayResponse.Code, replayResponse.Body.String())
	}
	replayed := decodeJob(replayResponse)
	if replayed.ID != created.ID || !replayed.Reused || replayed.Match != generationapp.RequestMatchIdempotentReplay || replayed.Reservation == nil || replayed.Reservation.ID != created.Reservation.ID {
		t.Fatalf("HTTP replay changed the persisted quota reservation: created=%+v replayed=%+v", created, replayed)
	}

	overBudget := strings.ReplaceAll(body, "generation-http-quota-request", "generation-http-budget-overrun")
	overBudget = strings.ReplaceAll(overBudget, "00000000-0000-4000-8000-000000000911", "00000000-0000-4000-8000-000000000921")
	overBudget = strings.ReplaceAll(overBudget, "00000000-0000-4000-8000-000000000912", "00000000-0000-4000-8000-000000000922")
	overBudget = strings.ReplaceAll(overBudget, "00000000-0000-4000-8000-000000000913", "00000000-0000-4000-8000-000000000923")
	overBudget = strings.ReplaceAll(overBudget, "00000000-0000-4000-8000-000000000914", "00000000-0000-4000-8000-000000000924")
	overBudget = strings.ReplaceAll(overBudget, `"seed":911`, `"seed":921`)
	overBudgetResponse := call(overBudget)
	if overBudgetResponse.Code != http.StatusConflict || !strings.Contains(overBudgetResponse.Body.String(), `"code":"CONFLICT"`) {
		t.Fatalf("over-budget HTTP request status=%d body=%s", overBudgetResponse.Code, overBudgetResponse.Body.String())
	}

	var jobs, reservations int64
	if err := database.WithContext(ctx).Table("generation_jobs").Where("owner_id = ?", ownerID).Count(&jobs).Error; err != nil || jobs != 1 {
		t.Fatalf("over-budget HTTP request persisted a task: count=%d err=%v", jobs, err)
	}
	if err := database.WithContext(ctx).Table("generation_quota_reservations").Where("owner_id = ? AND state = ?", ownerID, string(generationapp.ReservationReserved)).Count(&reservations).Error; err != nil || reservations != 1 {
		t.Fatalf("HTTP retry or rejection changed active reservations: count=%d err=%v", reservations, err)
	}
}

func TestGenerationPersistenceLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	password := rand.Text()
	container := integrationContainer(t, ctx, testcontainers.ContainerRequest{
		Image:        "postgres:18.4@sha256:a02db8cac496f15b094798a38254f14d6e00741f709360e5e00bb6668ea31636",
		Env:          map[string]string{"POSTGRES_USER": "then_test", "POSTGRES_DB": "then_test", "POSTGRES_PASSWORD": password},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(time.Minute),
	})
	databaseAddress := mappedAddress(t, ctx, container, "5432/tcp")
	databaseURL := "postgres://then_test:" + url.QueryEscape(password) + "@" + databaseAddress + "/then_test?sslmode=disable"
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open generation database: %v", err)
	}
	if err := store.Migrate(ctx, database); err != nil {
		t.Fatalf("migrate generation database: %v", err)
	}

	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	if err != nil {
		t.Fatalf("construct account service: %v", err)
	}
	first, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "generation-first@example.test", DisplayName: "Generation First", Password: "correct-password-generation-first"})
	if err != nil {
		t.Fatalf("register first generation owner: %v", err)
	}
	second, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "generation-second@example.test", DisplayName: "Generation Second", Password: "correct-password-generation-second"})
	if err != nil {
		t.Fatalf("register second generation owner: %v", err)
	}
	third, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "generation-third@example.test", DisplayName: "Generation Third", Password: "correct-password-generation-third"})
	if err != nil {
		t.Fatalf("register third generation owner: %v", err)
	}
	fourth, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "generation-http@example.test", DisplayName: "Generation HTTP", Password: "correct-password-generation-http"})
	if err != nil {
		t.Fatalf("register generation HTTP owner: %v", err)
	}
	fifth, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "generation-http-quota@example.test", DisplayName: "Generation HTTP Quota", Password: "correct-password-generation-http-quota"})
	if err != nil {
		t.Fatalf("register generation HTTP quota owner: %v", err)
	}

	policy := generationapp.AdmissionPolicy{Enabled: true, ZeroCost: true, MaxConcurrentTasks: 4, MaxQuotaUnits: 4}
	generations, err := generationapp.NewService(accounts, store.NewGenerationRepository(database), policy)
	if err != nil {
		t.Fatalf("construct generation service: %v", err)
	}
	acceptedAt := time.Now().UTC().Add(-time.Second)
	input := generationapp.CreateServiceInput{
		IdempotencyKey: "generation-request-one",
		LookID:         "00000000-0000-4000-8000-000000000101",
		LookRevision:   2,
		Purpose:        generationapp.PurposeImage,
		Provider:       "fixture",
		Model:          "look-image-v1",
		Parameters:     []byte(`{"seed":"one"}`),
		Inputs: generationapp.InputSnapshot{
			LookID:       "00000000-0000-4000-8000-000000000101",
			LookRevision: 2,
			References: []generationapp.InputReference{
				{MediaID: "00000000-0000-4000-8000-000000000201", Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				{MediaID: "00000000-0000-4000-8000-000000000202", Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 3, SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
			},
		},
		Consent: generationapp.ConsentReceipt{ID: "00000000-0000-4000-8000-000000000301", Purpose: generationapp.PurposeImage, PolicyVersion: "local-image-v1", AcceptedAt: acceptedAt},
	}

	created, err := generations.Create(ctx, first.Token, input)
	if err != nil {
		t.Fatalf("accept generation task: %v", err)
	}
	if created.Reused || created.Match != generationapp.RequestMatchNone || created.View.Task.Status != generationapp.StatusQueued || created.View.Reservation != nil {
		t.Fatalf("unexpected new generation task: %+v", created)
	}

	var outbox struct {
		EventType   string
		AggregateID string
		Payload     []byte
	}
	if err := database.WithContext(ctx).Table("outbox_events").Select("event_type, aggregate_id, payload").Where("aggregate_id = ?", created.View.Task.ID).First(&outbox).Error; err != nil {
		t.Fatalf("read generation outbox event: %v", err)
	}
	if outbox.EventType != generationapp.GenerationRequestedEvent || outbox.AggregateID != created.View.Task.ID || len(outbox.Payload) == 0 {
		t.Fatalf("invalid generation outbox event: %+v", outbox)
	}
	mediaOutbox := store.NewMediaRepository(database)
	mediaEvents, err := mediaOutbox.PendingOutbox(ctx, 100)
	if err != nil {
		t.Fatalf("read media worker outbox: %v", err)
	}
	for _, event := range mediaEvents {
		if event.EventType == generationapp.GenerationRequestedEvent {
			t.Fatalf("media worker claimed generation outbox event: %+v", event)
		}
	}
	var generationPublished int64
	if err := database.WithContext(ctx).Table("outbox_events").Where("aggregate_id = ? AND published_at IS NOT NULL", created.View.Task.ID).Count(&generationPublished).Error; err != nil {
		t.Fatalf("read generation outbox publication state: %v", err)
	}
	if generationPublished != 0 {
		t.Fatal("generation outbox event was marked published by the media worker")
	}

	replayed, err := generations.Create(ctx, first.Token, input)
	if err != nil {
		t.Fatalf("replay generation task: %v", err)
	}
	if !replayed.Reused || replayed.Match != generationapp.RequestMatchIdempotentReplay || replayed.View.Task.ID != created.View.Task.ID {
		t.Fatalf("idempotent replay did not return the original task: %+v", replayed)
	}
	var outboxCount int64
	if err := database.WithContext(ctx).Table("outbox_events").Where("aggregate_id = ?", created.View.Task.ID).Count(&outboxCount).Error; err != nil {
		t.Fatalf("count generation outbox events: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("idempotent replay created %d outbox events", outboxCount)
	}

	deduplicatedInput := input
	deduplicatedInput.IdempotencyKey = "generation-request-two"
	deduplicated, err := generations.Create(ctx, first.Token, deduplicatedInput)
	if err != nil {
		t.Fatalf("content-deduplicate generation task: %v", err)
	}
	if !deduplicated.Reused || deduplicated.Match != generationapp.RequestMatchContentDedupe || deduplicated.View.Task.ID != created.View.Task.ID {
		t.Fatalf("content dedupe did not return the original task: %+v", deduplicated)
	}

	conflict := input
	conflict.Parameters = []byte(`{"seed":"two"}`)
	if _, err := generations.Create(ctx, first.Token, conflict); !errors.Is(err, generationapp.ErrGenerationIdempotencyConflict) {
		t.Fatalf("changed request with the same idempotency key returned %v", err)
	}

	if _, err := generations.Get(ctx, second.Token, created.View.Task.ID); !errors.Is(err, generationapp.ErrGenerationNotFound) {
		t.Fatalf("cross-owner generation lookup returned %v", err)
	}
	secondPage, err := generations.List(ctx, second.Token, 20, nil)
	if err != nil {
		t.Fatalf("list second owner's generation tasks: %v", err)
	}
	if len(secondPage.Items) != 0 || secondPage.NextAfterID != nil {
		t.Fatalf("cross-owner generation list leaked tasks: %+v", secondPage)
	}

	paidPolicy := generationapp.AdmissionPolicy{Enabled: true, Currency: "USD", MaxConcurrentTasks: 4, MaxQuotaUnits: 4, MaxBudgetMinorUnits: 100}
	paidEstimator := generationapp.CostEstimatorFunc(func(generationapp.Purpose, string, string, []byte) (generationapp.CostEstimate, error) {
		return generationapp.CostEstimate{Currency: "USD", EstimatedMinorUnits: 10, ReservedQuotaUnits: 2}, nil
	})
	paidGenerations, err := generationapp.NewServiceWithCostEstimator(accounts, store.NewGenerationRepository(database), paidPolicy, paidEstimator)
	if err != nil {
		t.Fatalf("construct paid generation service: %v", err)
	}
	paidInput := input
	paidInput.IdempotencyKey = "generation-paid-request"
	paidInput.Parameters = []byte(`{"seed":"paid"}`)
	paidCreated, err := paidGenerations.Create(ctx, second.Token, paidInput)
	if err != nil {
		t.Fatalf("accept quota-backed generation task: %v", err)
	}
	if paidCreated.Reused || paidCreated.View.Reservation == nil || paidCreated.View.Reservation.State != generationapp.ReservationReserved || paidCreated.View.Reservation.ReservedQuotaUnits != 2 || paidCreated.View.Reservation.EstimatedMinorUnits != 10 {
		t.Fatalf("quota-backed generation task did not persist its reservation: %+v", paidCreated)
	}
	var reservationCount int64
	if err := database.WithContext(ctx).Table("generation_quota_reservations").Where("task_id = ? AND owner_id = ? AND state = ?", paidCreated.View.Task.ID, second.User.ID, string(generationapp.ReservationReserved)).Count(&reservationCount).Error; err != nil {
		t.Fatalf("count generation quota reservations: %v", err)
	}
	if reservationCount != 1 {
		t.Fatalf("expected one active generation quota reservation, got %d", reservationCount)
	}

	workerRepository := store.NewGenerationRepository(database)
	queuedClaimAt := created.View.Task.UpdatedAt
	queuedClaim, queuedLease, found, err := workerRepository.ClaimNextSubmissionLease(ctx, "worker-queue", queuedClaimAt, time.Minute)
	if err != nil || !found || queuedClaim.Task.ID != created.View.Task.ID || queuedClaim.Task.Status != generationapp.StatusRunning || queuedLease.FencingToken != 1 || queuedLease.Attempt != 1 {
		t.Fatalf("submission queue claim was incorrect: found=%v task=%+v lease=%+v err=%v", found, queuedClaim.Task, queuedLease, err)
	}
	if _, err := workerRepository.ReleaseLease(ctx, queuedLease, queuedClaimAt); err != nil {
		t.Fatalf("release queued submission claim: %v", err)
	}
	if _, _, err := workerRepository.AcquireObservationLease(ctx, paidCreated.View.Task.ID, "observe-before-accept", paidCreated.View.Task.UpdatedAt.Add(time.Minute), time.Minute); !errors.Is(err, generationapp.ErrGenerationObservationUnavailable) {
		t.Fatalf("queued generation task was eligible for observation: %v", err)
	}
	queuedView, err := paidGenerations.Get(ctx, second.Token, paidCreated.View.Task.ID)
	if err != nil {
		t.Fatalf("reload queued generation task: %v", err)
	}
	if queuedView.Task.Status != generationapp.StatusQueued || queuedView.Task.LeaseOwner != "" {
		t.Fatalf("observation claim mutated paid task unexpectedly: %+v", queuedView.Task)
	}
	leaseAt := paidCreated.View.Task.UpdatedAt.Add(time.Minute)
	leased, lease, err := workerRepository.AcquireLease(ctx, paidCreated.View.Task.ID, "worker-a", leaseAt, time.Minute)
	if err != nil {
		t.Fatalf("acquire generation lease: %v", err)
	}
	if leased.Task.Status != generationapp.StatusRunning || lease.Owner != "worker-a" || lease.FencingToken != 1 || lease.Attempt != 1 {
		t.Fatalf("generation lease did not move task to running: task=%+v lease=%+v", leased.Task, lease)
	}
	if _, _, err := workerRepository.AcquireLease(ctx, paidCreated.View.Task.ID, "worker-b", leaseAt.Add(30*time.Second), time.Minute); !errors.Is(err, generationapp.ErrGenerationLeaseHeld) {
		t.Fatalf("active generation lease was replaced early: %v", err)
	}
	renewedView, renewed, err := workerRepository.RenewLease(ctx, lease, leaseAt.Add(30*time.Second), 2*time.Minute)
	if err != nil {
		t.Fatalf("renew generation lease: %v", err)
	}
	if renewedView.Task.Status != generationapp.StatusRunning || renewed.FencingToken != lease.FencingToken || !renewed.ExpiresAt.After(lease.ExpiresAt) {
		t.Fatalf("generation lease renewal changed fencing or expiry incorrectly: task=%+v lease=%+v", renewedView.Task, renewed)
	}
	recoveredView, recovered, err := workerRepository.AcquireLease(ctx, paidCreated.View.Task.ID, "worker-b", renewed.ExpiresAt, time.Minute)
	if err != nil {
		t.Fatalf("recover expired generation lease: %v", err)
	}
	if recoveredView.Task.Status != generationapp.StatusRunning || recovered.Owner != "worker-b" || recovered.FencingToken != lease.FencingToken+1 || recovered.Attempt != lease.Attempt+1 {
		t.Fatalf("generation lease recovery did not fence the old worker: task=%+v lease=%+v", recoveredView.Task, recovered)
	}
	if _, _, err := workerRepository.RenewLease(ctx, renewed, renewed.ExpiresAt, time.Minute); !errors.Is(err, generationapp.ErrGenerationLeaseConflict) {
		t.Fatalf("stale generation lease was accepted after recovery: %v", err)
	}
	released, err := workerRepository.ReleaseLease(ctx, recovered, recovered.ExpiresAt.Add(-time.Second))
	if err != nil {
		t.Fatalf("release generation lease: %v", err)
	}
	if released.Task.LeaseOwner != "" || released.Task.LeaseUntil != nil || released.Task.Status != generationapp.StatusRunning {
		t.Fatalf("generation lease release left an active lease: %+v", released.Task)
	}

	workflowAt := released.Task.UpdatedAt.Add(time.Minute)
	workflowView, workflowLease, err := workerRepository.AcquireLease(ctx, paidCreated.View.Task.ID, "worker-c", workflowAt, 10*time.Minute)
	if err != nil {
		t.Fatalf("reacquire generation lease for submission workflow: %v", err)
	}
	if workflowView.Task.Status != generationapp.StatusRunning || workflowLease.FencingToken != recovered.FencingToken+1 || workflowLease.Attempt != recovered.Attempt+1 {
		t.Fatalf("submission workflow lease was not fenced: task=%+v lease=%+v", workflowView.Task, workflowLease)
	}
	started, submission, err := workerRepository.BeginSubmission(ctx, workflowLease, workflowAt)
	if err != nil {
		t.Fatalf("begin generation submission: %v", err)
	}
	if started.Task.SubmissionState != generationapp.SubmissionInFlight || started.Task.SubmissionAttempt != 1 || submission.Attempt != 1 || submission.TaskID != paidCreated.View.Task.ID {
		t.Fatalf("generation submission was not persisted as in flight: task=%+v submission=%+v", started.Task, submission)
	}
	unknown, err := workerRepository.MarkSubmissionUnknown(ctx, workflowLease, workflowAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("mark generation submission unknown: %v", err)
	}
	if unknown.Task.SubmissionState != generationapp.SubmissionUnknown || unknown.Task.SubmissionUnknownAt == nil {
		t.Fatalf("unknown generation submission was not persisted: %+v", unknown.Task)
	}
	if _, _, err := workerRepository.BeginSubmission(ctx, workflowLease, workflowAt.Add(90*time.Second)); !errors.Is(err, generationapp.ErrSubmissionOutcomeUnknown) {
		t.Fatalf("unknown generation submission was resubmitted: %v", err)
	}
	reconciled, err := workerRepository.ReconcileSubmissionNotAccepted(ctx, workflowLease, workflowAt.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("reconcile generation submission: %v", err)
	}
	if reconciled.Task.SubmissionState != generationapp.SubmissionNotStarted || reconciled.Task.SubmissionUnknownAt != nil || reconciled.Task.SubmissionAttempt != 1 {
		t.Fatalf("generation submission reconciliation was incorrect: %+v", reconciled.Task)
	}
	retryAt := workflowAt.Add(2 * time.Minute)
	scheduled, err := workerRepository.ScheduleSubmissionRetry(ctx, workflowLease, retryAt, generationapp.RetryPolicy{MaxAttempts: 3, BaseDelay: 30 * time.Second, MaxDelay: time.Minute})
	if err != nil {
		t.Fatalf("schedule generation retry: %v", err)
	}
	if scheduled.Task.NextAttemptAt == nil || !scheduled.Task.NextAttemptAt.Equal(retryAt.Add(30*time.Second)) {
		t.Fatalf("generation retry window was not persisted: %+v", scheduled.Task)
	}
	startedAgain, submissionAgain, err := workerRepository.BeginSubmission(ctx, workflowLease, workflowAt.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("begin reconciled generation submission: %v", err)
	}
	if startedAgain.Task.SubmissionState != generationapp.SubmissionInFlight || startedAgain.Task.SubmissionAttempt != 2 || submissionAgain.Attempt != 2 || startedAgain.Task.NextAttemptAt != nil {
		t.Fatalf("reconciled generation submission did not increment attempt: task=%+v submission=%+v", startedAgain.Task, submissionAgain)
	}
	external, err := workerRepository.RecordExternalTaskID(ctx, workflowLease, "provider-task-1", workflowAt.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("record generation provider task identity: %v", err)
	}
	if external.Task.SubmissionState != generationapp.SubmissionAccepted || external.Task.ExternalTaskID != "provider-task-1" || external.Task.SubmissionAttempt != 2 {
		t.Fatalf("generation provider identity was not persisted: %+v", external.Task)
	}
	if _, err := workerRepository.ReleaseLease(ctx, workflowLease, workflowAt.Add(4*time.Minute)); err != nil {
		t.Fatalf("release provider submission lease before observation claim: %v", err)
	}
	observed, observationLease, found, err := workerRepository.ClaimNextObservationLease(ctx, "worker-observer", workflowAt.Add(5*time.Minute), 10*time.Minute)
	if err != nil || !found || observed.Task.Status != generationapp.StatusRunning || observationLease.Owner != "worker-observer" {
		t.Fatalf("observation queue claim was incorrect: found=%v task=%+v lease=%+v err=%v", found, observed.Task, observationLease, err)
	}
	observed, err = workerRepository.ApplyProviderState(ctx, observationLease, "provider-task-1", generationapp.StatusValidating, "", workflowAt.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("apply generation provider state: %v", err)
	}
	if observed.Task.Status != generationapp.StatusValidating || observed.Task.ExternalTaskID != "provider-task-1" || observed.Task.SubmissionState != generationapp.SubmissionAccepted {
		t.Fatalf("generation provider state observation was not persisted: %+v", observed.Task)
	}
	if _, err := workerRepository.ReleaseLease(ctx, observationLease, workflowAt.Add(5*time.Minute)); err != nil {
		t.Fatalf("release generation observation lease: %v", err)
	}
	observedView, observationLease, err := workerRepository.AcquireObservationLease(ctx, paidCreated.View.Task.ID, "worker-observer", workflowAt.Add(6*time.Minute), 10*time.Minute)
	if err != nil {
		t.Fatalf("acquire accepted generation observation lease: %v", err)
	}
	if observedView.Task.Status != generationapp.StatusValidating || observationLease.Owner != "worker-observer" {
		t.Fatalf("accepted generation observation lease was not fenced: task=%+v lease=%+v", observedView.Task, observationLease)
	}
	if _, err := workerRepository.ReleaseLease(ctx, observationLease, workflowAt.Add(6*time.Minute)); err != nil {
		t.Fatalf("release accepted observation lease before result lease: %v", err)
	}
	resultView, resultLease, found, err := workerRepository.ClaimNextResultLease(ctx, "worker-result", workflowAt.Add(6*time.Minute), 10*time.Minute)
	if err != nil || !found {
		t.Fatalf("claim validating generation result lease: found=%v err=%v", found, err)
	}
	outputAt := workflowAt.Add(7 * time.Minute)
	outputObjectKey, err := generationapp.OutputObjectKey(resultView.Task)
	if err != nil {
		t.Fatalf("build task-bound output key: %v", err)
	}
	asset, err := generationapp.NewOutputAsset(
		"00000000-0000-4000-8000-000000000401",
		resultView.Task,
		generationapp.OutputFact{ObjectKey: outputObjectKey, ContentType: generationapp.OutputContentTypeJPEG, ByteSize: 4096, SHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", ObjectVersionID: "local-object-v1"},
		outputAt,
	)
	if err != nil {
		t.Fatalf("build generation output asset: %v", err)
	}
	published, err := workerRepository.PublishOutput(ctx, resultLease, asset, outputAt)
	if err != nil {
		t.Fatalf("publish generation output: %v", err)
	}
	if published.Task.Status != generationapp.StatusSucceeded || published.Task.ResultAssetID != asset.ID || published.Task.LeaseOwner != "" || published.Task.LeaseUntil != nil || published.Reservation == nil || published.Reservation.State != generationapp.ReservationConsumed || published.Asset == nil || published.Asset.ID != asset.ID {
		t.Fatalf("generation output settlement was not atomic: %+v", published)
	}
	var storedOutputObjectKey string
	if err := database.WithContext(ctx).Table("generation_outputs").Where("task_id = ?", resultView.Task.ID).Select("object_key").Scan(&storedOutputObjectKey).Error; err != nil || storedOutputObjectKey != outputObjectKey {
		t.Fatalf("generation output key was not persisted: key=%q err=%v", storedOutputObjectKey, err)
	}
	if err := database.WithContext(ctx).Table("generation_outputs").Where("task_id = ?", resultView.Task.ID).Update("object_key", "").Error; err != nil {
		t.Fatalf("simulate legacy output without object key: %v", err)
	}
	legacyOutputView, err := workerRepository.Get(ctx, second.User.ID, resultView.Task.ID)
	if err != nil || legacyOutputView.Asset == nil || legacyOutputView.Asset.ObjectKey != "" {
		t.Fatalf("legacy output could not be read: view=%+v err=%v", legacyOutputView, err)
	}
	if err := database.WithContext(ctx).Table("generation_outputs").Where("task_id = ?", resultView.Task.ID).Update("object_key", outputObjectKey).Error; err != nil {
		t.Fatalf("restore generation output object key: %v", err)
	}
	replayedOutput, err := workerRepository.PublishOutput(ctx, resultLease, *published.Asset, outputAt)
	if err != nil {
		t.Fatalf("replay generation output settlement: %v", err)
	}
	if replayedOutput.Task.StatusRevision != published.Task.StatusRevision || replayedOutput.Task.ResultAssetID != published.Task.ResultAssetID || replayedOutput.Reservation == nil || replayedOutput.Reservation.State != generationapp.ReservationConsumed || replayedOutput.Asset == nil || replayedOutput.Asset.ID != published.Asset.ID {
		t.Fatalf("generation output settlement replay was not idempotent: %+v", replayedOutput)
	}
	usedQuotaPolicy := paidPolicy
	usedQuotaPolicy.MaxQuotaUnits = 3
	usedQuotaGenerations, err := generationapp.NewServiceWithCostEstimator(accounts, store.NewGenerationRepository(database), usedQuotaPolicy, paidEstimator)
	if err != nil {
		t.Fatalf("construct used quota generation service: %v", err)
	}
	usedQuotaInput := paidInput
	usedQuotaInput.IdempotencyKey = "generation-after-consumed-quota"
	usedQuotaInput.Parameters = []byte(`{"seed":"after-consumed-quota"}`)
	var jobsBeforeQuotaRejection int64
	if err := database.WithContext(ctx).Table("generation_jobs").Where("owner_id = ? AND purpose = ?", second.User.ID, string(generationapp.PurposeImage)).Count(&jobsBeforeQuotaRejection).Error; err != nil {
		t.Fatalf("count image tasks before consumed-quota rejection: %v", err)
	}
	if _, err := usedQuotaGenerations.Create(ctx, second.Token, usedQuotaInput); !errors.Is(err, generationapp.ErrGenerationQuotaExceeded) {
		t.Fatalf("successful image quota was not charged against the next request: %v", err)
	}
	var jobsAfterQuotaRejection int64
	if err := database.WithContext(ctx).Table("generation_jobs").Where("owner_id = ? AND purpose = ?", second.User.ID, string(generationapp.PurposeImage)).Count(&jobsAfterQuotaRejection).Error; err != nil {
		t.Fatalf("count image tasks after consumed-quota rejection: %v", err)
	}
	if jobsAfterQuotaRejection != jobsBeforeQuotaRejection {
		t.Fatalf("quota-rejected request persisted an image task: before=%d after=%d", jobsBeforeQuotaRejection, jobsAfterQuotaRejection)
	}

	cancelInput := paidInput
	cancelInput.IdempotencyKey = "generation-cancel-after-acceptance"
	cancelInput.Parameters = []byte(`{"seed":"cancel"}`)
	cancelInput.Inputs.References = []generationapp.InputReference{
		{MediaID: "00000000-0000-4000-8000-000000000203", Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{MediaID: "00000000-0000-4000-8000-000000000204", Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 3, SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	cancelCreated, err := paidGenerations.Create(ctx, second.Token, cancelInput)
	if err != nil {
		t.Fatalf("accept cancellation workflow task: %v", err)
	}
	cancelAt := time.Now().UTC()
	_, cancelLease, err := workerRepository.AcquireLease(ctx, cancelCreated.View.Task.ID, "worker-cancel-submit", cancelAt, 10*time.Minute)
	if err != nil {
		t.Fatalf("acquire cancellation workflow lease: %v", err)
	}
	if _, _, err := workerRepository.BeginSubmission(ctx, cancelLease, cancelAt); err != nil {
		t.Fatalf("begin cancellation workflow submission: %v", err)
	}
	cancelAccepted, err := workerRepository.RecordExternalTaskID(ctx, cancelLease, "provider-task-cancel", cancelAt)
	if err != nil || cancelAccepted.Task.SubmissionState != generationapp.SubmissionAccepted {
		t.Fatalf("record cancellation workflow provider identity: view=%+v err=%v", cancelAccepted, err)
	}
	if _, err := workerRepository.ReleaseLease(ctx, cancelLease, cancelAt); err != nil {
		t.Fatalf("release cancellation workflow submission lease: %v", err)
	}
	cancelObserveAt := time.Now().UTC()
	_, cancelObservationLease, found, err := workerRepository.ClaimNextObservationLease(ctx, "worker-cancel-observe", cancelObserveAt, 10*time.Minute)
	if err != nil || !found {
		t.Fatalf("claim cancellation workflow observation lease: found=%v err=%v", found, err)
	}
	_, err = workerRepository.ApplyProviderState(ctx, cancelObservationLease, "provider-task-cancel", generationapp.StatusValidating, "", cancelObserveAt)
	if err != nil {
		t.Fatalf("move cancellation workflow to validating: %v", err)
	}
	if _, err := workerRepository.ReleaseLease(ctx, cancelObservationLease, cancelObserveAt); err != nil {
		t.Fatalf("release cancellation workflow observation lease: %v", err)
	}
	cancelRequested, err := paidGenerations.Cancel(ctx, second.Token, cancelCreated.View.Task.ID)
	if err != nil || cancelRequested.Task.CancelRequestedAt == nil || cancelRequested.Task.Status != generationapp.StatusValidating {
		t.Fatalf("request cancellation after provider acceptance: view=%+v err=%v", cancelRequested, err)
	}
	cancelResultView, cancelResultLease, found, err := workerRepository.ClaimNextResultLease(ctx, "worker-cancel-result", cancelRequested.Task.UpdatedAt, 10*time.Minute)
	if err != nil || !found {
		t.Fatalf("claim canceled result lease: found=%v err=%v", found, err)
	}
	canceledResult, err := workerRepository.FinalizeWithoutOutput(ctx, cancelResultLease, generationapp.StatusCanceled, "", cancelResultView.Task.UpdatedAt)
	if err != nil {
		t.Fatalf("settle canceled result before fetch: %v", err)
	}
	if canceledResult.Task.Status != generationapp.StatusCanceled || canceledResult.Task.ResultAssetID != "" || canceledResult.Task.LeaseOwner != "" || canceledResult.Reservation == nil || canceledResult.Reservation.State != generationapp.ReservationReleased {
		t.Fatalf("canceled result settlement was not atomic: %+v", canceledResult)
	}

	modelInput := paidInput
	modelInput.IdempotencyKey = "generation-model-dependent"
	modelInput.Purpose = generationapp.PurposeModel
	modelInput.Model = "look-model-v1"
	modelInput.Parameters = []byte(`{"seed":"model"}`)
	modelInput.Inputs = generationapp.InputSnapshot{
		LookID:       paidInput.LookID,
		LookRevision: paidInput.LookRevision,
		References:   []generationapp.InputReference{{MediaID: asset.ID, Role: generationapp.InputRoleLookImage, Ordinal: 0, Revision: 1, SHA256: asset.SHA256}},
		ImageAssetID: asset.ID,
		ImageSHA256:  asset.SHA256,
	}
	modelInput.Consent = generationapp.ConsentReceipt{ID: "00000000-0000-4000-8000-000000000402", Purpose: generationapp.PurposeModel, PolicyVersion: "local-model-v1", AcceptedAt: acceptedAt}
	if _, err := paidGenerations.Create(ctx, first.Token, modelInput); !errors.Is(err, generationapp.ErrGenerationSourceUnavailable) {
		t.Fatalf("cross-owner model source was accepted with error %v", err)
	}
	invalidSource := modelInput
	invalidSource.IdempotencyKey = "generation-model-hash-mismatch"
	invalidSource.Inputs.References = append([]generationapp.InputReference(nil), modelInput.Inputs.References...)
	invalidSource.Inputs.ImageSHA256 = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	invalidSource.Inputs.References[0].SHA256 = invalidSource.Inputs.ImageSHA256
	if _, err := paidGenerations.Create(ctx, second.Token, invalidSource); !errors.Is(err, generationapp.ErrGenerationSourceUnavailable) {
		t.Fatalf("model source hash mismatch was accepted with error %v", err)
	}
	purposePolicy := generationapp.AdmissionPolicy{Enabled: true, Currency: "USD", MaxConcurrentTasks: 1, MaxQuotaUnits: 5, MaxBudgetMinorUnits: 100}
	purposeEstimator := generationapp.CostEstimatorFunc(func(generationapp.Purpose, string, string, []byte) (generationapp.CostEstimate, error) {
		return generationapp.CostEstimate{Currency: "USD", EstimatedMinorUnits: 60, ReservedQuotaUnits: 3}, nil
	})
	purposeGenerations, err := generationapp.NewServiceWithCostEstimator(accounts, store.NewGenerationRepository(database), purposePolicy, purposeEstimator)
	if err != nil {
		t.Fatalf("construct purpose-scoped generation service: %v", err)
	}
	purposeImageInput := paidInput
	purposeImageInput.IdempotencyKey = "generation-purpose-scoped-image"
	purposeImageInput.Parameters = []byte(`{"seed":"purpose-scoped-image"}`)
	purposeImageInput.Inputs.References = []generationapp.InputReference{
		{MediaID: "00000000-0000-4000-8000-000000000211", Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},
		{MediaID: "00000000-0000-4000-8000-000000000212", Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 1, SHA256: "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"},
	}
	purposeImage, err := purposeGenerations.Create(ctx, second.Token, purposeImageInput)
	if err != nil {
		t.Fatalf("accept image task with purpose-scoped quota: %v", err)
	}
	if purposeImage.View.Task.Status != generationapp.StatusQueued || purposeImage.View.Reservation == nil || purposeImage.View.Reservation.Purpose != generationapp.PurposeImage || purposeImage.View.Reservation.EstimatedMinorUnits != 60 || purposeImage.View.Reservation.ReservedQuotaUnits != 3 {
		t.Fatalf("image purpose reservation was not persisted: %+v", purposeImage)
	}
	modelCreated, err := purposeGenerations.Create(ctx, second.Token, modelInput)
	if err != nil {
		t.Fatalf("accept model task with independent purpose limits: %v", err)
	}
	if modelCreated.View.Task.Status != generationapp.StatusQueued || modelCreated.View.Reservation == nil || modelCreated.View.Reservation.Purpose != generationapp.PurposeModel || modelCreated.View.Reservation.EstimatedMinorUnits != 60 || modelCreated.View.Reservation.ReservedQuotaUnits != 3 {
		t.Fatalf("model purpose reservation was not persisted independently: %+v", modelCreated)
	}
	purposeImageLeaseAt := purposeImage.View.Task.UpdatedAt.Add(time.Minute)
	_, purposeImageLease, err := workerRepository.AcquireLease(ctx, purposeImage.View.Task.ID, "worker-purpose-image", purposeImageLeaseAt, 5*time.Minute)
	if err != nil {
		t.Fatalf("acquire synthetic purpose image lease: %v", err)
	}
	purposeImageFailure, err := workerRepository.FinalizeWithoutOutput(ctx, purposeImageLease, generationapp.StatusFailed, "provider_error", purposeImageLeaseAt.Add(time.Minute))
	if err != nil || purposeImageFailure.Reservation == nil || purposeImageFailure.Reservation.State != generationapp.ReservationReleased {
		t.Fatalf("settle synthetic purpose image task: view=%+v err=%v", purposeImageFailure, err)
	}
	workflowPolicy := paidPolicy
	workflowPolicy.MaxQuotaUnits = 10
	paidGenerations, err = generationapp.NewServiceWithCostEstimator(accounts, store.NewGenerationRepository(database), workflowPolicy, paidEstimator)
	if err != nil {
		t.Fatalf("construct generation service for remaining lifecycle checks: %v", err)
	}
	deletionAt := published.Task.UpdatedAt.Add(time.Minute)
	workerRepository = store.NewGenerationRepository(database)
	sourceRequests, err := workerRepository.RequestSourceCleanup(ctx, second.User.ID, paidInput.Inputs.References[0].MediaID, deletionAt)
	if err != nil || len(sourceRequests) != 1 || sourceRequests[0].TaskID != paidCreated.View.Task.ID || sourceRequests[0].Scope != generationapp.CleanupScopeSource {
		t.Fatalf("source cleanup did not claim the image task: requests=%+v err=%v", sourceRequests, err)
	}
	dependentModel, err := workerRepository.Get(ctx, second.User.ID, modelCreated.View.Task.ID)
	if err != nil {
		t.Fatalf("reload dependent model cleanup: %v", err)
	}
	if dependentModel.Cleanup == nil || dependentModel.Cleanup.Scope != generationapp.CleanupScopeTask || dependentModel.Cleanup.Status != generationapp.CleanupPending || dependentModel.Task.CancelRequestedAt == nil {
		t.Fatalf("source cleanup did not cascade to dependent model: %+v", dependentModel)
	}
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("task_id = ? AND scope = ?", modelCreated.View.Task.ID, string(generationapp.CleanupScopeTask)).Update("next_attempt_at", time.Now().UTC().Add(time.Hour)).Error; err != nil {
		t.Fatalf("delay dependent model cleanup outside queue-claim assertions: %v", err)
	}
	revokedModelInput := modelInput
	revokedModelInput.IdempotencyKey = "generation-model-after-source-revocation"
	if _, err := paidGenerations.Create(ctx, second.Token, revokedModelInput); !errors.Is(err, generationapp.ErrGenerationSourceUnavailable) {
		t.Fatalf("revoked source was reused for a new model task with error %v", err)
	}

	deletedView, deletedCleanup, err := workerRepository.RequestTaskCleanup(ctx, second.User.ID, paidCreated.View.Task.ID, deletionAt)
	if err != nil {
		t.Fatalf("request generation cleanup: %v", err)
	}
	if deletedView.Task.AccessRevokedAt == nil || deletedView.Asset != nil || deletedView.Cleanup == nil || deletedView.Cleanup.ID != deletedCleanup.ID || deletedCleanup.Status != generationapp.CleanupPending || len(deletedCleanup.Targets) != 2 || deletedCleanup.Targets[0].ObjectKey != outputObjectKey {
		t.Fatalf("generation cleanup did not revoke access or retain targets: view=%+v cleanup=%+v", deletedView, deletedCleanup)
	}
	claimedCleanup, targets, err := workerRepository.BeginTaskCleanup(ctx, deletedCleanup.ID, deletedCleanup.AccessRevokedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("begin generation cleanup: %v", err)
	}
	if claimedCleanup.Status != generationapp.CleanupRunning || len(targets) != 2 || targets[0].Kind != generationapp.CleanupTargetObject || targets[0].ObjectKey != outputObjectKey || targets[0].ObjectVersionID != "local-object-v1" || targets[1].Kind != generationapp.CleanupTargetProvider {
		t.Fatalf("generation cleanup claim lost its target snapshot: request=%+v targets=%+v", claimedCleanup, targets)
	}
	completedCleanup, err := workerRepository.CompleteTaskCleanup(ctx, claimedCleanup, claimedCleanup.UpdatedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("complete generation cleanup: %v", err)
	}
	if completedCleanup.Status != generationapp.CleanupComplete || completedCleanup.CompletedAt == nil {
		t.Fatalf("generation cleanup did not complete: %+v", completedCleanup)
	}
	loadedDeleted, err := workerRepository.Get(ctx, second.User.ID, paidCreated.View.Task.ID)
	if err != nil {
		t.Fatalf("reload revoked generation task: %v", err)
	}
	if loadedDeleted.Task.AccessRevokedAt == nil || loadedDeleted.Asset != nil || loadedDeleted.Cleanup == nil || loadedDeleted.Cleanup.Status != generationapp.CleanupComplete {
		t.Fatalf("revoked generation output became visible again: %+v", loadedDeleted)
	}
	var outputCount int64
	if err := database.WithContext(ctx).Table("generation_outputs").Where("task_id = ?", paidCreated.View.Task.ID).Count(&outputCount).Error; err != nil {
		t.Fatalf("count cleaned generation output: %v", err)
	}
	if outputCount != 0 {
		t.Fatalf("generation output metadata remained after cleanup: %d", outputCount)
	}
	deletedViewAgain, deletedCleanupAgain, err := workerRepository.RequestTaskCleanup(ctx, second.User.ID, paidCreated.View.Task.ID, completedCleanup.CompletedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("repeat generation cleanup request: %v", err)
	}
	if deletedViewAgain.Task.AccessRevokedAt == nil || deletedCleanupAgain.Status != generationapp.CleanupComplete || deletedCleanupAgain.ID != deletedCleanup.ID {
		t.Fatalf("repeat generation cleanup was not idempotent: view=%+v cleanup=%+v", deletedViewAgain, deletedCleanupAgain)
	}

	failureInput := paidInput
	failureInput.IdempotencyKey = "generation-paid-failure"
	failureInput.Parameters = []byte(`{"seed":"failure"}`)
	failureCreated, err := paidGenerations.Create(ctx, second.Token, failureInput)
	if err != nil {
		t.Fatalf("accept failed generation task: %v", err)
	}
	failureLeaseAt := failureCreated.View.Task.UpdatedAt.Add(time.Minute)
	_, failureLease, err := workerRepository.AcquireLease(ctx, failureCreated.View.Task.ID, "worker-failure", failureLeaseAt, 10*time.Minute)
	if err != nil {
		t.Fatalf("acquire failed generation lease: %v", err)
	}
	failureAt := failureLeaseAt.Add(time.Minute)
	finalized, err := workerRepository.FinalizeWithoutOutput(ctx, failureLease, generationapp.StatusFailed, "provider_error", failureAt)
	if err != nil {
		t.Fatalf("finalize failed generation task: %v", err)
	}
	if finalized.Task.Status != generationapp.StatusFailed || finalized.Task.FailureCode != "provider_error" || finalized.Task.LeaseOwner != "" || finalized.Task.LeaseUntil != nil || finalized.Reservation == nil || finalized.Reservation.State != generationapp.ReservationReleased || finalized.Asset != nil {
		t.Fatalf("failed generation settlement was not atomic: %+v", finalized)
	}
	replayedFailure, err := workerRepository.FinalizeWithoutOutput(ctx, failureLease, generationapp.StatusFailed, "provider_error", failureAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("replay failed generation settlement: %v", err)
	}
	if replayedFailure.Task.StatusRevision != finalized.Task.StatusRevision || replayedFailure.Task.FailureCode != finalized.Task.FailureCode || replayedFailure.Reservation == nil || replayedFailure.Reservation.State != generationapp.ReservationReleased || replayedFailure.Asset != nil {
		t.Fatalf("failed generation settlement replay was not idempotent: %+v", replayedFailure)
	}

	canceled, err := generations.Cancel(ctx, first.Token, created.View.Task.ID)
	if err != nil {
		t.Fatalf("request generation cancellation: %v", err)
	}
	if canceled.Task.CancelRequestedAt == nil || canceled.Task.Status != generationapp.StatusRunning || canceled.Task.StatusRevision != 4 {
		t.Fatalf("cancellation did not persist the request: %+v", canceled.Task)
	}
	canceledAgain, err := generations.Cancel(ctx, first.Token, created.View.Task.ID)
	if err != nil {
		t.Fatalf("repeat generation cancellation: %v", err)
	}
	if canceledAgain.Task.StatusRevision != canceled.Task.StatusRevision || !canceledAgain.Task.CancelRequestedAt.Equal(*canceled.Task.CancelRequestedAt) {
		t.Fatalf("repeat cancellation was not idempotent: %+v", canceledAgain.Task)
	}

	page, err := generations.List(ctx, first.Token, 20, nil)
	if err != nil {
		t.Fatalf("list first owner's generation tasks: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Task.ID != created.View.Task.ID || page.NextAfterID != nil {
		t.Fatalf("owner-scoped generation list was incorrect: %+v", page)
	}
	loaded, err := generations.Get(ctx, first.Token, created.View.Task.ID)
	if err != nil {
		t.Fatalf("reload canceled generation task: %v", err)
	}
	if loaded.Task.CancelRequestedAt == nil || loaded.Task.StatusRevision != 4 {
		t.Fatalf("canceled generation task was not durable: %+v", loaded.Task)
	}
	sourceRequests, err = workerRepository.RequestSourceCleanup(ctx, first.User.ID, "00000000-0000-4000-8000-000000000201", loaded.Task.UpdatedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("request source generation cleanup: %v", err)
	}
	if len(sourceRequests) != 1 || sourceRequests[0].Scope != generationapp.CleanupScopeSource || sourceRequests[0].SourceMediaID != "00000000-0000-4000-8000-000000000201" {
		t.Fatalf("source cleanup did not retain the source relationship: %+v", sourceRequests)
	}
	claimedSource, sourceTargets, found, err := workerRepository.ClaimNextCleanup(ctx, sourceRequests[0].UpdatedAt.Add(time.Minute), time.Minute)
	if err != nil || !found || claimedSource.Status != generationapp.CleanupRunning || len(sourceTargets) != 0 {
		t.Fatalf("cleanup worker did not claim source cleanup: found=%v request=%+v targets=%+v err=%v", found, claimedSource, sourceTargets, err)
	}
	recoveryAt := claimedSource.UpdatedAt.Add(time.Minute)
	recoveredSource, _, found, err := workerRepository.ClaimNextCleanup(ctx, recoveryAt, time.Minute)
	if err != nil || !found || recoveredSource.Status != generationapp.CleanupRunning || recoveredSource.Attempts != claimedSource.Attempts+1 {
		t.Fatalf("stale cleanup claim was not recovered: found=%v old=%+v new=%+v err=%v", found, claimedSource, recoveredSource, err)
	}
	if _, err := workerRepository.CompleteTaskCleanup(ctx, claimedSource, recoveryAt.Add(time.Minute)); !errors.Is(err, generationapp.ErrGenerationCleanupClaim) {
		t.Fatalf("stale cleanup worker completed after recovery: %v", err)
	}
	if _, err := workerRepository.CompleteTaskCleanup(ctx, recoveredSource, recoveryAt.Add(time.Minute)); err != nil {
		t.Fatalf("recovered cleanup worker could not complete: %v", err)
	}
	accountDeletion, err := accounts.DeleteCurrentUser(ctx, first.Token)
	if err != nil {
		t.Fatalf("begin account deletion with generation cleanup: %v", err)
	}
	if accountDeletion.Status != accountapp.AccountDeletionPending || accountDeletion.GenerationCount != 1 || accountDeletion.RemainingGenerationCount != 1 || accountDeletion.Phase != "generation_cleanup" {
		t.Fatalf("account deletion did not wait for generation cleanup: %+v", accountDeletion)
	}
	var accountCleanupID string
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("account_deletion_id = ?", accountDeletion.ID).Pluck("id", &accountCleanupID).Error; err != nil {
		t.Fatalf("find account generation cleanup request: %v", err)
	}
	accountCleanup, _, err := workerRepository.BeginTaskCleanup(ctx, accountCleanupID, accountDeletion.RequestedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("begin account generation cleanup: %v", err)
	}
	if _, err := workerRepository.CompleteTaskCleanup(ctx, accountCleanup, accountCleanup.UpdatedAt.Add(time.Minute)); err != nil {
		t.Fatalf("complete account generation cleanup: %v", err)
	}
	completedAccountDeletion, err := accounts.GetDeletionReceipt(ctx, accountDeletion.ID, accountDeletion.ReceiptToken)
	if err != nil {
		t.Fatalf("read completed account deletion receipt: %v", err)
	}
	if completedAccountDeletion.Status != accountapp.AccountDeletionComplete || completedAccountDeletion.RemainingGenerationCount != 0 || completedAccountDeletion.Phase != "complete" {
		t.Fatalf("account deletion did not complete after generation cleanup: %+v", completedAccountDeletion)
	}

	lateInput := input
	lateInput.IdempotencyKey = "generation-late-acceptance-cleanup"
	lateInput.Parameters = []byte(`{"seed":"late-cleanup"}`)
	lateCreated, err := generations.Create(ctx, third.Token, lateInput)
	if err != nil {
		t.Fatalf("accept late generation task: %v", err)
	}
	lateLeaseAt := lateCreated.View.Task.UpdatedAt.Add(time.Minute)
	_, lateLease, err := workerRepository.AcquireLease(ctx, lateCreated.View.Task.ID, "worker-late-submit", lateLeaseAt, 10*time.Minute)
	if err != nil {
		t.Fatalf("acquire late generation lease: %v", err)
	}
	if _, _, err := workerRepository.BeginSubmission(ctx, lateLease, lateLeaseAt); err != nil {
		t.Fatalf("begin late generation submission: %v", err)
	}
	lateDelete, err := generations.Delete(ctx, third.Token, lateCreated.View.Task.ID)
	if err != nil {
		t.Fatalf("request cleanup before late provider acceptance: %v", err)
	}
	lateDeleted, lateCleanup := lateDelete.View, lateDelete.Cleanup
	if lateCleanup.Status != generationapp.CleanupPending || len(lateCleanup.Targets) != 0 || lateDeleted.Task.AccessRevokedAt == nil {
		t.Fatalf("late acceptance cleanup started with unexpected manifest: view=%+v cleanup=%+v", lateDeleted, lateCleanup)
	}
	claimedLate, lateTargets, err := workerRepository.BeginTaskCleanup(ctx, lateCleanup.ID, lateCleanup.UpdatedAt.Add(time.Minute))
	if err != nil || len(lateTargets) != 0 {
		t.Fatalf("claim empty late acceptance cleanup: request=%+v targets=%+v err=%v", claimedLate, lateTargets, err)
	}
	lateAccepted, err := workerRepository.RecordExternalTaskID(ctx, lateLease, "provider-late-cleanup", claimedLate.UpdatedAt.Add(time.Minute))
	if err != nil || lateAccepted.Task.ExternalTaskID != "provider-late-cleanup" {
		t.Fatalf("late provider acceptance was not retained: view=%+v err=%v", lateAccepted, err)
	}
	if _, err := workerRepository.CompleteTaskCleanup(ctx, claimedLate, lateAccepted.Task.UpdatedAt.Add(time.Minute)); !errors.Is(err, generationapp.ErrGenerationCleanupClaim) {
		t.Fatalf("stale cleanup worker completed after late target insertion: %v", err)
	}
	lateReloaded, err := generations.Get(ctx, third.Token, lateCreated.View.Task.ID)
	if err != nil {
		t.Fatalf("reload late acceptance cleanup: %v", err)
	}
	if lateReloaded.Cleanup == nil || lateReloaded.Cleanup.Status != generationapp.CleanupPending || len(lateReloaded.Cleanup.Targets) != 1 || lateReloaded.Cleanup.Targets[0].Kind != generationapp.CleanupTargetProvider || lateReloaded.Cleanup.Targets[0].ID != "provider-late-cleanup" {
		t.Fatalf("late provider identity was not added to cleanup manifest: %+v", lateReloaded)
	}
	reclaimedLate, lateTargets, err := workerRepository.BeginTaskCleanup(ctx, lateReloaded.Cleanup.ID, lateReloaded.Cleanup.UpdatedAt.Add(time.Minute))
	if err != nil || len(lateTargets) != 1 || lateTargets[0].Kind != generationapp.CleanupTargetProvider || lateTargets[0].ID != "provider-late-cleanup" {
		t.Fatalf("reopened late acceptance cleanup lost provider target: request=%+v targets=%+v err=%v", reclaimedLate, lateTargets, err)
	}
	if _, err := workerRepository.CompleteTaskCleanup(ctx, reclaimedLate, reclaimedLate.UpdatedAt.Add(time.Minute)); err != nil {
		t.Fatalf("complete reopened late acceptance cleanup: %v", err)
	}
	if _, err := workerRepository.ReleaseLease(ctx, lateLease, lateAccepted.Task.UpdatedAt.Add(time.Minute)); err != nil {
		t.Fatalf("release late acceptance lease: %v", err)
	}
	maxCleanupRequests, err := workerRepository.RequestSourceCleanup(ctx, third.User.ID, lateInput.Inputs.References[0].MediaID, reclaimedLate.UpdatedAt.Add(time.Minute))
	if err != nil || len(maxCleanupRequests) != 1 {
		t.Fatalf("create cleanup retry exhaustion fixture: requests=%+v err=%v", maxCleanupRequests, err)
	}
	maxCleanup := maxCleanupRequests[0]
	claimAt := maxCleanup.UpdatedAt.Add(2 * time.Minute)
	failedAt := claimAt.Add(time.Minute)
	// This lifecycle leaves other cleanup fixtures pending for later assertions;
	// delay them so the queue claims below isolate the exhaustion fixture.
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("id <> ? AND status IN ?", maxCleanup.ID, []string{string(generationapp.CleanupPending), string(generationapp.CleanupFailed)}).Update("next_attempt_at", claimAt.Add(time.Hour)).Error; err != nil {
		t.Fatalf("delay unrelated cleanup fixtures: %v", err)
	}
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("id = ?", maxCleanup.ID).Updates(map[string]any{
		"status":          string(generationapp.CleanupFailed),
		"completed_at":    nil,
		"stable_error":    "target_delete_failed",
		"attempts":        generationapp.MaxCleanupAttempts - 1,
		"next_attempt_at": claimAt.Add(-time.Second),
		"updated_at":      claimAt.Add(-time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed final-attempt cleanup: %v", err)
	}
	claimedFinal, _, found, err := workerRepository.ClaimNextCleanup(ctx, claimAt, time.Minute)
	if err != nil || !found {
		t.Fatalf("final-attempt cleanup was not claimed: found=%v err=%v", found, err)
	}
	failedFinal, err := workerRepository.FailTaskCleanup(ctx, claimedFinal, failedAt, "target_delete_failed", failedAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("persist final target failure: %v", err)
	}
	if failedFinal.Status != generationapp.CleanupFailed || failedFinal.StableError != "cleanup_retry_exhausted" || failedFinal.NextAttemptAt != nil || failedFinal.Attempts != generationapp.MaxCleanupAttempts {
		t.Fatalf("final target failure was not durably exhausted: %+v", failedFinal)
	}
	if _, _, found, err := workerRepository.ClaimNextCleanup(ctx, failedAt.Add(time.Minute), time.Minute); err != nil || found {
		t.Fatalf("exhausted failed cleanup remained claimable: found=%v err=%v", found, err)
	}
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("id = ?", maxCleanup.ID).Updates(map[string]any{
		"status":          string(generationapp.CleanupRunning),
		"completed_at":    nil,
		"stable_error":    "",
		"attempts":        generationapp.MaxCleanupAttempts,
		"next_attempt_at": nil,
		"updated_at":      failedAt,
	}).Error; err != nil {
		t.Fatalf("seed stale exhausted running cleanup: %v", err)
	}
	recoveredAt := failedAt.Add(2 * time.Minute)
	if _, _, found, err := workerRepository.ClaimNextCleanup(ctx, recoveredAt, time.Minute); err != nil || found {
		t.Fatalf("stale exhausted cleanup was claimed: found=%v err=%v", found, err)
	}
	var exhaustedState struct {
		Status      string
		StableError string
		Attempts    int
	}
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Select("status, stable_error, attempts").Where("id = ?", maxCleanup.ID).Scan(&exhaustedState).Error; err != nil {
		t.Fatalf("read exhausted cleanup state: %v", err)
	}
	if exhaustedState.Status != string(generationapp.CleanupFailed) || exhaustedState.StableError != "cleanup_retry_exhausted" || exhaustedState.Attempts != generationapp.MaxCleanupAttempts {
		t.Fatalf("stale exhausted cleanup was not durably settled: %+v", exhaustedState)
	}

	recoveryInput := input
	recoveryInput.IdempotencyKey = "generation-recovered-in-flight"
	recoveryInput.LookID = "00000000-0000-4000-8000-000000000102"
	recoveryInput.Parameters = []byte(`{"seed":"recovery"}`)
	recoveryInput.Inputs.LookID = recoveryInput.LookID
	recoveryTask, err := generations.Create(ctx, third.Token, recoveryInput)
	if err != nil {
		t.Fatalf("create stale in-flight recovery task: %v", err)
	}
	crashedAt := recoveryTask.View.Task.CreatedAt
	_, crashedLease, err := workerRepository.AcquireLease(ctx, recoveryTask.View.Task.ID, "worker-crashed", crashedAt, time.Minute)
	if err != nil {
		t.Fatalf("acquire lease before simulated worker crash: %v", err)
	}
	if _, _, err := workerRepository.BeginSubmission(ctx, crashedLease, crashedAt); err != nil {
		t.Fatalf("persist in-flight submission before simulated crash: %v", err)
	}
	if err := database.WithContext(ctx).Table("generation_jobs").Where("id = ?", recoveryTask.View.Task.ID).Update("lease_until", time.Now().UTC().Add(-time.Second)).Error; err != nil {
		t.Fatalf("expire crashed worker lease: %v", err)
	}

	provider := &generationRecoveryProviderStub{}
	recoveryWorker, err := generationapp.NewSubmissionWorker(workerRepository, provider, generationapp.SubmissionWorkerPolicy{
		WorkerID:        "worker-recovery",
		LeaseTTL:        time.Minute,
		ProviderTimeout: time.Second,
		Retry:           generationapp.RetryPolicy{MaxAttempts: 3, BaseDelay: time.Second, MaxDelay: time.Minute},
	})
	if err != nil {
		t.Fatalf("construct submission recovery worker: %v", err)
	}
	found, recoveredResult, err := recoveryWorker.RunNext(ctx)
	if !errors.Is(err, generationapp.ErrGenerationSubmissionUnknown) || !found || recoveredResult.View.Task.ID != recoveryTask.View.Task.ID {
		t.Fatalf("expired in-flight task was not durably recovered: found=%v task=%+v err=%v", found, recoveredResult.View.Task, err)
	}
	if recoveredResult.View.Task.SubmissionState != generationapp.SubmissionUnknown || recoveredResult.View.Task.ExternalTaskID != "" || recoveredResult.View.Task.LeaseOwner != "" || recoveredResult.View.Task.FencingToken != crashedLease.FencingToken+1 || provider.submitCalls != 0 {
		t.Fatalf("recovery did not fence without resubmitting: task=%+v provider submits=%d", recoveredResult.View.Task, provider.submitCalls)
	}
	if found, _, err := recoveryWorker.RunNext(ctx); err != nil || found {
		t.Fatalf("unknown task remained eligible for submission: found=%v err=%v", found, err)
	}

	if err := database.WithContext(ctx).Table("users").Where("id = ?", third.User.ID).Update("role", "admin").Error; err != nil {
		t.Fatalf("grant test administrator role: %v", err)
	}
	adminGenerations, err := generationapp.NewServiceWithCostEstimator(accounts, workerRepository, paidPolicy, paidEstimator)
	if err != nil {
		t.Fatalf("construct admin generation service: %v", err)
	}
	unknownTask := func(key string) (generationapp.TaskView, generationapp.Lease) {
		t.Helper()
		request := paidInput
		request.IdempotencyKey = key
		request.Parameters = []byte(`{"seed":"` + key + `"}`)
		created, err := adminGenerations.Create(ctx, third.Token, request)
		if err != nil {
			t.Fatalf("create %s reconciliation task: %v", key, err)
		}
		at := time.Now().UTC()
		_, lease, err := workerRepository.AcquireLease(ctx, created.View.Task.ID, "worker-"+key, at, 10*time.Minute)
		if err != nil {
			t.Fatalf("acquire %s reconciliation lease: %v", key, err)
		}
		if _, _, err := workerRepository.BeginSubmission(ctx, lease, at); err != nil {
			t.Fatalf("begin %s reconciliation submission: %v", key, err)
		}
		if _, err := workerRepository.MarkSubmissionUnknown(ctx, lease, at); err != nil {
			t.Fatalf("mark %s reconciliation submission unknown: %v", key, err)
		}
		view, err := workerRepository.Get(ctx, third.User.ID, created.View.Task.ID)
		if err != nil {
			t.Fatalf("reload %s unknown submission: %v", key, err)
		}
		return view, lease
	}
	evidence := func(taskID string, revision int, decision generationapp.SubmissionDecision, externalID, reference string) generationapp.ReconcileUnknownInput {
		return generationapp.ReconcileUnknownInput{TaskID: taskID, ExpectedRevision: revision, Decision: decision, ExternalTaskID: externalID, EvidenceType: generationapp.SubmissionEvidenceProviderQuery, EvidenceReference: reference}
	}

	acceptedUnknown, acceptedLease := unknownTask("admin-accepted")
	unknownPage, err := adminGenerations.ListUnknown(ctx, third.Token, 20, nil)
	if err != nil {
		t.Fatalf("list unknown submissions as admin: %v", err)
	}
	foundAcceptedUnknown := false
	for _, item := range unknownPage.Items {
		if item.ID == acceptedUnknown.Task.ID {
			foundAcceptedUnknown = true
			break
		}
	}
	if !foundAcceptedUnknown {
		t.Fatalf("admin listing omitted unknown submission %s: %+v", acceptedUnknown.Task.ID, unknownPage)
	}
	if _, err := adminGenerations.ReconcileUnknown(ctx, third.Token, evidence(acceptedUnknown.Task.ID, acceptedUnknown.Task.StatusRevision, generationapp.SubmissionDecisionAccepted, "provider-admin-accepted", "query-admin-accepted")); !errors.Is(err, generationapp.ErrGenerationLeaseHeld) {
		t.Fatalf("active lease was not rejected: %v", err)
	}
	if _, err := workerRepository.ReleaseLease(ctx, acceptedLease, time.Now().UTC()); err != nil {
		t.Fatalf("release active lease before admin reconciliation: %v", err)
	}
	acceptedCurrent, err := workerRepository.Get(ctx, third.User.ID, acceptedUnknown.Task.ID)
	if err != nil {
		t.Fatalf("reload unknown submission after lease release: %v", err)
	}
	if _, err := adminGenerations.ReconcileUnknown(ctx, third.Token, evidence(acceptedUnknown.Task.ID, acceptedUnknown.Task.StatusRevision, generationapp.SubmissionDecisionAccepted, "provider-admin-accepted", "query-admin-stale")); !errors.Is(err, generationapp.ErrGenerationRevisionConflict) {
		t.Fatalf("stale revision was not rejected: %v", err)
	}
	acceptedResult, err := adminGenerations.ReconcileUnknown(ctx, third.Token, evidence(acceptedUnknown.Task.ID, acceptedCurrent.Task.StatusRevision, generationapp.SubmissionDecisionAccepted, "provider-admin-accepted", "query-admin-accepted"))
	if err != nil {
		t.Fatalf("record confirmed provider acceptance: %v", err)
	}
	if acceptedResult.View.Task.SubmissionState != generationapp.SubmissionAccepted || acceptedResult.View.Task.ExternalTaskID != "provider-admin-accepted" || acceptedResult.View.Task.LeaseOwner != "" || acceptedResult.AuditID == "" {
		t.Fatalf("accepted reconciliation did not attach provider identity and audit: %+v", acceptedResult)
	}
	if _, err := generations.ListUnknown(ctx, second.Token, 20, nil); !errors.Is(err, generationapp.ErrGenerationForbidden) {
		t.Fatalf("non-admin unknown listing returned %v", err)
	}
	finishAcceptedAt := time.Now().UTC()
	_, finishAcceptedLease, err := workerRepository.AcquireLease(ctx, acceptedUnknown.Task.ID, "worker-admin-accepted-finished", finishAcceptedAt, time.Minute)
	if err != nil {
		t.Fatalf("acquire accepted reconciliation task for test settlement: %v", err)
	}
	if _, err := workerRepository.FinalizeWithoutOutput(ctx, finishAcceptedLease, generationapp.StatusFailed, "test_settled", finishAcceptedAt); err != nil {
		t.Fatalf("settle accepted reconciliation fixture: %v", err)
	}

	rejectedUnknown, rejectedLease := unknownTask("admin-not-accepted")
	if _, err := workerRepository.ReleaseLease(ctx, rejectedLease, time.Now().UTC()); err != nil {
		t.Fatalf("release not-accepted lease: %v", err)
	}
	rejectedCurrent, err := workerRepository.Get(ctx, third.User.ID, rejectedUnknown.Task.ID)
	if err != nil {
		t.Fatalf("reload not-accepted unknown submission: %v", err)
	}
	rejectedResult, err := adminGenerations.ReconcileUnknown(ctx, third.Token, evidence(rejectedUnknown.Task.ID, rejectedCurrent.Task.StatusRevision, generationapp.SubmissionDecisionNotAccepted, "", "query-admin-negative"))
	if err != nil {
		t.Fatalf("record confirmed non-acceptance: %v", err)
	}
	if rejectedResult.View.Task.Status != generationapp.StatusFailed || rejectedResult.View.Task.FailureCode != "submission_not_accepted" || rejectedResult.View.Task.SubmissionState != generationapp.SubmissionNotStarted || rejectedResult.View.Reservation == nil || rejectedResult.View.Reservation.State != generationapp.ReservationReleased {
		t.Fatalf("not-accepted reconciliation did not fail and release quota: %+v", rejectedResult.View)
	}

	revokedUnknown, revokedLease := unknownTask("admin-accepted-revoked")
	revoked, err := adminGenerations.Delete(ctx, third.Token, revokedUnknown.Task.ID)
	if err != nil {
		t.Fatalf("revoke unknown submission before late acceptance: %v", err)
	}
	if revoked.View.Task.AccessRevokedAt == nil || revoked.Cleanup.Status != generationapp.CleanupPending {
		t.Fatalf("unknown submission was not revoked before late acceptance: %+v", revoked)
	}
	if _, err := workerRepository.ReleaseLease(ctx, revokedLease, time.Now().UTC()); err != nil {
		t.Fatalf("release revoked submission lease: %v", err)
	}
	revokedCurrent, err := workerRepository.Get(ctx, third.User.ID, revokedUnknown.Task.ID)
	if err != nil {
		t.Fatalf("reload revoked unknown submission: %v", err)
	}
	lateResult, err := adminGenerations.ReconcileUnknown(ctx, third.Token, evidence(revokedUnknown.Task.ID, revokedCurrent.Task.StatusRevision, generationapp.SubmissionDecisionAccepted, "provider-admin-late", "query-admin-late"))
	if err != nil {
		t.Fatalf("record late provider acceptance after revocation: %v", err)
	}
	if lateResult.View.Cleanup == nil || lateResult.View.Cleanup.Status != generationapp.CleanupPending || len(lateResult.View.Cleanup.Targets) != 1 || lateResult.View.Cleanup.Targets[0].ID != "provider-admin-late" {
		t.Fatalf("late accepted identity was not added to cleanup: %+v", lateResult.View.Cleanup)
	}

	revokedRejected, revokedRejectedLease := unknownTask("admin-not-accepted-revoked")
	if _, err := adminGenerations.Delete(ctx, third.Token, revokedRejected.Task.ID); err != nil {
		t.Fatalf("revoke unknown submission before negative reconciliation: %v", err)
	}
	if _, err := workerRepository.ReleaseLease(ctx, revokedRejectedLease, time.Now().UTC()); err != nil {
		t.Fatalf("release revoked negative submission lease: %v", err)
	}
	revokedRejectedCurrent, err := workerRepository.Get(ctx, third.User.ID, revokedRejected.Task.ID)
	if err != nil {
		t.Fatalf("reload revoked negative submission: %v", err)
	}
	reconciledCanceled, err := adminGenerations.ReconcileUnknown(ctx, third.Token, evidence(revokedRejected.Task.ID, revokedRejectedCurrent.Task.StatusRevision, generationapp.SubmissionDecisionNotAccepted, "", "query-admin-revoked-negative"))
	if err != nil {
		t.Fatalf("record non-acceptance for revoked task: %v", err)
	}
	if reconciledCanceled.View.Task.Status != generationapp.StatusCanceled || reconciledCanceled.View.Task.FailureCode != "" || reconciledCanceled.View.Reservation == nil || reconciledCanceled.View.Reservation.State != generationapp.ReservationReleased {
		t.Fatalf("revoked not-accepted submission did not cancel and release quota: %+v", reconciledCanceled.View)
	}
	var reconciliationAuditCount int64
	if err := database.WithContext(ctx).Table("generation_submission_reconciliations").Where("actor_id = ?", third.User.ID).Count(&reconciliationAuditCount).Error; err != nil {
		t.Fatalf("count reconciliation audit rows: %v", err)
	}
	if reconciliationAuditCount != 4 {
		t.Fatalf("expected four atomic reconciliation audit rows, got %d", reconciliationAuditCount)
	}
	verifyGenerationHTTPPersistence(t, ctx, database, generations, fourth.User.ID, fourth.Token, second.Token)
	quotaHTTPGenerations, err := generationapp.NewServiceWithCostEstimator(accounts, store.NewGenerationRepository(database), paidPolicy, generationapp.CostEstimatorFunc(func(generationapp.Purpose, string, string, []byte) (generationapp.CostEstimate, error) {
		return generationapp.CostEstimate{Currency: "USD", EstimatedMinorUnits: 60, ReservedQuotaUnits: 1}, nil
	}))
	if err != nil {
		t.Fatalf("construct synthetic HTTP quota estimator: %v", err)
	}
	verifyGenerationHTTPQuotaPersistence(t, ctx, database, quotaHTTPGenerations, fifth.User.ID, fifth.Token)
}
