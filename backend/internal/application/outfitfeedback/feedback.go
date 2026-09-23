package outfitfeedback

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	"github.com/google/uuid"
)

var (
	ErrInvalid  = errors.New("invalid feedback input")
	ErrNotFound = errors.New("feedback not found")
	ErrConflict = errors.New("feedback conflict")
)

type Input struct {
	ThermalComfort  *string
	ActivityComfort *string
	OccasionFit     *string
	RepeatIntent    *string
	IssueTags       []string
	Note            *string
}

type Feedback struct {
	ID          string
	WearEventID string
	Input       Input
	Revision    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ItemUse struct {
	ItemID string
	Count  int
}

type Statistics struct {
	From       string
	To         string
	WearDays   int
	WearEvents int
	ItemUses   []ItemUse
}

type Repository interface {
	Get(context.Context, string, string) (Feedback, error)
	Save(context.Context, string, string, string, string, *int, Input, time.Time) (Feedback, error)
	Delete(context.Context, string, string, string, string, int) error
	Statistics(context.Context, string, string, string) (Statistics, error)
}

type Authenticator interface {
	CurrentUser(context.Context, string) (accountapp.User, error)
}

type Service struct {
	auth Authenticator
	repo Repository
	now  func() time.Time
}

func NewService(auth Authenticator, repo Repository) (*Service, error) {
	if auth == nil || repo == nil {
		return nil, errors.New("invalid feedback dependencies")
	}
	return &Service{auth: auth, repo: repo, now: time.Now}, nil
}

func (s *Service) Get(ctx context.Context, token, eventID string) (Feedback, error) {
	u, err := s.auth.CurrentUser(ctx, token)
	if err != nil {
		return Feedback{}, err
	}
	if _, err := uuid.Parse(eventID); err != nil {
		return Feedback{}, ErrInvalid
	}
	return s.repo.Get(ctx, u.ID, eventID)
}

func (s *Service) Save(ctx context.Context, token, eventID, feedbackID, mutationID string, expected *int, input Input) (Feedback, error) {
	u, err := s.auth.CurrentUser(ctx, token)
	if err != nil {
		return Feedback{}, err
	}
	for _, id := range []string{eventID, feedbackID, mutationID} {
		if _, err := uuid.Parse(id); err != nil {
			return Feedback{}, ErrInvalid
		}
	}
	if expected != nil && *expected < 1 {
		return Feedback{}, ErrInvalid
	}
	input, ok := normalize(input)
	if !ok {
		return Feedback{}, ErrInvalid
	}
	return s.repo.Save(ctx, u.ID, eventID, feedbackID, mutationID, expected, input, s.now().UTC())
}

func (s *Service) Delete(ctx context.Context, token, eventID, feedbackID, mutationID string, expected int) error {
	u, err := s.auth.CurrentUser(ctx, token)
	if err != nil {
		return err
	}
	for _, id := range []string{eventID, feedbackID, mutationID} {
		if _, err := uuid.Parse(id); err != nil {
			return ErrInvalid
		}
	}
	if expected < 1 {
		return ErrInvalid
	}
	return s.repo.Delete(ctx, u.ID, eventID, feedbackID, mutationID, expected)
}

func (s *Service) Statistics(ctx context.Context, token, from, to string) (Statistics, error) {
	u, err := s.auth.CurrentUser(ctx, token)
	if err != nil {
		return Statistics{}, err
	}
	a, ea := time.Parse("2006-01-02", from)
	b, eb := time.Parse("2006-01-02", to)
	if ea != nil || eb != nil || a.Format("2006-01-02") != from || b.Format("2006-01-02") != to || b.Before(a) || b.Sub(a) > 365*24*time.Hour {
		return Statistics{}, ErrInvalid
	}
	return s.repo.Statistics(ctx, u.ID, from, to)
}

func normalize(input Input) (Input, bool) {
	valid := func(value *string, options ...string) bool {
		if value == nil {
			return true
		}
		for _, option := range options {
			if *value == option {
				return true
			}
		}
		return false
	}
	if !valid(input.ThermalComfort, "cold", "comfortable", "hot") || !valid(input.ActivityComfort, "uncomfortable", "okay", "comfortable") || !valid(input.OccasionFit, "tooCasual", "right", "tooFormal") || !valid(input.RepeatIntent, "yes", "unsure", "no") || len(input.IssueTags) > 5 {
		return Input{}, false
	}
	allowed := map[string]bool{"shoeDiscomfort": true, "awkwardLayering": true, "rainUnsuitable": true, "insufficientPockets": true, "maintenanceNeeded": true}
	seen := map[string]bool{}
	for _, tag := range input.IssueTags {
		if !allowed[tag] || seen[tag] {
			return Input{}, false
		}
		seen[tag] = true
	}
	sort.Strings(input.IssueTags)
	if input.IssueTags == nil {
		input.IssueTags = []string{}
	}
	if input.Note != nil {
		note := strings.TrimSpace(*input.Note)
		if utf8.RuneCountInString(note) > 240 {
			return Input{}, false
		}
		for _, c := range note {
			if unicode.IsControl(c) {
				return Input{}, false
			}
		}
		if note == "" {
			input.Note = nil
		} else {
			input.Note = &note
		}
	}
	if input.ThermalComfort == nil && input.ActivityComfort == nil && input.OccasionFit == nil && input.RepeatIntent == nil && len(input.IssueTags) == 0 && input.Note == nil {
		return Input{}, false
	}
	return input, true
}
