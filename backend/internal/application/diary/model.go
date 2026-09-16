package diary

import (
	"errors"
	"time"
)

var (
	ErrInvalidDiaryInput = errors.New("invalid diary input")
	ErrDiaryNotFound     = errors.New("diary entry not found")
	ErrDiaryConflict     = errors.New("diary entry conflicts with request")
	ErrDiaryUnavailable  = errors.New("diary service unavailable")
)

type DiaryEntryInput struct {
	LocalDate   string
	TimeZone    string
	Title       *string
	Body        *string
	Mood        *string
	Occasion    *string
	PlanID      *string
	WearEventID *string
	MediaIDs    []string
}

type DiaryEntry struct {
	ID          string
	LocalDate   string
	TimeZone    string
	Title       *string
	Body        *string
	Mood        *string
	Occasion    *string
	PlanID      *string
	WearEventID *string
	MediaIDs    []string
	Revision    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type DiaryEntryPage struct {
	Entries     []DiaryEntry
	NextAfterID *string
}

type DiaryDeletionImpact struct {
	EntryID            string
	Revision           int
	MediaCount         int
	PublishedPostCount int
	MediaRetained      bool
}

type CalendarDay struct {
	LocalDate      string
	PlanCount      int
	WearEventCount int
	DiaryCount     int
}

type CalendarMonth struct {
	Month string
	Days  []CalendarDay
}
