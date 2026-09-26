//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/adapter/generationfixture"
	"github.com/StephenQiu30/then-server/backend/internal/adapter/httpapi"
	"github.com/StephenQiu30/then-server/backend/internal/adapter/objectstore"
	store "github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
	privacyapp "github.com/StephenQiu30/then-server/backend/internal/application/privacy"
	"github.com/StephenQiu30/then-server/backend/tests/internal/testcontainer"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type generationRecoveryProviderStub struct{ submitCalls int }

type generationFixtureResultFetcher struct {
	objects       *objectstore.Store
	data          []byte
	version       string
	writeThenFail bool
	fetchCalls    int
}

func (f *generationFixtureResultFetcher) Fetch(ctx context.Context, request generationapp.FetchRequest) (generationapp.FetchedResult, error) {
	f.fetchCalls++
	versionID, err := f.objects.PutDerived(ctx, request.ObjectKey, bytes.NewReader(f.data), int64(len(f.data)))
	if err != nil {
		return generationapp.FetchedResult{}, err
	}
	f.version = versionID
	if f.writeThenFail {
		return generationapp.FetchedResult{}, errors.New("fixture fetch interrupted after object write")
	}
	digest := sha256.Sum256(f.data)
	return generationapp.FetchedResult{
		ExternalTaskID: request.ExternalTaskID,
		TaskID:         request.TaskID,
		Purpose:        request.Purpose,
		LookID:         request.LookID,
		LookRevision:   request.LookRevision,
		Inputs:         request.Inputs,
		Fact: generationapp.OutputFact{
			ObjectKey: request.ObjectKey, ContentType: generationapp.OutputContentTypeJPEG,
			ByteSize: int64(len(f.data)), SHA256: hex.EncodeToString(digest[:]), ObjectVersionID: versionID,
		},
	}, nil
}

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
		Image:        testcontainer.MinIOImage(),
		Cmd:          []string{"server", "/data"},
		Env:          map[string]string{"MINIO_ROOT_USER": "then_test", "MINIO_ROOT_PASSWORD": password},
		ExposedPorts: []string{"9000/tcp"},
		WaitingFor:   wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").WithStartupTimeout(time.Minute),
	})
	endpoint := mappedAddress(t, ctx, container, "9000/tcp")
	objects := openGenerationObjectStore(t, ctx, endpoint, password)

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
			AccessRevokedAt: &at,
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
	seedGenerationInputMedia(t, ctx, database, ownerID, []generationapp.InputReference{
		{MediaID: "00000000-0000-4000-8000-000000000902", Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: strings.Repeat("a", 64)},
		{MediaID: "00000000-0000-4000-8000-000000000903", Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 1, SHA256: strings.Repeat("b", 64)},
	})

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

	changedPolicyWithSameKey := strings.ReplaceAll(body, "local-image-v1", "local-image-v2")
	changedPolicyWithSameKey = strings.ReplaceAll(changedPolicyWithSameKey, "00000000-0000-4000-8000-000000000904", "00000000-0000-4000-8000-000000000905")
	policyConflictResponse := httptest.NewRecorder()
	router.ServeHTTP(policyConflictResponse, request(http.MethodPost, "/generation-jobs", ownerToken, changedPolicyWithSameKey))
	if policyConflictResponse.Code != http.StatusConflict {
		t.Fatalf("same idempotency key with a changed consent policy status=%d body=%s", policyConflictResponse.Code, policyConflictResponse.Body.String())
	}

	changedPolicyWithNewKey := strings.ReplaceAll(changedPolicyWithSameKey, "generation-http-request", "generation-http-policy-v2")
	policyChangedResponse := httptest.NewRecorder()
	router.ServeHTTP(policyChangedResponse, request(http.MethodPost, "/generation-jobs", ownerToken, changedPolicyWithNewKey))
	if policyChangedResponse.Code != http.StatusAccepted {
		t.Fatalf("new consent policy request status=%d body=%s", policyChangedResponse.Code, policyChangedResponse.Body.String())
	}
	policyChanged := decodeJob(policyChangedResponse)
	if policyChanged.ID == created.ID || policyChanged.Reused || policyChanged.Match != generationapp.RequestMatchNone {
		t.Fatalf("changed consent policy reused the previous generation task: created=%+v changed=%+v", created, policyChanged)
	}
	var persistedPolicy struct{ Consent []byte }
	if err := database.WithContext(ctx).Table("generation_jobs").Select("consent").Where("id = ?", policyChanged.ID).Take(&persistedPolicy).Error; err != nil {
		t.Fatalf("read task accepted under changed consent policy: %v", err)
	}
	var acceptedPolicy generationapp.ConsentReceipt
	if err := json.Unmarshal(persistedPolicy.Consent, &acceptedPolicy); err != nil || acceptedPolicy.PolicyVersion != "local-image-v2" {
		t.Fatalf("new task did not persist the confirmed consent policy: receipt=%+v err=%v", acceptedPolicy, err)
	}

	cancelResponse := httptest.NewRecorder()
	router.ServeHTTP(cancelResponse, request(http.MethodPost, "/generation-jobs/"+created.ID+"/cancel", ownerToken, ""))
	if cancelResponse.Code != http.StatusAccepted {
		t.Fatalf("cancel generation over HTTP status=%d body=%s", cancelResponse.Code, cancelResponse.Body.String())
	}
	canceled := decodeJob(cancelResponse)
	if canceled.ID != created.ID || canceled.Status != generationapp.StatusCanceled || canceled.CancelRequestedAt == nil {
		t.Fatalf("unsubmitted HTTP cancellation was not settled immediately: %+v", canceled)
	}
	readAfterCancel := httptest.NewRecorder()
	router.ServeHTTP(readAfterCancel, request(http.MethodGet, "/generation-jobs/"+created.ID, ownerToken, ""))
	if readAfterCancel.Code != http.StatusOK || decodeJob(readAfterCancel).Status != generationapp.StatusCanceled || decodeJob(readAfterCancel).CancelRequestedAt == nil {
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

func generationOutputAccessRouter(t *testing.T, ctx context.Context, service *generationapp.Service, objects *objectstore.Store) http.Handler {
	t.Helper()
	router, err := httpapi.NewRouterWithGeneration(
		ctx,
		false,
		generationIntegrationProbe{},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		httpapi.NewGenerationHandler(service, false).WithOutputSigner(objects),
		time.Second,
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
	)
	if err != nil {
		t.Fatalf("construct generation output router: %v", err)
	}
	return router
}

func generationOutputAccessRequest(method, taskID, token string) *http.Request {
	request := httptest.NewRequest(method, "/generation-jobs/"+taskID+"/output-access", nil)
	request.AddCookie(&http.Cookie{Name: "then_session", Value: token})
	return request
}

func generationImageConfirmationRequest(taskID, token string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/generation-jobs/"+taskID+"/confirm-image", nil)
	request.AddCookie(&http.Cookie{Name: "then_session", Value: token})
	return request
}

func verifyGenerationHTTPQuotaPersistence(t *testing.T, ctx context.Context, database *gorm.DB, service *generationapp.Service, ownerID, ownerToken string) {
	t.Helper()
	seedGenerationInputMedia(t, ctx, database, ownerID, []generationapp.InputReference{
		{MediaID: "00000000-0000-4000-8000-000000000912", Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: strings.Repeat("a", 64)},
		{MediaID: "00000000-0000-4000-8000-000000000913", Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 1, SHA256: strings.Repeat("b", 64)},
	})

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

func seedGenerationInputMedia(t *testing.T, ctx context.Context, database *gorm.DB, ownerID string, references []generationapp.InputReference) {
	t.Helper()

	mediaRepository := store.NewMediaRepository(database)
	privacyRepository := store.NewPrivacyRepository(database)
	at := time.Now().UTC()
	for _, reference := range references {
		category := ""
		switch reference.Role {
		case generationapp.InputRolePerson:
			category = mediaapp.MediaCategoryPersonPhoto
			if _, err := privacyRepository.ConfirmSelfAdultDeclaration(ctx, ownerID, privacyapp.CurrentSelfAdultPolicyVersion, at); err != nil {
				t.Fatalf("confirm synthetic adult declaration: %v", err)
			}
		case generationapp.InputRoleGarment:
			category = mediaapp.MediaCategoryOrdinaryImage
		default:
			t.Fatalf("unsupported generation fixture role %q", reference.Role)
		}

		consent, err := mediaRepository.CreateConsent(ctx, ownerID, mediaapp.CreateConsentInput{
			Purpose:         mediaapp.MediaPurposeGenerationInput,
			Category:        category,
			PolicyVersion:   mediaapp.CurrentGenerationInputPolicyVersion,
			ActivelyAgreed:  true,
			TrainingAllowed: false,
		}, at)
		if err != nil {
			t.Fatalf("create synthetic generation-input consent: %v", err)
		}

		var existing int64
		if err := database.WithContext(ctx).Table("media_assets").Where("id = ?", reference.MediaID).Count(&existing).Error; err != nil {
			t.Fatalf("check synthetic generation media: %v", err)
		}
		if existing != 0 {
			continue
		}
		rawObjectKey := "owners/" + ownerID + "/media/" + reference.MediaID + "/source.jpg"
		if err := database.WithContext(ctx).Table("media_assets").Create(map[string]any{
			"id": reference.MediaID, "owner_id": ownerID, "consent_id": consent.ID,
			"purpose": mediaapp.MediaPurposeGenerationInput, "category": category,
			"content_type": mediaapp.MediaContentTypeJPEG, "byte_size": int64(128), "sha256": reference.SHA256,
			"raw_object_key": rawObjectKey, "object_version_id": "source-version-" + reference.MediaID,
			"status": string(mediaapp.MediaReady), "stable_reason": "", "pixel_width": 10, "pixel_height": 10,
			"upload_expires_at": at.Add(mediaapp.UploadIntentLifetime), "created_at": at, "updated_at": at,
		}).Error; err != nil {
			t.Fatalf("insert synthetic ready generation media: %v", err)
		}
		if err := database.WithContext(ctx).Table("media_derivations").Create(map[string]any{
			"id": uuid.NewString(), "media_id": reference.MediaID,
			"object_key":        "media/" + reference.MediaID + "/normalized.jpg",
			"object_version_id": "normalized-version-" + reference.MediaID, "created_at": at,
		}).Error; err != nil {
			t.Fatalf("insert synthetic normalized generation media: %v", err)
		}
	}
}

func removeSyntheticGenerationInputMedia(t *testing.T, ctx context.Context, database *gorm.DB, ownerID string, references []generationapp.InputReference) {
	t.Helper()
	for _, reference := range references {
		if err := database.WithContext(ctx).Table("media_derivations").Where("media_id = ?", reference.MediaID).Delete(map[string]any{}).Error; err != nil {
			t.Fatalf("remove synthetic generation derivation: %v", err)
		}
		if err := database.WithContext(ctx).Table("media_assets").Where("id = ? AND owner_id = ?", reference.MediaID, ownerID).Delete(map[string]any{}).Error; err != nil {
			t.Fatalf("remove synthetic generation input: %v", err)
		}
	}
}

func verifyGenerationInputAdmissionGuards(t *testing.T, ctx context.Context, database *gorm.DB, service *generationapp.Service, ownerToken, ownerID string, input generationapp.CreateServiceInput, foreignMediaID string) {
	t.Helper()
	personMediaID := input.Inputs.References[0].MediaID
	garmentMediaID := input.Inputs.References[1].MediaID
	reject := func(name string, references []generationapp.InputReference) {
		t.Helper()
		candidate := input
		candidate.IdempotencyKey = "generation-input-guard-" + name
		candidate.Parameters = []byte(`{"guard":"` + name + `"}`)
		candidate.Inputs.References = append([]generationapp.InputReference(nil), references...)
		if _, err := service.Create(ctx, ownerToken, candidate); !errors.Is(err, generationapp.ErrGenerationSourceUnavailable) {
			t.Fatalf("%s generation source was not rejected: %v", name, err)
		}
	}

	wrongHash := append([]generationapp.InputReference(nil), input.Inputs.References...)
	wrongHash[0].SHA256 = strings.Repeat("c", 64)
	reject("hash-mismatch", wrongHash)

	wrongOwner := append([]generationapp.InputReference(nil), input.Inputs.References...)
	wrongOwner[0].MediaID = foreignMediaID
	reject("cross-owner", wrongOwner)

	if err := database.WithContext(ctx).Table("media_assets").Where("id = ?", personMediaID).Update("status", string(mediaapp.MediaChecking)).Error; err != nil {
		t.Fatalf("mark synthetic person media unvalidated: %v", err)
	}
	reject("unvalidated", input.Inputs.References)
	if err := database.WithContext(ctx).Table("media_assets").Where("id = ?", personMediaID).Update("status", string(mediaapp.MediaReady)).Error; err != nil {
		t.Fatalf("restore synthetic person media status: %v", err)
	}

	if err := database.WithContext(ctx).Table("media_assets").Where("id = ?", garmentMediaID).Update("source_deleted_at", time.Now().UTC()).Error; err != nil {
		t.Fatalf("mark synthetic garment source deleted: %v", err)
	}
	reject("deleted-source", input.Inputs.References)
	if err := database.WithContext(ctx).Table("media_assets").Where("id = ?", garmentMediaID).Update("source_deleted_at", nil).Error; err != nil {
		t.Fatalf("restore synthetic garment source: %v", err)
	}

	if err := database.WithContext(ctx).Table("media_assets").Where("id = ?", personMediaID).Update("purpose", mediaapp.MediaPurposeAvatarSourcePreparation).Error; err != nil {
		t.Fatalf("change synthetic person media purpose: %v", err)
	}
	reject("wrong-purpose", input.Inputs.References)
	if err := database.WithContext(ctx).Table("media_assets").Where("id = ?", personMediaID).Update("purpose", mediaapp.MediaPurposeGenerationInput).Error; err != nil {
		t.Fatalf("restore synthetic person media purpose: %v", err)
	}

	var consentID string
	if err := database.WithContext(ctx).Table("media_assets").Select("consent_id").Where("id = ?", personMediaID).Scan(&consentID).Error; err != nil || consentID == "" {
		t.Fatalf("read synthetic person media consent: id=%q err=%v", consentID, err)
	}
	withdrawnAt := time.Now().UTC()
	if err := database.WithContext(ctx).Table("consent_records").Where("id = ?", consentID).Update("withdrawn_at", withdrawnAt).Error; err != nil {
		t.Fatalf("withdraw synthetic generation-input consent: %v", err)
	}
	reject("withdrawn-consent", input.Inputs.References)
	if err := database.WithContext(ctx).Table("consent_records").Where("id = ?", consentID).Update("withdrawn_at", nil).Error; err != nil {
		t.Fatalf("restore synthetic generation-input consent: %v", err)
	}

	if err := database.WithContext(ctx).Table("self_adult_declarations").Where("user_id = ? AND policy_version = ?", ownerID, privacyapp.CurrentSelfAdultPolicyVersion).Update("withdrawn_at", withdrawnAt).Error; err != nil {
		t.Fatalf("withdraw synthetic adult declaration: %v", err)
	}
	reject("withdrawn-adult-declaration", input.Inputs.References)
	if _, err := store.NewPrivacyRepository(database).ConfirmSelfAdultDeclaration(ctx, ownerID, privacyapp.CurrentSelfAdultPolicyVersion, time.Now().UTC()); err != nil {
		t.Fatalf("restore synthetic adult declaration: %v", err)
	}

	if err := database.WithContext(ctx).Table("media_derivations").Where("media_id = ?", garmentMediaID).Delete(map[string]any{}).Error; err != nil {
		t.Fatalf("remove synthetic normalized derivation: %v", err)
	}
	reject("missing-normalized-derivation", input.Inputs.References)
	if err := database.WithContext(ctx).Table("media_derivations").Create(map[string]any{
		"id": uuid.NewString(), "media_id": garmentMediaID,
		"object_key":        "media/" + garmentMediaID + "/normalized.jpg",
		"object_version_id": "normalized-version-" + garmentMediaID, "created_at": time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("restore synthetic normalized derivation: %v", err)
	}
}

func TestWithdrawGenerationInputConsentRevokesDependentJobs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	password := rand.Text()
	container := integrationContainer(t, ctx, testcontainers.ContainerRequest{
		Image:        "postgres:18.4@sha256:a02db8cac496f15b094798a38254f14d6e00741f709360e5e00bb6668ea31636",
		Env:          map[string]string{"POSTGRES_USER": "then_test", "POSTGRES_DB": "then_test", "POSTGRES_PASSWORD": password},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(time.Minute),
	})
	databaseURL := fmt.Sprintf(
		"postgres://then_test:%s@%s/then_test?sslmode=disable",
		url.QueryEscape(password), mappedAddress(t, ctx, container, "5432/tcp"),
	)
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open generation consent database: %v", err)
	}
	if err := store.Migrate(ctx, database); err != nil {
		t.Fatalf("migrate generation consent database: %v", err)
	}

	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	if err != nil {
		t.Fatalf("construct generation consent account service: %v", err)
	}
	owner, err := accounts.Register(ctx, accountapp.RegisterAccountInput{
		Email: "generation-consent@example.test", DisplayName: "Generation Consent", Password: "correct-password-generation-consent",
	})
	if err != nil {
		t.Fatalf("register generation consent owner: %v", err)
	}
	foreignOwner, err := accounts.Register(ctx, accountapp.RegisterAccountInput{
		Email: "generation-consent-foreign@example.test", DisplayName: "Generation Consent Foreign", Password: "correct-password-generation-consent-foreign",
	})
	if err != nil {
		t.Fatalf("register unrelated generation owner: %v", err)
	}

	person := generationapp.InputReference{
		MediaID: "00000000-0000-4000-8000-000000000941", Role: generationapp.InputRolePerson,
		Ordinal: 0, Revision: 1, SHA256: strings.Repeat("a", 64),
	}
	firstGarment := generationapp.InputReference{
		MediaID: "00000000-0000-4000-8000-000000000942", Role: generationapp.InputRoleGarment,
		Ordinal: 1, Revision: 1, SHA256: strings.Repeat("b", 64),
	}
	secondGarment := generationapp.InputReference{
		MediaID: "00000000-0000-4000-8000-000000000943", Role: generationapp.InputRoleGarment,
		Ordinal: 1, Revision: 1, SHA256: strings.Repeat("c", 64),
	}
	seedGenerationInputMedia(t, ctx, database, owner.User.ID, []generationapp.InputReference{person, firstGarment, secondGarment})

	repository := store.NewGenerationRepository(database)
	generations, err := generationapp.NewServiceWithCostEstimator(accounts, repository, generationapp.AdmissionPolicy{
		Enabled: true, Currency: "USD", MaxConcurrentTasks: 4, MaxQuotaUnits: 4, MaxBudgetMinorUnits: 1000,
	}, generationapp.CostEstimatorFunc(func(generationapp.Purpose, string, string, []byte) (generationapp.CostEstimate, error) {
		return generationapp.CostEstimate{Currency: "USD", EstimatedMinorUnits: 60, ReservedQuotaUnits: 1}, nil
	}))
	if err != nil {
		t.Fatalf("construct generation consent service: %v", err)
	}
	createTask := func(token, key, lookID, seed string, references []generationapp.InputReference) generationapp.TaskView {
		t.Helper()
		at := time.Now().UTC().Add(-time.Minute)
		created, err := generations.Create(ctx, token, generationapp.CreateServiceInput{
			IdempotencyKey: key,
			LookID:         lookID,
			LookRevision:   1,
			Purpose:        generationapp.PurposeImage,
			Provider:       "fixture",
			Model:          "fixture-image-v1",
			Parameters:     []byte(`{"seed":"` + seed + `"}`),
			Inputs: generationapp.InputSnapshot{
				LookID: lookID, LookRevision: 1,
				References: references,
			},
			Consent: generationapp.ConsentReceipt{ID: uuid.NewString(), Purpose: generationapp.PurposeImage, PolicyVersion: "local-image-v1", AcceptedAt: at},
		})
		if err != nil {
			t.Fatalf("accept generation job %q: %v", key, err)
		}
		return created.View
	}
	first := createTask(
		owner.Token, "consent-revoke-first", "00000000-0000-4000-8000-000000000951", "first",
		[]generationapp.InputReference{person, firstGarment},
	)
	second := createTask(
		owner.Token, "consent-revoke-second", "00000000-0000-4000-8000-000000000952", "second",
		[]generationapp.InputReference{person, secondGarment},
	)
	foreignPerson := generationapp.InputReference{
		MediaID: "00000000-0000-4000-8000-000000000944", Role: generationapp.InputRolePerson,
		Ordinal: 0, Revision: 1, SHA256: strings.Repeat("d", 64),
	}
	foreignGarment := generationapp.InputReference{
		MediaID: "00000000-0000-4000-8000-000000000945", Role: generationapp.InputRoleGarment,
		Ordinal: 1, Revision: 1, SHA256: strings.Repeat("e", 64),
	}
	seedGenerationInputMedia(t, ctx, database, foreignOwner.User.ID, []generationapp.InputReference{foreignPerson, foreignGarment})
	foreign := createTask(
		foreignOwner.Token, "consent-revoke-foreign", "00000000-0000-4000-8000-000000000953", "foreign",
		[]generationapp.InputReference{foreignPerson, foreignGarment},
	)
	for _, created := range []generationapp.TaskView{first, second, foreign} {
		if created.Reservation == nil || created.Reservation.State != generationapp.ReservationReserved || created.Reservation.EstimatedMinorUnits != 60 || created.Reservation.ReservedQuotaUnits != 1 {
			t.Fatalf("generation task did not reserve the server quote: %+v", created)
		}
	}

	var consentID string
	if err := database.WithContext(ctx).Table("media_assets").Select("consent_id").Where("id = ?", firstGarment.MediaID).Scan(&consentID).Error; err != nil || consentID == "" {
		t.Fatalf("read generation garment consent: id=%q err=%v", consentID, err)
	}
	media := store.NewMediaRepository(database)
	withdrawnAt := time.Now().UTC()
	consent, err := media.WithdrawConsent(ctx, owner.User.ID, consentID, withdrawnAt)
	if err != nil || consent.WithdrawnAt == nil || !consent.WithdrawnAt.Equal(withdrawnAt) {
		t.Fatalf("withdraw generation input consent: consent=%+v err=%v", consent, err)
	}

	firstView, err := generations.Get(ctx, owner.Token, first.Task.ID)
	if err != nil {
		t.Fatalf("read revoked generation task: %v", err)
	}
	assertRevoked := func(view generationapp.TaskView, sourceMediaID string) {
		t.Helper()
		if view.Task.Status != generationapp.StatusCanceled ||
			view.Task.AccessRevokedAt == nil ||
			view.Task.CancelRequestedAt == nil ||
			view.Reservation == nil || view.Reservation.State != generationapp.ReservationReleased ||
			view.Cleanup == nil || view.Cleanup.Scope != generationapp.CleanupScopeSource || view.Cleanup.SourceMediaID != sourceMediaID {
			t.Fatalf("consent withdrawal did not cancel, release quota, and persist cleanup for %q: %+v", sourceMediaID, view)
		}
	}
	assertRevoked(firstView, firstGarment.MediaID)
	secondView, err := generations.Get(ctx, owner.Token, second.Task.ID)
	if err != nil {
		t.Fatalf("read second generation task using the same category consent: %v", err)
	}
	assertRevoked(secondView, secondGarment.MediaID)
	foreignView, err := generations.Get(ctx, foreignOwner.Token, foreign.Task.ID)
	if err != nil {
		t.Fatalf("read unrelated account generation task: %v", err)
	}
	if foreignView.Task.Status != generationapp.StatusQueued ||
		foreignView.Task.AccessRevokedAt != nil || foreignView.Task.CancelRequestedAt != nil ||
		foreignView.Cleanup != nil || foreignView.Reservation == nil || foreignView.Reservation.State != generationapp.ReservationReserved {
		t.Fatalf("withdrawing one account's consent affected another account: %+v", foreignView)
	}

	if _, err := media.WithdrawConsent(ctx, owner.User.ID, consentID, withdrawnAt.Add(time.Minute)); err != nil {
		t.Fatalf("repeat generation input consent withdrawal: %v", err)
	}
	var cleanupCount int64
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("task_id IN ? AND scope = ?", []string{first.Task.ID, second.Task.ID}, string(generationapp.CleanupScopeSource)).Count(&cleanupCount).Error; err != nil || cleanupCount != 2 {
		t.Fatalf("repeated withdrawal created duplicate cleanup requests: count=%d err=%v", cleanupCount, err)
	}
}

func TestGenerationPersistenceLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
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

	objectStorePassword := rand.Text()
	objectStoreContainer := integrationContainer(t, ctx, testcontainers.ContainerRequest{
		Image:        testcontainer.MinIOImage(),
		Cmd:          []string{"server", "/data"},
		Env:          map[string]string{"MINIO_ROOT_USER": "then_test", "MINIO_ROOT_PASSWORD": objectStorePassword},
		ExposedPorts: []string{"9000/tcp"},
		WaitingFor:   wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").WithStartupTimeout(time.Minute),
	})
	objectStoreAddress := mappedAddress(t, ctx, objectStoreContainer, "9000/tcp")
	objects := openGenerationObjectStore(t, ctx, objectStoreAddress, objectStorePassword)

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
	seedGenerationInputMedia(t, ctx, database, first.User.ID, input.Inputs.References)

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
	paidInput.Inputs.References = []generationapp.InputReference{
		{MediaID: "00000000-0000-4000-8000-000000000205", Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{MediaID: "00000000-0000-4000-8000-000000000206", Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 3, SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	seedGenerationInputMedia(t, ctx, database, second.User.ID, paidInput.Inputs.References)
	verifyGenerationInputAdmissionGuards(t, ctx, database, generations, first.Token, first.User.ID, input, paidInput.Inputs.References[0].MediaID)
	paidCreated, err := paidGenerations.Create(ctx, second.Token, paidInput)
	if err != nil {
		t.Fatalf("accept quota-backed generation task: %v", err)
	}
	if _, err := paidGenerations.ConfirmImage(ctx, second.Token, paidCreated.View.Task.ID); !errors.Is(err, generationapp.ErrImageNotConfirmable) {
		t.Fatalf("queued image task was confirmable: %v", err)
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

	queuedCancelInput := paidInput
	queuedCancelInput.IdempotencyKey = "generation-cancel-before-submit"
	queuedCancelInput.Parameters = []byte(`{"seed":"cancel-before-submit"}`)
	queuedCancelInput.Inputs.References = []generationapp.InputReference{
		{MediaID: "00000000-0000-4000-8000-000000000209", Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{MediaID: "00000000-0000-4000-8000-000000000210", Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 3, SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	seedGenerationInputMedia(t, ctx, database, second.User.ID, queuedCancelInput.Inputs.References)
	queuedCancel, err := paidGenerations.Create(ctx, second.Token, queuedCancelInput)
	if err != nil || queuedCancel.View.Reservation == nil || queuedCancel.View.Reservation.State != generationapp.ReservationReserved {
		t.Fatalf("accept queued cancellation task: view=%+v err=%v", queuedCancel, err)
	}
	settledCancellation, err := paidGenerations.Cancel(ctx, second.Token, queuedCancel.View.Task.ID)
	if err != nil || settledCancellation.Task.Status != generationapp.StatusCanceled || settledCancellation.Task.CancelRequestedAt == nil || settledCancellation.Reservation == nil || settledCancellation.Reservation.State != generationapp.ReservationReleased {
		t.Fatalf("unsubmitted cancellation did not atomically release quota: view=%+v err=%v", settledCancellation, err)
	}
	cancelReplay, err := paidGenerations.Cancel(ctx, second.Token, queuedCancel.View.Task.ID)
	if err != nil || cancelReplay.Task.StatusRevision != settledCancellation.Task.StatusRevision || cancelReplay.Task.Status != generationapp.StatusCanceled || cancelReplay.Reservation == nil || cancelReplay.Reservation.State != generationapp.ReservationReleased {
		t.Fatalf("repeated cancellation changed the settled task or quota: view=%+v err=%v", cancelReplay, err)
	}
	quotaProbe := queuedCancelInput
	quotaProbe.IdempotencyKey = "generation-cancel-released-quota-probe"
	quotaProbe.Parameters = []byte(`{"seed":"cancel-released-quota-probe"}`)
	quotaProbe.Inputs.References = []generationapp.InputReference{
		{MediaID: "00000000-0000-4000-8000-000000000219", Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{MediaID: "00000000-0000-4000-8000-000000000220", Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 3, SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	seedGenerationInputMedia(t, ctx, database, second.User.ID, quotaProbe.Inputs.References)
	quotaProbeCreated, err := paidGenerations.Create(ctx, second.Token, quotaProbe)
	if err != nil || quotaProbeCreated.View.Reservation == nil || quotaProbeCreated.View.Reservation.State != generationapp.ReservationReserved {
		t.Fatalf("released quota was not available to the next explicit request: view=%+v err=%v", quotaProbeCreated, err)
	}
	if _, err := paidGenerations.Cancel(ctx, second.Token, quotaProbeCreated.View.Task.ID); err != nil {
		t.Fatalf("release quota probe task: %v", err)
	}

	workerRepository := store.NewGenerationRepository(database)
	queuedClaimAt := created.View.Task.UpdatedAt
	queuedClaim, queuedLease, found, err := workerRepository.ClaimNextSubmissionLease(ctx, "worker-queue", queuedClaimAt, time.Minute)
	if err != nil || !found || queuedClaim.Task.ID != created.View.Task.ID || queuedClaim.Task.Status != generationapp.StatusRunning || queuedLease.FencingToken != 1 || queuedLease.Attempt != 1 {
		t.Fatalf("submission queue claim was incorrect: found=%v task=%+v lease=%+v err=%v", found, queuedClaim.Task, queuedLease, err)
	}
	orphanOutputKey, err := generationapp.OutputObjectKey(queuedClaim.Task)
	if err != nil {
		t.Fatalf("derive synthetic orphan output key: %v", err)
	}
	orphanBytes := []byte("synthetic orphan output")
	orphanVersionID, err := objects.PutDerived(ctx, orphanOutputKey, bytes.NewReader(orphanBytes), int64(len(orphanBytes)))
	if err != nil {
		t.Fatalf("write synthetic orphan output version: %v", err)
	}
	if err := workerRepository.RecordUnpublishedOutput(ctx, queuedLease, generationapp.CleanupTarget{
		Kind: generationapp.CleanupTargetObject, ID: queuedClaim.Task.ID, ObjectKey: orphanOutputKey, ObjectVersionID: orphanVersionID,
	}, queuedClaimAt); err != nil {
		t.Fatalf("record synthetic orphan output: %v", err)
	}
	var orphanCleanupID string
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("task_id = ? AND scope = ?", queuedClaim.Task.ID, string(generationapp.CleanupScopeOrphanOutput)).Pluck("id", &orphanCleanupID).Error; err != nil || orphanCleanupID == "" {
		t.Fatalf("find synthetic orphan cleanup: id=%q err=%v", orphanCleanupID, err)
	}
	claimedOrphan, _, err := workerRepository.BeginTaskCleanup(ctx, orphanCleanupID, queuedClaimAt.Add(time.Second))
	if err != nil {
		t.Fatalf("claim synthetic orphan cleanup: %v", err)
	}
	orphanFailedAt := queuedClaimAt.Add(2 * time.Second)
	if _, err := workerRepository.FailTaskCleanup(ctx, claimedOrphan, orphanFailedAt, "target_delete_failed", orphanFailedAt.Add(24*time.Hour)); err != nil {
		t.Fatalf("delay synthetic orphan cleanup: %v", err)
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
	observationAt := workflowAt.Add(5 * time.Minute)
	if _, err := workerRepository.ApplyProviderState(ctx, observationLease, "provider-task-1", generationapp.StatusRunning, "", observationAt); err != nil {
		t.Fatalf("persist running provider observation: %v", err)
	}
	scheduledPoll, err := workerRepository.ReleaseLeaseForRetry(ctx, observationLease, observationAt, 10*time.Second)
	if err != nil || scheduledPoll.Task.NextAttemptAt == nil || !scheduledPoll.Task.NextAttemptAt.Equal(observationAt.Add(10*time.Second)) {
		t.Fatalf("running provider poll was not durably scheduled: task=%+v err=%v", scheduledPoll.Task, err)
	}
	if _, _, found, err := workerRepository.ClaimNextObservationLease(ctx, "worker-too-early", observationAt.Add(9*time.Second), 10*time.Minute); err != nil || found {
		t.Fatalf("observation queue claimed before scheduled poll: found=%v err=%v", found, err)
	}
	pollAt := observationAt.Add(10 * time.Second)
	observed, observationLease, found, err = workerRepository.ClaimNextObservationLease(ctx, "worker-observer", pollAt, 10*time.Minute)
	if err != nil || !found || observed.Task.NextAttemptAt != nil {
		t.Fatalf("observation queue did not claim at scheduled time: found=%v task=%+v err=%v", found, observed.Task, err)
	}
	observed, err = workerRepository.ApplyProviderState(ctx, observationLease, "provider-task-1", generationapp.StatusValidating, "", pollAt)
	if err != nil {
		t.Fatalf("apply generation provider state: %v", err)
	}
	if observed.Task.Status != generationapp.StatusValidating || observed.Task.ExternalTaskID != "provider-task-1" || observed.Task.SubmissionState != generationapp.SubmissionAccepted {
		t.Fatalf("generation provider state observation was not persisted: %+v", observed.Task)
	}
	if _, err := workerRepository.ReleaseLease(ctx, observationLease, pollAt); err != nil {
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
	outputAt := workflowAt.Add(7 * time.Minute)
	resultFetcher := &generationFixtureResultFetcher{objects: objects, data: integrationGenerationJPEG(t), writeThenFail: true}
	resultWorker, err := generationapp.NewResultWorker(workerRepository, resultFetcher, objects, generationapp.ResultWorkerPolicy{
		WorkerID: "worker-result", LeaseTTL: 10 * time.Minute, FetchTimeout: time.Minute, RetryDelay: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("create provider-neutral result worker: %v", err)
	}
	initialOutputKey, err := generationapp.OutputObjectKey(paidCreated.View.Task)
	if err != nil {
		t.Fatalf("derive output key before result recovery: %v", err)
	}
	neighborKey := initialOutputKey + "/neighbor"
	neighborData := []byte("unrelated prefix neighbor")
	neighborVersionID, err := objects.PutDerived(ctx, neighborKey, bytes.NewReader(neighborData), int64(len(neighborData)))
	if err != nil {
		t.Fatalf("write unrelated output-key prefix neighbor: %v", err)
	}
	interruptedResult, err := resultWorker.RunOnceAt(ctx, paidCreated.View.Task.ID, outputAt)
	if !errors.Is(err, generationapp.ErrGenerationOutputFetchUnknown) || interruptedResult.View.Task.Status != generationapp.StatusValidating || interruptedResult.View.Task.LeaseOwner != "" || interruptedResult.View.Reservation == nil || interruptedResult.View.Reservation.State != generationapp.ReservationReserved {
		t.Fatalf("write-before-return failure did not leave the result retryable: view=%+v err=%v", interruptedResult.View, err)
	}
	if interruptedResult.View.Task.NextAttemptAt == nil || !interruptedResult.View.Task.NextAttemptAt.Equal(outputAt.Add(5*time.Second)) {
		t.Fatalf("result fetch retry was not durably scheduled: task=%+v", interruptedResult.View.Task)
	}
	if _, _, found, err := workerRepository.ClaimNextResultLease(ctx, "worker-result-early", outputAt.Add(time.Second), 10*time.Minute); err != nil || found {
		t.Fatalf("result queue claimed before retry time: found=%v err=%v", found, err)
	}
	interruptedVersionID := resultFetcher.version
	interruptedObjectKey, err := generationapp.OutputObjectKey(interruptedResult.View.Task)
	if err != nil || interruptedObjectKey != initialOutputKey {
		t.Fatalf("derive interrupted result object key: %v", err)
	}
	interruptedObject, err := objects.OpenDerivedVersion(ctx, interruptedObjectKey, interruptedVersionID)
	if err != nil {
		t.Fatalf("fixture did not leave the unreported object version behind: version=%q err=%v", interruptedVersionID, err)
	}
	interruptedObject.Close()
	outputAt = outputAt.Add(5 * time.Second)
	inventoryResult, err := resultWorker.RunOnceAt(ctx, paidCreated.View.Task.ID, outputAt)
	if !errors.Is(err, generationapp.ErrGenerationOutputCleanupPending) || inventoryResult.View.Task.Status != generationapp.StatusValidating || inventoryResult.View.Task.LeaseOwner != "" || resultFetcher.fetchCalls != 1 {
		t.Fatalf("existing output was not queued before a second fetch: view=%+v fetches=%d err=%v", inventoryResult.View, resultFetcher.fetchCalls, err)
	}
	var recoveredCleanupID string
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("task_id = ? AND scope = ?", paidCreated.View.Task.ID, string(generationapp.CleanupScopeOrphanOutput)).Pluck("id", &recoveredCleanupID).Error; err != nil || recoveredCleanupID == "" {
		t.Fatalf("find recovered output cleanup request: id=%q err=%v", recoveredCleanupID, err)
	}
	recoveredCleanup, recoveredTargets, err := workerRepository.BeginTaskCleanup(ctx, recoveredCleanupID, outputAt.Add(time.Second))
	if err != nil || len(recoveredTargets) != 1 || recoveredTargets[0].ObjectVersionID != interruptedVersionID {
		t.Fatalf("claim recovered output version: request=%+v targets=%+v err=%v", recoveredCleanup, recoveredTargets, err)
	}
	recoveryCleanupExecutor, err := objectstore.NewGenerationCleanupExecutor(objects)
	if err != nil {
		t.Fatalf("construct cleanup executor for unreported output: %v", err)
	}
	if err := recoveryCleanupExecutor.DeleteObject(ctx, recoveredTargets[0]); err != nil {
		t.Fatalf("delete recovered unreported output version: %v", err)
	}
	if _, err := workerRepository.CompleteTaskCleanup(ctx, recoveredCleanup, outputAt.Add(2*time.Second)); err != nil {
		t.Fatalf("complete recovered output cleanup: %v", err)
	}
	if object, err := objects.OpenDerivedVersion(ctx, interruptedObjectKey, interruptedVersionID); err == nil {
		object.Close()
		t.Fatal("recovered cleanup left the unreported MinIO version in place")
	}
	neighbor, err := objects.OpenDerivedVersion(ctx, neighborKey, neighborVersionID)
	if err != nil {
		t.Fatalf("exact-key cleanup removed a prefix neighbor: %v", err)
	}
	neighbor.Close()
	resultFetcher.writeThenFail = false
	resultFetcher.data = []byte("invalid jpeg")
	outputAt = outputAt.Add(3 * time.Minute)
	failedResult, err := resultWorker.RunOnceAt(ctx, paidCreated.View.Task.ID, outputAt)
	if !errors.Is(err, generationapp.ErrInvalidGenerationOutput) || failedResult.View.Task.Status != generationapp.StatusValidating || failedResult.View.Task.AccessRevokedAt != nil || failedResult.View.Reservation == nil || failedResult.View.Reservation.State != generationapp.ReservationReserved {
		t.Fatalf("invalid generation output did not remain retryable with reserved quota: view=%+v err=%v", failedResult.View, err)
	}
	var orphanRecord struct {
		ID              string
		Status          string
		AccessRevokedAt *time.Time
		Targets         []byte
	}
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Select("id, status, access_revoked_at, targets").Where("task_id = ? AND scope = ?", paidCreated.View.Task.ID, string(generationapp.CleanupScopeOrphanOutput)).Scan(&orphanRecord).Error; err != nil {
		t.Fatalf("load durable orphan output cleanup: %v", err)
	}
	var orphanTargets []generationapp.CleanupTarget
	if err := json.Unmarshal(orphanRecord.Targets, &orphanTargets); err != nil {
		t.Fatalf("decode orphan cleanup targets: %v", err)
	}
	expectedOutputKey, _ := generationapp.OutputObjectKey(paidCreated.View.Task)
	queuedVersions := make(map[string]bool, len(orphanTargets))
	for _, target := range orphanTargets {
		if target.ObjectKey != expectedOutputKey {
			t.Fatalf("orphan cleanup recorded an unexpected object key: %+v", target)
		}
		queuedVersions[target.ObjectVersionID] = true
	}
	if orphanRecord.ID == "" || orphanRecord.Status != string(generationapp.CleanupPending) || orphanRecord.AccessRevokedAt != nil || len(orphanTargets) != 2 || !queuedVersions[interruptedVersionID] || !queuedVersions[resultFetcher.version] {
		t.Fatalf("orphan output cleanup did not retain its exact object version: record=%+v targets=%+v", orphanRecord, orphanTargets)
	}
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("id = ?", orphanRecord.ID).Update("access_revoked_at", outputAt).Error; err == nil {
		t.Fatal("PostgreSQL accepted an orphan cleanup record with access marked revoked")
	}
	claimedOrphan, claimedTargets, err := workerRepository.BeginTaskCleanup(ctx, orphanRecord.ID, outputAt.Add(time.Second))
	if err != nil || claimedOrphan.Scope != generationapp.CleanupScopeOrphanOutput || len(claimedTargets) != 2 {
		t.Fatalf("claim exact orphan output cleanup: request=%+v targets=%+v err=%v", claimedOrphan, claimedTargets, err)
	}
	orphanCleanupExecutor, err := objectstore.NewGenerationCleanupExecutor(objects)
	if err != nil {
		t.Fatalf("construct generation cleanup executor for orphan: %v", err)
	}
	for _, target := range claimedTargets {
		if err := orphanCleanupExecutor.DeleteObject(ctx, target); err != nil {
			t.Fatalf("delete exact orphan output object version: %v", err)
		}
	}
	completedOrphan, err := workerRepository.CompleteTaskCleanup(ctx, claimedOrphan, outputAt.Add(2*time.Second))
	if err != nil || completedOrphan.Status != generationapp.CleanupComplete {
		t.Fatalf("complete orphan output cleanup: request=%+v err=%v", completedOrphan, err)
	}
	for _, versionID := range []string{interruptedVersionID, resultFetcher.version} {
		if _, err := objects.OpenDerivedVersion(ctx, expectedOutputKey, versionID); err == nil {
			t.Fatalf("orphan cleanup left unpublished MinIO version %q in place", versionID)
		}
	}
	outputAt = outputAt.Add(3 * time.Minute)
	resultFetcher.data = integrationGenerationJPEG(t)
	result, err := resultWorker.RunOnceAt(ctx, paidCreated.View.Task.ID, outputAt)
	if err != nil {
		t.Fatalf("run provider-neutral result worker after orphan cleanup: %v", err)
	}
	published := result.View
	outputObjectKey := published.Asset.ObjectKey
	outputVersionID := resultFetcher.version
	if published.Task.Status != generationapp.StatusSucceeded || published.Task.ResultAssetID == "" || published.Task.LeaseOwner != "" || published.Task.LeaseUntil != nil || published.Reservation == nil || published.Reservation.State != generationapp.ReservationConsumed || published.Asset == nil {
		t.Fatalf("generation output settlement was not atomic: %+v", published)
	}
	var storedOutputObjectKey string
	if err := database.WithContext(ctx).Table("generation_outputs").Where("task_id = ?", published.Task.ID).Select("object_key").Scan(&storedOutputObjectKey).Error; err != nil || storedOutputObjectKey != outputObjectKey {
		t.Fatalf("generation output key was not persisted: key=%q err=%v", storedOutputObjectKey, err)
	}
	outputRouter := generationOutputAccessRouter(t, ctx, paidGenerations, objects)
	newerBytes := []byte("newer output version")
	newerVersionID, err := objects.PutDerived(ctx, outputObjectKey, bytes.NewReader(newerBytes), int64(len(newerBytes)))
	if err != nil || newerVersionID == published.Asset.ObjectVersionID {
		t.Fatalf("write competing MinIO version: version=%q err=%v", newerVersionID, err)
	}
	accessResponse := httptest.NewRecorder()
	outputRouter.ServeHTTP(accessResponse, generationOutputAccessRequest(http.MethodGet, published.Task.ID, second.Token))
	if accessResponse.Code != http.StatusOK || accessResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("owner output access status=%d cache-control=%q body=%s", accessResponse.Code, accessResponse.Header().Get("Cache-Control"), accessResponse.Body.String())
	}
	var access httpapi.GenerationOutputAccessResponse
	if err := json.Unmarshal(accessResponse.Body.Bytes(), &access); err != nil {
		t.Fatalf("decode output access response %q: %v", accessResponse.Body.String(), err)
	}
	signedURL, err := url.Parse(access.URL)
	if err != nil || signedURL.Query().Get("versionId") != published.Asset.ObjectVersionID || access.ByteSize != int64(len(resultFetcher.data)) || access.SHA256 != published.Asset.SHA256 || access.ContentType != generationapp.OutputContentTypeJPEG || !access.ExpiresAt.After(time.Now().UTC()) || access.ExpiresAt.After(time.Now().UTC().Add(5*time.Minute+time.Second)) {
		t.Fatalf("output access was not bound to the exact stored version and metadata: access=%+v err=%v", access, err)
	}
	download, err := http.Get(access.URL)
	if err != nil {
		t.Fatalf("download signed generation output: %v", err)
	}
	downloadData, readErr := io.ReadAll(download.Body)
	download.Body.Close()
	if readErr != nil || download.StatusCode != http.StatusOK || download.Header.Get("Content-Type") != generationapp.OutputContentTypeJPEG || !bytes.Equal(downloadData, resultFetcher.data) {
		t.Fatalf("signed URL did not return its fixed version: status=%d content-type=%q size=%d read-error=%v", download.StatusCode, download.Header.Get("Content-Type"), len(downloadData), readErr)
	}
	if err := objects.DeleteVersion(ctx, objectstore.DerivedBucket, outputObjectKey, newerVersionID); err != nil {
		t.Fatalf("remove competing MinIO version: %v", err)
	}
	otherAccount := httptest.NewRecorder()
	outputRouter.ServeHTTP(otherAccount, generationOutputAccessRequest(http.MethodGet, published.Task.ID, first.Token))
	if otherAccount.Code != http.StatusNotFound {
		t.Fatalf("cross-account output access status=%d body=%s", otherAccount.Code, otherAccount.Body.String())
	}
	for _, overBudget := range []struct {
		contentType string
		byteSize    int64
	}{
		{contentType: generationapp.OutputContentTypeJPEG, byteSize: generationapp.MaxGenerationImageOutputBytes + 1},
		{contentType: generationapp.OutputContentTypeGLB, byteSize: generationapp.MaxGenerationModelOutputBytes + 1},
	} {
		err := database.WithContext(ctx).Table("generation_outputs").Where("task_id = ?", published.Task.ID).Updates(map[string]any{
			"content_type": overBudget.contentType,
			"byte_size":    overBudget.byteSize,
		}).Error
		if err == nil {
			t.Fatalf("PostgreSQL accepted over-budget generation output: type=%s size=%d", overBudget.contentType, overBudget.byteSize)
		}
	}
	if err := database.WithContext(ctx).Table("generation_outputs").Where("task_id = ?", published.Task.ID).Update("object_key", "").Error; err != nil {
		t.Fatalf("simulate legacy output without object key: %v", err)
	}
	legacyOutputView, err := workerRepository.Get(ctx, second.User.ID, published.Task.ID)
	if err != nil || legacyOutputView.Asset == nil || legacyOutputView.Asset.ObjectKey != "" {
		t.Fatalf("legacy output could not be read: view=%+v err=%v", legacyOutputView, err)
	}
	if err := database.WithContext(ctx).Table("generation_outputs").Where("task_id = ?", published.Task.ID).Update("object_key", outputObjectKey).Error; err != nil {
		t.Fatalf("restore generation output object key: %v", err)
	}
	resultLease := generationapp.Lease{TaskID: published.Task.ID, Owner: "worker-result", FencingToken: published.Task.FencingToken, Attempt: published.Task.LeaseAttempt, ExpiresAt: outputAt.Add(time.Minute)}
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
	seedGenerationInputMedia(t, ctx, database, second.User.ID, cancelInput.Inputs.References)
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
	cancelOutputKey, err := generationapp.OutputObjectKey(cancelRequested.Task)
	if err != nil {
		t.Fatalf("derive canceled task output key: %v", err)
	}
	staleOutput := []byte("unreported output before cancellation")
	staleVersion, err := objects.PutDerived(ctx, cancelOutputKey, bytes.NewReader(staleOutput), int64(len(staleOutput)))
	if err != nil {
		t.Fatalf("write unreported canceled output: %v", err)
	}
	fetchesBeforeCancel := resultFetcher.fetchCalls
	canceledResult, err := resultWorker.RunOnceAt(ctx, cancelCreated.View.Task.ID, cancelRequested.Task.UpdatedAt.Add(time.Second))
	if err != nil || canceledResult.Outcome != generationapp.ResultOutcomeCanceled || canceledResult.View.Task.Status != generationapp.StatusCanceled || canceledResult.View.Task.ResultAssetID != "" || canceledResult.View.Task.LeaseOwner != "" || canceledResult.View.Reservation == nil || canceledResult.View.Reservation.State != generationapp.ReservationReleased || resultFetcher.fetchCalls != fetchesBeforeCancel {
		t.Fatalf("canceled result settlement skipped recovery or fetched output: result=%+v fetches=%d err=%v", canceledResult, resultFetcher.fetchCalls, err)
	}
	var cancelCleanup struct {
		ID      string
		Targets []byte
	}
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Select("id, targets").Where("task_id = ? AND scope = ?", cancelCreated.View.Task.ID, string(generationapp.CleanupScopeOrphanOutput)).Scan(&cancelCleanup).Error; err != nil {
		t.Fatalf("read canceled output cleanup: %v", err)
	}
	var cancelTargets []generationapp.CleanupTarget
	if err := json.Unmarshal(cancelCleanup.Targets, &cancelTargets); err != nil || cancelCleanup.ID == "" || len(cancelTargets) != 1 || cancelTargets[0].ObjectKey != cancelOutputKey || cancelTargets[0].ObjectVersionID != staleVersion {
		t.Fatalf("cancellation did not retain exact orphan output version: targets=%+v err=%v", cancelTargets, err)
	}
	cancelCleanupAt := canceledResult.View.Task.UpdatedAt.Add(time.Second)
	claimedCancelCleanup, targets, err := workerRepository.BeginTaskCleanup(ctx, cancelCleanup.ID, cancelCleanupAt)
	if err != nil || len(targets) != 1 {
		t.Fatalf("claim canceled output cleanup: targets=%+v err=%v", targets, err)
	}
	if err := orphanCleanupExecutor.DeleteObject(ctx, targets[0]); err != nil {
		t.Fatalf("delete canceled output version: %v", err)
	}
	if _, err := workerRepository.CompleteTaskCleanup(ctx, claimedCancelCleanup, cancelCleanupAt.Add(time.Second)); err != nil {
		t.Fatalf("complete canceled output cleanup: %v", err)
	}
	if output, err := objects.OpenDerivedVersion(ctx, cancelOutputKey, staleVersion); err == nil {
		output.Close()
		t.Fatal("canceled output version remained readable after cleanup")
	}

	modelInput := paidInput
	modelInput.IdempotencyKey = "generation-model-dependent"
	modelInput.Purpose = generationapp.PurposeModel
	modelInput.Model = "look-model-v1"
	modelInput.Parameters = []byte(`{"seed":"model"}`)
	modelInput.Inputs = generationapp.InputSnapshot{
		LookID:       paidInput.LookID,
		LookRevision: paidInput.LookRevision,
		References:   []generationapp.InputReference{{MediaID: published.Asset.ID, Role: generationapp.InputRoleLookImage, Ordinal: 0, Revision: 1, SHA256: published.Asset.SHA256}},
		ImageAssetID: published.Asset.ID,
		ImageSHA256:  published.Asset.SHA256,
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
	if _, err := paidGenerations.Create(ctx, second.Token, modelInput); !errors.Is(err, generationapp.ErrGenerationSourceUnavailable) {
		t.Fatalf("unconfirmed image was accepted as a model source: %v", err)
	}
	foreignConfirmation := httptest.NewRecorder()
	outputRouter.ServeHTTP(foreignConfirmation, generationImageConfirmationRequest(published.Task.ID, first.Token))
	if foreignConfirmation.Code != http.StatusNotFound {
		t.Fatalf("cross-owner image confirmation status=%d body=%s", foreignConfirmation.Code, foreignConfirmation.Body.String())
	}
	confirmed, err := workerRepository.ConfirmImage(ctx, second.User.ID, published.Task.ID, published.Asset.PublishedAt.Add(time.Second), 0)
	if err != nil || confirmed.ImageConfirmedAt == nil || confirmed.Asset == nil || confirmed.Asset.ID != published.Asset.ID {
		t.Fatalf("confirm published image at synthetic worker time: view=%+v err=%v", confirmed, err)
	}
	replayConfirmation := httptest.NewRecorder()
	outputRouter.ServeHTTP(replayConfirmation, generationImageConfirmationRequest(published.Task.ID, second.Token))
	var replayedConfirmation httpapi.GenerationJobResponse
	if err := json.Unmarshal(replayConfirmation.Body.Bytes(), &replayedConfirmation); err != nil || replayConfirmation.Code != http.StatusOK || replayedConfirmation.Output == nil || replayedConfirmation.Output.ConfirmedAt == nil || !replayedConfirmation.Output.ConfirmedAt.Equal(*confirmed.ImageConfirmedAt) {
		t.Fatalf("image confirmation replay changed the decision: status=%d job=%+v err=%v", replayConfirmation.Code, replayedConfirmation, err)
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
	seedGenerationInputMedia(t, ctx, database, second.User.ID, purposeImageInput.Inputs.References)
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
	unfinished := httptest.NewRecorder()
	outputRouter.ServeHTTP(unfinished, generationOutputAccessRequest(http.MethodGet, modelCreated.View.Task.ID, second.Token))
	if unfinished.Code != http.StatusNotFound {
		t.Fatalf("unfinished output access status=%d body=%s", unfinished.Code, unfinished.Body.String())
	}
	if modelCreated.View.Task.Status != generationapp.StatusQueued || modelCreated.View.Reservation == nil || modelCreated.View.Reservation.Purpose != generationapp.PurposeModel || modelCreated.View.Reservation.EstimatedMinorUnits != 60 || modelCreated.View.Reservation.ReservedQuotaUnits != 3 {
		t.Fatalf("model purpose reservation was not persisted independently: %+v", modelCreated)
	}
	if _, err := purposeGenerations.ConfirmImage(ctx, second.Token, modelCreated.View.Task.ID); !errors.Is(err, generationapp.ErrImageNotConfirmable) {
		t.Fatalf("model task was confirmable as an image: %v", err)
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
	revokedConfirmation := httptest.NewRecorder()
	outputRouter.ServeHTTP(revokedConfirmation, generationImageConfirmationRequest(published.Task.ID, second.Token))
	if revokedConfirmation.Code != http.StatusConflict {
		t.Fatalf("revoked image remained confirmable: status=%d body=%s", revokedConfirmation.Code, revokedConfirmation.Body.String())
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
	revokedAccess := httptest.NewRecorder()
	outputRouter.ServeHTTP(revokedAccess, generationOutputAccessRequest(http.MethodGet, published.Task.ID, second.Token))
	if revokedAccess.Code != http.StatusNotFound {
		t.Fatalf("revoked output access status=%d body=%s", revokedAccess.Code, revokedAccess.Body.String())
	}
	if len(deletedCleanup.Targets) != 2 || deletedCleanup.Targets[0].Kind != generationapp.CleanupTargetObject || deletedCleanup.Targets[0].ObjectKey != outputObjectKey || deletedCleanup.Targets[0].ObjectVersionID != outputVersionID || deletedCleanup.Targets[1].Kind != generationapp.CleanupTargetProvider {
		t.Fatalf("generation cleanup request lost its target snapshot: request=%+v", deletedCleanup)
	}
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("id <> ? AND status IN ?", deletedCleanup.ID, []string{string(generationapp.CleanupPending), string(generationapp.CleanupFailed)}).Update("next_attempt_at", time.Now().UTC().Add(time.Hour)).Error; err != nil {
		t.Fatalf("delay unrelated cleanup fixtures: %v", err)
	}
	cleanupExecutor, err := objectstore.NewGenerationCleanupExecutor(objects)
	if err != nil {
		t.Fatalf("construct generation cleanup executor: %v", err)
	}
	cleanupWorker, err := generationapp.NewCleanupWorker(workerRepository, cleanupExecutor, generationapp.CleanupRetryPolicy{LeaseTTL: time.Minute, BaseDelay: time.Second, MaxDelay: time.Minute})
	if err != nil {
		t.Fatalf("construct generation cleanup worker: %v", err)
	}
	firstCleanupAt := deletedCleanup.AccessRevokedAt.Add(time.Minute)
	found, err = cleanupWorker.RunOnceAt(ctx, firstCleanupAt)
	if err != nil || !found {
		t.Fatalf("run generation cleanup worker: found=%v err=%v", found, err)
	}
	_, failedCleanup, err := workerRepository.RequestTaskCleanup(ctx, second.User.ID, paidCreated.View.Task.ID, firstCleanupAt.Add(time.Minute))
	if err != nil || failedCleanup.ID != deletedCleanup.ID || failedCleanup.Status != generationapp.CleanupFailed || failedCleanup.StableError != "provider_cleanup_unavailable" || failedCleanup.Attempts != 1 || failedCleanup.NextAttemptAt == nil {
		t.Fatalf("provider cleanup was not retained as a retryable failure: request=%+v err=%v", failedCleanup, err)
	}
	if _, err := objects.OpenDerivedVersion(ctx, outputObjectKey, outputVersionID); err == nil {
		t.Fatal("cleanup worker left the manifest-targeted MinIO object version in place")
	}
	cleanupRetryAt := firstCleanupAt.Add(2 * time.Second)
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("id = ?", deletedCleanup.ID).Update("next_attempt_at", cleanupRetryAt.Add(-time.Second)).Error; err != nil {
		t.Fatalf("make failed provider cleanup retry eligible: %v", err)
	}
	found, err = cleanupWorker.RunOnceAt(ctx, cleanupRetryAt)
	if err != nil || !found {
		t.Fatalf("retry generation cleanup worker: found=%v err=%v", found, err)
	}
	_, retriedCleanup, err := workerRepository.RequestTaskCleanup(ctx, second.User.ID, paidCreated.View.Task.ID, cleanupRetryAt.Add(time.Minute))
	if err != nil || retriedCleanup.Status != generationapp.CleanupFailed || retriedCleanup.StableError != "provider_cleanup_unavailable" || retriedCleanup.Attempts != 2 || retriedCleanup.NextAttemptAt == nil {
		t.Fatalf("cleanup retry did not retain the unavailable provider target: request=%+v err=%v", retriedCleanup, err)
	}
	loadedDeleted, err := workerRepository.Get(ctx, second.User.ID, paidCreated.View.Task.ID)
	if err != nil {
		t.Fatalf("reload revoked generation task: %v", err)
	}
	if loadedDeleted.Task.AccessRevokedAt == nil || loadedDeleted.Asset != nil || loadedDeleted.Cleanup == nil || loadedDeleted.Cleanup.Status != generationapp.CleanupFailed {
		t.Fatalf("revoked generation output became visible again: %+v", loadedDeleted)
	}
	var outputCount int64
	if err := database.WithContext(ctx).Table("generation_outputs").Where("task_id = ?", paidCreated.View.Task.ID).Count(&outputCount).Error; err != nil {
		t.Fatalf("count cleaned generation output: %v", err)
	}
	if outputCount != 1 {
		t.Fatalf("output metadata was deleted before all manifest targets succeeded: %d", outputCount)
	}
	deletedViewAgain, deletedCleanupAgain, err := workerRepository.RequestTaskCleanup(ctx, second.User.ID, paidCreated.View.Task.ID, cleanupRetryAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("repeat generation cleanup request: %v", err)
	}
	if deletedViewAgain.Task.AccessRevokedAt == nil || deletedCleanupAgain.Status != generationapp.CleanupFailed || deletedCleanupAgain.ID != deletedCleanup.ID {
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
	sourceClaimAt := sourceRequests[0].UpdatedAt.Add(time.Minute).UTC().Truncate(time.Microsecond).Add(123 * time.Nanosecond)
	claimedSource, sourceTargets, found, err := workerRepository.ClaimNextCleanup(ctx, sourceClaimAt, time.Minute)
	if err != nil || !found || claimedSource.Status != generationapp.CleanupRunning || len(sourceTargets) != 0 {
		t.Fatalf("cleanup worker did not claim source cleanup: found=%v request=%+v targets=%+v err=%v", found, claimedSource, sourceTargets, err)
	}
	if !claimedSource.UpdatedAt.Equal(sourceClaimAt.UTC().Truncate(time.Microsecond)) {
		t.Fatalf("cleanup claim timestamp did not match PostgreSQL precision: claimed=%s input=%s", claimedSource.UpdatedAt, sourceClaimAt)
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
	removeSyntheticGenerationInputMedia(t, ctx, database, first.User.ID, input.Inputs.References)
	accountDeletion, err := accounts.DeleteCurrentUser(ctx, first.Token)
	if err != nil {
		t.Fatalf("begin account deletion with generation cleanup: %v", err)
	}
	if accountDeletion.Status != accountapp.AccountDeletionPending || accountDeletion.GenerationCount != 2 || accountDeletion.RemainingGenerationCount != 2 || accountDeletion.Phase != "generation_cleanup" {
		t.Fatalf("account deletion did not wait for generation cleanup: %+v", accountDeletion)
	}
	var accountCleanupIDs []string
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("account_deletion_id = ?", accountDeletion.ID).Pluck("id", &accountCleanupIDs).Error; err != nil || len(accountCleanupIDs) != 2 {
		t.Fatalf("find account generation cleanup request: %v", err)
	}
	accountCleanupAt := accountDeletion.RequestedAt.Add(time.Minute)
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("id = ?", orphanCleanupID).Update("next_attempt_at", accountCleanupAt.Add(24*time.Hour)).Error; err != nil {
		t.Fatalf("hold orphan cleanup while account cleanup converges: %v", err)
	}
	found, err = cleanupWorker.RunOnceAt(ctx, accountCleanupAt)
	if err != nil || !found {
		t.Fatalf("run account generation cleanup worker: found=%v err=%v", found, err)
	}
	pendingAccountDeletion, err := accounts.GetDeletionReceipt(ctx, accountDeletion.ID, accountDeletion.ReceiptToken)
	if err != nil || pendingAccountDeletion.Status != accountapp.AccountDeletionPending || pendingAccountDeletion.RemainingGenerationCount != 1 {
		t.Fatalf("account deletion did not wait for its orphan output cleanup: receipt=%+v err=%v", pendingAccountDeletion, err)
	}
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("id = ?", orphanCleanupID).Update("next_attempt_at", accountCleanupAt.Add(time.Second)).Error; err != nil {
		t.Fatalf("make linked orphan cleanup eligible: %v", err)
	}
	found, err = cleanupWorker.RunOnceAt(ctx, accountCleanupAt.Add(2*time.Minute))
	if err != nil || !found {
		t.Fatalf("run linked orphan output cleanup worker: found=%v err=%v", found, err)
	}
	completedAccountDeletion, err := accounts.GetDeletionReceipt(ctx, accountDeletion.ID, accountDeletion.ReceiptToken)
	if err != nil {
		t.Fatalf("read completed account deletion receipt: %v", err)
	}
	if completedAccountDeletion.Status != accountapp.AccountDeletionComplete || completedAccountDeletion.RemainingGenerationCount != 0 || completedAccountDeletion.Phase != "complete" {
		t.Fatalf("account deletion did not complete after generation cleanup: %+v", completedAccountDeletion)
	}
	var remainingAccountTasks int64
	if err := database.WithContext(ctx).Table("generation_jobs").Where("owner_id = ?", first.User.ID).Count(&remainingAccountTasks).Error; err != nil {
		t.Fatalf("count generation tasks after account deletion: %v", err)
	}
	if remainingAccountTasks != 0 {
		t.Fatalf("account generation tasks remained after deletion receipt completion: %d", remainingAccountTasks)
	}
	var remainingAccountCleanup int64
	if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("account_deletion_id = ?", accountDeletion.ID).Count(&remainingAccountCleanup).Error; err != nil {
		t.Fatalf("count generation cleanup rows after account deletion: %v", err)
	}
	if remainingAccountCleanup != 0 {
		t.Fatalf("account cleanup evidence remained after completion: %d", remainingAccountCleanup)
	}

	lateInput := input
	lateInput.IdempotencyKey = "generation-late-acceptance-cleanup"
	lateInput.Parameters = []byte(`{"seed":"late-cleanup"}`)
	lateInput.Inputs.References = []generationapp.InputReference{
		{MediaID: "00000000-0000-4000-8000-000000000221", Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{MediaID: "00000000-0000-4000-8000-000000000222", Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 3, SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	seedGenerationInputMedia(t, ctx, database, third.User.ID, lateInput.Inputs.References)
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
	recoveryInput.Inputs.References = []generationapp.InputReference{
		{MediaID: "00000000-0000-4000-8000-000000000223", Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{MediaID: "00000000-0000-4000-8000-000000000224", Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 3, SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	seedGenerationInputMedia(t, ctx, database, third.User.ID, recoveryInput.Inputs.References)
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
		request.Inputs.References = []generationapp.InputReference{
			{MediaID: "00000000-0000-4000-8000-000000000231", Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			{MediaID: "00000000-0000-4000-8000-000000000232", Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 3, SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		}
		seedGenerationInputMedia(t, ctx, database, third.User.ID, request.Inputs.References)
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

func openGenerationObjectStore(t *testing.T, ctx context.Context, endpoint, password string) *objectstore.Store {
	t.Helper()
	for attempt := 0; ; attempt++ {
		objects, err := objectstore.Open(ctx, endpoint, "then_test", password, false)
		if err == nil {
			return objects
		}
		if attempt == 19 {
			t.Fatalf("open isolated generation object store after retries: %v", err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("open isolated generation object store: %v", ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func integrationGenerationJPEG(t *testing.T) []byte {
	t.Helper()
	var source bytes.Buffer
	if err := jpeg.Encode(&source, image.NewRGBA(image.Rect(0, 0, 8, 6)), &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	data, err := generationapp.NormalizeGenerationImage(source.Bytes())
	if err != nil {
		t.Fatalf("normalize synthetic generation JPEG: %v", err)
	}
	return data
}

func TestGenerationWorkersCompleteProviderNeutralPostgresMinIOWorkflow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	password := rand.Text()
	postgresContainer := integrationContainer(t, ctx, testcontainers.ContainerRequest{
		Image:        "postgres:18.4@sha256:a02db8cac496f15b094798a38254f14d6e00741f709360e5e00bb6668ea31636",
		Env:          map[string]string{"POSTGRES_USER": "then_test", "POSTGRES_DB": "then_test", "POSTGRES_PASSWORD": password},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(time.Minute),
	})
	databaseAddress := mappedAddress(t, ctx, postgresContainer, "5432/tcp")
	databaseURL := "postgres://then_test:" + url.QueryEscape(password) + "@" + databaseAddress + "/then_test?sslmode=disable"
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open worker workflow database: %v", err)
	}
	if err := store.Migrate(ctx, database); err != nil {
		t.Fatalf("migrate worker workflow database: %v", err)
	}

	objectStorePassword := rand.Text()
	objectStoreContainer := integrationContainer(t, ctx, testcontainers.ContainerRequest{
		Image:        testcontainer.MinIOImage(),
		Cmd:          []string{"server", "/data"},
		Env:          map[string]string{"MINIO_ROOT_USER": "then_test", "MINIO_ROOT_PASSWORD": objectStorePassword},
		ExposedPorts: []string{"9000/tcp"},
		WaitingFor:   wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").WithStartupTimeout(time.Minute),
	})
	objectStoreAddress := mappedAddress(t, ctx, objectStoreContainer, "9000/tcp")
	objects := openGenerationObjectStore(t, ctx, objectStoreAddress, objectStorePassword)

	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	if err != nil {
		t.Fatalf("construct worker workflow account service: %v", err)
	}
	owner, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "generation-worker@example.test", DisplayName: "Generation Worker", Password: "generation-worker-password"})
	if err != nil {
		t.Fatalf("register worker workflow owner: %v", err)
	}
	repository := store.NewGenerationRepository(database)
	generations, err := generationapp.NewService(accounts, repository, generationapp.AdmissionPolicy{Enabled: true, ZeroCost: true, MaxConcurrentTasks: 2, MaxQuotaUnits: 2})
	if err != nil {
		t.Fatalf("construct worker workflow generation service: %v", err)
	}
	lookID := uuid.NewString()
	inputs := generationapp.InputSnapshot{
		LookID:       lookID,
		LookRevision: 3,
		References: []generationapp.InputReference{
			{MediaID: uuid.NewString(), Role: generationapp.InputRolePerson, Ordinal: 0, Revision: 1, SHA256: strings.Repeat("a", 64)},
			{MediaID: uuid.NewString(), Role: generationapp.InputRoleGarment, Ordinal: 1, Revision: 2, SHA256: strings.Repeat("b", 64)},
		},
	}
	seedGenerationInputMedia(t, ctx, database, owner.User.ID, inputs.References)
	foreignProviderTask, err := generations.Create(ctx, owner.Token, generationapp.CreateServiceInput{
		IdempotencyKey: uuid.NewString(),
		LookID:         lookID,
		LookRevision:   inputs.LookRevision,
		Purpose:        generationapp.PurposeImage,
		Provider:       "seedream",
		Model:          "remote-image-v1",
		Parameters:     []byte(`{"seed":"must-remain-untouched"}`),
		Inputs:         inputs,
		Consent: generationapp.ConsentReceipt{
			ID: uuid.NewString(), Purpose: generationapp.PurposeImage,
			PolicyVersion: "local-image-v1", AcceptedAt: time.Now().UTC().Add(-time.Second),
		},
	})
	if err != nil || foreignProviderTask.View.Task.Status != generationapp.StatusQueued {
		t.Fatalf("create foreign provider isolation task: view=%+v err=%v", foreignProviderTask.View, err)
	}
	created, err := generations.Create(ctx, owner.Token, generationapp.CreateServiceInput{
		IdempotencyKey: uuid.NewString(),
		LookID:         lookID,
		LookRevision:   inputs.LookRevision,
		Purpose:        generationapp.PurposeImage,
		Provider:       generationfixture.ProviderName,
		Model:          generationfixture.ImageModel,
		Parameters:     []byte(`{"seed":"worker-workflow"}`),
		Inputs:         inputs,
		Consent: generationapp.ConsentReceipt{
			ID: uuid.NewString(), Purpose: generationapp.PurposeImage,
			PolicyVersion: "local-image-v1", AcceptedAt: time.Now().UTC().Add(-time.Second),
		},
	})
	if err != nil || created.View.Task.Status != generationapp.StatusQueued || created.View.Reservation != nil {
		t.Fatalf("create zero-cost worker workflow task: view=%+v err=%v", created.View, err)
	}

	fixture, err := generationfixture.New(objects)
	if err != nil {
		t.Fatalf("construct local fixture generation adapter: %v", err)
	}
	submissionWorker, err := generationapp.NewSubmissionWorker(repository, fixture, generationapp.SubmissionWorkerPolicy{
		WorkerID: "workflow-submitter", Provider: generationfixture.ProviderName, LeaseTTL: time.Minute, ProviderTimeout: time.Second,
		Retry: generationapp.RetryPolicy{MaxAttempts: 2, BaseDelay: time.Second, MaxDelay: time.Minute},
	})
	if err != nil {
		t.Fatalf("construct submission worker: %v", err)
	}
	found, submitted, err := submissionWorker.RunNext(ctx)
	if err != nil || !found || submitted.Outcome != generationapp.SubmissionOutcomeAccepted || submitted.View.Task.SubmissionState != generationapp.SubmissionAccepted || !strings.HasPrefix(submitted.View.Task.ExternalTaskID, "fixture:") || submitted.View.Task.LeaseOwner != "" {
		t.Fatalf("submitter did not persist accepted provider identity and release lease: found=%v result=%+v err=%v", found, submitted, err)
	}

	observationWorker, err := generationapp.NewObservationWorker(repository, fixture, generationapp.ObservationWorkerPolicy{
		WorkerID: "workflow-observer", Provider: generationfixture.ProviderName, LeaseTTL: time.Minute, ProviderTimeout: time.Second, PollInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("construct observation worker: %v", err)
	}
	found, observed, err := observationWorker.RunNext(ctx)
	if err != nil || !found || observed.Outcome != generationapp.ObservationOutcomeValidating || observed.View.Task.Status != generationapp.StatusValidating || observed.View.Task.LeaseOwner != "" {
		t.Fatalf("observer did not move provider success to validating: found=%v result=%+v err=%v", found, observed, err)
	}

	resultWorker, err := generationapp.NewResultWorker(repository, fixture, objects, generationapp.ResultWorkerPolicy{
		WorkerID: "workflow-result", Provider: generationfixture.ProviderName, LeaseTTL: time.Minute, FetchTimeout: time.Second, RetryDelay: time.Second,
	})
	if err != nil {
		t.Fatalf("construct result worker: %v", err)
	}
	found, published, err := resultWorker.RunNext(ctx)
	if err != nil || !found || published.Outcome != generationapp.ResultOutcomePublished || published.View.Task.Status != generationapp.StatusSucceeded || published.View.Asset == nil || published.View.Task.ResultAssetID != published.View.Asset.ID || published.View.Task.LeaseOwner != "" {
		t.Fatalf("result worker did not validate and publish fixture output: found=%v result=%+v err=%v", found, published, err)
	}
	if published.View.ImageConfirmedAt != nil {
		t.Fatal("worker implicitly confirmed the image")
	}
	outputBytes, err := objects.ReadOutputVersion(ctx, published.View.Asset.ObjectKey, published.View.Asset.ObjectVersionID, generationapp.MaxGenerationImageOutputBytes)
	if err != nil || len(outputBytes) != int(published.View.Asset.ByteSize) {
		t.Fatalf("published private output version was not readable: bytes=%d asset=%+v err=%v", len(outputBytes), published.View.Asset, err)
	}
	untouchedForeignTask, err := repository.Get(ctx, owner.User.ID, foreignProviderTask.View.Task.ID)
	if err != nil || untouchedForeignTask.Task.Status != generationapp.StatusQueued || untouchedForeignTask.Task.SubmissionState != generationapp.SubmissionNotStarted || untouchedForeignTask.Task.ExternalTaskID != "" || untouchedForeignTask.Task.LeaseOwner != "" {
		t.Fatalf("local fixture workers changed a foreign provider task: view=%+v err=%v", untouchedForeignTask, err)
	}

	modelRequest := generationapp.CreateServiceInput{
		IdempotencyKey: uuid.NewString(),
		LookID:         lookID,
		LookRevision:   inputs.LookRevision,
		Purpose:        generationapp.PurposeModel,
		Provider:       generationfixture.ProviderName,
		Model:          generationfixture.ModelModel,
		Parameters:     []byte(`{"seed":"model-workflow"}`),
		Inputs: generationapp.InputSnapshot{
			LookID: lookID, LookRevision: inputs.LookRevision,
			ImageAssetID: published.View.Asset.ID, ImageSHA256: published.View.Asset.SHA256,
			References: []generationapp.InputReference{{
				MediaID: published.View.Asset.ID, Role: generationapp.InputRoleLookImage,
				Ordinal: 0, Revision: 1, SHA256: published.View.Asset.SHA256,
			}},
		},
		Consent: generationapp.ConsentReceipt{
			ID: uuid.NewString(), Purpose: generationapp.PurposeModel,
			PolicyVersion: "local-model-v1", AcceptedAt: time.Now().UTC().Add(-time.Second),
		},
	}
	if _, err := generations.Create(ctx, owner.Token, modelRequest); !errors.Is(err, generationapp.ErrGenerationSourceUnavailable) {
		t.Fatalf("fixture model accepted unconfirmed image: %v", err)
	}
	fixtureRouter := generationOutputAccessRouter(t, ctx, generations, objects)
	confirmedFixtureResponse := httptest.NewRecorder()
	fixtureRouter.ServeHTTP(confirmedFixtureResponse, generationImageConfirmationRequest(published.View.Task.ID, owner.Token))
	var confirmedFixture httpapi.GenerationJobResponse
	if err := json.Unmarshal(confirmedFixtureResponse.Body.Bytes(), &confirmedFixture); err != nil || confirmedFixtureResponse.Code != http.StatusOK || confirmedFixture.Output == nil || confirmedFixture.Output.ConfirmedAt == nil || confirmedFixture.Output.ID != published.View.Asset.ID {
		t.Fatalf("HTTP confirmation did not persist fixture image decision: status=%d job=%+v err=%v", confirmedFixtureResponse.Code, confirmedFixture, err)
	}
	modelCreated, err := generations.Create(ctx, owner.Token, modelRequest)
	if err != nil || modelCreated.View.Task.Status != generationapp.StatusQueued || modelCreated.View.Reservation != nil {
		t.Fatalf("create zero-cost model from published image: view=%+v err=%v", modelCreated.View, err)
	}
	if found, submitted, err := submissionWorker.RunNext(ctx); err != nil || !found || submitted.View.Task.ID != modelCreated.View.Task.ID || submitted.Outcome != generationapp.SubmissionOutcomeAccepted {
		t.Fatalf("fixture model submission failed: found=%v result=%+v err=%v", found, submitted, err)
	}
	if found, observed, err := observationWorker.RunNext(ctx); err != nil || !found || observed.View.Task.ID != modelCreated.View.Task.ID || observed.Outcome != generationapp.ObservationOutcomeValidating {
		t.Fatalf("fixture model observation failed: found=%v result=%+v err=%v", found, observed, err)
	}
	found, modelPublished, err := resultWorker.RunNext(ctx)
	if err != nil || !found || modelPublished.Outcome != generationapp.ResultOutcomePublished || modelPublished.View.Task.Status != generationapp.StatusSucceeded || modelPublished.View.Asset == nil {
		t.Fatalf("fixture model result failed: found=%v result=%+v err=%v", found, modelPublished, err)
	}
	modelAsset := modelPublished.View.Asset
	if modelAsset.ContentType != generationapp.OutputContentTypeGLB || modelAsset.Lineage.SourceImageAssetID != published.View.Asset.ID || modelAsset.Lineage.SourceImageSHA256 != published.View.Asset.SHA256 || modelAsset.Lineage.LookRevision != inputs.LookRevision {
		t.Fatalf("fixture model source lineage or GLB type changed: asset=%+v", modelAsset)
	}
	modelBytes, err := objects.ReadOutputVersion(ctx, modelAsset.ObjectKey, modelAsset.ObjectVersionID, generationapp.MaxGenerationModelOutputBytes)
	if err != nil {
		t.Fatalf("read published model version: %v", err)
	}
	verifiedModel, err := generationapp.VerifyOutputContent(generationapp.PurposeModel, modelBytes)
	if err != nil || verifiedModel.SHA256 != modelAsset.SHA256 || verifiedModel.ByteSize != modelAsset.ByteSize {
		t.Fatalf("published model bytes failed independent verification: fact=%+v asset=%+v err=%v", verifiedModel, modelAsset, err)
	}
	if err := database.WithContext(ctx).Table("generation_outputs").Where("id = ?", modelAsset.ID).Update("confirmed_at", time.Now().UTC()).Error; err == nil {
		t.Fatal("database accepted confirmation of a model output")
	}
	if err := database.WithContext(ctx).Table("generation_outputs").Where("id = ?", published.View.Asset.ID).Update("confirmed_at", published.View.Asset.PublishedAt.Add(-time.Second)).Error; err == nil {
		t.Fatal("database accepted image confirmation before publication")
	}
	untouchedForeignTask, err = repository.Get(ctx, owner.User.ID, foreignProviderTask.View.Task.ID)
	if err != nil || untouchedForeignTask.Task.Status != generationapp.StatusQueued || untouchedForeignTask.Task.ExternalTaskID != "" {
		t.Fatalf("model workers changed a foreign provider task: view=%+v err=%v", untouchedForeignTask, err)
	}

	deletedImage, err := generations.Delete(ctx, owner.Token, published.View.Task.ID)
	if err != nil || deletedImage.Cleanup.Status != generationapp.CleanupPending || deletedImage.View.Asset != nil {
		t.Fatalf("image deletion did not revoke its output: result=%+v err=%v", deletedImage, err)
	}
	baseCleanup, err := objectstore.NewGenerationCleanupExecutor(objects)
	if err != nil {
		t.Fatalf("construct private object cleanup: %v", err)
	}
	fixtureCleanup, err := generationfixture.NewCleanupExecutor(baseCleanup)
	if err != nil {
		t.Fatalf("construct stateless fixture cleanup: %v", err)
	}
	cleanupWorker, err := generationapp.NewCleanupWorker(repository, fixtureCleanup, generationapp.CleanupRetryPolicy{LeaseTTL: time.Minute, BaseDelay: time.Second, MaxDelay: time.Minute})
	if err != nil {
		t.Fatalf("construct fixture cleanup worker: %v", err)
	}
	setCleanupIdentity := func(taskID, provider, externalID string) {
		t.Helper()
		var row struct{ Targets []byte }
		query := database.WithContext(ctx).Table("generation_cleanup_requests").Where("owner_id = ? AND task_id = ? AND scope = ?", owner.User.ID, taskID, string(generationapp.CleanupScopeTask))
		if err := query.Select("targets").Take(&row).Error; err != nil {
			t.Fatalf("read fixture cleanup manifest: %v", err)
		}
		var targets []generationapp.CleanupTarget
		if err := json.Unmarshal(row.Targets, &targets); err != nil {
			t.Fatalf("decode fixture cleanup manifest: %v", err)
		}
		foundProvider := false
		for index := range targets {
			if targets[index].Kind == generationapp.CleanupTargetProvider {
				targets[index].Provider = provider
				targets[index].ID = externalID
				foundProvider = true
			}
		}
		if !foundProvider {
			t.Fatal("fixture cleanup manifest has no provider target")
		}
		encoded, err := json.Marshal(targets)
		if err != nil {
			t.Fatalf("encode fixture cleanup manifest: %v", err)
		}
		if err := database.WithContext(ctx).Table("generation_cleanup_requests").Where("owner_id = ? AND task_id = ? AND scope = ?", owner.User.ID, taskID, string(generationapp.CleanupScopeTask)).Updates(map[string]any{
			"targets": encoded, "status": string(generationapp.CleanupPending), "stable_error": "", "next_attempt_at": nil,
		}).Error; err != nil {
			t.Fatalf("write fixture cleanup manifest: %v", err)
		}
	}
	setCleanupIdentity(published.View.Task.ID, "seedream", published.View.Task.ExternalTaskID)
	blocked, completed := 0, 0
	for range 2 {
		found, err := cleanupWorker.RunOnce(ctx)
		if err != nil {
			t.Fatalf("cleanup queue stopped on an identity mismatch: %v", err)
		}
		if found {
			completed++
		} else {
			blocked++
		}
	}
	if blocked != 1 || completed != 1 {
		t.Fatalf("bad cleanup blocked good cleanup: blocked=%d completed=%d", blocked, completed)
	}
	blockedView, err := repository.Get(ctx, owner.User.ID, published.View.Task.ID)
	if err != nil || blockedView.Cleanup == nil || blockedView.Cleanup.Status != generationapp.CleanupFailed || blockedView.Cleanup.StableError != generationapp.CleanupIdentityMismatchCode || blockedView.Cleanup.NextAttemptAt != nil {
		t.Fatalf("unsafe cleanup was not retained for repair: view=%+v err=%v", blockedView, err)
	}
	if _, err := objects.ReadOutputVersion(ctx, published.View.Asset.ObjectKey, published.View.Asset.ObjectVersionID, published.View.Asset.ByteSize); err != nil {
		t.Fatalf("unsafe cleanup deleted the image version: %v", err)
	}
	if _, err := objects.ReadOutputVersion(ctx, modelAsset.ObjectKey, modelAsset.ObjectVersionID, modelAsset.ByteSize); err == nil {
		t.Fatal("valid model cleanup did not delete its object version")
	}
	setCleanupIdentity(published.View.Task.ID, generationfixture.ProviderName, "fixture:"+uuid.NewString())
	if found, err := cleanupWorker.RunOnce(ctx); found || err != nil {
		t.Fatalf("mismatched external task cleanup was not refused: found=%v err=%v", found, err)
	}
	if _, err := objects.ReadOutputVersion(ctx, published.View.Asset.ObjectKey, published.View.Asset.ObjectVersionID, published.View.Asset.ByteSize); err != nil {
		t.Fatalf("mismatched external task cleanup deleted the image version: %v", err)
	}
	setCleanupIdentity(published.View.Task.ID, "", published.View.Task.ExternalTaskID) // Pre-qualification legacy manifest.
	if found, err := cleanupWorker.RunOnce(ctx); err != nil || !found {
		t.Fatalf("repaired fixture image cleanup did not converge: found=%v err=%v", found, err)
	}
	if found, err := cleanupWorker.RunOnce(ctx); err != nil || found {
		t.Fatalf("fixture cleanup retained unexpected work: found=%v err=%v", found, err)
	}
	for _, taskID := range []string{published.View.Task.ID, modelPublished.View.Task.ID} {
		view, err := repository.Get(ctx, owner.User.ID, taskID)
		if err != nil || view.Asset != nil || view.Cleanup == nil || view.Cleanup.Status != generationapp.CleanupComplete {
			t.Fatalf("fixture task did not reach complete cleanup: task=%s view=%+v err=%v", taskID, view, err)
		}
		for _, target := range view.Cleanup.Targets {
			if target.Kind == generationapp.CleanupTargetProvider && target.Provider != generationfixture.ProviderName {
				t.Fatalf("cleanup did not persist the task provider: task=%s target=%+v", taskID, target)
			}
		}
	}
	for _, asset := range []*generationapp.OutputAsset{published.View.Asset, modelAsset} {
		if _, err := objects.ReadOutputVersion(ctx, asset.ObjectKey, asset.ObjectVersionID, asset.ByteSize); err == nil {
			t.Fatalf("fixture cleanup left output version readable: asset=%s", asset.ID)
		}
	}
	untouchedForeignTask, err = repository.Get(ctx, owner.User.ID, foreignProviderTask.View.Task.ID)
	if err != nil || untouchedForeignTask.Task.Status != generationapp.StatusQueued || untouchedForeignTask.Cleanup != nil {
		t.Fatalf("fixture cleanup touched foreign provider task: view=%+v err=%v", untouchedForeignTask, err)
	}

	removeSyntheticGenerationInputMedia(t, ctx, database, owner.User.ID, inputs.References)
	accountDeletion, err := accounts.DeleteCurrentUser(ctx, owner.Token)
	if err != nil || accountDeletion.Status != accountapp.AccountDeletionPending || accountDeletion.GenerationCount != 3 || accountDeletion.RemainingGenerationCount != 3 {
		t.Fatalf("account deletion did not retain fixture cleanup work: receipt=%+v err=%v", accountDeletion, err)
	}
	for range 3 {
		if found, err := cleanupWorker.RunOnce(ctx); err != nil || !found {
			t.Fatalf("account fixture cleanup did not converge: found=%v err=%v", found, err)
		}
	}
	accountReceipt, err := accounts.GetDeletionReceipt(ctx, accountDeletion.ID, accountDeletion.ReceiptToken)
	if err != nil || accountReceipt.Status != accountapp.AccountDeletionComplete || accountReceipt.RemainingGenerationCount != 0 || accountReceipt.Phase != "complete" {
		t.Fatalf("account deletion did not complete after fixture cleanup: receipt=%+v err=%v", accountReceipt, err)
	}
	var remainingTasks int64
	if err := database.WithContext(ctx).Table("generation_jobs").Where("owner_id = ?", owner.User.ID).Count(&remainingTasks).Error; err != nil || remainingTasks != 0 {
		t.Fatalf("account generation jobs remained after completed receipt: count=%d err=%v", remainingTasks, err)
	}
}
