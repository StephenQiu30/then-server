package service

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/StephenQiu30/then-server/backend/internal/model"
)

type WearEventRepository interface {
	CreateWearEvent(context.Context, string, string, model.WearEventInput, time.Time) (model.WearEvent, error)
	ListWearEvents(context.Context, string, int, *string, *string) (model.WearEventPage, error)
	GetWearEvent(context.Context, string, string) (model.WearEvent, error)
	UpdateWearEvent(context.Context, string, string, int, model.WearEventInput, time.Time) (model.WearEvent, error)
	DeleteWearEvent(context.Context, string, string, int, time.Time) error
}

type WearEventService struct {
	authenticator PrivacyAuthenticator
	repository    WearEventRepository
	now           func() time.Time
}

func NewWearEventService(authenticator PrivacyAuthenticator, repository WearEventRepository) (*WearEventService, error) {
	if authenticator == nil || repository == nil {
		return nil, model.ErrWearEventServiceUnavailable
	}
	return &WearEventService{authenticator: authenticator, repository: repository, now: time.Now}, nil
}

func (s *WearEventService) CreateWearEvent(ctx context.Context, token, eventID string, input model.WearEventInput) (model.WearEvent, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.WearEvent{}, err
	}
	normalized, date, location, ok := normalizeWearEventInput(eventID, input)
	if !ok {
		return model.WearEvent{}, model.ErrInvalidWearEventInput
	}
	now := s.now().UTC()
	if date.After(outfitToday(now, location)) {
		return model.WearEvent{}, model.ErrInvalidWearEventInput
	}
	return s.repository.CreateWearEvent(ctx, user.ID, eventID, normalized, now)
}

func (s *WearEventService) ListWearEvents(ctx context.Context, token string, limit int, afterID, localDate *string) (model.WearEventPage, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.WearEventPage{}, err
	}
	if limit < 1 || limit > 50 || afterID != nil && !validUUID(*afterID) {
		return model.WearEventPage{}, model.ErrInvalidWearEventInput
	}
	if localDate != nil {
		if _, ok := parseOutfitLocalDate(*localDate); !ok {
			return model.WearEventPage{}, model.ErrInvalidWearEventInput
		}
	}
	return s.repository.ListWearEvents(ctx, user.ID, limit, afterID, localDate)
}

func (s *WearEventService) GetWearEvent(ctx context.Context, token, eventID string) (model.WearEvent, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.WearEvent{}, err
	}
	if !validUUID(eventID) {
		return model.WearEvent{}, model.ErrInvalidWearEventInput
	}
	return s.repository.GetWearEvent(ctx, user.ID, eventID)
}

func (s *WearEventService) UpdateWearEvent(ctx context.Context, token, eventID string, expectedRevision int, input model.WearEventInput) (model.WearEvent, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.WearEvent{}, err
	}
	normalized, date, location, ok := normalizeWearEventInput(eventID, input)
	if !ok || expectedRevision < 1 || date.After(outfitToday(s.now().UTC(), location)) {
		return model.WearEvent{}, model.ErrInvalidWearEventInput
	}
	return s.repository.UpdateWearEvent(ctx, user.ID, eventID, expectedRevision, normalized, s.now().UTC())
}

func (s *WearEventService) DeleteWearEvent(ctx context.Context, token, eventID string, expectedRevision int) error {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return err
	}
	if !validUUID(eventID) || expectedRevision < 1 {
		return model.ErrInvalidWearEventInput
	}
	return s.repository.DeleteWearEvent(ctx, user.ID, eventID, expectedRevision, s.now().UTC())
}

func normalizeWearEventInput(eventID string, input model.WearEventInput) (model.WearEventInput, time.Time, *time.Location, bool) {
	if !validUUID(eventID) || len(input.Items) < 1 || len(input.Items) > 20 || len(input.TimeZone) < 1 || len(input.TimeZone) > 255 || !validWearCompleteness(input.Completeness) || !validWearSource(input.SourceKind) {
		return model.WearEventInput{}, time.Time{}, nil, false
	}
	date, ok := parseOutfitLocalDate(input.LocalDate)
	if !ok {
		return model.WearEventInput{}, time.Time{}, nil, false
	}
	location, err := time.LoadLocation(input.TimeZone)
	if err != nil || input.TimeZone == "Local" {
		return model.WearEventInput{}, time.Time{}, nil, false
	}
	for _, character := range input.TimeZone {
		if unicode.IsControl(character) {
			return model.WearEventInput{}, time.Time{}, nil, false
		}
	}
	selected := make(map[string]bool, len(input.Items))
	for _, item := range input.Items {
		if !validUUID(item.ItemID) || item.Revision < 1 || selected[item.ItemID] {
			return model.WearEventInput{}, time.Time{}, nil, false
		}
		selected[item.ItemID] = true
	}
	if !validWearItemSubset(input.LaundryItemIDs, selected) || !validWearItemSubset(input.ConfirmedUnavailableIDs, selected) {
		return model.WearEventInput{}, time.Time{}, nil, false
	}
	if (input.SourcePlanID == nil) != (input.SourcePlanRevision == nil) || input.SourcePlanRevision != nil && *input.SourcePlanRevision < 1 || input.SourceKind == model.WearEventUnplanned && input.SourcePlanID != nil || input.SourceKind != model.WearEventUnplanned && input.SourcePlanID == nil {
		return model.WearEventInput{}, time.Time{}, nil, false
	}
	if input.SourcePlanID != nil && !validUUID(*input.SourcePlanID) {
		return model.WearEventInput{}, time.Time{}, nil, false
	}
	seenCandidates := map[string]bool{}
	for _, candidate := range input.DuplicateConfirmations {
		if !validUUID(candidate.ID) || candidate.ID == eventID || candidate.Revision < 1 || seenCandidates[candidate.ID] {
			return model.WearEventInput{}, time.Time{}, nil, false
		}
		seenCandidates[candidate.ID] = true
	}
	if input.ContextSummary != nil {
		value := strings.TrimSpace(*input.ContextSummary)
		if value == "" {
			input.ContextSummary = nil
		} else {
			if utf8.RuneCountInString(value) > 120 {
				return model.WearEventInput{}, time.Time{}, nil, false
			}
			for _, character := range value {
				if unicode.IsControl(character) {
					return model.WearEventInput{}, time.Time{}, nil, false
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

func validWearCompleteness(value model.WearEventCompleteness) bool {
	return value == model.WearEventPartial || value == model.WearEventComplete
}

func validWearSource(value model.WearEventSourceKind) bool {
	switch value {
	case model.WearEventFollowedPlan, model.WearEventChangedPlan, model.WearEventDifferentOutfit, model.WearEventUnplanned:
		return true
	default:
		return false
	}
}
