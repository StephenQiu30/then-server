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

	"github.com/StephenQiu30/then-server/backend/internal/domain"
)

type diaryTransportStub struct {
	input    domain.DiaryEntryInput
	expected int
	err      error
}

func (s *diaryTransportStub) Create(_ context.Context, _, id string, input domain.DiaryEntryInput) (domain.DiaryEntry, error) {
	s.input = input
	return diaryFixture(id), s.err
}
func (s *diaryTransportStub) List(context.Context, string, int, *string, *string, *string) (domain.DiaryEntryPage, error) {
	return domain.DiaryEntryPage{Entries: []domain.DiaryEntry{diaryFixture("11111111-1111-4111-8111-111111111111")}}, s.err
}
func (s *diaryTransportStub) Get(_ context.Context, _, id string) (domain.DiaryEntry, error) {
	return diaryFixture(id), s.err
}
func (s *diaryTransportStub) Update(_ context.Context, _, id string, expected int, input domain.DiaryEntryInput) (domain.DiaryEntry, error) {
	s.expected, s.input = expected, input
	entry := diaryFixture(id)
	entry.Revision = expected + 1
	return entry, s.err
}
func (s *diaryTransportStub) DeletionImpact(_ context.Context, _, id string) (domain.DiaryDeletionImpact, error) {
	return domain.DiaryDeletionImpact{EntryID: id, Revision: 1, MediaCount: 1, MediaRetained: true}, s.err
}
func (s *diaryTransportStub) Delete(_ context.Context, _, _ string, expected int) error {
	s.expected = expected
	return s.err
}
func (s *diaryTransportStub) Calendar(context.Context, string, string) (domain.CalendarMonth, error) {
	return domain.CalendarMonth{Month: "2026-09", Days: []domain.CalendarDay{{LocalDate: "2026-09-16", PlanCount: 1, WearEventCount: 1, DiaryCount: 2}}}, s.err
}

func diaryFixture(id string) domain.DiaryEntry {
	body := "今天的穿搭"
	return domain.DiaryEntry{ID: id, LocalDate: "2026-09-16", TimeZone: "Asia/Shanghai", Body: &body, MediaIDs: []string{"33333333-3333-4333-8333-333333333333"}, Revision: 1, CreatedAt: time.Date(2026, 9, 16, 4, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 9, 16, 4, 0, 0, 0, time.UTC)}
}

func diaryRouter(t *testing.T, service DiaryHTTPService) *Router {
	t.Helper()
	router, err := NewRouterWithDiary(context.Background(), false, probeFunc(func(context.Context) error { return nil }), nil, nil, nil, nil, nil, NewDiaryHandler(service, true), time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func TestDiaryCreateAndCalendarHTTPContract(t *testing.T) {
	service := new(diaryTransportStub)
	router := diaryRouter(t, service)
	token := strings.Repeat("a", 43)
	entryID := "11111111-1111-4111-8111-111111111111"
	create := httptest.NewRequest(http.MethodPost, "/diary-entries", strings.NewReader(`{"id":"`+entryID+`","local_date":"2026-09-16","time_zone":"Asia/Shanghai","body":"今天的穿搭","media_ids":["33333333-3333-4333-8333-333333333333"]}`))
	create.Header.Set("Content-Type", "application/json")
	create.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, create)
	if response.Code != http.StatusCreated || strings.Contains(response.Body.String(), "owner_id") || service.input.Body == nil {
		t.Fatalf("diary create failed: status=%d body=%s", response.Code, response.Body.String())
	}
	calendar := httptest.NewRequest(http.MethodGet, "/calendar?month=2026-09", nil)
	calendar.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response = httptest.NewRecorder()
	router.ServeHTTP(response, calendar)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"diary_count":2`) || !strings.Contains(response.Body.String(), `"wear_event_count":1`) {
		t.Fatalf("calendar failed: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDiaryRejectsPrivateFactInjectionAndMapsConflicts(t *testing.T) {
	service := new(diaryTransportStub)
	router := diaryRouter(t, service)
	token := strings.Repeat("a", 43)
	request := httptest.NewRequest(http.MethodPost, "/diary-entries", strings.NewReader(`{"id":"11111111-1111-4111-8111-111111111111","local_date":"2026-09-16","time_zone":"Asia/Shanghai","body":"记录","media_ids":[],"owner_id":"other"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || service.input.Body != nil {
		t.Fatalf("owner injection reached diary service: status=%d body=%s", response.Code, response.Body.String())
	}
	service.err = domain.ErrDiaryConflict
	request = httptest.NewRequest(http.MethodDelete, "/diary-entries/11111111-1111-4111-8111-111111111111?expected_revision=1", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"CONFLICT"`) {
		t.Fatalf("conflict mapping failed: status=%d body=%s", response.Code, response.Body.String())
	}
}
