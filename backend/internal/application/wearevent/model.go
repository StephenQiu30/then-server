package wearevent

import (
	"errors"
	"time"

	outfitplanapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitplan"
)

var (
	ErrInvalidWearEventInput       = errors.New("invalid wear event input")
	ErrWearEventNotFound           = errors.New("wear event not found")
	ErrWearEventConflict           = errors.New("wear event conflict")
	ErrWearEventDuplicate          = errors.New("wear event duplicate confirmation required")
	ErrWearEventItemsUnavailable   = errors.New("wear event items unavailable")
	ErrWearEventServiceUnavailable = errors.New("wear event service unavailable")
)

type WearEventCompleteness string

const (
	WearEventPartial  WearEventCompleteness = "partial"
	WearEventComplete WearEventCompleteness = "complete"
)

type WearEventSourceKind string

const (
	WearEventFollowedPlan    WearEventSourceKind = "followed_plan"
	WearEventChangedPlan     WearEventSourceKind = "changed_plan"
	WearEventDifferentOutfit WearEventSourceKind = "different_outfit"
	WearEventUnplanned       WearEventSourceKind = "unplanned"
)

type WearEventCandidate struct {
	ID       string
	Revision int
}

type WearEventInput struct {
	LocalDate               string
	TimeZone                string
	Completeness            WearEventCompleteness
	ContextSummary          *string
	Items                   []outfitplanapp.OutfitSelection
	LaundryItemIDs          []string
	ConfirmedUnavailableIDs []string
	SourcePlanID            *string
	SourcePlanRevision      *int
	SourceKind              WearEventSourceKind
	DuplicateConfirmations  []WearEventCandidate
}

type WearEvent struct {
	ID                 string
	OwnerID            string
	LocalDate          string
	TimeZone           string
	Completeness       WearEventCompleteness
	ContextSummary     *string
	SourcePlanID       *string
	SourcePlanRevision *int
	SourceKind         WearEventSourceKind
	Revision           int
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Items              []outfitplanapp.OutfitPlanItemSnapshot
}

type WearEventPage struct {
	Events      []WearEvent
	NextAfterID *string
}

type WearEventDuplicateError struct {
	Candidates []WearEventCandidate
}

func (e *WearEventDuplicateError) Error() string { return ErrWearEventDuplicate.Error() }
func (e *WearEventDuplicateError) Unwrap() error { return ErrWearEventDuplicate }
