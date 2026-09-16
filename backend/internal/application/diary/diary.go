package diary

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
	"github.com/google/uuid"
)

type Authenticator interface {
	CurrentUser(context.Context, string) (domain.User, error)
}

type Repository interface {
	CreateDiaryEntry(context.Context, string, string, domain.DiaryEntryInput, time.Time) (domain.DiaryEntry, error)
	ListDiaryEntries(context.Context, string, int, *string, *string, *string) (domain.DiaryEntryPage, error)
	GetDiaryEntry(context.Context, string, string) (domain.DiaryEntry, error)
	UpdateDiaryEntry(context.Context, string, string, int, domain.DiaryEntryInput, time.Time) (domain.DiaryEntry, error)
	DiaryDeletionImpact(context.Context, string, string) (domain.DiaryDeletionImpact, error)
	DeleteDiaryEntry(context.Context, string, string, int, time.Time) error
	CalendarMonth(context.Context, string, string) (domain.CalendarMonth, error)
}

type Service struct {
	authenticator Authenticator
	repository    Repository
	now           func() time.Time
}

func NewService(authenticator Authenticator, repository Repository) (*Service, error) {
	if authenticator == nil || repository == nil {
		return nil, domain.ErrDiaryUnavailable
	}
	return &Service{authenticator: authenticator, repository: repository, now: time.Now}, nil
}

func (s *Service) Create(ctx context.Context, token, entryID string, input domain.DiaryEntryInput) (domain.DiaryEntry, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.DiaryEntry{}, err
	}
	normalized, date, location, ok := normalizeInput(entryID, input)
	if !ok || date.After(today(s.now().UTC(), location)) {
		return domain.DiaryEntry{}, domain.ErrInvalidDiaryInput
	}
	return s.repository.CreateDiaryEntry(ctx, user.ID, entryID, normalized, s.now().UTC())
}

func (s *Service) List(ctx context.Context, token string, limit int, afterID, dateFrom, dateTo *string) (domain.DiaryEntryPage, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.DiaryEntryPage{}, err
	}
	if limit < 1 || limit > 50 || afterID != nil && !validUUID(*afterID) {
		return domain.DiaryEntryPage{}, domain.ErrInvalidDiaryInput
	}
	var from, to time.Time
	if dateFrom != nil {
		if from, err = parseDate(*dateFrom); err != nil {
			return domain.DiaryEntryPage{}, domain.ErrInvalidDiaryInput
		}
	}
	if dateTo != nil {
		if to, err = parseDate(*dateTo); err != nil {
			return domain.DiaryEntryPage{}, domain.ErrInvalidDiaryInput
		}
	}
	if dateFrom != nil && dateTo != nil && (to.Before(from) || to.Sub(from) > 365*24*time.Hour) {
		return domain.DiaryEntryPage{}, domain.ErrInvalidDiaryInput
	}
	return s.repository.ListDiaryEntries(ctx, user.ID, limit, afterID, dateFrom, dateTo)
}

func (s *Service) Get(ctx context.Context, token, entryID string) (domain.DiaryEntry, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.DiaryEntry{}, err
	}
	if !validUUID(entryID) {
		return domain.DiaryEntry{}, domain.ErrInvalidDiaryInput
	}
	return s.repository.GetDiaryEntry(ctx, user.ID, entryID)
}

func (s *Service) Update(ctx context.Context, token, entryID string, expectedRevision int, input domain.DiaryEntryInput) (domain.DiaryEntry, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.DiaryEntry{}, err
	}
	normalized, date, location, ok := normalizeInput(entryID, input)
	if !ok || expectedRevision < 1 || date.After(today(s.now().UTC(), location)) {
		return domain.DiaryEntry{}, domain.ErrInvalidDiaryInput
	}
	return s.repository.UpdateDiaryEntry(ctx, user.ID, entryID, expectedRevision, normalized, s.now().UTC())
}

func (s *Service) DeletionImpact(ctx context.Context, token, entryID string) (domain.DiaryDeletionImpact, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.DiaryDeletionImpact{}, err
	}
	if !validUUID(entryID) {
		return domain.DiaryDeletionImpact{}, domain.ErrInvalidDiaryInput
	}
	return s.repository.DiaryDeletionImpact(ctx, user.ID, entryID)
}

func (s *Service) Delete(ctx context.Context, token, entryID string, expectedRevision int) error {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return err
	}
	if !validUUID(entryID) || expectedRevision < 1 {
		return domain.ErrInvalidDiaryInput
	}
	return s.repository.DeleteDiaryEntry(ctx, user.ID, entryID, expectedRevision, s.now().UTC())
}

func (s *Service) Calendar(ctx context.Context, token, month string) (domain.CalendarMonth, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.CalendarMonth{}, err
	}
	if _, err := time.Parse("2006-01", month); err != nil || len(month) != 7 {
		return domain.CalendarMonth{}, domain.ErrInvalidDiaryInput
	}
	return s.repository.CalendarMonth(ctx, user.ID, month)
}

func normalizeInput(entryID string, input domain.DiaryEntryInput) (domain.DiaryEntryInput, time.Time, *time.Location, bool) {
	if !validUUID(entryID) || len(input.TimeZone) < 1 || len(input.TimeZone) > 255 || len(input.MediaIDs) > 9 {
		return domain.DiaryEntryInput{}, time.Time{}, nil, false
	}
	date, err := parseDate(input.LocalDate)
	if err != nil {
		return domain.DiaryEntryInput{}, time.Time{}, nil, false
	}
	location, err := time.LoadLocation(input.TimeZone)
	if err != nil || input.TimeZone == "Local" || hasDisallowedControls(input.TimeZone, false) {
		return domain.DiaryEntryInput{}, time.Time{}, nil, false
	}
	title, ok := normalizeOptional(input.Title, 80, false)
	if !ok {
		return domain.DiaryEntryInput{}, time.Time{}, nil, false
	}
	body, ok := normalizeOptional(input.Body, 5000, true)
	if !ok {
		return domain.DiaryEntryInput{}, time.Time{}, nil, false
	}
	mood, ok := normalizeOptional(input.Mood, 40, false)
	if !ok {
		return domain.DiaryEntryInput{}, time.Time{}, nil, false
	}
	occasion, ok := normalizeOptional(input.Occasion, 40, false)
	if !ok {
		return domain.DiaryEntryInput{}, time.Time{}, nil, false
	}
	if body == nil && len(input.MediaIDs) == 0 {
		return domain.DiaryEntryInput{}, time.Time{}, nil, false
	}
	for _, reference := range []*string{input.PlanID, input.WearEventID} {
		if reference != nil && !validUUID(*reference) {
			return domain.DiaryEntryInput{}, time.Time{}, nil, false
		}
	}
	seen := make(map[string]bool, len(input.MediaIDs))
	for _, mediaID := range input.MediaIDs {
		if !validUUID(mediaID) || seen[mediaID] {
			return domain.DiaryEntryInput{}, time.Time{}, nil, false
		}
		seen[mediaID] = true
	}
	input.Title, input.Body, input.Mood, input.Occasion = title, body, mood, occasion
	input.MediaIDs = append([]string(nil), input.MediaIDs...)
	return input, date, location, true
}

func normalizeOptional(value *string, limit int, multiline bool) (*string, bool) {
	if value == nil {
		return nil, true
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil, true
	}
	if utf8.RuneCountInString(normalized) > limit || hasDisallowedControls(normalized, multiline) {
		return nil, false
	}
	return &normalized, true
}

func hasDisallowedControls(value string, multiline bool) bool {
	for _, character := range value {
		if unicode.IsControl(character) && !(multiline && (character == '\n' || character == '\r' || character == '\t')) {
			return true
		}
	}
	return false
}

func parseDate(value string) (time.Time, error) {
	if len(value) != 10 || value[4] != '-' || value[7] != '-' {
		return time.Time{}, domain.ErrInvalidDiaryInput
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil || date.Format("2006-01-02") != value {
		return time.Time{}, domain.ErrInvalidDiaryInput
	}
	return date, nil
}

func today(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.String() == strings.ToLower(value)
}
