package outfitplan

import (
	"context"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode"
	"unicode/utf8"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
	"github.com/google/uuid"
)

type OutfitPlanRepository interface {
	CreateOutfitPlan(context.Context, string, string, domain.OutfitPlanInput, time.Time) (domain.OutfitPlan, error)
	ListOutfitPlans(context.Context, string, int, *string, *string) (domain.OutfitPlanPage, error)
	GetOutfitPlan(context.Context, string, string) (domain.OutfitPlan, error)
	UpdateOutfitPlan(context.Context, string, string, int, domain.OutfitPlanInput, time.Time) (domain.OutfitPlan, error)
	CancelOutfitPlan(context.Context, string, string, int, time.Time) (domain.OutfitPlan, error)
	MarkOutfitPlanNotWorn(context.Context, string, string, int, time.Time) (domain.OutfitPlan, error)
	RestoreOutfitPlan(context.Context, string, string, int, time.Time) (domain.OutfitPlan, error)
	DeleteOutfitPlan(context.Context, string, string, int, time.Time) error
}

type OutfitPlanService struct {
	authenticator PrivacyAuthenticator
	repository    OutfitPlanRepository
	now           func() time.Time
}

func NewOutfitPlanService(authenticator PrivacyAuthenticator, repository OutfitPlanRepository) (*OutfitPlanService, error) {
	if authenticator == nil || repository == nil {
		return nil, domain.ErrOutfitPlanUnavailable
	}
	return &OutfitPlanService{authenticator: authenticator, repository: repository, now: time.Now}, nil
}

func (s *OutfitPlanService) CreateOutfitPlan(ctx context.Context, token, planID string, input domain.OutfitPlanInput) (domain.OutfitPlan, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.OutfitPlan{}, err
	}
	normalized, localDate, location, ok := normalizeOutfitPlanInput(planID, input)
	if !ok {
		return domain.OutfitPlan{}, domain.ErrInvalidOutfitPlanInput
	}
	now := s.now().UTC()
	if localDate.Before(outfitToday(now, location)) {
		return domain.OutfitPlan{}, domain.ErrInvalidOutfitPlanInput
	}
	return s.repository.CreateOutfitPlan(ctx, user.ID, planID, normalized, now)
}

func (s *OutfitPlanService) ListOutfitPlans(ctx context.Context, token string, limit int, afterID, localDate *string) (domain.OutfitPlanPage, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.OutfitPlanPage{}, err
	}
	if limit < 1 || limit > 50 || afterID != nil && !validUUID(*afterID) {
		return domain.OutfitPlanPage{}, domain.ErrInvalidOutfitPlanInput
	}
	if localDate != nil {
		if _, ok := parseOutfitLocalDate(*localDate); !ok {
			return domain.OutfitPlanPage{}, domain.ErrInvalidOutfitPlanInput
		}
	}
	return s.repository.ListOutfitPlans(ctx, user.ID, limit, afterID, localDate)
}

func (s *OutfitPlanService) GetOutfitPlan(ctx context.Context, token, planID string) (domain.OutfitPlan, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.OutfitPlan{}, err
	}
	if !validUUID(planID) {
		return domain.OutfitPlan{}, domain.ErrInvalidOutfitPlanInput
	}
	return s.repository.GetOutfitPlan(ctx, user.ID, planID)
}

func (s *OutfitPlanService) UpdateOutfitPlan(ctx context.Context, token, planID string, expectedRevision int, input domain.OutfitPlanInput) (domain.OutfitPlan, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.OutfitPlan{}, err
	}
	normalized, _, _, ok := normalizeOutfitPlanInput(planID, input)
	if !ok || expectedRevision < 1 {
		return domain.OutfitPlan{}, domain.ErrInvalidOutfitPlanInput
	}
	return s.repository.UpdateOutfitPlan(ctx, user.ID, planID, expectedRevision, normalized, s.now().UTC())
}

func (s *OutfitPlanService) CancelOutfitPlan(ctx context.Context, token, planID string, expectedRevision int) (domain.OutfitPlan, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.OutfitPlan{}, err
	}
	if !validUUID(planID) || expectedRevision < 1 {
		return domain.OutfitPlan{}, domain.ErrInvalidOutfitPlanInput
	}
	return s.repository.CancelOutfitPlan(ctx, user.ID, planID, expectedRevision, s.now().UTC())
}

func (s *OutfitPlanService) MarkOutfitPlanNotWorn(ctx context.Context, token, planID string, expectedRevision int) (domain.OutfitPlan, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.OutfitPlan{}, err
	}
	if !validUUID(planID) || expectedRevision < 1 {
		return domain.OutfitPlan{}, domain.ErrInvalidOutfitPlanInput
	}
	return s.repository.MarkOutfitPlanNotWorn(ctx, user.ID, planID, expectedRevision, s.now().UTC())
}

func (s *OutfitPlanService) RestoreOutfitPlan(ctx context.Context, token, planID string, expectedRevision int) (domain.OutfitPlan, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.OutfitPlan{}, err
	}
	if !validUUID(planID) || expectedRevision < 1 {
		return domain.OutfitPlan{}, domain.ErrInvalidOutfitPlanInput
	}
	return s.repository.RestoreOutfitPlan(ctx, user.ID, planID, expectedRevision, s.now().UTC())
}

func (s *OutfitPlanService) DeleteOutfitPlan(ctx context.Context, token, planID string, expectedRevision int) error {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return err
	}
	if !validUUID(planID) || expectedRevision < 1 {
		return domain.ErrInvalidOutfitPlanInput
	}
	return s.repository.DeleteOutfitPlan(ctx, user.ID, planID, expectedRevision, s.now().UTC())
}

func normalizeOutfitPlanInput(planID string, input domain.OutfitPlanInput) (domain.OutfitPlanInput, time.Time, *time.Location, bool) {
	if !validUUID(planID) || len(input.Items) < 1 || len(input.Items) > 20 || len(input.TimeZone) < 1 || len(input.TimeZone) > 255 {
		return domain.OutfitPlanInput{}, time.Time{}, nil, false
	}
	date, ok := parseOutfitLocalDate(input.LocalDate)
	if !ok {
		return domain.OutfitPlanInput{}, time.Time{}, nil, false
	}
	location, err := time.LoadLocation(input.TimeZone)
	if err != nil || input.TimeZone == "Local" {
		return domain.OutfitPlanInput{}, time.Time{}, nil, false
	}
	for _, character := range input.TimeZone {
		if unicode.IsControl(character) {
			return domain.OutfitPlanInput{}, time.Time{}, nil, false
		}
	}
	seen := make(map[string]bool, len(input.Items))
	for _, item := range input.Items {
		if !validUUID(item.ItemID) || item.Revision < 1 || seen[item.ItemID] {
			return domain.OutfitPlanInput{}, time.Time{}, nil, false
		}
		seen[item.ItemID] = true
	}
	confirmed := make(map[string]bool, len(input.ConfirmedUnavailableIDs))
	for _, itemID := range input.ConfirmedUnavailableIDs {
		if !seen[itemID] || confirmed[itemID] {
			return domain.OutfitPlanInput{}, time.Time{}, nil, false
		}
		confirmed[itemID] = true
	}
	var summary *string
	if input.ContextSummary != nil {
		value := strings.TrimSpace(*input.ContextSummary)
		if value != "" {
			if utf8.RuneCountInString(value) > 120 {
				return domain.OutfitPlanInput{}, time.Time{}, nil, false
			}
			for _, character := range value {
				if unicode.IsControl(character) {
					return domain.OutfitPlanInput{}, time.Time{}, nil, false
				}
			}
			summary = &value
		}
	}
	input.ContextSummary = summary
	return input, date, location, true
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

func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.String() == strings.ToLower(value)
}
