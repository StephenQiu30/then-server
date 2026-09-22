package httpapi

import (
	"time"

	weareventapp "github.com/StephenQiu30/then-server/backend/internal/application/wearevent"
)

type WearEventCandidateResponse struct {
	ID       string `json:"id" format:"uuid"`
	Revision int    `json:"revision" minimum:"1"`
}

type WearEventFieldsRequest struct {
	LocalDate               string                             `json:"local_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	TimeZone                string                             `json:"time_zone" minLength:"1" maxLength:"255"`
	Completeness            weareventapp.WearEventCompleteness `json:"completeness" enum:"partial,complete"`
	ContextSummary          *string                            `json:"context_summary,omitempty" maxLength:"120"`
	Items                   []OutfitSelectionRequest           `json:"items" minItems:"1" maxItems:"20"`
	LaundryItemIDs          []string                           `json:"laundry_item_ids" maxItems:"20"`
	ConfirmedUnavailableIDs []string                           `json:"confirmed_unavailable_ids" maxItems:"20"`
	SourcePlanID            *string                            `json:"source_plan_id,omitempty" format:"uuid"`
	SourcePlanRevision      *int                               `json:"source_plan_revision,omitempty" minimum:"1"`
	SourceKind              weareventapp.WearEventSourceKind   `json:"source_kind" enum:"followed_plan,changed_plan,different_outfit,unplanned"`
	DuplicateConfirmations  []WearEventCandidateResponse       `json:"duplicate_confirmations" maxItems:"50"`
}

type CreateWearEventRequest struct {
	ID string `json:"id" format:"uuid"`
	WearEventFieldsRequest
}

type UpdateWearEventRequest struct {
	ExpectedRevision int `json:"expected_revision" minimum:"1"`
	WearEventFieldsRequest
}

type WearEventResponse struct {
	ID                 string                             `json:"id" format:"uuid"`
	LocalDate          string                             `json:"local_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	TimeZone           string                             `json:"time_zone" minLength:"1" maxLength:"255"`
	Completeness       weareventapp.WearEventCompleteness `json:"completeness" enum:"partial,complete"`
	ContextSummary     *string                            `json:"context_summary,omitempty" maxLength:"120"`
	SourcePlanID       *string                            `json:"source_plan_id,omitempty" format:"uuid"`
	SourcePlanRevision *int                               `json:"source_plan_revision,omitempty" minimum:"1"`
	SourceKind         weareventapp.WearEventSourceKind   `json:"source_kind" enum:"followed_plan,changed_plan,different_outfit,unplanned"`
	Revision           int                                `json:"revision" minimum:"1"`
	CreatedAt          time.Time                          `json:"created_at" format:"date-time"`
	UpdatedAt          time.Time                          `json:"updated_at" format:"date-time"`
	Items              []OutfitPlanItemResponse           `json:"items" minItems:"1" maxItems:"20"`
}

type WearEventPageResponse struct {
	Events      []WearEventResponse `json:"events" maxItems:"50"`
	NextAfterID *string             `json:"next_after_id,omitempty" format:"uuid"`
}

type createWearEventInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    CreateWearEventRequest
}

type listWearEventsInput struct {
	Session   string `cookie:"then_session" hidden:"true"`
	Limit     int    `query:"limit" default:"20" minimum:"1" maximum:"50"`
	AfterID   string `query:"after_id" format:"uuid" required:"false"`
	LocalDate string `query:"local_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$" required:"false"`
}

type wearEventInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"wear_event_id" format:"uuid"`
}

type updateWearEventInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"wear_event_id" format:"uuid"`
	Body    UpdateWearEventRequest
}

type deleteWearEventInput struct {
	Session          string `cookie:"then_session" hidden:"true"`
	ID               string `path:"wear_event_id" format:"uuid"`
	ExpectedRevision int    `query:"expected_revision" minimum:"1"`
}

type wearEventOutput struct {
	RequestID string            `header:"X-Request-ID"`
	Body      WearEventResponse `json:"body"`
}

type wearEventPageOutput struct {
	RequestID string                `header:"X-Request-ID"`
	Body      WearEventPageResponse `json:"body"`
}

type deleteWearEventOutput struct {
	RequestID string `header:"X-Request-ID"`
}
