package wardrobe

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
)

func recommendationItem(id string, category WardrobeCategory, availability WardrobeAvailability) WardrobeItem {
	return WardrobeItem{ID: id, Name: id, Category: category, Availability: availability, Revision: 1}
}

func TestRecommendationUsesOnlyAvailableCompleteOwnedWardrobe(t *testing.T) {
	archived := recommendationItem("0-archived", WardrobeTop, WardrobeWearable)
	archivedAt := time.Now()
	archived.ArchivedAt = &archivedAt
	repository := &wardrobeRepositoryStub{page: WardrobePage{Items: []WardrobeItem{
		recommendationItem("1-laundry", WardrobeTop, WardrobeLaundry),
		recommendationItem("2-top", WardrobeTop, WardrobeWearable),
		recommendationItem("3-bottom", WardrobeBottom, WardrobeWearable),
		recommendationItem("4-shoes", WardrobeShoes, WardrobeWearable),
		recommendationItem("5-packed", WardrobeTop, WardrobePacked),
		archived,
	}}}
	service := newWardrobeServiceForTest(t, repository)
	input := RecommendationContext{LocalDate: "2026-09-26", TimeZone: "Asia/Shanghai"}
	result, err := service.RecommendWardrobe(context.Background(), "session", input)
	if err != nil {
		t.Fatal(err)
	}
	if repository.ownerID != "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11" || repository.lifecycle != string(WardrobeActive) {
		t.Fatal("recommendation did not read the authenticated active wardrobe")
	}
	if result.Gap != "" || len(result.Candidates) != 1 || len(result.Candidates[0].Items) != 3 || result.Candidates[0].Items[0].ID != "2-top" {
		t.Fatalf("unexpected candidate: %+v", result)
	}
	input.IncludePackedItems = true
	result, err = service.RecommendWardrobe(context.Background(), "session", input)
	if err != nil || len(result.Candidates) != 2 {
		t.Fatalf("explicit packed allowance did not add the second real candidate: %+v %v", result, err)
	}
	if !slices.Contains(result.Candidates[1].Reasons, "explicit_packed_items") {
		t.Fatal("packed candidate must explain its explicit allowance")
	}
}

func TestRecommendationEnforcesConfirmedConstraintsAndPreservesUnknown(t *testing.T) {
	top := recommendationItem("1-top", WardrobeTop, WardrobeWearable)
	bottom := recommendationItem("2-bottom", WardrobeBottom, WardrobeWearable)
	shoes := recommendationItem("3-shoes", WardrobeShoes, WardrobeWearable)
	input := RecommendationContext{LocalDate: "2026-09-26", TimeZone: "Asia/Shanghai"}
	result := generateRecommendation(input, []WardrobeItem{top, bottom, shoes})
	if len(result.Candidates) != 1 || len(result.Candidates[0].Uncertainties) != 4 {
		t.Fatalf("unknown confirmed fields were hidden: %+v", result)
	}
	input.FormalityBand = wardrobeValue(WardrobeFormalitySmartCasual)
	result = generateRecommendation(input, []WardrobeItem{top, bottom, shoes})
	if result.Gap != "confirmed_constraints_conflict" || len(result.Candidates) != 0 {
		t.Fatalf("unknown formality satisfied a hard constraint: %+v", result)
	}
	for _, item := range []*WardrobeItem{&top, &bottom, &shoes} {
		item.Attributes.FormalityBand = wardrobeValue(WardrobeFormalitySmartCasual)
	}
	input.FormalityBand = wardrobeValue(WardrobeFormalitySmartCasual)
	input.RequiresWalkingSuitability = true
	result = generateRecommendation(input, []WardrobeItem{top, bottom, shoes})
	if result.Gap != "confirmed_constraints_conflict" {
		t.Fatal("unknown walking suitability satisfied a hard constraint")
	}
	shoes.Attributes.WalkingUse = wardrobeValue(WardrobeUseSuitable)
	result = generateRecommendation(input, []WardrobeItem{top, bottom, shoes})
	if len(result.Candidates) != 1 || result.Candidates[0].Reasons[len(result.Candidates[0].Reasons)-1] != "confirmed_walking_shoes" {
		t.Fatalf("confirmed walking shoes not explained: %+v", result)
	}
}

func TestRecommendationReportsMissingCompletePath(t *testing.T) {
	input := RecommendationContext{LocalDate: "2026-09-26", TimeZone: "Asia/Shanghai"}
	cases := []struct {
		items []WardrobeItem
		gap   string
	}{
		{nil, "no_available_items"},
		{[]WardrobeItem{recommendationItem("1", WardrobeTop, WardrobeWearable)}, "missing_shoes"},
		{[]WardrobeItem{recommendationItem("1", WardrobeShoes, WardrobeWearable)}, "missing_top_or_one_piece"},
		{[]WardrobeItem{recommendationItem("1", WardrobeTop, WardrobeWearable), recommendationItem("2", WardrobeShoes, WardrobeWearable)}, "missing_bottom_for_top"},
	}
	for _, test := range cases {
		result := generateRecommendation(input, test.items)
		if result.Gap != test.gap || len(result.Candidates) != 0 {
			t.Fatalf("gap = %q, want %q", result.Gap, test.gap)
		}
	}
}

func TestRecommendationRejectsInvalidContextAndSession(t *testing.T) {
	repository := new(wardrobeRepositoryStub)
	service := newWardrobeServiceForTest(t, repository)
	for _, input := range []RecommendationContext{{LocalDate: "2026-02-30", TimeZone: "Asia/Shanghai"}, {LocalDate: "2026-09-26", TimeZone: "Invalid/Zone"}, {LocalDate: "2026-09-26", TimeZone: "Asia/Shanghai", FormalityBand: wardrobeValue(WardrobeFormalityBand("guessed"))}} {
		if _, err := service.RecommendWardrobe(context.Background(), "session", input); !errors.Is(err, ErrInvalidWardrobeInput) {
			t.Fatalf("invalid context accepted: %+v %v", input, err)
		}
	}
	denied, err := NewWardrobeService(wardrobeAuthenticatorStub{err: accountapp.ErrAuthentication}, repository)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := denied.RecommendWardrobe(context.Background(), "stale", RecommendationContext{LocalDate: "2026-09-26", TimeZone: "Asia/Shanghai"}); !errors.Is(err, accountapp.ErrAuthentication) {
		t.Fatalf("invalid session reached recommendation: %v", err)
	}
}
