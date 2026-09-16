package wearevent

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
	"github.com/google/uuid"
)

type WearEventRepository interface {
	CreateWearEvent(context.Context, string, string, domain.WearEventInput, time.Time) (domain.WearEvent, error)
	ListWearEvents(context.Context, string, int, *string, *string) (domain.WearEventPage, error)
	GetWearEvent(context.Context, string, string) (domain.WearEvent, error)
	UpdateWearEvent(context.Context, string, string, int, domain.WearEventInput, time.Time) (domain.WearEvent, error)
	DeleteWearEvent(context.Context, string, string, int, time.Time) error
}

type WearEventService struct {
	authenticator PrivacyAuthenticator
	repository    WearEventRepository
	now           func() time.Time
}

func NewWearEventService(authenticator PrivacyAuthenticator, repository WearEventRepository) (*WearEventService, error) {
	if authenticator == nil || repository == nil {
		return nil, domain.ErrWearEventServiceUnavailable
	}
	return &WearEventService{authenticator: authenticator, repository: repository, now: time.Now}, nil
}

func (s *WearEventService) CreateWearEvent(ctx context.Context, token, eventID string, input domain.WearEventInput) (domain.WearEvent, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.WearEvent{}, err
	}
	normalized, date, location, ok := normalizeWearEventInput(eventID, input)
	if !ok {
		return domain.WearEvent{}, domain.ErrInvalidWearEventInput
	}
	now := s.now().UTC()
	if date.After(outfitToday(now, location)) {
		return domain.WearEvent{}, domain.ErrInvalidWearEventInput
	}
	return s.repository.CreateWearEvent(ctx, user.ID, eventID, normalized, now)
}

func (s *WearEventService) ListWearEvents(ctx context.Context, token string, limit int, afterID, localDate *string) (domain.WearEventPage, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.WearEventPage{}, err
	}
	if limit < 1 || limit > 50 || afterID != nil && !validUUID(*afterID) {
		return domain.WearEventPage{}, domain.ErrInvalidWearEventInput
	}
	if localDate != nil {
		if _, ok := parseOutfitLocalDate(*localDate); !ok {
			return domain.WearEventPage{}, domain.ErrInvalidWearEventInput
		}
	}
	return s.repository.ListWearEvents(ctx, user.ID, limit, afterID, localDate)
}

func (s *WearEventService) GetWearEvent(ctx context.Context, token, eventID string) (domain.WearEvent, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.WearEvent{}, err
	}
	if !validUUID(eventID) {
		return domain.WearEvent{}, domain.ErrInvalidWearEventInput
	}
	return s.repository.GetWearEvent(ctx, user.ID, eventID)
}

func (s *WearEventService) UpdateWearEvent(ctx context.Context, token, eventID string, expectedRevision int, input domain.WearEventInput) (domain.WearEvent, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.WearEvent{}, err
	}
	normalized, date, location, ok := normalizeWearEventInput(eventID, input)
	if !ok || expectedRevision < 1 || date.After(outfitToday(s.now().UTC(), location)) {
		return domain.WearEvent{}, domain.ErrInvalidWearEventInput
	}
	return s.repository.UpdateWearEvent(ctx, user.ID, eventID, expectedRevision, normalized, s.now().UTC())
}

func (s *WearEventService) DeleteWearEvent(ctx context.Context, token, eventID string, expectedRevision int) error {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return err
	}
	if !validUUID(eventID) || expectedRevision < 1 {
		return domain.ErrInvalidWearEventInput
	}
	return s.repository.DeleteWearEvent(ctx, user.ID, eventID, expectedRevision, s.now().UTC())
}

func normalizeWearEventInput(eventID string, input domain.WearEventInput) (domain.WearEventInput, time.Time, *time.Location, bool) {
	if !validUUID(eventID) || len(input.Items) < 1 || len(input.Items) > 20 || len(input.TimeZone) < 1 || len(input.TimeZone) > 255 || !validWearCompleteness(input.Completeness) || !validWearSource(input.SourceKind) {
		return domain.WearEventInput{}, time.Time{}, nil, false
	}
	date, ok := parseOutfitLocalDate(input.LocalDate)
	if !ok {
		return domain.WearEventInput{}, time.Time{}, nil, false
	}
	location, err := time.LoadLocation(input.TimeZone)
	if err != nil || input.TimeZone == "Local" {
		return domain.WearEventInput{}, time.Time{}, nil, false
	}
	for _, character := range input.TimeZone {
		if unicode.IsControl(character) {
			return domain.WearEventInput{}, time.Time{}, nil, false
		}
	}
	selected := make(map[string]bool, len(input.Items))
	for _, item := range input.Items {
		if !validUUID(item.ItemID) || item.Revision < 1 || selected[item.ItemID] {
			return domain.WearEventInput{}, time.Time{}, nil, false
		}
		selected[item.ItemID] = true
	}
	if !validWearItemSubset(input.LaundryItemIDs, selected) || !validWearItemSubset(input.ConfirmedUnavailableIDs, selected) {
		return domain.WearEventInput{}, time.Time{}, nil, false
	}
	if (input.SourcePlanID == nil) != (input.SourcePlanRevision == nil) || input.SourcePlanRevision != nil && *input.SourcePlanRevision < 1 || input.SourceKind == domain.WearEventUnplanned && input.SourcePlanID != nil || input.SourceKind != domain.WearEventUnplanned && input.SourcePlanID == nil {
		return domain.WearEventInput{}, time.Time{}, nil, false
	}
	if input.SourcePlanID != nil && !validUUID(*input.SourcePlanID) {
		return domain.WearEventInput{}, time.Time{}, nil, false
	}
	seenCandidates := map[string]bool{}
	for _, candidate := range input.DuplicateConfirmations {
		if !validUUID(candidate.ID) || candidate.ID == eventID || candidate.Revision < 1 || seenCandidates[candidate.ID] {
			return domain.WearEventInput{}, time.Time{}, nil, false
		}
		seenCandidates[candidate.ID] = true
	}
	if input.ContextSummary != nil {
		value := strings.TrimSpace(*input.ContextSummary)
		if value == "" {
			input.ContextSummary = nil
		} else {
			if utf8.RuneCountInString(value) > 120 {
				return domain.WearEventInput{}, time.Time{}, nil, false
			}
			for _, character := range value {
				if unicode.IsControl(character) {
					return domain.WearEventInput{}, time.Time{}, nil, false
				}
			}
			input.ContextSummary = &value
		}
	}
	return input, date, location, true
}

func validWearItemSubset(values []string, selected map[string]bool) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if !selected[value] || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

func validWearCompleteness(value domain.WearEventCompleteness) bool {
	return value == domain.WearEventPartial || value == domain.WearEventComplete
}

func validWearSource(value domain.WearEventSourceKind) bool {
	switch value {
	case domain.WearEventFollowedPlan, domain.WearEventChangedPlan, domain.WearEventDifferentOutfit, domain.WearEventUnplanned:
		return true
	default:
		return false
	}
}

func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.String() == strings.ToLower(value)
}

func parseOutfitLocalDate(value string) (time.Time, bool) {
	if len(value) != 10 || value[4] != '-' || value[7] != '-' {
		return time.Time{}, false
	}
	date, err := time.Parse("2006-01-02", value)
	return date, err == nil && date.Format("2006-01-02") == value
}

func outfitToday(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}
