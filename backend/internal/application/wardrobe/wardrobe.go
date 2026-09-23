package wardrobe

import (
	"context"
	"encoding/hex"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

type WardrobeRepository interface {
	CreateWardrobeItem(context.Context, WardrobeItem) (WardrobeItem, error)
	ListWardrobeItems(context.Context, string, int, *string, WardrobeListFilter) (WardrobePage, error)
	GetWardrobeItem(context.Context, string, string) (WardrobeItem, error)
	UpdateWardrobeItem(context.Context, string, string, int, UpdateWardrobeItemInput, time.Time) (WardrobeItem, error)
	ArchiveWardrobeItem(context.Context, string, string, int, time.Time) (WardrobeItem, error)
	RestoreWardrobeItem(context.Context, string, string, int, time.Time) (WardrobeItem, error)
	GetWardrobeDeletionImpact(context.Context, string, string) (WardrobeDeletionImpact, error)
	DeleteWardrobeItem(context.Context, string, string, int, WardrobeHistoryPolicy, string, time.Time) error
}

type WardrobeService struct {
	authenticator PrivacyAuthenticator
	repository    WardrobeRepository
	now           func() time.Time
}

func NewWardrobeService(authenticator PrivacyAuthenticator, repository WardrobeRepository) (*WardrobeService, error) {
	if authenticator == nil || repository == nil {
		return nil, ErrWardrobeUnavailable
	}
	return &WardrobeService{authenticator: authenticator, repository: repository, now: time.Now}, nil
}

func (s *WardrobeService) CreateWardrobeItem(ctx context.Context, token string, input CreateWardrobeItemInput) (WardrobeItem, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return WardrobeItem{}, err
	}
	name, ok := validWardrobeFields(input.ID, input.Name, input.Category, input.Availability)
	if !ok || !validWardrobeSource(input.Source) || !validWardrobeAttributes(input.Attributes) {
		return WardrobeItem{}, ErrInvalidWardrobeInput
	}
	now := s.now().UTC()
	return s.repository.CreateWardrobeItem(ctx, WardrobeItem{
		ID: input.ID, OwnerID: user.ID, Name: name, Category: input.Category,
		Availability: input.Availability, Source: input.Source, Attributes: input.Attributes, Revision: 1, CreatedAt: now, UpdatedAt: now,
	})
}

func (s *WardrobeService) ListWardrobeItems(ctx context.Context, token string, limit int, afterID *string, filter WardrobeListFilter) (WardrobePage, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return WardrobePage{}, err
	}
	if limit < 1 || limit > 100 || (afterID != nil && !validUUID(*afterID)) || !validWardrobeListFilter(filter) {
		return WardrobePage{}, ErrInvalidWardrobeInput
	}
	return s.repository.ListWardrobeItems(ctx, user.ID, limit, afterID, filter)
}

func (s *WardrobeService) GetWardrobeItem(ctx context.Context, token, itemID string) (WardrobeItem, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return WardrobeItem{}, err
	}
	if !validUUID(itemID) {
		return WardrobeItem{}, ErrInvalidWardrobeInput
	}
	return s.repository.GetWardrobeItem(ctx, user.ID, itemID)
}

func (s *WardrobeService) UpdateWardrobeItem(ctx context.Context, token, itemID string, expectedRevision int, input UpdateWardrobeItemInput) (WardrobeItem, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return WardrobeItem{}, err
	}
	name, ok := validWardrobeFields(itemID, input.Name, input.Category, input.Availability)
	if !ok || expectedRevision < 1 || !validWardrobeAttributes(input.Attributes) {
		return WardrobeItem{}, ErrInvalidWardrobeInput
	}
	input.Name = name
	return s.repository.UpdateWardrobeItem(ctx, user.ID, itemID, expectedRevision, input, s.now().UTC())
}

func (s *WardrobeService) ArchiveWardrobeItem(ctx context.Context, token, itemID string, expectedRevision int) (WardrobeItem, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return WardrobeItem{}, err
	}
	if !validUUID(itemID) || expectedRevision < 1 {
		return WardrobeItem{}, ErrInvalidWardrobeInput
	}
	return s.repository.ArchiveWardrobeItem(ctx, user.ID, itemID, expectedRevision, s.now().UTC())
}

func (s *WardrobeService) RestoreWardrobeItem(ctx context.Context, token, itemID string, expectedRevision int) (WardrobeItem, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return WardrobeItem{}, err
	}
	if !validUUID(itemID) || expectedRevision < 1 {
		return WardrobeItem{}, ErrInvalidWardrobeInput
	}
	return s.repository.RestoreWardrobeItem(ctx, user.ID, itemID, expectedRevision, s.now().UTC())
}

func (s *WardrobeService) GetWardrobeDeletionImpact(ctx context.Context, token, itemID string) (WardrobeDeletionImpact, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return WardrobeDeletionImpact{}, err
	}
	if !validUUID(itemID) {
		return WardrobeDeletionImpact{}, ErrInvalidWardrobeInput
	}
	return s.repository.GetWardrobeDeletionImpact(ctx, user.ID, itemID)
}

func (s *WardrobeService) DeleteWardrobeItem(ctx context.Context, token, itemID string, expectedRevision int, policy WardrobeHistoryPolicy, expectedImpact string) error {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return err
	}
	decodedImpact, decodeErr := hex.DecodeString(expectedImpact)
	if !validUUID(itemID) || expectedRevision < 1 || len(decodedImpact) != 32 || decodeErr != nil || !validWardrobeHistoryPolicy(policy) {
		return ErrInvalidWardrobeInput
	}
	return s.repository.DeleteWardrobeItem(ctx, user.ID, itemID, expectedRevision, policy, expectedImpact, s.now().UTC())
}

func validWardrobeFields(id, name string, category WardrobeCategory, availability WardrobeAvailability) (string, bool) {
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

func validWardrobeCategory(value WardrobeCategory) bool {
	switch value {
	case WardrobeTop, WardrobeBottom, WardrobeOnePiece, WardrobeOuterwear, WardrobeShoes, WardrobeBag, WardrobeAccessory:
		return true
	default:
		return false
	}
}

func validWardrobeAvailability(value WardrobeAvailability) bool {
	switch value {
	case WardrobeWearable, WardrobeLaundry, WardrobeLentOut, WardrobePacked:
		return true
	default:
		return false
	}
}

func validWardrobeListFilter(value WardrobeListFilter) bool {
	if value.Lifecycle == "" {
		return false
	}
	if value.Lifecycle != WardrobeActive && value.Lifecycle != WardrobeArchived && value.Lifecycle != WardrobeAll {
		return false
	}
	return value.Availability == nil || validWardrobeAvailability(*value.Availability)
}

func validWardrobeSource(value WardrobeSource) bool {
	return value == WardrobeSourceWardrobe || value == WardrobeSourceQuickAdd
}

func validWardrobeHistoryPolicy(value WardrobeHistoryPolicy) bool {
	return value == WardrobeHistoryRedactSnapshots || value == WardrobeHistoryDeleteAffectedHistory
}

func validWardrobeAttributes(value WardrobeAttributes) bool {
	if value.FormalityBand != nil {
		switch *value.FormalityBand {
		case WardrobeFormalityCasual, WardrobeFormalitySmartCasual, WardrobeFormalityFormal:
		default:
			return false
		}
	}
	if value.WarmthBand != nil {
		switch *value.WarmthBand {
		case WardrobeWarmthLight, WardrobeWarmthMedium, WardrobeWarmthWarm:
		default:
			return false
		}
	}
	for _, suitability := range []*WardrobeUseSuitability{value.RainUse, value.WalkingUse} {
		if suitability != nil && *suitability != WardrobeUseSuitable && *suitability != WardrobeUseUnsuitable {
			return false
		}
	}
	return true
}
