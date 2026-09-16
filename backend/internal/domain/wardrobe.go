package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidWardrobeInput = errors.New("invalid wardrobe input")
	ErrWardrobeNotFound     = errors.New("wardrobe item not found")
	ErrWardrobeConflict     = errors.New("wardrobe item conflict")
	ErrWardrobeUnavailable  = errors.New("wardrobe service unavailable")
)

type WardrobeCategory string

const (
	WardrobeTop       WardrobeCategory = "top"
	WardrobeBottom    WardrobeCategory = "bottom"
	WardrobeOnePiece  WardrobeCategory = "one_piece"
	WardrobeOuterwear WardrobeCategory = "outerwear"
	WardrobeShoes     WardrobeCategory = "shoes"
	WardrobeBag       WardrobeCategory = "bag"
	WardrobeAccessory WardrobeCategory = "accessory"
)

type WardrobeAvailability string

const (
	WardrobeWearable WardrobeAvailability = "wearable"
	WardrobeLaundry  WardrobeAvailability = "laundry"
	WardrobeLentOut  WardrobeAvailability = "lent_out"
	WardrobePacked   WardrobeAvailability = "packed"
)

type WardrobeSource string

const (
	WardrobeSourceWardrobe WardrobeSource = "wardrobe"
	WardrobeSourceQuickAdd WardrobeSource = "quick_add"
)

type WardrobeFormalityBand string

const (
	WardrobeFormalityCasual      WardrobeFormalityBand = "casual"
	WardrobeFormalitySmartCasual WardrobeFormalityBand = "smart_casual"
	WardrobeFormalityFormal      WardrobeFormalityBand = "formal"
)

type WardrobeWarmthBand string

const (
	WardrobeWarmthLight  WardrobeWarmthBand = "light"
	WardrobeWarmthMedium WardrobeWarmthBand = "medium"
	WardrobeWarmthWarm   WardrobeWarmthBand = "warm"
)

type WardrobeUseSuitability string

const (
	WardrobeUseSuitable   WardrobeUseSuitability = "suitable"
	WardrobeUseUnsuitable WardrobeUseSuitability = "unsuitable"
)

type WardrobeAttributeSource string

const WardrobeAttributeUserConfirmed WardrobeAttributeSource = "user_confirmed"

type WardrobeAttributes struct {
	FormalityBand *WardrobeFormalityBand
	WarmthBand    *WardrobeWarmthBand
	RainUse       *WardrobeUseSuitability
	WalkingUse    *WardrobeUseSuitability
}

type WardrobeItem struct {
	ID           string
	OwnerID      string
	Name         string
	Category     WardrobeCategory
	Availability WardrobeAvailability
	Source       WardrobeSource
	Attributes   WardrobeAttributes
	Revision     int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateWardrobeItemInput struct {
	ID           string
	Name         string
	Category     WardrobeCategory
	Availability WardrobeAvailability
	Source       WardrobeSource
	Attributes   WardrobeAttributes
}

type UpdateWardrobeItemInput struct {
	Name         string
	Category     WardrobeCategory
	Availability WardrobeAvailability
	Attributes   WardrobeAttributes
}

type WardrobePage struct {
	Items       []WardrobeItem
	NextAfterID *string
}
