//go:build services

package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/adapter/messagequeue"
	"github.com/StephenQiu30/then-server/backend/internal/adapter/objectstore"
	store "github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	eventworkerapp "github.com/StephenQiu30/then-server/backend/internal/application/eventworker"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
	privacyapp "github.com/StephenQiu30/then-server/backend/internal/application/privacy"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSyntheticPersonPhotoLifecycle(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	base, err := gorm.Open(postgres.Open(environment.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open media test database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create media test schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := base.WithContext(cleanup).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("media schema cleanup failed")
		}
	})
	databaseURL, err := url.Parse(environment.databaseURL)
	serviceOK(t, "parse media database URL", err)
	query := databaseURL.Query()
	query.Set("search_path", schema)
	databaseURL.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open isolated media schema", err)
	serviceOK(t, "migrate media schema", store.Migrate(ctx, database))

	objects, err := objectstore.Open(ctx, environment.minioEndpoint, environment.minioAccessKey, environment.minioSecretKey, false)
	serviceOK(t, "open private media store", err)
	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	serviceOK(t, "construct media account service", err)
	owner, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "media-owner@example.test", DisplayName: "Media Owner", Password: "correct-password-owner"})
	serviceOK(t, "register media owner", err)
	other, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "media-other@example.test", DisplayName: "Other Owner", Password: "correct-password-other"})
	serviceOK(t, "register second media owner", err)
	privacy, err := privacyapp.NewPrivacyService(accounts, store.NewPrivacyRepository(database))
	serviceOK(t, "construct media privacy service", err)
	_, err = privacy.ConfirmSelfAdultDeclaration(ctx, owner.Token, privacyapp.ConfirmSelfAdultDeclarationInput{PolicyVersion: privacyapp.CurrentSelfAdultPolicyVersion, ConfirmsSelfAndAdult: true})
	serviceOK(t, "confirm synthetic adult declaration", err)
	mediaRepository := store.NewMediaRepository(database)
	mediaService, err := mediaapp.NewMediaService(accounts, mediaRepository, objects)
	serviceOK(t, "construct media service", err)
	consent, err := mediaService.CreateConsent(ctx, owner.Token, mediaapp.CreateConsentInput{Purpose: mediaapp.MediaPurposeAvatarSourcePreparation, Category: mediaapp.MediaCategoryPersonPhoto, PolicyVersion: mediaapp.CurrentMediaPolicyVersion, ActivelyAgreed: true})
	serviceOK(t, "create fixed synthetic consent", err)
	assertConcurrentConsentIdempotency(t, ctx, database, mediaService, owner.Token, owner.User.ID, consent.ID)
	assertAccountDeleteAndMediaCreateSerialize(t, ctx, database, accounts, mediaService)
	assertUnfinishedUploadRetentionBoundary(t, ctx, database, mediaService, other)

	photo := syntheticJPEG(t)
	digest := fmt.Sprintf("%x", sha256.Sum256(photo))
	upload, err := mediaService.CreateMediaUpload(ctx, owner.Token, mediaapp.CreateMediaUploadInput{ConsentID: consent.ID, Purpose: mediaapp.MediaPurposeAvatarSourcePreparation, ContentType: mediaapp.MediaContentTypeJPEG, ByteSize: int64(len(photo)), SHA256: digest})
	serviceOK(t, "create signed media upload", err)
	if upload.Method != http.MethodPut || upload.ExpiresAt.Sub(time.Now()) > mediaapp.UploadIntentLifetime+time.Second {
		t.Fatal("upload intent did not use the fixed short PUT contract")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, upload.URL, bytes.NewReader(photo))
	serviceOK(t, "build signed media PUT", err)
	request.ContentLength = int64(len(photo))
	request.Header.Set("Content-Type", mediaapp.MediaContentTypeJPEG)
	request.Header.Set("X-Amz-Meta-Sha256", strings.Repeat("b", 64))
	invalidResponse, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	serviceOK(t, "perform tampered signed media PUT", err)
	invalidResponse.Body.Close()
	if invalidResponse.StatusCode != http.StatusForbidden {
		t.Fatal("signed PUT accepted tampered digest metadata")
	}
	request, err = http.NewRequestWithContext(ctx, http.MethodPut, upload.URL, bytes.NewReader(photo))
	serviceOK(t, "rebuild signed media PUT", err)
	request.ContentLength = int64(len(photo))
	for key, value := range upload.Headers {
		if key != "Content-Length" {
			request.Header.Set(key, value)
		}
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	serviceOK(t, "perform signed media PUT", err)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("X-Amz-Version-Id") == "" {
		t.Fatal("signed PUT did not return a fixed object version")
	}
	versionID := response.Header.Get("X-Amz-Version-Id")
	uploaded, err := mediaService.CompleteMediaUpload(ctx, owner.Token, upload.Media.ID, mediaapp.CompleteMediaUploadInput{VersionID: versionID})
	serviceOK(t, "complete fixed media upload", err)
	if uploaded.Status != mediaapp.MediaUploaded || uploaded.ObjectVersionID != versionID {
		t.Fatal("finalize did not pin the uploaded object version")
	}
	repeated, err := mediaService.CompleteMediaUpload(ctx, owner.Token, upload.Media.ID, mediaapp.CompleteMediaUploadInput{VersionID: versionID})
	serviceOK(t, "repeat identical media finalize", err)
	if repeated.ObjectVersionID != versionID {
		t.Fatal("repeated finalize changed the fixed version")
	}
	overwrite := bytes.Clone(photo)
	overwrite[len(overwrite)/2] ^= 0xff
	overwriteRequest, err := http.NewRequestWithContext(ctx, http.MethodPut, upload.URL, bytes.NewReader(overwrite))
	serviceOK(t, "build post-finalize overwrite", err)
	overwriteRequest.ContentLength = int64(len(overwrite))
	for key, value := range upload.Headers {
		if key != "Content-Length" {
			overwriteRequest.Header.Set(key, value)
		}
	}
	overwriteResponse, err := (&http.Client{Timeout: 10 * time.Second}).Do(overwriteRequest)
	serviceOK(t, "perform post-finalize overwrite", err)
	overwriteResponse.Body.Close()
	if overwriteResponse.StatusCode != http.StatusOK || overwriteResponse.Header.Get("X-Amz-Version-Id") == versionID {
		t.Fatal("versioning did not preserve a distinct overwrite version")
	}
	assertWithdrawnConsentRejectsQueuedMedia(t, ctx, database, privacy, mediaService, other, photo, digest)
	anonymous, err := http.Get("http://" + environment.minioEndpoint + "/" + objectstore.RawBucket + "/" + uploaded.RawObjectKey)
	serviceOK(t, "attempt anonymous raw object read", err)
	anonymous.Body.Close()
	if anonymous.StatusCode != http.StatusForbidden {
		t.Fatal("raw media bucket allowed anonymous read")
	}
	if _, err := mediaService.GetMedia(ctx, other.Token, uploaded.ID); !errors.Is(err, mediaapp.ErrMediaNotFound) {
		t.Fatal("cross-owner media lookup did not use the not-found boundary")
	}
	broker, err := messagequeue.Open(ctx, environment.kafkaBrokers, kafkaTestPrefix(t, ctx, environment.kafkaBrokers))
	serviceOK(t, "open media broker", err)
	defer broker.Close()
	runner, err := eventworkerapp.New(store.NewMediaRepository(database), broker, objects)
	serviceOK(t, "construct media worker", err)
	workerContext, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan error, 1)
	go func() {
		err := runner.Run(workerContext)
		workerDone <- err
		if err != nil {
			cancel()
		}
	}()
	defer func() {
		stopWorker()
		select {
		case err := <-workerDone:
			if err != nil {
				t.Errorf("media worker failed: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("media worker did not stop within shutdown budget")
		}
	}()

	waitForMediaStatus(t, ctx, mediaService, owner.Token, uploaded.ID, mediaapp.MediaReady)
	var derivationCount int64
	serviceOK(t, "count normalized derivations", database.WithContext(ctx).Table("media_derivations").Where("media_id = ?", uploaded.ID).Count(&derivationCount).Error)
	if derivationCount != 1 {
		t.Fatal("media check did not produce exactly one derivation")
	}
	serviceOK(t, "requeue completed media event", database.WithContext(ctx).Table("outbox_events").Where("aggregate_id = ? AND event_type = ?", uploaded.ID, "media.uploaded").Update("published_at", nil).Error)
	time.Sleep(600 * time.Millisecond)
	serviceOK(t, "recount normalized derivations", database.WithContext(ctx).Table("media_derivations").Where("media_id = ?", uploaded.ID).Count(&derivationCount).Error)
	if derivationCount != 1 {
		t.Fatal("duplicate delivery created another derivation")
	}
	deletion, err := mediaService.DeleteMedia(ctx, owner.Token, uploaded.ID)
	serviceOK(t, "request media deletion", err)
	if deletion.Status != mediaapp.DeletionPending || deletion.ReadRevokedAt.IsZero() {
		t.Fatal("delete did not immediately tombstone media")
	}
	repeatedDeletion, err := mediaService.DeleteMedia(ctx, owner.Token, uploaded.ID)
	serviceOK(t, "repeat media deletion", err)
	if repeatedDeletion.ID != deletion.ID {
		t.Fatal("repeated delete created a second request")
	}
	waitForDeletion(t, ctx, mediaService, owner.Token, deletion.ID)
	deleted, err := mediaService.GetMedia(ctx, owner.Token, uploaded.ID)
	serviceOK(t, "read deleted media status", err)
	if deleted.Status != mediaapp.MediaDeleted || deleted.SHA256 != strings.Repeat("0", 64) || deleted.ObjectVersionID != "" {
		t.Fatal("completed deletion retained an active object reference")
	}
	accountDeletion, err := accounts.DeleteCurrentUser(ctx, owner.Token)
	if err != nil || accountDeletion.Status != accountapp.AccountDeletionComplete || accountDeletion.MediaCount != 0 {
		t.Fatal("account deletion remained blocked after media deletion")
	}
	assertAccountDeletionCleansActiveMedia(t, ctx, database, accounts, mediaService, objects, photo, digest)
	assertMissingNotificationDeliveryIsAcknowledged(t, ctx, database, mediaRepository)
	stopWorker()

}

func assertMissingNotificationDeliveryIsAcknowledged(t *testing.T, ctx context.Context, database *gorm.DB, repository *store.MediaRepository) {
	t.Helper()
	eventID, notificationID := uuid.NewString(), uuid.NewString()
	serviceOK(t, "ack missing notification target", repository.DeliverNotification(ctx, eventID, notificationID, time.Now().UTC()))
	serviceOK(t, "replay missing notification target", repository.DeliverNotification(ctx, eventID, notificationID, time.Now().UTC()))
	var receipts int64
	serviceOK(t, "count missing notification receipt", database.WithContext(ctx).Table("inbox_receipts").Where("event_id = ? AND handler_name = ?", eventID, "community-notification").Count(&receipts).Error)
	if receipts != 1 {
		t.Fatalf("missing notification delivery receipts=%d, want 1", receipts)
	}
}

func assertAccountDeletionCleansActiveMedia(t *testing.T, ctx context.Context, database *gorm.DB, accounts *accountapp.AccountService, mediaService *mediaapp.MediaService, objects *objectstore.Store, photo []byte, digest string) {
	t.Helper()
	owner, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "account-delete-media@example.test", DisplayName: "Delete Media", Password: "correct-password-delete-media"})
	serviceOK(t, "register account deletion owner", err)
	upload, err := mediaService.CreateMediaUpload(ctx, owner.Token, mediaapp.CreateMediaUploadInput{Purpose: mediaapp.MediaPurposeDiaryImage, ContentType: mediaapp.MediaContentTypeJPEG, ByteSize: int64(len(photo)), SHA256: digest})
	serviceOK(t, "create account deletion media", err)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, upload.URL, bytes.NewReader(photo))
	serviceOK(t, "build account deletion media upload", err)
	request.ContentLength = int64(len(photo))
	for key, value := range upload.Headers {
		if key != "Content-Length" {
			request.Header.Set(key, value)
		}
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	serviceOK(t, "upload account deletion media", err)
	response.Body.Close()
	versionID := response.Header.Get("X-Amz-Version-Id")
	if response.StatusCode != http.StatusOK || versionID == "" {
		t.Fatal("account deletion fixture upload failed")
	}
	asset, err := mediaService.CompleteMediaUpload(ctx, owner.Token, upload.Media.ID, mediaapp.CompleteMediaUploadInput{VersionID: versionID})
	serviceOK(t, "complete account deletion media", err)
	deletion, err := accounts.DeleteCurrentUser(ctx, owner.Token)
	serviceOK(t, "request account deletion with active media", err)
	if deletion.Status != accountapp.AccountDeletionPending || deletion.MediaCount != 1 || deletion.CompletedAt != nil {
		t.Fatal("account deletion did not retain a pending cleanup receipt")
	}
	if _, err := accounts.CurrentUser(ctx, owner.Token); !errors.Is(err, accountapp.ErrAuthentication) {
		t.Fatal("account deletion did not revoke the active session immediately")
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		var receipt accountapp.AccountDeletionRequest
		var row struct {
			ID          string     `gorm:"column:id"`
			Status      string     `gorm:"column:status"`
			MediaCount  int        `gorm:"column:media_count"`
			RequestedAt time.Time  `gorm:"column:requested_at"`
			CompletedAt *time.Time `gorm:"column:completed_at"`
		}
		serviceOK(t, "read account deletion receipt", database.WithContext(ctx).Table("account_deletion_requests").Where("id = ?", deletion.ID).Scan(&row).Error)
		receipt = accountapp.AccountDeletionRequest{ID: row.ID, Status: accountapp.AccountDeletionStatus(row.Status), MediaCount: row.MediaCount, RequestedAt: row.RequestedAt, CompletedAt: row.CompletedAt}
		if receipt.Status == accountapp.AccountDeletionComplete && receipt.CompletedAt != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("account deletion did not complete after media cleanup")
		}
		time.Sleep(100 * time.Millisecond)
	}
	for table, query := range map[string]string{
		"user":  "SELECT count(*) FROM users WHERE id = ?",
		"media": "SELECT count(*) FROM media_assets WHERE id = ?",
	} {
		var count int64
		identifier := owner.User.ID
		if table == "media" {
			identifier = asset.ID
		}
		serviceOK(t, "verify deleted "+table, database.WithContext(ctx).Raw(query, identifier).Scan(&count).Error)
		if count != 0 {
			t.Fatalf("%s remained after account deletion", table)
		}
	}
	reader, err := objects.OpenVersion(ctx, asset.RawObjectKey, asset.ObjectVersionID)
	if err == nil {
		reader.Close()
		t.Fatal("raw object remained after account deletion")
	}
}

func assertUnfinishedUploadRetentionBoundary(t *testing.T, ctx context.Context, database *gorm.DB, mediaService *mediaapp.MediaService, user accountapp.AuthenticatedUser) {
	t.Helper()
	consent, err := mediaService.CreateConsent(ctx, user.Token, mediaapp.CreateConsentInput{Purpose: mediaapp.MediaPurposeAvatarSourcePreparation, Category: mediaapp.MediaCategoryPersonPhoto, PolicyVersion: mediaapp.CurrentMediaPolicyVersion, ActivelyAgreed: true})
	serviceOK(t, "create unfinished-upload consent", err)
	media, err := store.NewMediaRepository(database).CreateMedia(ctx, user.User.ID, mediaapp.CreateMediaUploadInput{ConsentID: consent.ID, Purpose: mediaapp.MediaPurposeAvatarSourcePreparation, ContentType: mediaapp.MediaContentTypeJPEG, ByteSize: 128, SHA256: strings.Repeat("a", 64)}, time.Now().UTC())
	serviceOK(t, "create unfinished-upload record", err)
	now := time.Now().UTC()
	serviceOK(t, "age unfinished upload below cleanup boundary", database.WithContext(ctx).Table("media_assets").Where("id = ?", media.ID).Updates(map[string]any{"created_at": now.Add(-23 * time.Hour), "upload_expires_at": now.Add(-time.Hour)}).Error)
	candidates, err := store.NewMediaRepository(database).SourceCleanupCandidates(ctx, now, 100)
	serviceOK(t, "query unfinished uploads below cleanup boundary", err)
	if containsMedia(candidates, media.ID) {
		t.Fatal("unfinished upload was selected before the fixed 24-hour cleanup boundary")
	}
	serviceOK(t, "age unfinished upload beyond cleanup boundary", database.WithContext(ctx).Table("media_assets").Where("id = ?", media.ID).Update("created_at", now.Add(-25*time.Hour)).Error)
	candidates, err = store.NewMediaRepository(database).SourceCleanupCandidates(ctx, now, 100)
	serviceOK(t, "query unfinished uploads beyond cleanup boundary", err)
	if !containsMedia(candidates, media.ID) {
		t.Fatal("unfinished upload was not selected after the fixed 24-hour cleanup boundary")
	}
}

func containsMedia(media []mediaapp.MediaAsset, id string) bool {
	for _, item := range media {
		if item.ID == id {
			return true
		}
	}
	return false
}

func assertAccountDeleteAndMediaCreateSerialize(t *testing.T, ctx context.Context, database *gorm.DB, accounts *accountapp.AccountService, mediaService *mediaapp.MediaService) {
	t.Helper()
	for attempt := range 6 {
		user, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: fmt.Sprintf("media-race-%d@example.test", attempt), DisplayName: "Media Race", Password: "correct-password-race-user"})
		serviceOK(t, "register media/account race user", err)
		consent, err := mediaService.CreateConsent(ctx, user.Token, mediaapp.CreateConsentInput{Purpose: mediaapp.MediaPurposeAvatarSourcePreparation, Category: mediaapp.MediaCategoryPersonPhoto, PolicyVersion: mediaapp.CurrentMediaPolicyVersion, ActivelyAgreed: true})
		serviceOK(t, "create media/account race consent", err)
		start := make(chan struct{})
		createResult := make(chan error, 1)
		deleteResult := make(chan error, 1)
		go func() {
			<-start
			_, err := store.NewMediaRepository(database).CreateMedia(ctx, user.User.ID, mediaapp.CreateMediaUploadInput{ConsentID: consent.ID, Purpose: mediaapp.MediaPurposeAvatarSourcePreparation, ContentType: mediaapp.MediaContentTypeJPEG, ByteSize: 128, SHA256: strings.Repeat("a", 64)}, time.Now().UTC())
			createResult <- err
		}()
		go func() {
			<-start
			at := time.Now().UTC()
			_, err := store.NewAccountRepository(database).BeginAccountDeletion(ctx, user.User.ID, bytes.Repeat([]byte{0x42}, 32), at, at.Add(7*24*time.Hour))
			deleteResult <- err
		}()
		close(start)
		createErr, deleteErr := <-createResult, <-deleteResult
		switch {
		case createErr == nil && deleteErr == nil:
		case (errors.Is(createErr, mediaapp.ErrConsentRequired) || errors.Is(createErr, mediaapp.ErrMediaConflict)) && deleteErr == nil:
		default:
			t.Fatalf("media creation/account deletion were not serialized: create=%v delete=%v", createErr, deleteErr)
		}
	}
}

func assertWithdrawnConsentRejectsQueuedMedia(t *testing.T, ctx context.Context, database *gorm.DB, privacy *privacyapp.PrivacyService, mediaService *mediaapp.MediaService, user accountapp.AuthenticatedUser, photo []byte, digest string) {
	t.Helper()
	_, err := privacy.ConfirmSelfAdultDeclaration(ctx, user.Token, privacyapp.ConfirmSelfAdultDeclarationInput{PolicyVersion: privacyapp.CurrentSelfAdultPolicyVersion, ConfirmsSelfAndAdult: true})
	serviceOK(t, "confirm second synthetic adult declaration", err)
	consent, err := mediaService.CreateConsent(ctx, user.Token, mediaapp.CreateConsentInput{Purpose: mediaapp.MediaPurposeAvatarSourcePreparation, Category: mediaapp.MediaCategoryPersonPhoto, PolicyVersion: mediaapp.CurrentMediaPolicyVersion, ActivelyAgreed: true})
	serviceOK(t, "create consent for withdrawal check", err)
	upload, err := mediaService.CreateMediaUpload(ctx, user.Token, mediaapp.CreateMediaUploadInput{ConsentID: consent.ID, Purpose: mediaapp.MediaPurposeAvatarSourcePreparation, ContentType: mediaapp.MediaContentTypeJPEG, ByteSize: int64(len(photo)), SHA256: digest})
	serviceOK(t, "create queued upload for withdrawal check", err)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, upload.URL, bytes.NewReader(photo))
	serviceOK(t, "build queued signed PUT", err)
	request.ContentLength = int64(len(photo))
	for key, value := range upload.Headers {
		if key != "Content-Length" {
			request.Header.Set(key, value)
		}
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	serviceOK(t, "perform queued signed PUT", err)
	response.Body.Close()
	versionID := response.Header.Get("X-Amz-Version-Id")
	if response.StatusCode != http.StatusOK || versionID == "" {
		t.Fatal("queued signed PUT did not return a fixed version")
	}
	_, err = mediaService.CompleteMediaUpload(ctx, user.Token, upload.Media.ID, mediaapp.CompleteMediaUploadInput{VersionID: versionID})
	serviceOK(t, "complete queued upload before withdrawal", err)
	withdrawn, err := mediaService.WithdrawConsent(ctx, user.Token, consent.ID)
	serviceOK(t, "withdraw consent before queued processing", err)
	repeated, err := mediaService.WithdrawConsent(ctx, user.Token, consent.ID)
	serviceOK(t, "repeat consent withdrawal", err)
	if withdrawn.Status != mediaapp.ConsentWithdrawn || repeated.Status != mediaapp.ConsentWithdrawn || withdrawn.WithdrawnAt == nil || repeated.WithdrawnAt == nil || !withdrawn.WithdrawnAt.Equal(*repeated.WithdrawnAt) {
		t.Fatal("consent withdrawal was not idempotent")
	}
	media, process, err := store.NewMediaRepository(database).BeginMediaCheck(ctx, upload.Media.ID, time.Now().UTC())
	serviceOK(t, "begin queued media check after withdrawal", err)
	if process || media.Status != mediaapp.MediaRejected || media.StableReason != "consent_withdrawn" {
		t.Fatal("withdrawn consent did not reject queued media before object processing")
	}
}

func assertConcurrentConsentIdempotency(t *testing.T, ctx context.Context, database *gorm.DB, mediaService *mediaapp.MediaService, token, ownerID, consentID string) {
	t.Helper()
	const attempts = 12
	var wait sync.WaitGroup
	results := make(chan string, attempts)
	errorsFound := make(chan error, attempts)
	for range attempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			consent, err := mediaService.CreateConsent(ctx, token, mediaapp.CreateConsentInput{Purpose: mediaapp.MediaPurposeAvatarSourcePreparation, Category: mediaapp.MediaCategoryPersonPhoto, PolicyVersion: mediaapp.CurrentMediaPolicyVersion, ActivelyAgreed: true})
			if err != nil {
				errorsFound <- err
				return
			}
			results <- consent.ID
		}()
	}
	wait.Wait()
	close(results)
	close(errorsFound)
	for err := range errorsFound {
		serviceOK(t, "create concurrent fixed consent", err)
	}
	for id := range results {
		if id != consentID {
			t.Fatal("concurrent consent creation returned a second active record")
		}
	}
	var activeCount int64
	serviceOK(t, "count active consent records", database.WithContext(ctx).Table("consent_records").Where("owner_id = ? AND withdrawn_at IS NULL", ownerID).Count(&activeCount).Error)
	if activeCount != 1 {
		t.Fatalf("concurrent consent creation stored %d active records", activeCount)
	}
}

func syntheticJPEG(t *testing.T) []byte {
	t.Helper()
	pixels := image.NewRGBA(image.Rect(0, 0, 32, 48))
	for y := range 48 {
		for x := range 32 {
			pixels.Set(x, y, color.RGBA{R: uint8(x * 5), G: uint8(y * 3), B: 160, A: 255})
		}
	}
	var output bytes.Buffer
	if err := jpeg.Encode(&output, pixels, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func waitForMediaStatus(t *testing.T, ctx context.Context, mediaService *mediaapp.MediaService, token, mediaID string, expected mediaapp.MediaStatus) {
	t.Helper()
	for {
		media, err := mediaService.GetMedia(ctx, token, mediaID)
		serviceOK(t, "poll media state", err)
		if media.Status == expected {
			return
		}
		if media.Status == mediaapp.MediaRejected {
			t.Fatalf("synthetic JPEG was rejected: %s", media.StableReason)
		}
		select {
		case <-ctx.Done():
			t.Fatal("media state did not reach expected status")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func waitForDeletion(t *testing.T, ctx context.Context, mediaService *mediaapp.MediaService, token, requestID string) {
	t.Helper()
	for {
		deletion, err := mediaService.GetDeletionRequest(ctx, token, requestID)
		serviceOK(t, "poll deletion state", err)
		if deletion.Status == mediaapp.DeletionComplete {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("deletion did not complete")
		case <-time.After(50 * time.Millisecond):
		}
	}
}
