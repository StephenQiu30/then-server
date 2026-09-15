package model

import (
	"errors"
	"time"
)

var (
	ErrInvalidOutfitPlanInput = errors.New("invalid outfit plan input")
	ErrOutfitPlanNotFound     = errors.New("outfit plan not found")
	ErrOutfitPlanConflict     = errors.New("outfit plan conflict")
	ErrOutfitItemsUnavailable = errors.New("outfit plan items unavailable")
	ErrOutfitPlanUnavailable  = errors.New("outfit plan service unavailable")
)

type OutfitPlanStatus string

const (
	OutfitPlanActive    OutfitPlanStatus = "active"
	OutfitPlanCompleted OutfitPlanStatus = "completed"
	OutfitPlanNotWorn   OutfitPlanStatus = "not_worn"
	OutfitPlanCancelled OutfitPlanStatus = "cancelled"
)

type OutfitSelection struct {
	ItemID   string
	Revision int
}

type OutfitPlanInput struct {
	LocalDate               string
	TimeZone                string
	ContextSummary          *string
	Items                   []OutfitSelection
	ConfirmedUnavailableIDs []string
}

type OutfitItemContent struct {
	ItemID       string
	ItemRevision int
	Name         string
	Category     WardrobeCategory
	Availability WardrobeAvailability
	Attributes   WardrobeAttributes
}

type OutfitPlanItemSnapshot struct {
	Ordinal int
	Content *OutfitItemContent
}

type OutfitPlan struct {
	ID             string
	OwnerID        string
	LocalDate      string
	TimeZone       string
	ContextSummary *string
	Status         OutfitPlanStatus
	Revision       int
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Items          []OutfitPlanItemSnapshot
}

type OutfitPlanPage struct {
	Plans       []OutfitPlan
	NextAfterID *string
}

type WardrobeHistoryPolicy string

const (
	WardrobeHistoryRedactSnapshots       WardrobeHistoryPolicy = "redact_snapshots"
	WardrobeHistoryDeleteAffectedHistory WardrobeHistoryPolicy = "delete_affected_history"
)

type WardrobeDeletionImpact struct {
	AffectedPlanCount      int
	AffectedWearEventCount int
	ExpectedImpact         string
}
