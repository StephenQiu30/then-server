package httpapi

import (
	"time"

	outfitplanapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitplan"
	wardrobeapp "github.com/StephenQiu30/then-server/backend/internal/application/wardrobe"
)

type OutfitSelectionRequest struct {
	ItemID   string `json:"item_id" format:"uuid"`
	Revision int    `json:"revision" minimum:"1"`
}

type CreateOutfitPlanRequest struct {
	ID                      string                   `json:"id" format:"uuid"`
	LocalDate               string                   `json:"local_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	TimeZone                string                   `json:"time_zone" minLength:"1" maxLength:"255"`
	ContextSummary          *string                  `json:"context_summary,omitempty" maxLength:"120"`
	Items                   []OutfitSelectionRequest `json:"items" minItems:"1" maxItems:"20"`
	ConfirmedUnavailableIDs []string                 `json:"confirmed_unavailable_ids" maxItems:"20"`
}

type UpdateOutfitPlanRequest struct {
	ExpectedRevision        int                      `json:"expected_revision" minimum:"1"`
	LocalDate               string                   `json:"local_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	TimeZone                string                   `json:"time_zone" minLength:"1" maxLength:"255"`
	ContextSummary          *string                  `json:"context_summary,omitempty" maxLength:"120"`
	Items                   []OutfitSelectionRequest `json:"items" minItems:"1" maxItems:"20"`
	ConfirmedUnavailableIDs []string                 `json:"confirmed_unavailable_ids" maxItems:"20"`
}

type CancelOutfitPlanRequest struct {
	ExpectedRevision int `json:"expected_revision" minimum:"1"`
}

type TransitionOutfitPlanRequest struct {
	ExpectedRevision int `json:"expected_revision" minimum:"1"`
}

type OutfitPlanItemContentResponse struct {
	ItemID       string                           `json:"item_id" format:"uuid"`
	ItemRevision int                              `json:"item_revision" minimum:"1"`
	Name         string                           `json:"name" minLength:"1" maxLength:"80"`
	Category     wardrobeapp.WardrobeCategory     `json:"category" enum:"top,bottom,one_piece,outerwear,shoes,bag,accessory"`
	Availability wardrobeapp.WardrobeAvailability `json:"availability" enum:"wearable,laundry,lent_out,packed"`
	Attributes   WardrobeAttributesResponse       `json:"attributes"`
}

type OutfitPlanItemResponse struct {
	Ordinal int                            `json:"ordinal" minimum:"0" maximum:"19"`
	Content *OutfitPlanItemContentResponse `json:"content" doc:"NULL 表示衣物已按用户选择从历史快照清除"`
}

type OutfitPlanResponse struct {
	ID             string                         `json:"id" format:"uuid"`
	LocalDate      string                         `json:"local_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	TimeZone       string                         `json:"time_zone" minLength:"1" maxLength:"255"`
	ContextSummary *string                        `json:"context_summary,omitempty" maxLength:"120"`
	Status         outfitplanapp.OutfitPlanStatus `json:"status" enum:"active,completed,not_worn,cancelled"`
	Revision       int                            `json:"revision" minimum:"1"`
	CreatedAt      time.Time                      `json:"created_at" format:"date-time"`
	UpdatedAt      time.Time                      `json:"updated_at" format:"date-time"`
	Items          []OutfitPlanItemResponse       `json:"items" minItems:"1" maxItems:"20"`
}

type OutfitPlanPageResponse struct {
	Plans       []OutfitPlanResponse `json:"plans" maxItems:"50"`
	NextAfterID *string              `json:"next_after_id,omitempty" format:"uuid"`
}

type createOutfitPlanInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    CreateOutfitPlanRequest
}

type listOutfitPlansInput struct {
	Session   string `cookie:"then_session" hidden:"true"`
	Limit     int    `query:"limit" default:"20" minimum:"1" maximum:"50"`
	AfterID   string `query:"after_id" format:"uuid" required:"false"`
	LocalDate string `query:"local_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$" required:"false"`
}

type outfitPlanInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"plan_id" format:"uuid"`
}

type updateOutfitPlanInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"plan_id" format:"uuid"`
	Body    UpdateOutfitPlanRequest
}

type cancelOutfitPlanInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"plan_id" format:"uuid"`
	Body    CancelOutfitPlanRequest
}

type markOutfitPlanNotWornInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"plan_id" format:"uuid"`
	Body    TransitionOutfitPlanRequest
}

type restoreOutfitPlanInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"plan_id" format:"uuid"`
	Body    TransitionOutfitPlanRequest
}

type deleteOutfitPlanInput struct {
	Session          string `cookie:"then_session" hidden:"true"`
	ID               string `path:"plan_id" format:"uuid"`
	ExpectedRevision int    `query:"expected_revision" minimum:"1"`
}

type outfitPlanOutput struct {
	RequestID string             `header:"X-Request-ID"`
	Body      OutfitPlanResponse `json:"body"`
}

type outfitPlanPageOutput struct {
	RequestID string                 `header:"X-Request-ID"`
	Body      OutfitPlanPageResponse `json:"body"`
}

type deleteOutfitPlanOutput struct {
	RequestID string `header:"X-Request-ID"`
}
