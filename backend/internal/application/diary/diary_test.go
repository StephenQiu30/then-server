package diary

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
)

const testEntryID = "11111111-1111-4111-8111-111111111111"

type fakeAuthenticator struct{}

func (fakeAuthenticator) CurrentUser(context.Context, string) (domain.User, error) {
	return domain.User{ID: "22222222-2222-4222-8222-222222222222"}, nil
}

type fakeRepository struct {
	created domain.DiaryEntryInput
}

func (r *fakeRepository) CreateDiaryEntry(_ context.Context, _ string, id string, input domain.DiaryEntryInput, at time.Time) (domain.DiaryEntry, error) {
	r.created = input
	return domain.DiaryEntry{ID: id, LocalDate: input.LocalDate, TimeZone: input.TimeZone, Body: input.Body, MediaIDs: input.MediaIDs, Revision: 1, CreatedAt: at, UpdatedAt: at}, nil
}
func (*fakeRepository) ListDiaryEntries(context.Context, string, int, *string, *string, *string) (domain.DiaryEntryPage, error) {
	return domain.DiaryEntryPage{}, nil
}
func (*fakeRepository) GetDiaryEntry(context.Context, string, string) (domain.DiaryEntry, error) {
	return domain.DiaryEntry{}, nil
}
func (*fakeRepository) UpdateDiaryEntry(context.Context, string, string, int, domain.DiaryEntryInput, time.Time) (domain.DiaryEntry, error) {
	return domain.DiaryEntry{}, nil
}
func (*fakeRepository) DiaryDeletionImpact(context.Context, string, string) (domain.DiaryDeletionImpact, error) {
	return domain.DiaryDeletionImpact{}, nil
}
func (*fakeRepository) DeleteDiaryEntry(context.Context, string, string, int, time.Time) error {
	return nil
}
func (*fakeRepository) CalendarMonth(context.Context, string, string) (domain.CalendarMonth, error) {
	return domain.CalendarMonth{}, nil
}

func TestCreateNormalizesPrivateDiaryAndPreservesMediaOrder(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(fakeAuthenticator{}, repository)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 9, 16, 2, 0, 0, 0, time.UTC) }
	body := "  今天的穿搭\n很轻松  "
	empty := "  "
	media := []string{"33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444"}
	entry, err := service.Create(context.Background(), "token", testEntryID, domain.DiaryEntryInput{LocalDate: "2026-09-16", TimeZone: "Asia/Shanghai", Title: &empty, Body: &body, MediaIDs: media})
	if err != nil {
		t.Fatal(err)
	}
	if repository.created.Title != nil || repository.created.Body == nil || *repository.created.Body != "今天的穿搭\n很轻松" {
		t.Fatalf("unexpected normalized input: %#v", repository.created)
	}
	if len(entry.MediaIDs) != 2 || entry.MediaIDs[0] != media[0] || entry.MediaIDs[1] != media[1] {
		t.Fatalf("media order changed: %#v", entry.MediaIDs)
	}
}

func TestCreateRejectsFutureEmptyAndDuplicateMedia(t *testing.T) {
	service, err := NewService(fakeAuthenticator{}, &fakeRepository{})
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 9, 16, 2, 0, 0, 0, time.UTC) }
	body := "记录"
	mediaID := "33333333-3333-4333-8333-333333333333"
	cases := []domain.DiaryEntryInput{
		{LocalDate: "2026-09-17", TimeZone: "Asia/Shanghai", Body: &body},
		{LocalDate: "2026-09-16", TimeZone: "Asia/Shanghai"},
		{LocalDate: "2026-09-16", TimeZone: "Asia/Shanghai", MediaIDs: []string{mediaID, mediaID}},
	}
	for _, input := range cases {
		if _, err := service.Create(context.Background(), "token", testEntryID, input); !errors.Is(err, domain.ErrInvalidDiaryInput) {
			t.Fatalf("expected invalid input for %#v, got %v", input, err)
		}
	}
}

func TestListRejectsMoreThan366Days(t *testing.T) {
	service, err := NewService(fakeAuthenticator{}, &fakeRepository{})
	if err != nil {
		t.Fatal(err)
	}
	from, to := "2025-01-01", "2026-01-02"
	if _, err := service.List(context.Background(), "token", 20, nil, &from, &to); !errors.Is(err, domain.ErrInvalidDiaryInput) {
		t.Fatalf("expected range rejection, got %v", err)
	}
}
