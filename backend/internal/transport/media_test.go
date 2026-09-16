package transport

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
)

type mediaTransportStub struct {
	media model.MediaAsset
	err   error
}

func (s *mediaTransportStub) CreateConsent(context.Context, string, model.CreateConsentInput) (model.ConsentRecord, error) {
	return model.ConsentRecord{}, s.err
}
func (s *mediaTransportStub) GetConsent(context.Context, string, string) (model.ConsentRecord, error) {
	return model.ConsentRecord{}, s.err
}
func (s *mediaTransportStub) WithdrawConsent(context.Context, string, string) (model.ConsentRecord, error) {
	return model.ConsentRecord{}, s.err
}
func (s *mediaTransportStub) CreateMediaUpload(context.Context, string, model.CreateMediaUploadInput) (model.MediaUpload, error) {
	return model.MediaUpload{}, s.err
}
func (s *mediaTransportStub) CompleteMediaUpload(context.Context, string, string, model.CompleteMediaUploadInput) (model.MediaAsset, error) {
	return s.media, s.err
}
func (s *mediaTransportStub) GetMedia(context.Context, string, string) (model.MediaAsset, error) {
	return s.media, s.err
}
func (s *mediaTransportStub) DeleteMedia(context.Context, string, string) (model.DeletionRequest, error) {
	return model.DeletionRequest{}, s.err
}
func (s *mediaTransportStub) GetDeletionRequest(context.Context, string, string) (model.DeletionRequest, error) {
	return model.DeletionRequest{}, s.err
}

func TestMediaStatusRequiresCookieAndHidesObjectReferences(t *testing.T) {
	now := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	service := &mediaTransportStub{media: model.MediaAsset{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11", ConsentID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Purpose: model.MediaPurposeAvatarSourcePreparation, Category: model.MediaCategoryPersonPhoto, ContentType: model.MediaContentTypeJPEG, ByteSize: 1024, SHA256: strings.Repeat("a", 64), RawObjectKey: "synthetic-secret-key", ObjectVersionID: "synthetic-secret-version", Status: model.MediaReady, CreatedAt: now, UpdatedAt: now}}
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
	service := &mediaTransportStub{err: model.ErrMediaTooLarge}
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
