package wardrobe

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
)

type wardrobeAuthenticatorStub struct {
	user domain.User
	err  error
}

func (s wardrobeAuthenticatorStub) CurrentUser(context.Context, string) (domain.User, error) {
	return s.user, s.err
}

type wardrobeRepositoryStub struct {
	created  domain.WardrobeItem
	ownerID  string
	updated  domain.UpdateWardrobeItemInput
	deleted  string
	expected int
	policy   domain.WardrobeHistoryPolicy
	impact   string
	page     domain.WardrobePage
	err      error
}

func (s *wardrobeRepositoryStub) CreateWardrobeItem(_ context.Context, item domain.WardrobeItem) (domain.WardrobeItem, error) {
	s.created, s.ownerID = item, item.OwnerID
	return item, s.err
}
func (s *wardrobeRepositoryStub) ListWardrobeItems(_ context.Context, ownerID string, limit int, afterID *string) (domain.WardrobePage, error) {
	s.ownerID = ownerID
	return s.page, s.err
}
func (s *wardrobeRepositoryStub) GetWardrobeItem(_ context.Context, ownerID, itemID string) (domain.WardrobeItem, error) {
	s.ownerID = ownerID
	return domain.WardrobeItem{ID: itemID, OwnerID: ownerID}, s.err
}
func (s *wardrobeRepositoryStub) UpdateWardrobeItem(_ context.Context, ownerID, itemID string, expected int, input domain.UpdateWardrobeItemInput, _ time.Time) (domain.WardrobeItem, error) {
	s.ownerID, s.updated, s.expected = ownerID, input, expected
	return domain.WardrobeItem{ID: itemID, OwnerID: ownerID, Name: input.Name, Revision: expected + 1}, s.err
}
func (s *wardrobeRepositoryStub) GetWardrobeDeletionImpact(_ context.Context, ownerID, _ string) (domain.WardrobeDeletionImpact, error) {
	s.ownerID = ownerID
	return domain.WardrobeDeletionImpact{ExpectedImpact: emptyWardrobeImpact}, s.err
}
func (s *wardrobeRepositoryStub) DeleteWardrobeItem(_ context.Context, ownerID, itemID string, expected int, policy domain.WardrobeHistoryPolicy, impact string, _ time.Time) error {
	s.ownerID, s.deleted, s.expected, s.policy, s.impact = ownerID, itemID, expected, policy, impact
	return s.err
}

func newWardrobeServiceForTest(t *testing.T, repository *wardrobeRepositoryStub) *WardrobeService {
	t.Helper()
	service, err := NewWardrobeService(wardrobeAuthenticatorStub{user: domain.User{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11"}}, repository)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC) }
	return service
}

func TestWardrobeCreateNormalizesAndUsesAuthenticatedOwner(t *testing.T) {
	repository := new(wardrobeRepositoryStub)
	service := newWardrobeServiceForTest(t, repository)
	item, err := service.CreateWardrobeItem(context.Background(), "session", domain.CreateWardrobeItemInput{
		ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Name: "  蓝色衬衫  ", Category: domain.WardrobeTop,
		Availability: domain.WardrobeWearable, Source: domain.WardrobeSourceWardrobe,
		Attributes: domain.WardrobeAttributes{FormalityBand: wardrobeValue(domain.WardrobeFormalitySmartCasual), WarmthBand: wardrobeValue(domain.WardrobeWarmthLight), RainUse: wardrobeValue(domain.WardrobeUseSuitable)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.ownerID == "" || item.Name != "蓝色衬衫" || item.Revision != 1 || item.CreatedAt.IsZero() || !item.CreatedAt.Equal(item.UpdatedAt) || item.Attributes.FormalityBand == nil || *item.Attributes.FormalityBand != domain.WardrobeFormalitySmartCasual {
		t.Fatal("create did not normalize or establish server-owned fields")
	}
}

func TestWardrobeRejectsInvalidInputBeforeRepository(t *testing.T) {
	invalid := []domain.CreateWardrobeItemInput{
		{ID: "not-a-uuid", Name: "Coat", Category: domain.WardrobeOuterwear, Availability: domain.WardrobeWearable, Source: domain.WardrobeSourceWardrobe},
		{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Name: "\u0000", Category: domain.WardrobeTop, Availability: domain.WardrobeWearable, Source: domain.WardrobeSourceWardrobe},
		{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Name: "Coat", Category: "unknown", Availability: domain.WardrobeWearable, Source: domain.WardrobeSourceWardrobe},
		{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Name: "Coat", Category: domain.WardrobeOuterwear, Availability: domain.WardrobeWearable, Source: domain.WardrobeSourceWardrobe, Attributes: domain.WardrobeAttributes{FormalityBand: wardrobeValue(domain.WardrobeFormalityBand("guessed"))}},
		{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Name: "Coat", Category: domain.WardrobeOuterwear, Availability: domain.WardrobeWearable, Source: domain.WardrobeSourceWardrobe, Attributes: domain.WardrobeAttributes{WarmthBand: wardrobeValue(domain.WardrobeWarmthBand("hot"))}},
		{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Name: "Coat", Category: domain.WardrobeOuterwear, Availability: domain.WardrobeWearable, Source: domain.WardrobeSourceWardrobe, Attributes: domain.WardrobeAttributes{RainUse: wardrobeValue(domain.WardrobeUseSuitability("maybe"))}},
		{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Name: "Coat", Category: domain.WardrobeOuterwear, Availability: domain.WardrobeWearable, Source: domain.WardrobeSourceWardrobe, Attributes: domain.WardrobeAttributes{WalkingUse: wardrobeValue(domain.WardrobeUseSuitability("sometimes"))}},
	}
	for _, input := range invalid {
		repository := new(wardrobeRepositoryStub)
		if _, err := newWardrobeServiceForTest(t, repository).CreateWardrobeItem(context.Background(), "session", input); !errors.Is(err, domain.ErrInvalidWardrobeInput) || repository.ownerID != "" {
			t.Fatal("invalid wardrobe input reached repository")
		}
	}
}

func TestWardrobeUpdateDeleteAndListValidateRevisionAndLimit(t *testing.T) {
	repository := new(wardrobeRepositoryStub)
	service := newWardrobeServiceForTest(t, repository)
	id := "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12"
	if _, err := service.UpdateWardrobeItem(context.Background(), "session", id, 2, domain.UpdateWardrobeItemInput{Name: " Jacket ", Category: domain.WardrobeOuterwear, Availability: domain.WardrobePacked, Attributes: domain.WardrobeAttributes{WalkingUse: wardrobeValue(domain.WardrobeUseUnsuitable)}}); err != nil {
		t.Fatal(err)
	}
	if repository.updated.Name != "Jacket" || repository.expected != 2 || repository.updated.Attributes.WalkingUse == nil || *repository.updated.Attributes.WalkingUse != domain.WardrobeUseUnsuitable {
		t.Fatal("update did not normalize and forward the expected revision")
	}
	if _, err := service.UpdateWardrobeItem(context.Background(), "session", id, 2, domain.UpdateWardrobeItemInput{Name: "Jacket", Category: domain.WardrobeOuterwear, Availability: domain.WardrobePacked, Attributes: domain.WardrobeAttributes{WarmthBand: wardrobeValue(domain.WardrobeWarmthBand("boiling"))}}); !errors.Is(err, domain.ErrInvalidWardrobeInput) {
		t.Fatal("invalid update attribute was accepted")
	}
	if err := service.DeleteWardrobeItem(context.Background(), "session", id, 3, domain.WardrobeHistoryRedactSnapshots, emptyWardrobeImpact); err != nil || repository.expected != 3 || repository.policy != domain.WardrobeHistoryRedactSnapshots {
		t.Fatal("delete did not forward the expected revision")
	}
	for _, limit := range []int{0, 101} {
		if _, err := service.ListWardrobeItems(context.Background(), "session", limit, nil); !errors.Is(err, domain.ErrInvalidWardrobeInput) {
			t.Fatal("invalid list limit was accepted")
		}
	}
	if err := service.DeleteWardrobeItem(context.Background(), "session", id, 0, domain.WardrobeHistoryRedactSnapshots, emptyWardrobeImpact); !errors.Is(err, domain.ErrInvalidWardrobeInput) {
		t.Fatal("invalid revision was accepted")
	}
	if err := service.DeleteWardrobeItem(context.Background(), "session", id, 1, "keep_everything", emptyWardrobeImpact); !errors.Is(err, domain.ErrInvalidWardrobeInput) {
		t.Fatal("invalid history policy was accepted")
	}
}

func wardrobeValue[T any](value T) *T { return &value }

const emptyWardrobeImpact = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
