//go:build services

package services

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/adapter/objectstore"
	store "github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	exportapp "github.com/StephenQiu30/then-server/backend/internal/application/dataexport"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDataExportStructuredPrivateLifecycle(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	base, err := gorm.Open(postgres.Open(environment.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open export database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create export schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if err := base.WithContext(cleanup).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("export schema cleanup failed")
		}
	})
	databaseURL, err := url.Parse(environment.databaseURL)
	serviceOK(t, "parse export database URL", err)
	query := databaseURL.Query()
	query.Set("search_path", schema)
	databaseURL.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open isolated export schema", err)
	serviceOK(t, "migrate export schema", store.Migrate(ctx, database))
	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	serviceOK(t, "construct export accounts", err)
	objects, err := objectstore.Open(ctx, environment.minioEndpoint, environment.minioAccessKey, environment.minioSecretKey, false)
	serviceOK(t, "open private object store", err)
	repository := store.NewDataExportRepository(database)
	exports, err := exportapp.New(accounts, repository, objects)
	serviceOK(t, "construct export service", err)
	first, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "export-one@example.test", DisplayName: "First", Password: "first-password-2026"})
	serviceOK(t, "register export owner", err)
	second, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "export-two@example.test", DisplayName: "Second", Password: "second-password-2026"})
	serviceOK(t, "register other export owner", err)
	_, err = exports.Create(ctx, first.Token, "wrong-password", exportapp.ModeStructured)
	if !errors.Is(err, accountapp.ErrAuthentication) {
		t.Fatal("wrong current password accepted")
	}
	_, err = exports.Create(ctx, first.Token, "first-password-2026", "unknown")
	if !errors.Is(err, exportapp.ErrInvalidMode) {
		t.Fatal("unfinished media mode was accepted")
	}
	wardrobeID := uuid.NewString()
	serviceOK(t, "insert synthetic wardrobe row", database.WithContext(ctx).Exec(`INSERT INTO wardrobe_items (owner_id,id,name,category,availability,source,revision,created_at,updated_at) VALUES (?,?,?,'top','wearable','wardrobe',1,now(),now())`, first.User.ID, wardrobeID, "Synthetic top").Error)
	job, err := exports.Create(ctx, first.Token, "first-password-2026", exportapp.ModeStructured)
	serviceOK(t, "create export job", err)
	if _, err := exports.Get(ctx, second.Token, job.ID); !errors.Is(err, exportapp.ErrNotFound) {
		t.Fatal("other owner read export status")
	}
	if _, err := exports.Open(ctx, first.Token, job.ID); !errors.Is(err, exportapp.ErrNotReady) {
		t.Fatal("preparing export was downloadable")
	}
	processed, err := exports.ProcessNext(ctx)
	serviceOK(t, "process export", err)
	if !processed {
		t.Fatal("export worker did not claim pending job")
	}
	ready, err := exports.Get(ctx, first.Token, job.ID)
	serviceOK(t, "read ready export", err)
	if ready.Status != exportapp.StatusReady || ready.Counts["wardrobe"] != 1 || ready.Counts["account"] != 1 {
		t.Fatal("structured export counts are wrong")
	}
	reader, err := exports.Open(ctx, first.Token, job.ID)
	serviceOK(t, "open private archive", err)
	archive, err := io.ReadAll(io.LimitReader(reader, 33<<20))
	serviceOK(t, "read private archive", err)
	serviceOK(t, "close private archive", reader.Close())
	zipReader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	serviceOK(t, "parse private archive", err)
	for _, file := range zipReader.File {
		if file.Name == "manifest.json" {
			continue
		}
		entry, err := file.Open()
		serviceOK(t, "open export entry for private field check", err)
		data, err := io.ReadAll(entry)
		serviceOK(t, "read export entry for private field check", err)
		entry.Close()
		for _, forbidden := range []string{"password_hash", "raw_object_key", "object_version", "token_hash", "mail_challenge"} {
			if bytes.Contains(data, []byte(forbidden)) {
				t.Fatal("private internal field leaked into archive")
			}
		}
	}
	var wardrobe []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	for _, file := range zipReader.File {
		if file.Name != "wardrobe.json" {
			continue
		}
		entry, err := file.Open()
		serviceOK(t, "open wardrobe entry", err)
		data, err := io.ReadAll(entry)
		serviceOK(t, "read wardrobe entry", err)
		entry.Close()
		serviceOK(t, "decode wardrobe entry", json.Unmarshal(data, &wardrobe))
	}
	if len(wardrobe) != 1 || wardrobe[0].ID != wardrobeID || wardrobe[0].Name != "Synthetic top" {
		t.Fatal("owner wardrobe record missing from archive")
	}
	if _, err := exports.Open(ctx, second.Token, job.ID); !errors.Is(err, exportapp.ErrNotFound) {
		t.Fatal("other owner downloaded private archive")
	}
	serviceOK(t, "revoke export", exports.Revoke(ctx, first.Token, job.ID))
	serviceOK(t, "repeat export revocation", exports.Revoke(ctx, first.Token, job.ID))
	if _, err := exports.Open(ctx, first.Token, job.ID); !errors.Is(err, exportapp.ErrNotReady) {
		t.Fatal("revoked export was downloadable")
	}
	cleaned, err := exports.CleanupNext(ctx)
	serviceOK(t, "clean revoked export", err)
	if !cleaned {
		t.Fatal("revoked archive cleanup not claimed")
	}
	if _, err := objects.OpenArchive(ctx, ready.ObjectKey, ready.ObjectVersion); err == nil {
		t.Fatal("revoked archive version remains")
	}
	serviceOK(t, "age cleaned metadata", database.WithContext(ctx).Exec("UPDATE data_exports SET cleaned_at = ? WHERE id = ?", time.Now().Add(-8*24*time.Hour), job.ID).Error)
	serviceOK(t, "purge cleaned metadata", repository.Purge(ctx, time.Now().Add(-7*24*time.Hour)))
	if _, err := exports.Get(ctx, first.Token, job.ID); !errors.Is(err, exportapp.ErrNotFound) {
		t.Fatal("retained stale export metadata")
	}
	secondJob, err := exports.Create(ctx, first.Token, "first-password-2026", exportapp.ModeStructured)
	serviceOK(t, "create expiring export", err)
	processed, err = exports.ProcessNext(ctx)
	serviceOK(t, "process expiring export", err)
	if !processed {
		t.Fatal("expiring export not claimed")
	}
	serviceOK(t, "advance synthetic expiry", database.WithContext(ctx).Exec("UPDATE data_exports SET expires_at = ? WHERE id = ?", time.Now().Add(-time.Second), secondJob.ID).Error)
	expired, err := exports.Get(ctx, first.Token, secondJob.ID)
	serviceOK(t, "read expired export", err)
	if expired.Status != exportapp.StatusExpired {
		t.Fatal("expired export still ready")
	}
	if _, err := exports.Open(ctx, first.Token, secondJob.ID); !errors.Is(err, exportapp.ErrNotReady) {
		t.Fatal("expired export was downloadable")
	}
	cleaned, err = exports.CleanupNext(ctx)
	serviceOK(t, "clean expired export", err)
	if !cleaned {
		t.Fatal("expired archive cleanup not claimed")
	}
	recoveryJob, err := exports.Create(ctx, first.Token, "first-password-2026", exportapp.ModeStructured)
	serviceOK(t, "create recoverable export", err)
	claimed, found, err := repository.Claim(ctx, time.Now().UTC())
	serviceOK(t, "claim recoverable export", err)
	if !found || claimed.ID != recoveryJob.ID {
		t.Fatal("pending export claim missing")
	}
	serviceOK(t, "simulate stopped worker lease", database.WithContext(ctx).Exec("UPDATE data_exports SET lease_until = ? WHERE id = ?", time.Now().Add(-time.Second), recoveryJob.ID).Error)
	var group sync.WaitGroup
	type claimResult struct {
		processed bool
		err       error
	}
	results := make(chan claimResult, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			processed, err := exports.ProcessNext(ctx)
			results <- claimResult{processed, err}
		}()
	}
	group.Wait()
	close(results)
	claimCount := 0
	for result := range results {
		serviceOK(t, "process concurrent export", result.err)
		if result.processed {
			claimCount++
		}
	}
	if claimCount != 1 {
		t.Fatal("concurrent workers claimed the same export")
	}
	recovered, err := exports.Get(ctx, first.Token, recoveryJob.ID)
	serviceOK(t, "read recovered export", err)
	if recovered.Status != exportapp.StatusReady {
		t.Fatal("stopped worker export did not recover")
	}
	serviceOK(t, "revoke recovered export", exports.Revoke(ctx, first.Token, recoveryJob.ID))
	_, err = exports.CleanupNext(ctx)
	serviceOK(t, "clean recovered export", err)
	deletionJob, err := exports.Create(ctx, first.Token, "first-password-2026", exportapp.ModeStructured)
	serviceOK(t, "create pre-deletion export", err)
	claimed, found, err = repository.Claim(ctx, time.Now().UTC())
	serviceOK(t, "claim pre-deletion export", err)
	if !found || claimed.ID != deletionJob.ID {
		t.Fatal("pre-deletion job was not claimed")
	}
	_, err = accounts.DeleteCurrentUser(ctx, first.Token)
	serviceOK(t, "delete account with export", err)
	accepted, err := repository.Complete(ctx, claimed, map[string]int{"account": 1}, nil, "exports/"+deletionJob.ID+"/1.zip", "synthetic-version", time.Now().UTC())
	serviceOK(t, "reject late export publication", err)
	if accepted {
		t.Fatal("export published after account deletion")
	}
	if _, err := exports.Get(ctx, first.Token, deletionJob.ID); !errors.Is(err, accountapp.ErrAuthentication) {
		t.Fatal("deleted account retained export access")
	}
	serviceOK(t, "expire stopped export lease", database.WithContext(ctx).Exec("UPDATE data_exports SET lease_until = ? WHERE id = ?", time.Now().Add(-time.Second), deletionJob.ID).Error)
	cleaned, err = exports.CleanupNext(ctx)
	serviceOK(t, "clean account export", err)
	if !cleaned {
		t.Fatal("account export cleanup not claimed")
	}
}

func TestDataExportMediaFixedVersionAndDeletion(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	base, err := gorm.Open(postgres.Open(environment.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open media export database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create media export schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if err := base.WithContext(cleanup).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("media export schema cleanup failed")
		}
	})
	databaseURL, err := url.Parse(environment.databaseURL)
	serviceOK(t, "parse media export database URL", err)
	query := databaseURL.Query()
	query.Set("search_path", schema)
	databaseURL.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open media export schema", err)
	serviceOK(t, "migrate media export schema", store.Migrate(ctx, database))
	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	serviceOK(t, "construct media export accounts", err)
	objects, err := objectstore.Open(ctx, environment.minioEndpoint, environment.minioAccessKey, environment.minioSecretKey, false)
	serviceOK(t, "open media export object store", err)
	mediaService, err := mediaapp.NewMediaService(accounts, store.NewMediaRepository(database), objects)
	serviceOK(t, "construct media export service", err)
	exports, err := exportapp.New(accounts, store.NewDataExportRepository(database), objects)
	serviceOK(t, "construct media exports", err)
	owner, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "media-export-owner@example.test", DisplayName: "Owner", Password: "owner-password-2026"})
	serviceOK(t, "register media export owner", err)
	other, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "media-export-other@example.test", DisplayName: "Other", Password: "other-password-2026"})
	serviceOK(t, "register media export other owner", err)
	photo := syntheticJPEG(t)
	digest := fmt.Sprintf("%x", sha256.Sum256(photo))
	upload, err := mediaService.CreateMediaUpload(ctx, owner.Token, mediaapp.CreateMediaUploadInput{Purpose: mediaapp.MediaPurposeDiaryImage, ContentType: mediaapp.MediaContentTypeJPEG, ByteSize: int64(len(photo)), SHA256: digest})
	serviceOK(t, "create media export upload", err)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if err := objects.DeleteAllVersions(cleanup, objectstore.RawBucket, upload.Media.RawObjectKey); err != nil {
			t.Error("media export raw source cleanup failed")
		}
	})
	put := func(content []byte) string {
		request, err := http.NewRequestWithContext(ctx, http.MethodPut, upload.URL, bytes.NewReader(content))
		serviceOK(t, "build media export PUT", err)
		request.ContentLength = int64(len(content))
		for key, value := range upload.Headers {
			if key != "Content-Length" {
				request.Header.Set(key, value)
			}
		}
		response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
		serviceOK(t, "upload media export source", err)
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK || response.Header.Get("X-Amz-Version-Id") == "" {
			t.Fatal("media export source PUT failed")
		}
		return response.Header.Get("X-Amz-Version-Id")
	}
	version := put(photo)
	_, err = mediaService.CompleteMediaUpload(ctx, owner.Token, upload.Media.ID, mediaapp.CompleteMediaUploadInput{VersionID: version})
	serviceOK(t, "pin media export version", err)
	serviceOK(t, "mark synthetic source ready", database.WithContext(ctx).Exec("UPDATE media_assets SET status = 'ready' WHERE id = ?", upload.Media.ID).Error)
	changed := bytes.Clone(photo)
	changed[len(changed)/2] ^= 0xff
	if overwritten := put(changed); overwritten == version {
		t.Fatal("source overwrite did not create a new version")
	}
	job, err := exports.Create(ctx, owner.Token, "owner-password-2026", exportapp.ModeWithMedia)
	serviceOK(t, "create media export", err)
	processed, err := exports.ProcessNext(ctx)
	serviceOK(t, "process media export", err)
	if !processed {
		t.Fatal("media export not processed")
	}
	ready, err := exports.Get(ctx, owner.Token, job.ID)
	serviceOK(t, "get ready media export", err)
	if ready.Status != exportapp.StatusReady || ready.Counts["media_files"] != 1 || ready.Counts["media_omitted"] != 0 {
		t.Fatal("media export counts or status wrong")
	}
	if _, err := exports.Open(ctx, other.Token, job.ID); !errors.Is(err, exportapp.ErrNotFound) {
		t.Fatal("other owner read media export")
	}
	reader, err := exports.Open(ctx, owner.Token, job.ID)
	serviceOK(t, "open media export", err)
	archive, err := io.ReadAll(reader)
	serviceOK(t, "read media export", err)
	serviceOK(t, "close media export", reader.Close())
	zipReader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	serviceOK(t, "parse media export ZIP", err)
	seen := false
	for _, file := range zipReader.File {
		if file.Name == "manifest.json" {
			entry, err := file.Open()
			serviceOK(t, "open media manifest", err)
			content, err := io.ReadAll(entry)
			serviceOK(t, "read media manifest", err)
			serviceOK(t, "close media manifest", entry.Close())
			var manifest struct {
				Files []struct {
					Name   string `json:"name"`
					SHA256 string `json:"sha256"`
				} `json:"files"`
			}
			serviceOK(t, "decode media manifest", json.Unmarshal(content, &manifest))
			matched := false
			for _, entry := range manifest.Files {
				if entry.Name == "media/"+upload.Media.ID+".jpg" && entry.SHA256 == digest {
					matched = true
				}
			}
			if !matched || bytes.Contains(content, []byte(upload.Media.RawObjectKey)) {
				t.Fatal("media manifest digest missing or private object key leaked")
			}
		}
		if file.Name != "media/"+upload.Media.ID+".jpg" {
			continue
		}
		entry, err := file.Open()
		serviceOK(t, "open media entry", err)
		content, err := io.ReadAll(entry)
		serviceOK(t, "read media entry", err)
		serviceOK(t, "close media entry", entry.Close())
		if !bytes.Equal(content, photo) {
			t.Fatal("media ZIP did not contain the pinned original")
		}
		seen = true
	}
	if !seen || bytes.Contains(archive, []byte(upload.Media.RawObjectKey)) {
		t.Fatal("media ZIP entry missing or private object key leaked")
	}
	serviceOK(t, "mark source retention cleanup", database.WithContext(ctx).Exec("UPDATE media_assets SET source_deleted_at = ? WHERE id = ?", time.Now().UTC(), upload.Media.ID).Error)
	partialJob, err := exports.Create(ctx, owner.Token, "owner-password-2026", exportapp.ModeWithMedia)
	serviceOK(t, "create partial media export", err)
	_, err = exports.ProcessNext(ctx)
	serviceOK(t, "process partial media export", err)
	partial, err := exports.Get(ctx, owner.Token, partialJob.ID)
	serviceOK(t, "get partial media export", err)
	if partial.Status != exportapp.StatusPartial || partial.Counts["media_files"] != 0 || partial.Counts["media_omitted"] != 1 || !containsString(partial.Omissions, "media_source_removed:"+upload.Media.ID) {
		t.Fatal("removed source was not reported as partial")
	}
	partialReader, err := exports.Open(ctx, owner.Token, partialJob.ID)
	serviceOK(t, "download partial media export", err)
	partialReader.Close()
	_, err = mediaService.DeleteMedia(ctx, owner.Token, upload.Media.ID)
	serviceOK(t, "delete exported media", err)
	for _, id := range []string{job.ID, partialJob.ID} {
		if _, err := exports.Open(ctx, owner.Token, id); !errors.Is(err, exportapp.ErrNotReady) {
			t.Fatal("media deletion did not revoke archived media")
		}
		cleaned, err := exports.CleanupNext(ctx)
		serviceOK(t, "clean revoked media export", err)
		if !cleaned {
			t.Fatal("revoked media export not cleaned")
		}
	}
	if _, err := objects.OpenArchive(ctx, ready.ObjectKey, ready.ObjectVersion); err == nil {
		t.Fatal("deleted media remained in export storage")
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
