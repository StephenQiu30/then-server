package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
)

type outfitPlanRepositoryStub struct {
	ownerID  string
	planID   string
	input    model.OutfitPlanInput
	expected int
	afterID  *string
	date     *string
	err      error
}

func (s *outfitPlanRepositoryStub) CreateOutfitPlan(_ context.Context, ownerID, planID string, input model.OutfitPlanInput, _ time.Time) (model.OutfitPlan, error) {
	s.ownerID, s.planID, s.input = ownerID, planID, input
	return model.OutfitPlan{ID: planID, OwnerID: ownerID, LocalDate: input.LocalDate, TimeZone: input.TimeZone, ContextSummary: input.ContextSummary, Revision: 1}, s.err
}
func (s *outfitPlanRepositoryStub) ListOutfitPlans(_ context.Context, ownerID string, _ int, afterID, date *string) (model.OutfitPlanPage, error) {
	s.ownerID, s.afterID, s.date = ownerID, afterID, date
	return model.OutfitPlanPage{}, s.err
}
func (s *outfitPlanRepositoryStub) GetOutfitPlan(_ context.Context, ownerID, planID string) (model.OutfitPlan, error) {
	s.ownerID, s.planID = ownerID, planID
	return model.OutfitPlan{ID: planID, OwnerID: ownerID}, s.err
}
func (s *outfitPlanRepositoryStub) UpdateOutfitPlan(_ context.Context, ownerID, planID string, expected int, input model.OutfitPlanInput, _ time.Time) (model.OutfitPlan, error) {
	s.ownerID, s.planID, s.expected, s.input = ownerID, planID, expected, input
	return model.OutfitPlan{ID: planID, OwnerID: ownerID, Revision: expected + 1}, s.err
}
func (s *outfitPlanRepositoryStub) CancelOutfitPlan(_ context.Context, ownerID, planID string, expected int, _ time.Time) (model.OutfitPlan, error) {
	s.ownerID, s.planID, s.expected = ownerID, planID, expected
	return model.OutfitPlan{ID: planID, OwnerID: ownerID, Revision: expected + 1, Status: model.OutfitPlanCancelled}, s.err
}
func (s *outfitPlanRepositoryStub) MarkOutfitPlanNotWorn(_ context.Context, ownerID, planID string, expected int, _ time.Time) (model.OutfitPlan, error) {
	s.ownerID, s.planID, s.expected = ownerID, planID, expected
	return model.OutfitPlan{ID: planID, OwnerID: ownerID, Revision: expected + 1, Status: model.OutfitPlanNotWorn}, s.err
}
func (s *outfitPlanRepositoryStub) RestoreOutfitPlan(_ context.Context, ownerID, planID string, expected int, _ time.Time) (model.OutfitPlan, error) {
	s.ownerID, s.planID, s.expected = ownerID, planID, expected
	return model.OutfitPlan{ID: planID, OwnerID: ownerID, Revision: expected + 1, Status: model.OutfitPlanActive}, s.err
}
func (s *outfitPlanRepositoryStub) DeleteOutfitPlan(_ context.Context, ownerID, planID string, expected int, _ time.Time) error {
	s.ownerID, s.planID, s.expected = ownerID, planID, expected
	return s.err
}

func newOutfitPlanServiceForTest(t *testing.T, repository *outfitPlanRepositoryStub) *OutfitPlanService {
	t.Helper()
	service, err := NewOutfitPlanService(wardrobeAuthenticatorStub{user: model.User{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11"}}, repository)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Date(2026, 9, 16, 2, 0, 0, 0, time.UTC) }
	return service
}

func validOutfitPlanInput() model.OutfitPlanInput {
	summary := "  Work lunch  "
	return model.OutfitPlanInput{LocalDate: "2026-09-16", TimeZone: "Asia/Shanghai", ContextSummary: &summary, Items: []model.OutfitSelection{{ItemID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a12", Revision: 2}}, ConfirmedUnavailableIDs: []string{}}
}

func TestOutfitPlanCreateNormalizesAndUsesAuthenticatedOwner(t *testing.T) {
	repository := new(outfitPlanRepositoryStub)
	service := newOutfitPlanServiceForTest(t, repository)
	planID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400a13"
	plan, err := service.CreateOutfitPlan(context.Background(), "session", planID, validOutfitPlanInput())
	if err != nil {
		t.Fatal(err)
	}
	if repository.ownerID == "" || plan.ID != planID || repository.input.ContextSummary == nil || *repository.input.ContextSummary != "Work lunch" {
		t.Fatal("create did not normalize or use the authenticated owner")
	}
}

func TestOutfitPlanRejectsInvalidInputBeforeRepository(t *testing.T) {
	planID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400a13"
	tests := map[string]func(*model.OutfitPlanInput){
		"past date":        func(value *model.OutfitPlanInput) { value.LocalDate = "2026-09-15" },
		"invalid date":     func(value *model.OutfitPlanInput) { value.LocalDate = "2026-02-30" },
		"invalid timezone": func(value *model.OutfitPlanInput) { value.TimeZone = "Mars/Olympus" },
		"local timezone":   func(value *model.OutfitPlanInput) { value.TimeZone = "Local" },
		"duplicate item":   func(value *model.OutfitPlanInput) { value.Items = append(value.Items, value.Items[0]) },
		"zero revision":    func(value *model.OutfitPlanInput) { value.Items[0].Revision = 0 },
		"unknown confirmation": func(value *model.OutfitPlanInput) {
			value.ConfirmedUnavailableIDs = []string{"018f1f74-a2d0-7c6d-9c17-4a0ea2400a99"}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			repository := new(outfitPlanRepositoryStub)
			input := validOutfitPlanInput()
			mutate(&input)
			if _, err := newOutfitPlanServiceForTest(t, repository).CreateOutfitPlan(context.Background(), "session", planID, input); !errors.Is(err, model.ErrInvalidOutfitPlanInput) || repository.ownerID != "" {
				t.Fatal("invalid plan input reached repository")
			}
		})
	}
}

func TestOutfitPlanDateBoundaryUsesSubmittedTimezone(t *testing.T) {
	repository := new(outfitPlanRepositoryStub)
	service := newOutfitPlanServiceForTest(t, repository)
	service.now = func() time.Time { return time.Date(2026, 9, 16, 23, 30, 0, 0, time.UTC) }
	input := validOutfitPlanInput()
	if _, err := service.CreateOutfitPlan(context.Background(), "session", "018f1f74-a2d0-7c6d-9c17-4a0ea2400a13", input); !errors.Is(err, model.ErrInvalidOutfitPlanInput) || repository.ownerID != "" {
		t.Fatal("date already past in submitted timezone reached repository")
	}
	input.LocalDate = "2026-09-17"
	if _, err := service.CreateOutfitPlan(context.Background(), "session", "018f1f74-a2d0-7c6d-9c17-4a0ea2400a13", input); err != nil {
		t.Fatal("current local date in submitted timezone was rejected")
	}
}

func TestOutfitPlanCommandsAndListValidation(t *testing.T) {
	repository := new(outfitPlanRepositoryStub)
	service := newOutfitPlanServiceForTest(t, repository)
	planID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400a13"
	if _, err := service.UpdateOutfitPlan(context.Background(), "session", planID, 2, validOutfitPlanInput()); err != nil || repository.expected != 2 {
		t.Fatal("valid update did not reach repository")
	}
	if _, err := service.CancelOutfitPlan(context.Background(), "session", planID, 3); err != nil || repository.expected != 3 {
		t.Fatal("valid cancellation did not reach repository")
	}
	if _, err := service.MarkOutfitPlanNotWorn(context.Background(), "session", planID, 4); err != nil || repository.expected != 4 {
		t.Fatal("valid not-worn transition did not reach repository")
	}
	if _, err := service.RestoreOutfitPlan(context.Background(), "session", planID, 5); err != nil || repository.expected != 5 {
		t.Fatal("valid restore transition did not reach repository")
	}
	if err := service.DeleteOutfitPlan(context.Background(), "session", planID, 6); err != nil || repository.expected != 6 {
		t.Fatal("valid deletion did not reach repository")
	}
	for _, limit := range []int{0, 51} {
		if _, err := service.ListOutfitPlans(context.Background(), "session", limit, nil, nil); !errors.Is(err, model.ErrInvalidOutfitPlanInput) {
			t.Fatal("invalid plan list limit was accepted")
		}
	}
	invalidDate := "2026-02-30"
	if _, err := service.ListOutfitPlans(context.Background(), "session", 20, nil, &invalidDate); !errors.Is(err, model.ErrInvalidOutfitPlanInput) {
		t.Fatal("invalid plan list date was accepted")
	}
	if _, err := service.CancelOutfitPlan(context.Background(), "session", planID, 0); !errors.Is(err, model.ErrInvalidOutfitPlanInput) {
		t.Fatal("invalid plan revision was accepted")
	}
}
