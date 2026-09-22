package httpapi

import (
	"context"
	"errors"
	"net/http"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	outfitplanapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitplan"
)

type OutfitPlanHTTPService interface {
	CreateOutfitPlan(context.Context, string, string, outfitplanapp.OutfitPlanInput) (outfitplanapp.OutfitPlan, error)
	ListOutfitPlans(context.Context, string, int, *string, *string) (outfitplanapp.OutfitPlanPage, error)
	GetOutfitPlan(context.Context, string, string) (outfitplanapp.OutfitPlan, error)
	UpdateOutfitPlan(context.Context, string, string, int, outfitplanapp.OutfitPlanInput) (outfitplanapp.OutfitPlan, error)
	CancelOutfitPlan(context.Context, string, string, int) (outfitplanapp.OutfitPlan, error)
	MarkOutfitPlanNotWorn(context.Context, string, string, int) (outfitplanapp.OutfitPlan, error)
	RestoreOutfitPlan(context.Context, string, string, int) (outfitplanapp.OutfitPlan, error)
	DeleteOutfitPlan(context.Context, string, string, int) error
}

type OutfitPlanHandler struct {
	service      OutfitPlanHTTPService
	secureCookie bool
}

func NewOutfitPlanHandler(service OutfitPlanHTTPService, secureCookie bool) *OutfitPlanHandler {
	return &OutfitPlanHandler{service: service, secureCookie: secureCookie}
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
	case errors.Is(err, outfitplanapp.ErrInvalidOutfitPlanInput):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, outfitplanapp.ErrOutfitPlanNotFound):
		return newErrorResponse(http.StatusNotFound, requestID(ctx))
	case errors.Is(err, outfitplanapp.ErrOutfitPlanConflict):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Outfit plan or wardrobe facts changed."
		return response
	case errors.Is(err, outfitplanapp.ErrOutfitItemsUnavailable):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Confirm every currently unavailable wardrobe item."
		return response
	case errors.Is(err, accountapp.ErrAuthentication):
		return authenticatedSessionError(ctx, h.secureCookie)
	default:
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
}

func outfitPlanCreateInput(request CreateOutfitPlanRequest) outfitplanapp.OutfitPlanInput {
	return outfitPlanFields(request.LocalDate, request.TimeZone, request.ContextSummary, request.Items, request.ConfirmedUnavailableIDs)
}

func outfitPlanUpdateInput(request UpdateOutfitPlanRequest) outfitplanapp.OutfitPlanInput {
	return outfitPlanFields(request.LocalDate, request.TimeZone, request.ContextSummary, request.Items, request.ConfirmedUnavailableIDs)
}

func outfitPlanFields(localDate, timeZone string, summary *string, items []OutfitSelectionRequest, confirmed []string) outfitplanapp.OutfitPlanInput {
	selections := make([]outfitplanapp.OutfitSelection, 0, len(items))
	for _, item := range items {
		selections = append(selections, outfitplanapp.OutfitSelection{ItemID: item.ItemID, Revision: item.Revision})
	}
	return outfitplanapp.OutfitPlanInput{LocalDate: localDate, TimeZone: timeZone, ContextSummary: summary, Items: selections, ConfirmedUnavailableIDs: confirmed}
}

func outfitPlanResponse(plan outfitplanapp.OutfitPlan) OutfitPlanResponse {
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
