package service

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/StephenQiu30/then-server/backend/internal/model"
	"github.com/google/uuid"
)

type WardrobeRepository interface {
	CreateWardrobeItem(context.Context, model.WardrobeItem) (model.WardrobeItem, error)
	ListWardrobeItems(context.Context, string, int, *string) (model.WardrobePage, error)
	GetWardrobeItem(context.Context, string, string) (model.WardrobeItem, error)
	UpdateWardrobeItem(context.Context, string, string, int, model.UpdateWardrobeItemInput, time.Time) (model.WardrobeItem, error)
	DeleteWardrobeItem(context.Context, string, string, int) error
}

type WardrobeService struct {
	authenticator PrivacyAuthenticator
	repository    WardrobeRepository
	now           func() time.Time
}

func NewWardrobeService(authenticator PrivacyAuthenticator, repository WardrobeRepository) (*WardrobeService, error) {
	if authenticator == nil || repository == nil {
		return nil, model.ErrWardrobeUnavailable
	}
	return &WardrobeService{authenticator: authenticator, repository: repository, now: time.Now}, nil
}

func (s *WardrobeService) CreateWardrobeItem(ctx context.Context, token string, input model.CreateWardrobeItemInput) (model.WardrobeItem, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.WardrobeItem{}, err
	}
	name, ok := validWardrobeFields(input.ID, input.Name, input.Category, input.Availability)
	if !ok || !validWardrobeSource(input.Source) {
		return model.WardrobeItem{}, model.ErrInvalidWardrobeInput
	}
	now := s.now().UTC()
	return s.repository.CreateWardrobeItem(ctx, model.WardrobeItem{
		ID: input.ID, OwnerID: user.ID, Name: name, Category: input.Category,
		Availability: input.Availability, Source: input.Source, Revision: 1, CreatedAt: now, UpdatedAt: now,
	})
}

func (s *WardrobeService) ListWardrobeItems(ctx context.Context, token string, limit int, afterID *string) (model.WardrobePage, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.WardrobePage{}, err
	}
	if limit < 1 || limit > 100 || (afterID != nil && !validUUID(*afterID)) {
		return model.WardrobePage{}, model.ErrInvalidWardrobeInput
	}
	return s.repository.ListWardrobeItems(ctx, user.ID, limit, afterID)
}

func (s *WardrobeService) GetWardrobeItem(ctx context.Context, token, itemID string) (model.WardrobeItem, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.WardrobeItem{}, err
	}
	if !validUUID(itemID) {
		return model.WardrobeItem{}, model.ErrInvalidWardrobeInput
	}
	return s.repository.GetWardrobeItem(ctx, user.ID, itemID)
}

func (s *WardrobeService) UpdateWardrobeItem(ctx context.Context, token, itemID string, expectedRevision int, input model.UpdateWardrobeItemInput) (model.WardrobeItem, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return model.WardrobeItem{}, err
	}
	name, ok := validWardrobeFields(itemID, input.Name, input.Category, input.Availability)
	if !ok || expectedRevision < 1 {
		return model.WardrobeItem{}, model.ErrInvalidWardrobeInput
	}
	input.Name = name
	return s.repository.UpdateWardrobeItem(ctx, user.ID, itemID, expectedRevision, input, s.now().UTC())
}

func (s *WardrobeService) DeleteWardrobeItem(ctx context.Context, token, itemID string, expectedRevision int) error {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return err
	}
	if !validUUID(itemID) || expectedRevision < 1 {
		return model.ErrInvalidWardrobeInput
	}
	return s.repository.DeleteWardrobeItem(ctx, user.ID, itemID, expectedRevision)
}

func validWardrobeFields(id, name string, category model.WardrobeCategory, availability model.WardrobeAvailability) (string, bool) {
	name = strings.TrimSpace(name)
	if !validUUID(id) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 80 {
		return "", false
	}
	for _, character := range name {
		if unicode.IsControl(character) {
			return "", false
		}
	}
	return name, validWardrobeCategory(category) && validWardrobeAvailability(availability)
}

func validUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.String() == strings.ToLower(value)
}

func validWardrobeCategory(value model.WardrobeCategory) bool {
	switch value {
	case model.WardrobeTop, model.WardrobeBottom, model.WardrobeOnePiece, model.WardrobeOuterwear, model.WardrobeShoes, model.WardrobeBag, model.WardrobeAccessory:
		return true
	default:
		return false
	}
}

func validWardrobeAvailability(value model.WardrobeAvailability) bool {
	switch value {
	case model.WardrobeWearable, model.WardrobeLaundry, model.WardrobeLentOut, model.WardrobePacked:
		return true
	default:
		return false
	}
}

func validWardrobeSource(value model.WardrobeSource) bool {
	return value == model.WardrobeSourceWardrobe || value == model.WardrobeSourceQuickAdd
}
