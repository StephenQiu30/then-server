package wearevent

import (
	"context"
	"errors"
	"testing"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	outfitplanapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitplan"
)

type authenticatorStub struct {
	user accountapp.User
	err  error
}

func (s authenticatorStub) CurrentUser(context.Context, string) (accountapp.User, error) {
	return s.user, s.err
}

type wearEventRepositoryStub struct {
	ownerID  string
	eventID  string
	input    WearEventInput
	expected int
	err      error
}

func (s *wearEventRepositoryStub) CreateWearEvent(_ context.Context, ownerID, eventID string, input WearEventInput, _ time.Time) (WearEvent, error) {
	s.ownerID, s.eventID, s.input = ownerID, eventID, input
	return WearEvent{ID: eventID, OwnerID: ownerID, LocalDate: input.LocalDate, TimeZone: input.TimeZone, Revision: 1}, s.err
}
func (s *wearEventRepositoryStub) ListWearEvents(_ context.Context, ownerID string, _ int, _, _ *string) (WearEventPage, error) {
	s.ownerID = ownerID
	return WearEventPage{}, s.err
}
func (s *wearEventRepositoryStub) GetWearEvent(_ context.Context, ownerID, eventID string) (WearEvent, error) {
	s.ownerID, s.eventID = ownerID, eventID
	return WearEvent{ID: eventID}, s.err
}
func (s *wearEventRepositoryStub) UpdateWearEvent(_ context.Context, ownerID, eventID string, expected int, input WearEventInput, _ time.Time) (WearEvent, error) {
	s.ownerID, s.eventID, s.expected, s.input = ownerID, eventID, expected, input
	return WearEvent{ID: eventID, Revision: expected + 1}, s.err
}
func (s *wearEventRepositoryStub) DeleteWearEvent(_ context.Context, ownerID, eventID string, expected int, _ time.Time) error {
	s.ownerID, s.eventID, s.expected = ownerID, eventID, expected
	return s.err
}

func validWearEventInput() WearEventInput {
	summary := "  Office day  "
	return WearEventInput{LocalDate: "2026-09-16", TimeZone: "Asia/Shanghai", Completeness: WearEventComplete, ContextSummary: &summary, Items: []outfitplanapp.OutfitSelection{{ItemID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Revision: 2}}, SourceKind: WearEventUnplanned}
}

func newWearEventServiceForTest(t *testing.T, repository *wearEventRepositoryStub) *WearEventService {
	t.Helper()
	result, err := NewWearEventService(authenticatorStub{user: accountapp.User{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11"}}, repository)
	if err != nil {
		t.Fatal(err)
	}
	result.now = func() time.Time { return time.Date(2026, 9, 16, 2, 0, 0, 0, time.UTC) }
	return result
}

func TestWearEventNormalizesAndUsesAuthenticatedOwner(t *testing.T) {
	repository := new(wearEventRepositoryStub)
	eventID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400a13"
	event, err := newWearEventServiceForTest(t, repository).CreateWearEvent(context.Background(), "session", eventID, validWearEventInput())
	if err != nil || event.ID != eventID || repository.ownerID == "" || repository.input.ContextSummary == nil || *repository.input.ContextSummary != "Office day" {
		t.Fatal("wear event create did not normalize or use authenticated owner")
	}
}

func TestWearEventRejectsInvalidFactsBeforeRepository(t *testing.T) {
	eventID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400a13"
	tests := map[string]func(*WearEventInput){
		"future":         func(value *WearEventInput) { value.LocalDate = "2026-09-17" },
		"duplicate item": func(value *WearEventInput) { value.Items = append(value.Items, value.Items[0]) },
		"bad timezone":   func(value *WearEventInput) { value.TimeZone = "Local" },
		"laundry outside selection": func(value *WearEventInput) {
			value.LaundryItemIDs = []string{"018f1f74-a2d0-7c6d-9c17-4a0ea240099"}
		},
		"planned without source": func(value *WearEventInput) { value.SourceKind = WearEventFollowedPlan },
		"unplanned with source": func(value *WearEventInput) {
			id, revision := "018f1f74-a2d0-7c6d-9c17-4a0ea240088", 1
			value.SourcePlanID, value.SourcePlanRevision = &id, &revision
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			repository := new(wearEventRepositoryStub)
			input := validWearEventInput()
			mutate(&input)
			_, err := newWearEventServiceForTest(t, repository).CreateWearEvent(context.Background(), "session", eventID, input)
			if !errors.Is(err, ErrInvalidWearEventInput) || repository.ownerID != "" {
				t.Fatal("invalid wear event reached repository")
			}
		})
	}
}

func TestWearEventCommandsValidateRevisionAndDate(t *testing.T) {
	repository := new(wearEventRepositoryStub)
	service := newWearEventServiceForTest(t, repository)
	eventID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400a13"
	if _, err := service.UpdateWearEvent(context.Background(), "session", eventID, 2, validWearEventInput()); err != nil || repository.expected != 2 {
		t.Fatal("valid wear event update did not reach repository")
	}
	if err := service.DeleteWearEvent(context.Background(), "session", eventID, 3); err != nil || repository.expected != 3 {
		t.Fatal("valid wear event delete did not reach repository")
	}
	if _, err := service.ListWearEvents(context.Background(), "session", 0, nil, nil); !errors.Is(err, ErrInvalidWearEventInput) {
		t.Fatal("invalid list limit was accepted")
	}
	if err := service.DeleteWearEvent(context.Background(), "session", eventID, 0); !errors.Is(err, ErrInvalidWearEventInput) {
		t.Fatal("zero delete revision was accepted")
	}
}
