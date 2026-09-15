package model

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

type WardrobeItem struct {
	ID           string
	OwnerID      string
	Name         string
	Category     WardrobeCategory
	Availability WardrobeAvailability
	Source       WardrobeSource
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
}

type UpdateWardrobeItemInput struct {
	Name         string
	Category     WardrobeCategory
	Availability WardrobeAvailability
}

type WardrobePage struct {
	Items       []WardrobeItem
	NextAfterID *string
}
