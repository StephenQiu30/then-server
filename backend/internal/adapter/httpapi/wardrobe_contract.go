package httpapi

import (
	"time"

	wardrobeapp "github.com/StephenQiu30/then-server/backend/internal/application/wardrobe"
)

type CreateWardrobeItemRequest struct {
	ID           string                           `json:"id" format:"uuid" doc:"客户端生成的稳定衣物 ID"`
	Name         string                           `json:"name" minLength:"1" maxLength:"80" doc:"用户确认的衣物名称"`
	Category     wardrobeapp.WardrobeCategory     `json:"category" enum:"top,bottom,one_piece,outerwear,shoes,bag,accessory"`
	Availability wardrobeapp.WardrobeAvailability `json:"availability" enum:"wearable,laundry,lent_out,packed"`
	Source       wardrobeapp.WardrobeSource       `json:"source" enum:"wardrobe,quick_add"`
	Attributes   WardrobeAttributesRequest        `json:"attributes" doc:"用户明确确认的可选衣物属性；空对象表示全部未知"`
}

type UpdateWardrobeItemRequest struct {
	ExpectedRevision int                              `json:"expected_revision" minimum:"1"`
	Name             string                           `json:"name" minLength:"1" maxLength:"80"`
	Category         wardrobeapp.WardrobeCategory     `json:"category" enum:"top,bottom,one_piece,outerwear,shoes,bag,accessory"`
	Availability     wardrobeapp.WardrobeAvailability `json:"availability" enum:"wearable,laundry,lent_out,packed"`
	Attributes       WardrobeAttributesRequest        `json:"attributes" doc:"完整替换的用户确认属性；NULL 表示清除为未知"`
}

type WardrobeAttributesRequest struct {
	FormalityBand *wardrobeapp.WardrobeFormalityBand  `json:"formality_band,omitempty" enum:"casual,smart_casual,formal" doc:"用户确认的正式度；省略或 NULL 表示未知"`
	WarmthBand    *wardrobeapp.WardrobeWarmthBand     `json:"warmth_band,omitempty" enum:"light,medium,warm" doc:"用户确认的保暖感受；省略或 NULL 表示未知"`
	RainUse       *wardrobeapp.WardrobeUseSuitability `json:"rain_use,omitempty" enum:"suitable,unsuitable" doc:"用户确认的雨天适用判断；省略或 NULL 表示未知"`
	WalkingUse    *wardrobeapp.WardrobeUseSuitability `json:"walking_use,omitempty" enum:"suitable,unsuitable" doc:"用户确认的步行适用判断；省略或 NULL 表示未知"`
}

type WardrobeFormalityAttributeResponse struct {
	Value  wardrobeapp.WardrobeFormalityBand   `json:"value" enum:"casual,smart_casual,formal"`
	Source wardrobeapp.WardrobeAttributeSource `json:"source" enum:"user_confirmed"`
}

type WardrobeWarmthAttributeResponse struct {
	Value  wardrobeapp.WardrobeWarmthBand      `json:"value" enum:"light,medium,warm"`
	Source wardrobeapp.WardrobeAttributeSource `json:"source" enum:"user_confirmed"`
}

type WardrobeSuitabilityAttributeResponse struct {
	Value  wardrobeapp.WardrobeUseSuitability  `json:"value" enum:"suitable,unsuitable"`
	Source wardrobeapp.WardrobeAttributeSource `json:"source" enum:"user_confirmed"`
}

type WardrobeAttributesResponse struct {
	FormalityBand *WardrobeFormalityAttributeResponse   `json:"formality_band"`
	WarmthBand    *WardrobeWarmthAttributeResponse      `json:"warmth_band"`
	RainUse       *WardrobeSuitabilityAttributeResponse `json:"rain_use"`
	WalkingUse    *WardrobeSuitabilityAttributeResponse `json:"walking_use"`
}

type WardrobeItemResponse struct {
	ID           string                           `json:"id" format:"uuid"`
	Name         string                           `json:"name" minLength:"1" maxLength:"80"`
	Category     wardrobeapp.WardrobeCategory     `json:"category" enum:"top,bottom,one_piece,outerwear,shoes,bag,accessory"`
	Availability wardrobeapp.WardrobeAvailability `json:"availability" enum:"wearable,laundry,lent_out,packed"`
	Source       wardrobeapp.WardrobeSource       `json:"source" enum:"wardrobe,quick_add"`
	Attributes   WardrobeAttributesResponse       `json:"attributes"`
	Lifecycle    wardrobeapp.WardrobeLifecycle    `json:"lifecycle" enum:"active,archived"`
	ArchivedAt   *time.Time                       `json:"archived_at,omitempty" format:"date-time"`
	Revision     int                              `json:"revision" minimum:"1"`
	CreatedAt    time.Time                        `json:"created_at" format:"date-time"`
	UpdatedAt    time.Time                        `json:"updated_at" format:"date-time"`
}

type WardrobePageResponse struct {
	Items       []WardrobeItemResponse `json:"items" maxItems:"100"`
	NextAfterID *string                `json:"next_after_id,omitempty" format:"uuid"`
}

type WardrobeDeletionImpactResponse struct {
	AffectedPlanCount      int    `json:"affected_plan_count" minimum:"0"`
	AffectedWearEventCount int    `json:"affected_wear_event_count" minimum:"0"`
	ExpectedImpact         string `json:"expected_impact" minLength:"64" maxLength:"64" pattern:"^[0-9a-f]{64}$" doc:"确认删除影响所需的不透明摘要"`
}

type createWardrobeItemInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    CreateWardrobeItemRequest
}

type listWardrobeItemsInput struct {
	Session      string                           `cookie:"then_session" hidden:"true"`
	Limit        int                              `query:"limit" default:"50" minimum:"1" maximum:"100"`
	AfterID      string                           `query:"after_id" format:"uuid" required:"false"`
	Lifecycle    wardrobeapp.WardrobeLifecycle    `query:"lifecycle" enum:"active,archived,all" default:"active"`
	Availability wardrobeapp.WardrobeAvailability `query:"availability" enum:"wearable,laundry,lent_out,packed" required:"false"`
}

type wardrobeItemInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"item_id" format:"uuid"`
}

type updateWardrobeItemInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"item_id" format:"uuid"`
	Body    UpdateWardrobeItemRequest
}

type transitionWardrobeItemInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"item_id" format:"uuid"`
	Body    WardrobeLifecycleRequest
}

type WardrobeLifecycleRequest struct {
	ExpectedRevision int `json:"expected_revision" minimum:"1"`
}

type deleteWardrobeItemInput struct {
	Session          string                            `cookie:"then_session" hidden:"true"`
	ID               string                            `path:"item_id" format:"uuid"`
	ExpectedRevision int                               `query:"expected_revision" minimum:"1"`
	HistoryPolicy    wardrobeapp.WardrobeHistoryPolicy `query:"history_policy" enum:"redact_snapshots,delete_affected_history"`
	ExpectedImpact   string                            `query:"expected_impact" minLength:"64" maxLength:"64" pattern:"^[0-9a-f]{64}$"`
}

type wardrobeItemOutput struct {
	RequestID string               `header:"X-Request-ID"`
	Body      WardrobeItemResponse `json:"body"`
}

type wardrobePageOutput struct {
	RequestID string               `header:"X-Request-ID"`
	Body      WardrobePageResponse `json:"body"`
}

type deleteWardrobeItemOutput struct {
	RequestID string `header:"X-Request-ID"`
}

type wardrobeDeletionImpactOutput struct {
	RequestID string                         `header:"X-Request-ID"`
	Body      WardrobeDeletionImpactResponse `json:"body"`
}
