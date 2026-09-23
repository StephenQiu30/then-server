//go:build services

package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
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
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type avatarProbe struct{}

func (avatarProbe) Probe(context.Context) error { return nil }

type avatarLimiter struct{}

func (avatarLimiter) Allow(context.Context, string, string, int, time.Duration) (bool, time.Duration, error) {
	return true, 0, nil
}

func TestProfileAvatarOwnershipAndPublicVersion(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	base, err := gorm.Open(postgres.Open(environment.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open avatar database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create avatar schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := base.WithContext(cleanup).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("avatar schema cleanup failed")
		}
	})
	databaseURL, err := url.Parse(environment.databaseURL)
	serviceOK(t, "parse avatar database URL", err)
	query := databaseURL.Query()
	query.Set("search_path", schema)
	databaseURL.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open isolated avatar schema", err)
	serviceOK(t, "migrate avatar schema", store.Migrate(ctx, database))
	objects, err := objectstore.Open(ctx, environment.minioEndpoint, environment.minioAccessKey, environment.minioSecretKey, false)
	serviceOK(t, "open avatar objects", err)
	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	serviceOK(t, "create avatar accounts", err)
	owner, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "avatar-owner@example.test", DisplayName: "Owner", Password: "correct-password-owner"})
	serviceOK(t, "register avatar owner", err)
	other, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "avatar-other@example.test", DisplayName: "Other", Password: "correct-password-other"})
	serviceOK(t, "register avatar other", err)
	profile, err := accounts.PutCurrentProfile(ctx, owner.Token, accountapp.PutProfileInput{Handle: "avatar_owner", ExpectedRevision: 0})
	serviceOK(t, "create avatar profile", err)
	repository := store.NewMediaRepository(database)
	firstImage := avatarJPEG(t, color.RGBA{R: 240, A: 255})
	secondImage := avatarJPEG(t, color.RGBA{B: 240, A: 255})
	first := readyAvatarAsset(t, ctx, repository, objects, owner.User.ID, firstImage)
	second := readyAvatarAsset(t, ctx, repository, objects, owner.User.ID, secondImage)
	foreign := readyAvatarAsset(t, ctx, repository, objects, other.User.ID, avatarJPEG(t, color.RGBA{G: 240, A: 255}))
	community := createReadyCommunityMedia(t, ctx, repository, owner.User.ID, uuid.NewString())
	for _, wrong := range []string{foreign.ID, community.ID} {
		if _, err := accounts.PutProfileAvatar(ctx, owner.Token, wrong, profile.Revision); !errors.Is(err, accountapp.ErrProfileNotFound) {
			t.Fatalf("wrong owner or purpose accepted: %v", err)
		}
	}
	if _, err := accounts.PutProfileAvatar(ctx, other.Token, first.ID, 1); !errors.Is(err, accountapp.ErrProfileNotFound) {
		t.Fatalf("other account changed avatar: %v", err)
	}
	profile, err = accounts.PutProfileAvatar(ctx, owner.Token, first.ID, profile.Revision)
	serviceOK(t, "bind first avatar", err)
	if !profile.HasAvatar || profile.Revision != 2 {
		t.Fatal("avatar binding did not update public profile")
	}
	datasets, _, err := store.NewDataExportRepository(database).Snapshot(ctx, owner.User.ID, "structured")
	serviceOK(t, "export profile avatar association", err)
	avatarExported := false
	for _, dataset := range datasets {
		avatarExported = avatarExported || (dataset.Name == "profile" && bytes.Contains(dataset.Rows, []byte(first.ID)))
	}
	if !avatarExported {
		t.Fatal("account export omitted the avatar association")
	}
	if _, err := accounts.PutProfileAvatar(ctx, owner.Token, second.ID, 1); !errors.Is(err, accountapp.ErrProfileConflict) {
		t.Fatal("stale avatar revision accepted")
	}
	router, err := httpapi.NewRouter(ctx, false, avatarProbe{}, httpapi.NewAccountHandler(accounts, false, avatarLimiter{}).WithAvatarObjects(objects), nil, nil, nil, nil, time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	serviceOK(t, "build avatar HTTP router", err)
	read := func(path string, want int) []byte {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != want {
			t.Fatalf("avatar GET %s: status=%d body=%s", path, response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("avatar response was cacheable or sniffable")
		}
		return response.Body.Bytes()
	}
	if body := read("/profiles/avatar_owner", http.StatusOK); !bytes.Contains(body, []byte(`"avatar_url":"/profiles/avatar_owner/avatar"`)) || bytes.Contains(body, []byte(first.ID)) {
		t.Fatalf("public profile avatar reference is unsafe: %s", body)
	}
	overwrite := avatarJPEG(t, color.RGBA{R: 20, B: 200, A: 255})
	_, err = objects.PutDerived(ctx, fmt.Sprintf("media/%s/normalized.jpg", first.ID), bytes.NewReader(overwrite), int64(len(overwrite)))
	serviceOK(t, "overwrite avatar object key", err)
	if body := read("/profiles/avatar_owner/avatar", http.StatusOK); !bytes.Equal(body, firstImage) {
		t.Fatal("public endpoint did not read the fixed derived version")
	}
	putRequest := httptest.NewRequest(http.MethodPut, "/users/me/profile/avatar", strings.NewReader(fmt.Sprintf(`{"media_id":%q,"expected_revision":%d}`, second.ID, profile.Revision)))
	putRequest.Header.Set("Content-Type", "application/json")
	putRequest.AddCookie(&http.Cookie{Name: "then_session", Value: owner.Token})
	putResponse := httptest.NewRecorder()
	router.ServeHTTP(putResponse, putRequest)
	if putResponse.Code != http.StatusOK || !bytes.Contains(putResponse.Body.Bytes(), []byte(`"revision":3`)) {
		t.Fatalf("avatar HTTP replacement failed: status=%d body=%s", putResponse.Code, putResponse.Body.String())
	}
	profile, err = accounts.PublicProfile(ctx, "avatar_owner")
	serviceOK(t, "read replaced avatar profile", err)
	if body := read("/profiles/avatar_owner/avatar", http.StatusOK); !bytes.Equal(body, secondImage) {
		t.Fatal("old avatar remained publicly reachable after replacement")
	}
	old, err := repository.GetMedia(ctx, owner.User.ID, first.ID)
	serviceOK(t, "read retired avatar", err)
	if old.Status != mediaapp.MediaDeleting {
		t.Fatal("replaced avatar did not enter deletion")
	}
	if _, err := accounts.DeleteProfileAvatar(ctx, owner.Token, profile.Revision-1); !errors.Is(err, accountapp.ErrProfileConflict) {
		t.Fatal("stale avatar removal accepted")
	}
	_, err = repository.DeleteMedia(ctx, owner.User.ID, second.ID, time.Now().UTC())
	serviceOK(t, "delete bound avatar", err)
	read("/profiles/avatar_owner/avatar", http.StatusNotFound)
	profile, err = accounts.PublicProfile(ctx, "avatar_owner")
	serviceOK(t, "read profile after media deletion", err)
	if profile.HasAvatar || profile.Revision != 4 {
		t.Fatal("direct media deletion left avatar reference or stale revision")
	}
	if _, err := accounts.PutProfileAvatar(ctx, owner.Token, first.ID, profile.Revision); !errors.Is(err, accountapp.ErrProfileNotFound) {
		t.Fatal("deleted old avatar was rebound")
	}
	third := readyAvatarAsset(t, ctx, repository, objects, owner.User.ID, avatarJPEG(t, color.RGBA{R: 120, G: 120, A: 255}))
	profile, err = accounts.PutProfileAvatar(ctx, owner.Token, third.ID, profile.Revision)
	serviceOK(t, "bind third avatar", err)
	deleteRequest := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/users/me/profile/avatar?expected_revision=%d", profile.Revision), nil)
	deleteRequest.AddCookie(&http.Cookie{Name: "then_session", Value: owner.Token})
	deleteResponse := httptest.NewRecorder()
	router.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK || !bytes.Contains(deleteResponse.Body.Bytes(), []byte(`"avatar_url":null`)) {
		t.Fatalf("avatar HTTP removal failed: status=%d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	read("/profiles/avatar_owner/avatar", http.StatusNotFound)
	removed, err := repository.GetMedia(ctx, owner.User.ID, third.ID)
	serviceOK(t, "read removed avatar", err)
	if removed.Status != mediaapp.MediaDeleting {
		t.Fatal("removed avatar did not enter deletion")
	}
	fourth := readyAvatarAsset(t, ctx, repository, objects, owner.User.ID, avatarJPEG(t, color.RGBA{R: 180, G: 40, A: 255}))
	profile, err = accounts.PublicProfile(ctx, "avatar_owner")
	serviceOK(t, "read profile before account deletion", err)
	_, err = accounts.PutProfileAvatar(ctx, owner.Token, fourth.ID, profile.Revision)
	serviceOK(t, "bind avatar before account deletion", err)
	_, err = accounts.DeleteCurrentUser(ctx, owner.Token)
	serviceOK(t, "delete account with bound avatar", err)
	read("/profiles/avatar_owner/avatar", http.StatusNotFound)
	if _, err := accounts.PublicProfile(ctx, "avatar_owner"); !errors.Is(err, accountapp.ErrProfileNotFound) {
		t.Fatal("deleted account remained publicly readable")
	}
	if _, err := repository.CreateMedia(ctx, owner.User.ID, mediaapp.CreateMediaUploadInput{Purpose: mediaapp.MediaPurposeProfileAvatar, ContentType: mediaapp.MediaContentTypeJPEG, ByteSize: 128, SHA256: strings.Repeat("a", 64)}, time.Now().UTC()); !errors.Is(err, mediaapp.ErrMediaConflict) {
		t.Fatal("account deletion allowed a late avatar upload")
	}
	var pendingIDs []string
	serviceOK(t, "list account media cleanup", database.WithContext(ctx).Table("media_assets").Where("owner_id = ? AND status <> ?", owner.User.ID, string(mediaapp.MediaDeleted)).Pluck("id", &pendingIDs).Error)
	if len(pendingIDs) == 0 {
		t.Fatal("account deletion lost avatar cleanup work")
	}
	for _, id := range pendingIDs {
		serviceOK(t, "finish account media cleanup", repository.CompleteDeletion(ctx, uuid.NewString(), id, time.Now().UTC()))
	}
	var surviving int64
	serviceOK(t, "count deleted account", database.WithContext(ctx).Table("users").Where("id = ?", owner.User.ID).Count(&surviving).Error)
	if surviving != 0 {
		t.Fatal("bound avatar blocked account physical deletion")
	}
}

func avatarJPEG(t *testing.T, fill color.RGBA) []byte {
	t.Helper()
	imageData := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			imageData.Set(x, y, fill)
		}
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, imageData, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func readyAvatarAsset(t *testing.T, ctx context.Context, repository *store.MediaRepository, objects *objectstore.Store, ownerID string, data []byte) mediaapp.MediaAsset {
	t.Helper()
	now := time.Now().UTC()
	asset, err := repository.CreateMedia(ctx, ownerID, mediaapp.CreateMediaUploadInput{Purpose: mediaapp.MediaPurposeProfileAvatar, ContentType: mediaapp.MediaContentTypeJPEG, ByteSize: 128, SHA256: strings.Repeat("a", 64)}, now)
	serviceOK(t, "create avatar media", err)
	asset, err = repository.CompleteMedia(ctx, ownerID, asset.ID, "source-version", mediaapp.ObjectFact{}, now)
	serviceOK(t, "complete avatar media", err)
	asset, process, err := repository.BeginMediaCheck(ctx, asset.ID, now)
	serviceOK(t, "begin avatar check", err)
	if !process {
		t.Fatal("avatar was not processed")
	}
	key := fmt.Sprintf("media/%s/normalized.jpg", asset.ID)
	version, err := objects.PutDerived(ctx, key, bytes.NewReader(data), int64(len(data)))
	serviceOK(t, "store avatar derived object", err)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := objects.DeleteAllVersions(cleanup, objectstore.DerivedBucket, key); err != nil {
			t.Error("avatar object cleanup failed")
		}
	})
	serviceOK(t, "finish avatar check", repository.CompleteMediaCheck(ctx, uuid.NewString(), asset.ID, &mediaapp.MediaDerivation{ObjectKey: key, ObjectVersionID: version}, 2, 2, mediaapp.MediaReady, "ready", now))
	return asset
}
