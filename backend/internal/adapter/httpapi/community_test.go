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

	communityapp "github.com/StephenQiu30/then-server/backend/internal/application/community"
)

type communityTransportStub struct {
	CommunityHTTPService
	created communityapp.PostContentInput
	err     error
}

func (s *communityTransportStub) CreatePost(_ context.Context, _, id string, input communityapp.PostContentInput) (communityapp.Post, error) {
	s.created = input
	return communityPostFixture(id), s.err
}

func (s *communityTransportStub) GetPublicPost(_ context.Context, id string) (communityapp.PublicPost, error) {
	body := "public body"
	return communityapp.PublicPost{ID: id, AuthorHandle: "public_author", AuthorDisplayName: "Public Author", Body: &body, Tags: []string{"ootd"}, ImageCount: 1, PublishedVersion: 8, PublishedAt: time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC)}, s.err
}

func (s *communityTransportStub) GetPublicPostImage(context.Context, string, int) (communityapp.MediaObjectReference, error) {
	return communityapp.MediaObjectReference{ObjectKey: "private/key.jpg", ObjectVersionID: "private-version"}, s.err
}

func (s *communityTransportStub) ListModerationCandidates(context.Context, string, int, string) (communityapp.ModerationCandidatePage, error) {
	return communityapp.ModerationCandidatePage{}, s.err
}

type communityObjectStub struct{ data string }

func (s communityObjectStub) OpenDerivedVersion(context.Context, string, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(s.data)), nil
}

func communityPostFixture(id string) communityapp.Post {
	body := "draft body"
	now := time.Date(2026, 9, 16, 8, 0, 0, 0, time.UTC)
	version := 1
	return communityapp.Post{ID: id, State: communityapp.PostDraft, Revision: 1, DraftVersion: &version, Current: communityapp.PostRevision{Version: 1, Body: &body, MediaIDs: []string{}, Tags: []string{"ootd"}, ReviewState: communityapp.ReviewDraft, CreatedAt: now}, CreatedAt: now, UpdatedAt: now}
}

func communityRouter(t *testing.T, service CommunityHTTPService) *Router {
	t.Helper()
	router, err := NewRouterWithCommunity(context.Background(), false, probeFunc(func(context.Context) error { return nil }), nil, nil, nil, nil, nil, nil, NewCommunityHandler(service, communityObjectStub{data: "synthetic-jpeg"}, true), time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func TestCommunityHTTPSeparatesPrivateAndPublicDTOs(t *testing.T) {
	service := new(communityTransportStub)
	router := communityRouter(t, service)
	token := strings.Repeat("a", 43)
	postID := "11111111-1111-4111-8111-111111111111"
	request := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(`{"id":"`+postID+`","body":"draft body","media_ids":[],"tags":["ootd"]}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.created.Body == nil || !strings.Contains(response.Body.String(), `"review_state":"draft"`) {
		t.Fatalf("private post create failed: status=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/posts/"+postID, nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, `"author_handle":"public_author"`) {
		t.Fatalf("public post read failed: status=%d body=%s", response.Code, body)
	}
	for _, privateField := range []string{"source_diary_id", "media_ids", "published_version", "review_round", "owner_id", "object_key"} {
		if strings.Contains(body, privateField) {
			t.Fatalf("public post leaked %s: %s", privateField, body)
		}
	}

	request = httptest.NewRequest(http.MethodGet, "/posts/"+postID+"/images/0", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/jpeg" || response.Body.String() != "synthetic-jpeg" {
		t.Fatalf("public image response invalid: status=%d type=%s body=%q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
}

func TestCommunityHTTPRejectsPrivateInjectionAndMapsOperatorAuthorization(t *testing.T) {
	service := new(communityTransportStub)
	router := communityRouter(t, service)
	token := strings.Repeat("a", 43)
	request := httptest.NewRequest(http.MethodPost, "/posts", strings.NewReader(`{"id":"11111111-1111-4111-8111-111111111111","body":"draft body","media_ids":[],"tags":[],"owner_id":"22222222-2222-4222-8222-222222222222"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || service.created.Body != nil {
		t.Fatalf("private field injection reached service: status=%d body=%s", response.Code, response.Body.String())
	}

	service.err = communityapp.ErrCommunityForbidden
	request = httptest.NewRequest(http.MethodGet, "/admin/moderation/posts?limit=20", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"FORBIDDEN"`) {
		t.Fatalf("operator authorization mapping failed: status=%d body=%s", response.Code, response.Body.String())
	}
}
