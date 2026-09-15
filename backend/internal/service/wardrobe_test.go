package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
)

type wardrobeAuthenticatorStub struct {
	user model.User
	err  error
}

func (s wardrobeAuthenticatorStub) CurrentUser(context.Context, string) (model.User, error) {
	return s.user, s.err
}

type wardrobeRepositoryStub struct {
	created  model.WardrobeItem
	ownerID  string
	updated  model.UpdateWardrobeItemInput
	deleted  string
	expected int
	policy   model.WardrobeHistoryPolicy
	impact   string
	page     model.WardrobePage
	err      error
}

func (s *wardrobeRepositoryStub) CreateWardrobeItem(_ context.Context, item model.WardrobeItem) (model.WardrobeItem, error) {
	s.created, s.ownerID = item, item.OwnerID
	return item, s.err
}
func (s *wardrobeRepositoryStub) ListWardrobeItems(_ context.Context, ownerID string, limit int, afterID *string) (model.WardrobePage, error) {
	s.ownerID = ownerID
	return s.page, s.err
}
func (s *wardrobeRepositoryStub) GetWardrobeItem(_ context.Context, ownerID, itemID string) (model.WardrobeItem, error) {
	s.ownerID = ownerID
	return model.WardrobeItem{ID: itemID, OwnerID: ownerID}, s.err
}
func (s *wardrobeRepositoryStub) UpdateWardrobeItem(_ context.Context, ownerID, itemID string, expected int, input model.UpdateWardrobeItemInput, _ time.Time) (model.WardrobeItem, error) {
	s.ownerID, s.updated, s.expected = ownerID, input, expected
	return model.WardrobeItem{ID: itemID, OwnerID: ownerID, Name: input.Name, Revision: expected + 1}, s.err
}
func (s *wardrobeRepositoryStub) GetWardrobeDeletionImpact(_ context.Context, ownerID, _ string) (model.WardrobeDeletionImpact, error) {
	s.ownerID = ownerID
	return model.WardrobeDeletionImpact{ExpectedImpact: emptyWardrobeImpact}, s.err
}
func (s *wardrobeRepositoryStub) DeleteWardrobeItem(_ context.Context, ownerID, itemID string, expected int, policy model.WardrobeHistoryPolicy, impact string, _ time.Time) error {
	s.ownerID, s.deleted, s.expected, s.policy, s.impact = ownerID, itemID, expected, policy, impact
	return s.err
}

func newWardrobeServiceForTest(t *testing.T, repository *wardrobeRepositoryStub) *WardrobeService {
	t.Helper()
	service, err := NewWardrobeService(wardrobeAuthenticatorStub{user: model.User{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11"}}, repository)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC) }
	return service
}

func TestWardrobeCreateNormalizesAndUsesAuthenticatedOwner(t *testing.T) {
	repository := new(wardrobeRepositoryStub)
	service := newWardrobeServiceForTest(t, repository)
	item, err := service.CreateWardrobeItem(context.Background(), "session", model.CreateWardrobeItemInput{
		ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Name: "  蓝色衬衫  ", Category: model.WardrobeTop,
		Availability: model.WardrobeWearable, Source: model.WardrobeSourceWardrobe,
		Attributes: model.WardrobeAttributes{FormalityBand: wardrobeValue(model.WardrobeFormalitySmartCasual), WarmthBand: wardrobeValue(model.WardrobeWarmthLight), RainUse: wardrobeValue(model.WardrobeUseSuitable)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.ownerID == "" || item.Name != "蓝色衬衫" || item.Revision != 1 || item.CreatedAt.IsZero() || !item.CreatedAt.Equal(item.UpdatedAt) || item.Attributes.FormalityBand == nil || *item.Attributes.FormalityBand != model.WardrobeFormalitySmartCasual {
		t.Fatal("create did not normalize or establish server-owned fields")
	}
}

func TestWardrobeRejectsInvalidInputBeforeRepository(t *testing.T) {
	invalid := []model.CreateWardrobeItemInput{
		{ID: "not-a-uuid", Name: "Coat", Category: model.WardrobeOuterwear, Availability: model.WardrobeWearable, Source: model.WardrobeSourceWardrobe},
		{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Name: "\u0000", Category: model.WardrobeTop, Availability: model.WardrobeWearable, Source: model.WardrobeSourceWardrobe},
		{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Name: "Coat", Category: "unknown", Availability: model.WardrobeWearable, Source: model.WardrobeSourceWardrobe},
		{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Name: "Coat", Category: model.WardrobeOuterwear, Availability: model.WardrobeWearable, Source: model.WardrobeSourceWardrobe, Attributes: model.WardrobeAttributes{FormalityBand: wardrobeValue(model.WardrobeFormalityBand("guessed"))}},
		{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Name: "Coat", Category: model.WardrobeOuterwear, Availability: model.WardrobeWearable, Source: model.WardrobeSourceWardrobe, Attributes: model.WardrobeAttributes{WarmthBand: wardrobeValue(model.WardrobeWarmthBand("hot"))}},
		{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Name: "Coat", Category: model.WardrobeOuterwear, Availability: model.WardrobeWearable, Source: model.WardrobeSourceWardrobe, Attributes: model.WardrobeAttributes{RainUse: wardrobeValue(model.WardrobeUseSuitability("maybe"))}},
		{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Name: "Coat", Category: model.WardrobeOuterwear, Availability: model.WardrobeWearable, Source: model.WardrobeSourceWardrobe, Attributes: model.WardrobeAttributes{WalkingUse: wardrobeValue(model.WardrobeUseSuitability("sometimes"))}},
	}
	for _, input := range invalid {
		repository := new(wardrobeRepositoryStub)
		if _, err := newWardrobeServiceForTest(t, repository).CreateWardrobeItem(context.Background(), "session", input); !errors.Is(err, model.ErrInvalidWardrobeInput) || repository.ownerID != "" {
			t.Fatal("invalid wardrobe input reached repository")
		}
	}
}

func TestWardrobeUpdateDeleteAndListValidateRevisionAndLimit(t *testing.T) {
	repository := new(wardrobeRepositoryStub)
	service := newWardrobeServiceForTest(t, repository)
	id := "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12"
	if _, err := service.UpdateWardrobeItem(context.Background(), "session", id, 2, model.UpdateWardrobeItemInput{Name: " Jacket ", Category: model.WardrobeOuterwear, Availability: model.WardrobePacked, Attributes: model.WardrobeAttributes{WalkingUse: wardrobeValue(model.WardrobeUseUnsuitable)}}); err != nil {
		t.Fatal(err)
	}
	if repository.updated.Name != "Jacket" || repository.expected != 2 || repository.updated.Attributes.WalkingUse == nil || *repository.updated.Attributes.WalkingUse != model.WardrobeUseUnsuitable {
		t.Fatal("update did not normalize and forward the expected revision")
	}
	if _, err := service.UpdateWardrobeItem(context.Background(), "session", id, 2, model.UpdateWardrobeItemInput{Name: "Jacket", Category: model.WardrobeOuterwear, Availability: model.WardrobePacked, Attributes: model.WardrobeAttributes{WarmthBand: wardrobeValue(model.WardrobeWarmthBand("boiling"))}}); !errors.Is(err, model.ErrInvalidWardrobeInput) {
		t.Fatal("invalid update attribute was accepted")
	}
	if err := service.DeleteWardrobeItem(context.Background(), "session", id, 3, model.WardrobeHistoryRedactSnapshots, emptyWardrobeImpact); err != nil || repository.expected != 3 || repository.policy != model.WardrobeHistoryRedactSnapshots {
		t.Fatal("delete did not forward the expected revision")
	}
	for _, limit := range []int{0, 101} {
		if _, err := service.ListWardrobeItems(context.Background(), "session", limit, nil); !errors.Is(err, model.ErrInvalidWardrobeInput) {
			t.Fatal("invalid list limit was accepted")
		}
	}
	if err := service.DeleteWardrobeItem(context.Background(), "session", id, 0, model.WardrobeHistoryRedactSnapshots, emptyWardrobeImpact); !errors.Is(err, model.ErrInvalidWardrobeInput) {
		t.Fatal("invalid revision was accepted")
	}
	if err := service.DeleteWardrobeItem(context.Background(), "session", id, 1, "keep_everything", emptyWardrobeImpact); !errors.Is(err, model.ErrInvalidWardrobeInput) {
		t.Fatal("invalid history policy was accepted")
	}
}

func wardrobeValue[T any](value T) *T { return &value }

const emptyWardrobeImpact = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
