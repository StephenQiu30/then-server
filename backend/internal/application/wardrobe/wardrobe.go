package wardrobe

import (
	"context"
	"encoding/hex"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
	"github.com/google/uuid"
)

type WardrobeRepository interface {
	CreateWardrobeItem(context.Context, domain.WardrobeItem) (domain.WardrobeItem, error)
	ListWardrobeItems(context.Context, string, int, *string) (domain.WardrobePage, error)
	GetWardrobeItem(context.Context, string, string) (domain.WardrobeItem, error)
	UpdateWardrobeItem(context.Context, string, string, int, domain.UpdateWardrobeItemInput, time.Time) (domain.WardrobeItem, error)
	GetWardrobeDeletionImpact(context.Context, string, string) (domain.WardrobeDeletionImpact, error)
	DeleteWardrobeItem(context.Context, string, string, int, domain.WardrobeHistoryPolicy, string, time.Time) error
}

type WardrobeService struct {
	authenticator PrivacyAuthenticator
	repository    WardrobeRepository
	now           func() time.Time
}

func NewWardrobeService(authenticator PrivacyAuthenticator, repository WardrobeRepository) (*WardrobeService, error) {
	if authenticator == nil || repository == nil {
		return nil, domain.ErrWardrobeUnavailable
	}
	return &WardrobeService{authenticator: authenticator, repository: repository, now: time.Now}, nil
}

func (s *WardrobeService) CreateWardrobeItem(ctx context.Context, token string, input domain.CreateWardrobeItemInput) (domain.WardrobeItem, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.WardrobeItem{}, err
	}
	name, ok := validWardrobeFields(input.ID, input.Name, input.Category, input.Availability)
	if !ok || !validWardrobeSource(input.Source) || !validWardrobeAttributes(input.Attributes) {
		return domain.WardrobeItem{}, domain.ErrInvalidWardrobeInput
	}
	now := s.now().UTC()
	return s.repository.CreateWardrobeItem(ctx, domain.WardrobeItem{
		ID: input.ID, OwnerID: user.ID, Name: name, Category: input.Category,
		Availability: input.Availability, Source: input.Source, Attributes: input.Attributes, Revision: 1, CreatedAt: now, UpdatedAt: now,
	})
}

func (s *WardrobeService) ListWardrobeItems(ctx context.Context, token string, limit int, afterID *string) (domain.WardrobePage, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.WardrobePage{}, err
	}
	if limit < 1 || limit > 100 || (afterID != nil && !validUUID(*afterID)) {
		return domain.WardrobePage{}, domain.ErrInvalidWardrobeInput
	}
	return s.repository.ListWardrobeItems(ctx, user.ID, limit, afterID)
}

func (s *WardrobeService) GetWardrobeItem(ctx context.Context, token, itemID string) (domain.WardrobeItem, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.WardrobeItem{}, err
	}
	if !validUUID(itemID) {
		return domain.WardrobeItem{}, domain.ErrInvalidWardrobeInput
	}
	return s.repository.GetWardrobeItem(ctx, user.ID, itemID)
}

func (s *WardrobeService) UpdateWardrobeItem(ctx context.Context, token, itemID string, expectedRevision int, input domain.UpdateWardrobeItemInput) (domain.WardrobeItem, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.WardrobeItem{}, err
	}
	name, ok := validWardrobeFields(itemID, input.Name, input.Category, input.Availability)
	if !ok || expectedRevision < 1 || !validWardrobeAttributes(input.Attributes) {
		return domain.WardrobeItem{}, domain.ErrInvalidWardrobeInput
	}
	input.Name = name
	return s.repository.UpdateWardrobeItem(ctx, user.ID, itemID, expectedRevision, input, s.now().UTC())
}

func (s *WardrobeService) GetWardrobeDeletionImpact(ctx context.Context, token, itemID string) (domain.WardrobeDeletionImpact, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return domain.WardrobeDeletionImpact{}, err
	}
	if !validUUID(itemID) {
		return domain.WardrobeDeletionImpact{}, domain.ErrInvalidWardrobeInput
	}
	return s.repository.GetWardrobeDeletionImpact(ctx, user.ID, itemID)
}

func (s *WardrobeService) DeleteWardrobeItem(ctx context.Context, token, itemID string, expectedRevision int, policy domain.WardrobeHistoryPolicy, expectedImpact string) error {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return err
	}
	decodedImpact, decodeErr := hex.DecodeString(expectedImpact)
	if !validUUID(itemID) || expectedRevision < 1 || len(decodedImpact) != 32 || decodeErr != nil || !validWardrobeHistoryPolicy(policy) {
		return domain.ErrInvalidWardrobeInput
	}
	return s.repository.DeleteWardrobeItem(ctx, user.ID, itemID, expectedRevision, policy, expectedImpact, s.now().UTC())
}

func validWardrobeFields(id, name string, category domain.WardrobeCategory, availability domain.WardrobeAvailability) (string, bool) {
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

func validWardrobeCategory(value domain.WardrobeCategory) bool {
	switch value {
	case domain.WardrobeTop, domain.WardrobeBottom, domain.WardrobeOnePiece, domain.WardrobeOuterwear, domain.WardrobeShoes, domain.WardrobeBag, domain.WardrobeAccessory:
		return true
	default:
		return false
	}
}

func validWardrobeAvailability(value domain.WardrobeAvailability) bool {
	switch value {
	case domain.WardrobeWearable, domain.WardrobeLaundry, domain.WardrobeLentOut, domain.WardrobePacked:
		return true
	default:
		return false
	}
}

func validWardrobeSource(value domain.WardrobeSource) bool {
	return value == domain.WardrobeSourceWardrobe || value == domain.WardrobeSourceQuickAdd
}

func validWardrobeHistoryPolicy(value domain.WardrobeHistoryPolicy) bool {
	return value == domain.WardrobeHistoryRedactSnapshots || value == domain.WardrobeHistoryDeleteAffectedHistory
}

func validWardrobeAttributes(value domain.WardrobeAttributes) bool {
	if value.FormalityBand != nil {
		switch *value.FormalityBand {
		case domain.WardrobeFormalityCasual, domain.WardrobeFormalitySmartCasual, domain.WardrobeFormalityFormal:
		default:
			return false
		}
	}
	if value.WarmthBand != nil {
		switch *value.WarmthBand {
		case domain.WardrobeWarmthLight, domain.WardrobeWarmthMedium, domain.WardrobeWarmthWarm:
		default:
			return false
		}
	}
	for _, suitability := range []*domain.WardrobeUseSuitability{value.RainUse, value.WalkingUse} {
		if suitability != nil && *suitability != domain.WardrobeUseSuitable && *suitability != domain.WardrobeUseUnsuitable {
			return false
		}
	}
	return true
}
