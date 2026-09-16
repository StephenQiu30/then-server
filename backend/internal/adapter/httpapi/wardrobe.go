package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	wardrobeapp "github.com/StephenQiu30/then-server/backend/internal/application/wardrobe"

	"github.com/danielgtaylor/huma/v2"
)

type WardrobeHTTPService interface {
	CreateWardrobeItem(context.Context, string, wardrobeapp.CreateWardrobeItemInput) (wardrobeapp.WardrobeItem, error)
	ListWardrobeItems(context.Context, string, int, *string) (wardrobeapp.WardrobePage, error)
	GetWardrobeItem(context.Context, string, string) (wardrobeapp.WardrobeItem, error)
	UpdateWardrobeItem(context.Context, string, string, int, wardrobeapp.UpdateWardrobeItemInput) (wardrobeapp.WardrobeItem, error)
	GetWardrobeDeletionImpact(context.Context, string, string) (wardrobeapp.WardrobeDeletionImpact, error)
	DeleteWardrobeItem(context.Context, string, string, int, wardrobeapp.WardrobeHistoryPolicy, string) error
}

type WardrobeHandler struct {
	service      WardrobeHTTPService
	secureCookie bool
}

func NewWardrobeHandler(service WardrobeHTTPService, secureCookie bool) *WardrobeHandler {
	return &WardrobeHandler{service: service, secureCookie: secureCookie}
}

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
	Session string `cookie:"then_session" hidden:"true"`
	Limit   int    `query:"limit" default:"50" minimum:"1" maximum:"100"`
	AfterID string `query:"after_id" format:"uuid" required:"false"`
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

func registerWardrobeOperations(api huma.API, handler *WardrobeHandler) {
	errorsWithNotFound := wardrobeErrors(http.StatusNotFound)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createWardrobeItem", Method: http.MethodPost, Path: "/wardrobe/items", Tags: []string{"Wardrobe"}, Summary: "创建本人结构化衣物与确认属性", DefaultStatus: http.StatusCreated, MaxBodyBytes: 8 * 1024, Errors: wardrobeErrors(http.StatusBadRequest, http.StatusConflict)}), handler.create)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "listWardrobeItems", Method: http.MethodGet, Path: "/wardrobe/items", Tags: []string{"Wardrobe"}, Summary: "分页列出本人结构化衣物", Errors: errorsWithNotFound}), handler.list)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getWardrobeItem", Method: http.MethodGet, Path: "/wardrobe/items/{item_id}", Tags: []string{"Wardrobe"}, Summary: "读取本人结构化衣物", Errors: errorsWithNotFound}), handler.get)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getWardrobeDeletionImpact", Method: http.MethodGet, Path: "/wardrobe/items/{item_id}/deletion-impact", Tags: []string{"Wardrobe"}, Summary: "读取衣物删除对计划的当前影响", Errors: errorsWithNotFound}), handler.deletionImpact)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "updateWardrobeItem", Method: http.MethodPut, Path: "/wardrobe/items/{item_id}", Tags: []string{"Wardrobe"}, Summary: "按 revision 修改本人结构化衣物", MaxBodyBytes: 8 * 1024, Errors: wardrobeErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.update)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "deleteWardrobeItem", Method: http.MethodDelete, Path: "/wardrobe/items/{item_id}", Tags: []string{"Wardrobe"}, Summary: "按 revision 删除本人结构化衣物", DefaultStatus: http.StatusNoContent, Errors: wardrobeErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.delete)
}

func wardrobeErrors(statuses ...int) []int {
	return append([]int{http.StatusUnauthorized, http.StatusInternalServerError}, statuses...)
}

func (h *WardrobeHandler) available(ctx context.Context, session string) error {
	if h == nil || h.service == nil {
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if session == "" {
		return newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	return nil
}

func (h *WardrobeHandler) create(ctx context.Context, input *createWardrobeItemInput) (*wardrobeItemOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	item, err := h.service.CreateWardrobeItem(ctx, input.Session, wardrobeapp.CreateWardrobeItemInput{ID: input.Body.ID, Name: input.Body.Name, Category: input.Body.Category, Availability: input.Body.Availability, Source: input.Body.Source, Attributes: wardrobeAttributes(input.Body.Attributes)})
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &wardrobeItemOutput{RequestID: requestID(ctx), Body: wardrobeResponse(item)}, nil
}

func (h *WardrobeHandler) list(ctx context.Context, input *listWardrobeItemsInput) (*wardrobePageOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	var afterID *string
	if input.AfterID != "" {
		afterID = &input.AfterID
	}
	page, err := h.service.ListWardrobeItems(ctx, input.Session, input.Limit, afterID)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	items := make([]WardrobeItemResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, wardrobeResponse(item))
	}
	return &wardrobePageOutput{RequestID: requestID(ctx), Body: WardrobePageResponse{Items: items, NextAfterID: page.NextAfterID}}, nil
}

func (h *WardrobeHandler) get(ctx context.Context, input *wardrobeItemInput) (*wardrobeItemOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	item, err := h.service.GetWardrobeItem(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &wardrobeItemOutput{RequestID: requestID(ctx), Body: wardrobeResponse(item)}, nil
}

func (h *WardrobeHandler) update(ctx context.Context, input *updateWardrobeItemInput) (*wardrobeItemOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	item, err := h.service.UpdateWardrobeItem(ctx, input.Session, input.ID, input.Body.ExpectedRevision, wardrobeapp.UpdateWardrobeItemInput{Name: input.Body.Name, Category: input.Body.Category, Availability: input.Body.Availability, Attributes: wardrobeAttributes(input.Body.Attributes)})
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &wardrobeItemOutput{RequestID: requestID(ctx), Body: wardrobeResponse(item)}, nil
}

func (h *WardrobeHandler) delete(ctx context.Context, input *deleteWardrobeItemInput) (*deleteWardrobeItemOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	if err := h.service.DeleteWardrobeItem(ctx, input.Session, input.ID, input.ExpectedRevision, input.HistoryPolicy, input.ExpectedImpact); err != nil {
		return nil, h.error(ctx, err)
	}
	return &deleteWardrobeItemOutput{RequestID: requestID(ctx)}, nil
}

func (h *WardrobeHandler) deletionImpact(ctx context.Context, input *wardrobeItemInput) (*wardrobeDeletionImpactOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	impact, err := h.service.GetWardrobeDeletionImpact(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &wardrobeDeletionImpactOutput{RequestID: requestID(ctx), Body: WardrobeDeletionImpactResponse{AffectedPlanCount: impact.AffectedPlanCount, AffectedWearEventCount: impact.AffectedWearEventCount, ExpectedImpact: impact.ExpectedImpact}}, nil
}

func (h *WardrobeHandler) error(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, wardrobeapp.ErrInvalidWardrobeInput):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, wardrobeapp.ErrWardrobeNotFound):
		return newErrorResponse(http.StatusNotFound, requestID(ctx))
	case errors.Is(err, wardrobeapp.ErrWardrobeConflict):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Wardrobe item changed or the stable ID is already used."
		return response
	case errors.Is(err, accountapp.ErrAuthentication):
		return authenticatedSessionError(ctx, h.secureCookie)
	default:
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
}

func wardrobeResponse(item wardrobeapp.WardrobeItem) WardrobeItemResponse {
	return WardrobeItemResponse{ID: item.ID, Name: item.Name, Category: item.Category, Availability: item.Availability, Source: item.Source, Attributes: wardrobeAttributesResponse(item.Attributes), Revision: item.Revision, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func wardrobeAttributes(value WardrobeAttributesRequest) wardrobeapp.WardrobeAttributes {
	return wardrobeapp.WardrobeAttributes{FormalityBand: value.FormalityBand, WarmthBand: value.WarmthBand, RainUse: value.RainUse, WalkingUse: value.WalkingUse}
}

func wardrobeAttributesResponse(value wardrobeapp.WardrobeAttributes) WardrobeAttributesResponse {
	response := WardrobeAttributesResponse{}
	if value.FormalityBand != nil {
		response.FormalityBand = &WardrobeFormalityAttributeResponse{Value: *value.FormalityBand, Source: wardrobeapp.WardrobeAttributeUserConfirmed}
	}
	if value.WarmthBand != nil {
		response.WarmthBand = &WardrobeWarmthAttributeResponse{Value: *value.WarmthBand, Source: wardrobeapp.WardrobeAttributeUserConfirmed}
	}
	if value.RainUse != nil {
		response.RainUse = &WardrobeSuitabilityAttributeResponse{Value: *value.RainUse, Source: wardrobeapp.WardrobeAttributeUserConfirmed}
	}
	if value.WalkingUse != nil {
		response.WalkingUse = &WardrobeSuitabilityAttributeResponse{Value: *value.WalkingUse, Source: wardrobeapp.WardrobeAttributeUserConfirmed}
	}
	return response
}
