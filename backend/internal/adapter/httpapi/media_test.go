package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
)

type mediaTransportStub struct {
	media mediaapp.MediaAsset
	err   error
}

func (s *mediaTransportStub) CreateConsent(context.Context, string, mediaapp.CreateConsentInput) (mediaapp.ConsentRecord, error) {
	return mediaapp.ConsentRecord{}, s.err
}
func (s *mediaTransportStub) GetConsent(context.Context, string, string) (mediaapp.ConsentRecord, error) {
	return mediaapp.ConsentRecord{}, s.err
}
func (s *mediaTransportStub) WithdrawConsent(context.Context, string, string) (mediaapp.ConsentRecord, error) {
	return mediaapp.ConsentRecord{}, s.err
}
func (s *mediaTransportStub) CreateMediaUpload(context.Context, string, mediaapp.CreateMediaUploadInput) (mediaapp.MediaUpload, error) {
	return mediaapp.MediaUpload{}, s.err
}
func (s *mediaTransportStub) CompleteMediaUpload(context.Context, string, string, mediaapp.CompleteMediaUploadInput) (mediaapp.MediaAsset, error) {
	return s.media, s.err
}
func (s *mediaTransportStub) GetMedia(context.Context, string, string) (mediaapp.MediaAsset, error) {
	return s.media, s.err
}
func (s *mediaTransportStub) DeleteMedia(context.Context, string, string) (mediaapp.DeletionRequest, error) {
	return mediaapp.DeletionRequest{}, s.err
}
func (s *mediaTransportStub) GetDeletionRequest(context.Context, string, string) (mediaapp.DeletionRequest, error) {
	return mediaapp.DeletionRequest{}, s.err
}

func TestMediaStatusRequiresCookieAndHidesObjectReferences(t *testing.T) {
	now := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	service := &mediaTransportStub{media: mediaapp.MediaAsset{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11", ConsentID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Purpose: mediaapp.MediaPurposeAvatarSourcePreparation, Category: mediaapp.MediaCategoryPersonPhoto, ContentType: mediaapp.MediaContentTypeJPEG, ByteSize: 1024, SHA256: strings.Repeat("a", 64), RawObjectKey: "synthetic-secret-key", ObjectVersionID: "synthetic-secret-version", Status: mediaapp.MediaReady, CreatedAt: now, UpdatedAt: now}}
	router, err := NewRouter(context.Background(), false, probeFunc(func(context.Context) error { return nil }), nil, nil, nil, nil, nil, time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)), NewMediaHandler(service, true))
	if err != nil {
		t.Fatal(err)
	}
	path := "/media/018f1f74-a2d0-7c6d-9c17-4a0ea2400a11"
	withoutSession := httptest.NewRecorder()
	router.ServeHTTP(withoutSession, httptest.NewRequest(http.MethodGet, path, nil))
	if withoutSession.Code != http.StatusUnauthorized {
		t.Fatal("media route accepted a missing session")
	}
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "synthetic-session"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("media status=%d", response.Code)
	}
	if strings.Contains(response.Body.String(), "synthetic-secret") || strings.Contains(response.Body.String(), service.media.SHA256) || strings.Contains(response.Body.String(), "url") {
		t.Fatal("media status exposed an object reference, digest, or URL")
	}
}

func TestMediaDeclaredSizeLimitReturns413(t *testing.T) {
	service := &mediaTransportStub{err: mediaapp.ErrMediaTooLarge}
	router, err := NewRouter(context.Background(), false, probeFunc(func(context.Context) error { return nil }), nil, nil, nil, nil, nil, time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)), NewMediaHandler(service, true))
	if err != nil {
		t.Fatal(err)
	}
	body := `{"consent_id":"018f1f74-a2d0-7c6d-9c17-4a0ea2400a12","purpose":"avatar_source_preparation","content_type":"image/jpeg","byte_size":12582913,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
	request := httptest.NewRequest(http.MethodPost, "/media/uploads", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "synthetic-session"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), "PAYLOAD_TOO_LARGE") {
		t.Fatalf("unexpected oversized response: %d %s", response.Code, response.Body.String())
	}
}
