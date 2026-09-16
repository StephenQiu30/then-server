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
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
	"github.com/StephenQiu30/then-server/backend/internal/application/mediaworker"
	privacyapp "github.com/StephenQiu30/then-server/backend/internal/application/privacy"
	"github.com/StephenQiu30/then-server/backend/internal/domain"
	amqp "github.com/rabbitmq/amqp091-go"
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

	connection, err := amqp.Dial(environment.rabbitMQURL)
	serviceOK(t, "open queue cleanup connection", err)
	channel, err := connection.Channel()
	serviceOK(t, "open queue cleanup channel", err)
	for _, queue := range []string{"then.media-check", "then.media-delete"} {
		_, _ = channel.QueuePurge(queue, false)
	}
	channel.Close()
	connection.Close()

	objects, err := objectstore.Open(ctx, environment.minioEndpoint, environment.minioAccessKey, environment.minioSecretKey, false)
	serviceOK(t, "open private media store", err)
	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	serviceOK(t, "construct media account service", err)
	owner, err := accounts.Register(ctx, domain.RegisterAccountInput{Email: "media-owner@example.test", DisplayName: "Media Owner", Password: "correct-password-owner"})
	serviceOK(t, "register media owner", err)
	other, err := accounts.Register(ctx, domain.RegisterAccountInput{Email: "media-other@example.test", DisplayName: "Other Owner", Password: "correct-password-other"})
	serviceOK(t, "register second media owner", err)
	privacy, err := privacyapp.NewPrivacyService(accounts, store.NewPrivacyRepository(database))
	serviceOK(t, "construct media privacy service", err)
	_, err = privacy.ConfirmSelfAdultDeclaration(ctx, owner.Token, domain.ConfirmSelfAdultDeclarationInput{PolicyVersion: domain.CurrentSelfAdultPolicyVersion, ConfirmsSelfAndAdult: true})
	serviceOK(t, "confirm synthetic adult declaration", err)
	mediaService, err := mediaapp.NewMediaService(accounts, store.NewMediaRepository(database), objects)
	serviceOK(t, "construct media service", err)
	consent, err := mediaService.CreateConsent(ctx, owner.Token, domain.CreateConsentInput{Purpose: domain.MediaPurposeAvatarSourcePreparation, Category: domain.MediaCategoryPersonPhoto, PolicyVersion: domain.CurrentMediaPolicyVersion, ActivelyAgreed: true})
	serviceOK(t, "create fixed synthetic consent", err)
	assertConcurrentConsentIdempotency(t, ctx, database, mediaService, owner.Token, owner.User.ID, consent.ID)
	assertAccountDeleteAndMediaCreateSerialize(t, ctx, database, accounts, mediaService)
	assertUnfinishedUploadRetentionBoundary(t, ctx, database, mediaService, other)

	photo := syntheticJPEG(t)
	digest := fmt.Sprintf("%x", sha256.Sum256(photo))
	upload, err := mediaService.CreateMediaUpload(ctx, owner.Token, domain.CreateMediaUploadInput{ConsentID: consent.ID, Purpose: domain.MediaPurposeAvatarSourcePreparation, ContentType: domain.MediaContentTypeJPEG, ByteSize: int64(len(photo)), SHA256: digest})
	serviceOK(t, "create signed media upload", err)
	if upload.Method != http.MethodPut || upload.ExpiresAt.Sub(time.Now()) > domain.UploadIntentLifetime+time.Second {
		t.Fatal("upload intent did not use the fixed short PUT contract")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, upload.URL, bytes.NewReader(photo))
	serviceOK(t, "build signed media PUT", err)
	request.ContentLength = int64(len(photo))
	request.Header.Set("Content-Type", domain.MediaContentTypeJPEG)
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
	uploaded, err := mediaService.CompleteMediaUpload(ctx, owner.Token, upload.Media.ID, domain.CompleteMediaUploadInput{VersionID: versionID})
	serviceOK(t, "complete fixed media upload", err)
	if uploaded.Status != domain.MediaUploaded || uploaded.ObjectVersionID != versionID {
		t.Fatal("finalize did not pin the uploaded object version")
	}
	repeated, err := mediaService.CompleteMediaUpload(ctx, owner.Token, upload.Media.ID, domain.CompleteMediaUploadInput{VersionID: versionID})
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
	if _, err := mediaService.GetMedia(ctx, other.Token, uploaded.ID); !errors.Is(err, domain.ErrMediaNotFound) {
		t.Fatal("cross-owner media lookup did not use the not-found boundary")
	}
	if err := accounts.DeleteCurrentUser(ctx, owner.Token); !errors.Is(err, domain.ErrAccountMediaConflict) {
		t.Fatal("account deletion did not block while private media remained active")
	}

	broker, err := messagequeue.Open(environment.rabbitMQURL)
	serviceOK(t, "open media broker", err)
	defer broker.Close()
	runner, err := mediaworker.New(store.NewMediaRepository(database), broker, objects)
	serviceOK(t, "construct media worker", err)
	workerContext, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan error, 1)
	go func() { workerDone <- runner.Run(workerContext) }()
	waitForMediaStatus(t, ctx, mediaService, owner.Token, uploaded.ID, domain.MediaReady)
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
	if deletion.Status != domain.DeletionPending || deletion.ReadRevokedAt.IsZero() {
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
	if deleted.Status != domain.MediaDeleted || deleted.SHA256 != strings.Repeat("0", 64) || deleted.ObjectVersionID != "" {
		t.Fatal("completed deletion retained an active object reference")
	}
	if err := accounts.DeleteCurrentUser(ctx, owner.Token); err != nil {
		t.Fatal("account deletion remained blocked after media deletion")
	}
	stopWorker()
	if err := <-workerDone; err != nil {
		t.Fatal("media worker did not stop cleanly")
	}
}

func assertUnfinishedUploadRetentionBoundary(t *testing.T, ctx context.Context, database *gorm.DB, mediaService *mediaapp.MediaService, user domain.AuthenticatedUser) {
	t.Helper()
	consent, err := mediaService.CreateConsent(ctx, user.Token, domain.CreateConsentInput{Purpose: domain.MediaPurposeAvatarSourcePreparation, Category: domain.MediaCategoryPersonPhoto, PolicyVersion: domain.CurrentMediaPolicyVersion, ActivelyAgreed: true})
	serviceOK(t, "create unfinished-upload consent", err)
	media, err := store.NewMediaRepository(database).CreateMedia(ctx, user.User.ID, domain.CreateMediaUploadInput{ConsentID: consent.ID, Purpose: domain.MediaPurposeAvatarSourcePreparation, ContentType: domain.MediaContentTypeJPEG, ByteSize: 128, SHA256: strings.Repeat("a", 64)}, time.Now().UTC())
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

func containsMedia(media []domain.MediaAsset, id string) bool {
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
		user, err := accounts.Register(ctx, domain.RegisterAccountInput{Email: fmt.Sprintf("media-race-%d@example.test", attempt), DisplayName: "Media Race", Password: "correct-password-race-user"})
		serviceOK(t, "register media/account race user", err)
		consent, err := mediaService.CreateConsent(ctx, user.Token, domain.CreateConsentInput{Purpose: domain.MediaPurposeAvatarSourcePreparation, Category: domain.MediaCategoryPersonPhoto, PolicyVersion: domain.CurrentMediaPolicyVersion, ActivelyAgreed: true})
		serviceOK(t, "create media/account race consent", err)
		start := make(chan struct{})
		createResult := make(chan error, 1)
		deleteResult := make(chan error, 1)
		go func() {
			<-start
			_, err := store.NewMediaRepository(database).CreateMedia(ctx, user.User.ID, domain.CreateMediaUploadInput{ConsentID: consent.ID, Purpose: domain.MediaPurposeAvatarSourcePreparation, ContentType: domain.MediaContentTypeJPEG, ByteSize: 128, SHA256: strings.Repeat("a", 64)}, time.Now().UTC())
			createResult <- err
		}()
		go func() {
			<-start
			deleteResult <- store.NewAccountRepository(database).DeleteUser(ctx, user.User.ID)
		}()
		close(start)
		createErr, deleteErr := <-createResult, <-deleteResult
		switch {
		case createErr == nil && errors.Is(deleteErr, domain.ErrAccountMediaConflict):
		case errors.Is(createErr, domain.ErrConsentRequired) && deleteErr == nil:
		default:
			t.Fatalf("media creation/account deletion were not serialized: create=%v delete=%v", createErr, deleteErr)
		}
	}
}

func assertWithdrawnConsentRejectsQueuedMedia(t *testing.T, ctx context.Context, database *gorm.DB, privacy *privacyapp.PrivacyService, mediaService *mediaapp.MediaService, user domain.AuthenticatedUser, photo []byte, digest string) {
	t.Helper()
	_, err := privacy.ConfirmSelfAdultDeclaration(ctx, user.Token, domain.ConfirmSelfAdultDeclarationInput{PolicyVersion: domain.CurrentSelfAdultPolicyVersion, ConfirmsSelfAndAdult: true})
	serviceOK(t, "confirm second synthetic adult declaration", err)
	consent, err := mediaService.CreateConsent(ctx, user.Token, domain.CreateConsentInput{Purpose: domain.MediaPurposeAvatarSourcePreparation, Category: domain.MediaCategoryPersonPhoto, PolicyVersion: domain.CurrentMediaPolicyVersion, ActivelyAgreed: true})
	serviceOK(t, "create consent for withdrawal check", err)
	upload, err := mediaService.CreateMediaUpload(ctx, user.Token, domain.CreateMediaUploadInput{ConsentID: consent.ID, Purpose: domain.MediaPurposeAvatarSourcePreparation, ContentType: domain.MediaContentTypeJPEG, ByteSize: int64(len(photo)), SHA256: digest})
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
	_, err = mediaService.CompleteMediaUpload(ctx, user.Token, upload.Media.ID, domain.CompleteMediaUploadInput{VersionID: versionID})
	serviceOK(t, "complete queued upload before withdrawal", err)
	withdrawn, err := mediaService.WithdrawConsent(ctx, user.Token, consent.ID)
	serviceOK(t, "withdraw consent before queued processing", err)
	repeated, err := mediaService.WithdrawConsent(ctx, user.Token, consent.ID)
	serviceOK(t, "repeat consent withdrawal", err)
	if withdrawn.Status != domain.ConsentWithdrawn || repeated.Status != domain.ConsentWithdrawn || withdrawn.WithdrawnAt == nil || repeated.WithdrawnAt == nil || !withdrawn.WithdrawnAt.Equal(*repeated.WithdrawnAt) {
		t.Fatal("consent withdrawal was not idempotent")
	}
	media, process, err := store.NewMediaRepository(database).BeginMediaCheck(ctx, upload.Media.ID, time.Now().UTC())
	serviceOK(t, "begin queued media check after withdrawal", err)
	if process || media.Status != domain.MediaRejected || media.StableReason != "consent_withdrawn" {
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
			consent, err := mediaService.CreateConsent(ctx, token, domain.CreateConsentInput{Purpose: domain.MediaPurposeAvatarSourcePreparation, Category: domain.MediaCategoryPersonPhoto, PolicyVersion: domain.CurrentMediaPolicyVersion, ActivelyAgreed: true})
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

func waitForMediaStatus(t *testing.T, ctx context.Context, mediaService *mediaapp.MediaService, token, mediaID string, expected domain.MediaStatus) {
	t.Helper()
	for {
		media, err := mediaService.GetMedia(ctx, token, mediaID)
		serviceOK(t, "poll media state", err)
		if media.Status == expected {
			return
		}
		if media.Status == domain.MediaRejected {
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
		if deletion.Status == domain.DeletionComplete {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("deletion did not complete")
		case <-time.After(50 * time.Millisecond):
		}
	}
}
