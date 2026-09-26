package wardrobe

import (
	"context"
	"slices"
	"time"
)

const RecommendationPolicyVersion = "wardrobe-hard-constraints-v1"

type RecommendationContext struct {
	LocalDate                  string
	TimeZone                   string
	FormalityBand              *WardrobeFormalityBand
	WarmthBand                 *WardrobeWarmthBand
	RequiresRainSuitability    bool
	RequiresWalkingSuitability bool
	IncludePackedItems         bool
}

type RecommendationCandidate struct {
	Items         []WardrobeItem
	Reasons       []string
	Uncertainties []string
}

type RecommendationResult struct {
	PolicyVersion string
	Candidates    []RecommendationCandidate
	Gap           string
}

func (s *WardrobeService) RecommendWardrobe(ctx context.Context, token string, input RecommendationContext) (RecommendationResult, error) {
	user, err := s.authenticator.CurrentUser(ctx, token)
	if err != nil {
		return RecommendationResult{}, err
	}
	if !validRecommendationContext(input) {
		return RecommendationResult{}, ErrInvalidWardrobeInput
	}
	items := make([]WardrobeItem, 0)
	var afterID *string
	for {
		page, err := s.repository.ListWardrobeItems(ctx, user.ID, 100, afterID, WardrobeListFilter{Lifecycle: WardrobeActive})
		if err != nil {
			return RecommendationResult{}, err
		}
		for _, item := range page.Items {
			if item.ArchivedAt == nil && (item.Availability == WardrobeWearable || (input.IncludePackedItems && item.Availability == WardrobePacked)) {
				items = append(items, item)
			}
		}
		if page.NextAfterID == nil {
			break
		}
		if afterID != nil && *afterID == *page.NextAfterID {
			return RecommendationResult{}, ErrWardrobeUnavailable
		}
		afterID = page.NextAfterID
	}
	return generateRecommendation(input, items), nil
}

func validRecommendationContext(value RecommendationContext) bool {
	date, err := time.Parse("2006-01-02", value.LocalDate)
	if err != nil || date.Format("2006-01-02") != value.LocalDate || value.TimeZone == "" || len(value.TimeZone) > 64 {
		return false
	}
	if _, err := time.LoadLocation(value.TimeZone); err != nil {
		return false
	}
	return validWardrobeAttributes(WardrobeAttributes{FormalityBand: value.FormalityBand, WarmthBand: value.WarmthBand})
}

func generateRecommendation(input RecommendationContext, items []WardrobeItem) RecommendationResult {
	result := RecommendationResult{PolicyVersion: RecommendationPolicyVersion, Candidates: []RecommendationCandidate{}}
	if len(items) == 0 {
		result.Gap = "no_available_items"
		return result
	}
	slices.SortFunc(items, func(a, b WardrobeItem) int { return compareStrings(a.ID, b.ID) })
	var tops, bottoms, onePieces, shoes []WardrobeItem
	for _, item := range items {
		switch item.Category {
		case WardrobeTop:
			tops = append(tops, item)
		case WardrobeBottom:
			bottoms = append(bottoms, item)
		case WardrobeOnePiece:
			onePieces = append(onePieces, item)
		case WardrobeShoes:
			shoes = append(shoes, item)
		}
	}
	if len(shoes) == 0 {
		result.Gap = "missing_shoes"
		return result
	}
	if len(onePieces) == 0 && len(tops) == 0 {
		result.Gap = "missing_top_or_one_piece"
		return result
	}
	if len(onePieces) == 0 && len(bottoms) == 0 {
		result.Gap = "missing_bottom_for_top"
		return result
	}
	eligible := func(item WardrobeItem) bool {
		return (input.FormalityBand == nil || (item.Attributes.FormalityBand != nil && *item.Attributes.FormalityBand == *input.FormalityBand)) &&
			(input.WarmthBand == nil || (item.Attributes.WarmthBand != nil && *item.Attributes.WarmthBand == *input.WarmthBand))
	}
	var eligibleShoes []WardrobeItem
	for _, shoe := range shoes {
		if !eligible(shoe) || (input.RequiresRainSuitability && (shoe.Attributes.RainUse == nil || *shoe.Attributes.RainUse != WardrobeUseSuitable)) ||
			(input.RequiresWalkingSuitability && (shoe.Attributes.WalkingUse == nil || *shoe.Attributes.WalkingUse != WardrobeUseSuitable)) {
			continue
		}
		eligibleShoes = append(eligibleShoes, shoe)
	}
	if len(eligibleShoes) == 0 {
		result.Gap = "confirmed_constraints_conflict"
		return result
	}
	var eligibleBottoms []WardrobeItem
	for _, bottom := range bottoms {
		if eligible(bottom) {
			eligibleBottoms = append(eligibleBottoms, bottom)
		}
	}
	for _, dress := range onePieces {
		if eligible(dress) {
			shoe := eligibleShoes[len(result.Candidates)%len(eligibleShoes)]
			result.Candidates = append(result.Candidates, recommendationCandidate([]WardrobeItem{dress, shoe}, "complete_one_piece_path", input))
			if len(result.Candidates) == 3 {
				return result
			}
		}
	}
	for _, top := range tops {
		if !eligible(top) {
			continue
		}
		if len(eligibleBottoms) > 0 {
			index := len(result.Candidates)
			result.Candidates = append(result.Candidates, recommendationCandidate([]WardrobeItem{top, eligibleBottoms[index%len(eligibleBottoms)], eligibleShoes[index%len(eligibleShoes)]}, "complete_separate_path", input))
		}
		if len(result.Candidates) == 3 {
			return result
		}
	}
	if len(result.Candidates) == 0 {
		result.Gap = "confirmed_constraints_conflict"
	}
	return result
}

func recommendationCandidate(items []WardrobeItem, path string, input RecommendationContext) RecommendationCandidate {
	candidate := RecommendationCandidate{Items: items, Reasons: []string{"real_owner_items", path}, Uncertainties: []string{}}
	for _, item := range items {
		if item.Availability == WardrobePacked {
			candidate.Reasons = append(candidate.Reasons, "explicit_packed_items")
			break
		}
	}
	if input.FormalityBand != nil {
		candidate.Reasons = append(candidate.Reasons, "confirmed_formality")
	}
	if input.WarmthBand != nil {
		candidate.Reasons = append(candidate.Reasons, "confirmed_warmth")
	}
	if input.RequiresRainSuitability {
		candidate.Reasons = append(candidate.Reasons, "confirmed_rain_shoes")
	}
	if input.RequiresWalkingSuitability {
		candidate.Reasons = append(candidate.Reasons, "confirmed_walking_shoes")
	}
	for _, item := range items {
		if input.FormalityBand == nil && item.Attributes.FormalityBand == nil {
			candidate.Uncertainties = appendUnique(candidate.Uncertainties, "some_formality_unknown")
		}
		if input.WarmthBand == nil && item.Attributes.WarmthBand == nil {
			candidate.Uncertainties = appendUnique(candidate.Uncertainties, "some_warmth_unknown")
		}
		if item.Attributes.RainUse == nil {
			candidate.Uncertainties = appendUnique(candidate.Uncertainties, "some_rain_suitability_unknown")
		}
		if item.Attributes.WalkingUse == nil {
			candidate.Uncertainties = appendUnique(candidate.Uncertainties, "some_walking_suitability_unknown")
		}
	}
	return candidate
}

func appendUnique(values []string, value string) []string {
	if !slices.Contains(values, value) {
		return append(values, value)
	}
	return values
}

func compareStrings(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
