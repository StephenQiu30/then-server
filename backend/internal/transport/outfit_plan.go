package transport

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
	"github.com/danielgtaylor/huma/v2"
)

type OutfitPlanHTTPService interface {
	CreateOutfitPlan(context.Context, string, string, model.OutfitPlanInput) (model.OutfitPlan, error)
	ListOutfitPlans(context.Context, string, int, *string, *string) (model.OutfitPlanPage, error)
	GetOutfitPlan(context.Context, string, string) (model.OutfitPlan, error)
	UpdateOutfitPlan(context.Context, string, string, int, model.OutfitPlanInput) (model.OutfitPlan, error)
	CancelOutfitPlan(context.Context, string, string, int) (model.OutfitPlan, error)
	MarkOutfitPlanNotWorn(context.Context, string, string, int) (model.OutfitPlan, error)
	RestoreOutfitPlan(context.Context, string, string, int) (model.OutfitPlan, error)
	DeleteOutfitPlan(context.Context, string, string, int) error
}

type OutfitPlanHandler struct {
	service      OutfitPlanHTTPService
	secureCookie bool
}

func NewOutfitPlanHandler(service OutfitPlanHTTPService, secureCookie bool) *OutfitPlanHandler {
	return &OutfitPlanHandler{service: service, secureCookie: secureCookie}
}

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
	ItemID       string                     `json:"item_id" format:"uuid"`
	ItemRevision int                        `json:"item_revision" minimum:"1"`
	Name         string                     `json:"name" minLength:"1" maxLength:"80"`
	Category     model.WardrobeCategory     `json:"category" enum:"top,bottom,one_piece,outerwear,shoes,bag,accessory"`
	Availability model.WardrobeAvailability `json:"availability" enum:"wearable,laundry,lent_out,packed"`
	Attributes   WardrobeAttributesResponse `json:"attributes"`
}

type OutfitPlanItemResponse struct {
	Ordinal int                            `json:"ordinal" minimum:"0" maximum:"19"`
	Content *OutfitPlanItemContentResponse `json:"content" doc:"NULL 表示衣物已按用户选择从历史快照清除"`
}

type OutfitPlanResponse struct {
	ID             string                   `json:"id" format:"uuid"`
	LocalDate      string                   `json:"local_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	TimeZone       string                   `json:"time_zone" minLength:"1" maxLength:"255"`
	ContextSummary *string                  `json:"context_summary,omitempty" maxLength:"120"`
	Status         model.OutfitPlanStatus   `json:"status" enum:"active,completed,not_worn,cancelled"`
	Revision       int                      `json:"revision" minimum:"1"`
	CreatedAt      time.Time                `json:"created_at" format:"date-time"`
	UpdatedAt      time.Time                `json:"updated_at" format:"date-time"`
	Items          []OutfitPlanItemResponse `json:"items" minItems:"1" maxItems:"20"`
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

func registerOutfitPlanOperations(api huma.API, handler *OutfitPlanHandler) {
	errorsWithNotFound := outfitPlanErrors(http.StatusNotFound)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createOutfitPlan", Method: http.MethodPost, Path: "/outfit-plans", Tags: []string{"Outfit Plans"}, Summary: "创建本人真实衣物穿搭计划", DefaultStatus: http.StatusCreated, MaxBodyBytes: 16 * 1024, Errors: outfitPlanErrors(http.StatusBadRequest, http.StatusConflict)}), handler.create)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "listOutfitPlans", Method: http.MethodGet, Path: "/outfit-plans", Tags: []string{"Outfit Plans"}, Summary: "按本地日期稳定分页列出本人计划", Errors: errorsWithNotFound}), handler.list)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getOutfitPlan", Method: http.MethodGet, Path: "/outfit-plans/{plan_id}", Tags: []string{"Outfit Plans"}, Summary: "读取本人穿搭计划与服务端快照", Errors: errorsWithNotFound}), handler.get)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "updateOutfitPlan", Method: http.MethodPut, Path: "/outfit-plans/{plan_id}", Tags: []string{"Outfit Plans"}, Summary: "按 revision 完整更新 active 计划", MaxBodyBytes: 16 * 1024, Errors: outfitPlanErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.update)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "cancelOutfitPlan", Method: http.MethodPost, Path: "/outfit-plans/{plan_id}/cancel", Tags: []string{"Outfit Plans"}, Summary: "取消计划且不创建实际穿着", MaxBodyBytes: 1024, Errors: outfitPlanErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.cancel)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "markOutfitPlanNotWorn", Method: http.MethodPost, Path: "/outfit-plans/{plan_id}/not-worn", Tags: []string{"Outfit Plans"}, Summary: "确认计划最终未穿", MaxBodyBytes: 1024, Errors: outfitPlanErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.markNotWorn)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "restoreOutfitPlan", Method: http.MethodPost, Path: "/outfit-plans/{plan_id}/restore", Tags: []string{"Outfit Plans"}, Summary: "把未穿计划恢复为待确认", MaxBodyBytes: 1024, Errors: outfitPlanErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.restore)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "deleteOutfitPlan", Method: http.MethodDelete, Path: "/outfit-plans/{plan_id}", Tags: []string{"Outfit Plans"}, Summary: "永久删除计划并阻止迟到复活", DefaultStatus: http.StatusNoContent, Errors: outfitPlanErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.delete)
}

func outfitPlanErrors(statuses ...int) []int {
	return append([]int{http.StatusUnauthorized, http.StatusInternalServerError}, statuses...)
}

func (h *OutfitPlanHandler) available(ctx context.Context, session string) error {
	if h == nil || h.service == nil {
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if session == "" {
		return newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	return nil
}

func (h *OutfitPlanHandler) create(ctx context.Context, input *createOutfitPlanInput) (*outfitPlanOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	plan, err := h.service.CreateOutfitPlan(ctx, input.Session, input.Body.ID, outfitPlanCreateInput(input.Body))
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &outfitPlanOutput{RequestID: requestID(ctx), Body: outfitPlanResponse(plan)}, nil
}

func (h *OutfitPlanHandler) list(ctx context.Context, input *listOutfitPlansInput) (*outfitPlanPageOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	var afterID, localDate *string
	if input.AfterID != "" {
		afterID = &input.AfterID
	}
	if input.LocalDate != "" {
		localDate = &input.LocalDate
	}
	page, err := h.service.ListOutfitPlans(ctx, input.Session, input.Limit, afterID, localDate)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	plans := make([]OutfitPlanResponse, 0, len(page.Plans))
	for _, plan := range page.Plans {
		plans = append(plans, outfitPlanResponse(plan))
	}
	return &outfitPlanPageOutput{RequestID: requestID(ctx), Body: OutfitPlanPageResponse{Plans: plans, NextAfterID: page.NextAfterID}}, nil
}

func (h *OutfitPlanHandler) get(ctx context.Context, input *outfitPlanInput) (*outfitPlanOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	plan, err := h.service.GetOutfitPlan(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &outfitPlanOutput{RequestID: requestID(ctx), Body: outfitPlanResponse(plan)}, nil
}

func (h *OutfitPlanHandler) update(ctx context.Context, input *updateOutfitPlanInput) (*outfitPlanOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	plan, err := h.service.UpdateOutfitPlan(ctx, input.Session, input.ID, input.Body.ExpectedRevision, outfitPlanUpdateInput(input.Body))
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &outfitPlanOutput{RequestID: requestID(ctx), Body: outfitPlanResponse(plan)}, nil
}

func (h *OutfitPlanHandler) cancel(ctx context.Context, input *cancelOutfitPlanInput) (*outfitPlanOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	plan, err := h.service.CancelOutfitPlan(ctx, input.Session, input.ID, input.Body.ExpectedRevision)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &outfitPlanOutput{RequestID: requestID(ctx), Body: outfitPlanResponse(plan)}, nil
}

func (h *OutfitPlanHandler) markNotWorn(ctx context.Context, input *markOutfitPlanNotWornInput) (*outfitPlanOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	plan, err := h.service.MarkOutfitPlanNotWorn(ctx, input.Session, input.ID, input.Body.ExpectedRevision)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &outfitPlanOutput{RequestID: requestID(ctx), Body: outfitPlanResponse(plan)}, nil
}

func (h *OutfitPlanHandler) restore(ctx context.Context, input *restoreOutfitPlanInput) (*outfitPlanOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	plan, err := h.service.RestoreOutfitPlan(ctx, input.Session, input.ID, input.Body.ExpectedRevision)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &outfitPlanOutput{RequestID: requestID(ctx), Body: outfitPlanResponse(plan)}, nil
}

func (h *OutfitPlanHandler) delete(ctx context.Context, input *deleteOutfitPlanInput) (*deleteOutfitPlanOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	if err := h.service.DeleteOutfitPlan(ctx, input.Session, input.ID, input.ExpectedRevision); err != nil {
		return nil, h.error(ctx, err)
	}
	return &deleteOutfitPlanOutput{RequestID: requestID(ctx)}, nil
}

func (h *OutfitPlanHandler) error(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, model.ErrInvalidOutfitPlanInput):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, model.ErrOutfitPlanNotFound):
		return newErrorResponse(http.StatusNotFound, requestID(ctx))
	case errors.Is(err, model.ErrOutfitPlanConflict):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Outfit plan or wardrobe facts changed."
		return response
	case errors.Is(err, model.ErrOutfitItemsUnavailable):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Confirm every currently unavailable wardrobe item."
		return response
	case errors.Is(err, model.ErrAuthentication):
		return authenticatedSessionError(ctx, h.secureCookie)
	default:
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
}

func outfitPlanCreateInput(request CreateOutfitPlanRequest) model.OutfitPlanInput {
	return outfitPlanFields(request.LocalDate, request.TimeZone, request.ContextSummary, request.Items, request.ConfirmedUnavailableIDs)
}

func outfitPlanUpdateInput(request UpdateOutfitPlanRequest) model.OutfitPlanInput {
	return outfitPlanFields(request.LocalDate, request.TimeZone, request.ContextSummary, request.Items, request.ConfirmedUnavailableIDs)
}

func outfitPlanFields(localDate, timeZone string, summary *string, items []OutfitSelectionRequest, confirmed []string) model.OutfitPlanInput {
	selections := make([]model.OutfitSelection, 0, len(items))
	for _, item := range items {
		selections = append(selections, model.OutfitSelection{ItemID: item.ItemID, Revision: item.Revision})
	}
	return model.OutfitPlanInput{LocalDate: localDate, TimeZone: timeZone, ContextSummary: summary, Items: selections, ConfirmedUnavailableIDs: confirmed}
}

func outfitPlanResponse(plan model.OutfitPlan) OutfitPlanResponse {
	items := make([]OutfitPlanItemResponse, 0, len(plan.Items))
	for _, item := range plan.Items {
		response := OutfitPlanItemResponse{Ordinal: item.Ordinal}
		if item.Content != nil {
			response.Content = &OutfitPlanItemContentResponse{ItemID: item.Content.ItemID, ItemRevision: item.Content.ItemRevision, Name: item.Content.Name, Category: item.Content.Category, Availability: item.Content.Availability, Attributes: wardrobeAttributesResponse(item.Content.Attributes)}
		}
		items = append(items, response)
	}
	return OutfitPlanResponse{ID: plan.ID, LocalDate: plan.LocalDate, TimeZone: plan.TimeZone, ContextSummary: plan.ContextSummary, Status: plan.Status, Revision: plan.Revision, CreatedAt: plan.CreatedAt, UpdatedAt: plan.UpdatedAt, Items: items}
}
