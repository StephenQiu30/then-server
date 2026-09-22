package httpapi

import (
	"time"
)

type DiaryEntryRequest struct {
	LocalDate   string   `json:"local_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	TimeZone    string   `json:"time_zone" minLength:"1" maxLength:"255"`
	Title       *string  `json:"title,omitempty" maxLength:"80"`
	Body        *string  `json:"body,omitempty" maxLength:"5000"`
	Mood        *string  `json:"mood,omitempty" maxLength:"40"`
	Occasion    *string  `json:"occasion,omitempty" maxLength:"40"`
	PlanID      *string  `json:"plan_id,omitempty" format:"uuid"`
	WearEventID *string  `json:"wear_event_id,omitempty" format:"uuid"`
	MediaIDs    []string `json:"media_ids" maxItems:"9"`
}

type CreateDiaryEntryRequest struct {
	ID string `json:"id" format:"uuid"`
	DiaryEntryRequest
}

type UpdateDiaryEntryRequest struct {
	ExpectedRevision int `json:"expected_revision" minimum:"1"`
	DiaryEntryRequest
}

type DiaryEntryResponse struct {
	ID          string    `json:"id" format:"uuid"`
	LocalDate   string    `json:"local_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	TimeZone    string    `json:"time_zone" minLength:"1" maxLength:"255"`
	Title       *string   `json:"title,omitempty" maxLength:"80"`
	Body        *string   `json:"body,omitempty" maxLength:"5000"`
	Mood        *string   `json:"mood,omitempty" maxLength:"40"`
	Occasion    *string   `json:"occasion,omitempty" maxLength:"40"`
	PlanID      *string   `json:"plan_id,omitempty" format:"uuid"`
	WearEventID *string   `json:"wear_event_id,omitempty" format:"uuid"`
	MediaIDs    []string  `json:"media_ids" maxItems:"9"`
	Revision    int       `json:"revision" minimum:"1"`
	CreatedAt   time.Time `json:"created_at" format:"date-time"`
	UpdatedAt   time.Time `json:"updated_at" format:"date-time"`
}

type DiaryEntryPageResponse struct {
	Entries     []DiaryEntryResponse `json:"entries" maxItems:"50"`
	NextAfterID *string              `json:"next_after_id,omitempty" format:"uuid"`
}

type DiaryDeletionImpactResponse struct {
	EntryID            string `json:"entry_id" format:"uuid"`
	Revision           int    `json:"revision" minimum:"1"`
	MediaCount         int    `json:"media_count" minimum:"0" maximum:"9"`
	PublishedPostCount int    `json:"published_post_count" minimum:"0"`
	MediaRetained      bool   `json:"media_retained"`
}

type CalendarDayResponse struct {
	LocalDate      string `json:"local_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	PlanCount      int    `json:"plan_count" minimum:"0"`
	WearEventCount int    `json:"wear_event_count" minimum:"0"`
	DiaryCount     int    `json:"diary_count" minimum:"0"`
}

type CalendarMonthResponse struct {
	Month string                `json:"month" pattern:"^[0-9]{4}-[0-9]{2}$"`
	Days  []CalendarDayResponse `json:"days" maxItems:"31"`
}

type createDiaryEntryInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    CreateDiaryEntryRequest
}

type listDiaryEntriesInput struct {
	Session  string `cookie:"then_session" hidden:"true"`
	Limit    int    `query:"limit" default:"20" minimum:"1" maximum:"50"`
	AfterID  string `query:"after_id" format:"uuid" required:"false"`
	DateFrom string `query:"date_from" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$" required:"false"`
	DateTo   string `query:"date_to" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$" required:"false"`
}

type diaryEntryInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"entry_id" format:"uuid"`
}

type updateDiaryEntryInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"entry_id" format:"uuid"`
	Body    UpdateDiaryEntryRequest
}

type deleteDiaryEntryInput struct {
	Session          string `cookie:"then_session" hidden:"true"`
	ID               string `path:"entry_id" format:"uuid"`
	ExpectedRevision int    `query:"expected_revision" minimum:"1"`
}

type calendarInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Month   string `query:"month" pattern:"^[0-9]{4}-[0-9]{2}$"`
}

type diaryEntryOutput struct {
	RequestID string             `header:"X-Request-ID"`
	Body      DiaryEntryResponse `json:"body"`
}

type diaryEntryPageOutput struct {
	RequestID string                 `header:"X-Request-ID"`
	Body      DiaryEntryPageResponse `json:"body"`
}

type diaryDeletionImpactOutput struct {
	RequestID string                      `header:"X-Request-ID"`
	Body      DiaryDeletionImpactResponse `json:"body"`
}

type calendarOutput struct {
	RequestID string                `header:"X-Request-ID"`
	Body      CalendarMonthResponse `json:"body"`
}

type deleteDiaryEntryOutput struct {
	RequestID string `header:"X-Request-ID"`
}
