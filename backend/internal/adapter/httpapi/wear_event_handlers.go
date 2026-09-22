package httpapi

import (
	"context"
	"errors"
	"net/http"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	outfitplanapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitplan"
	weareventapp "github.com/StephenQiu30/then-server/backend/internal/application/wearevent"
)

type WearEventHTTPService interface {
	CreateWearEvent(context.Context, string, string, weareventapp.WearEventInput) (weareventapp.WearEvent, error)
	ListWearEvents(context.Context, string, int, *string, *string) (weareventapp.WearEventPage, error)
	GetWearEvent(context.Context, string, string) (weareventapp.WearEvent, error)
	UpdateWearEvent(context.Context, string, string, int, weareventapp.WearEventInput) (weareventapp.WearEvent, error)
	DeleteWearEvent(context.Context, string, string, int) error
}

type WearEventHandler struct {
	service      WearEventHTTPService
	secureCookie bool
}

func NewWearEventHandler(service WearEventHTTPService, secureCookie bool) *WearEventHandler {
	return &WearEventHandler{service: service, secureCookie: secureCookie}
}

func (h *WearEventHandler) available(ctx context.Context, session string) error {
	if h == nil || h.service == nil {
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if session == "" {
		return newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	return nil
}

func (h *WearEventHandler) create(ctx context.Context, input *createWearEventInput) (*wearEventOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	event, err := h.service.CreateWearEvent(ctx, input.Session, input.Body.ID, wearEventFields(input.Body.WearEventFieldsRequest))
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &wearEventOutput{RequestID: requestID(ctx), Body: wearEventResponse(event)}, nil
}

func (h *WearEventHandler) list(ctx context.Context, input *listWearEventsInput) (*wearEventPageOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	var afterID, date *string
	if input.AfterID != "" {
		afterID = &input.AfterID
	}
	if input.LocalDate != "" {
		date = &input.LocalDate
	}
	page, err := h.service.ListWearEvents(ctx, input.Session, input.Limit, afterID, date)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	events := make([]WearEventResponse, 0, len(page.Events))
	for _, event := range page.Events {
		events = append(events, wearEventResponse(event))
	}
	return &wearEventPageOutput{RequestID: requestID(ctx), Body: WearEventPageResponse{Events: events, NextAfterID: page.NextAfterID}}, nil
}

func (h *WearEventHandler) get(ctx context.Context, input *wearEventInput) (*wearEventOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	event, err := h.service.GetWearEvent(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &wearEventOutput{RequestID: requestID(ctx), Body: wearEventResponse(event)}, nil
}

func (h *WearEventHandler) update(ctx context.Context, input *updateWearEventInput) (*wearEventOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	event, err := h.service.UpdateWearEvent(ctx, input.Session, input.ID, input.Body.ExpectedRevision, wearEventFields(input.Body.WearEventFieldsRequest))
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &wearEventOutput{RequestID: requestID(ctx), Body: wearEventResponse(event)}, nil
}

func (h *WearEventHandler) delete(ctx context.Context, input *deleteWearEventInput) (*deleteWearEventOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	if err := h.service.DeleteWearEvent(ctx, input.Session, input.ID, input.ExpectedRevision); err != nil {
		return nil, h.error(ctx, err)
	}
	return &deleteWearEventOutput{RequestID: requestID(ctx)}, nil
}

func (h *WearEventHandler) error(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, weareventapp.ErrInvalidWearEventInput):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, weareventapp.ErrWearEventNotFound):
		return newErrorResponse(http.StatusNotFound, requestID(ctx))
	case errors.Is(err, weareventapp.ErrWearEventDuplicate):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Confirm the current similar wear events before saving."
		var duplicate *weareventapp.WearEventDuplicateError
		if errors.As(err, &duplicate) {
			for _, candidate := range duplicate.Candidates {
				response.DuplicateCandidates = append(response.DuplicateCandidates, WearEventCandidateResponse{ID: candidate.ID, Revision: candidate.Revision})
			}
		}
		return response
	case errors.Is(err, weareventapp.ErrWearEventConflict):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Wear event, outfit plan, or wardrobe facts changed."
		return response
	case errors.Is(err, weareventapp.ErrWearEventItemsUnavailable):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Confirm every currently unavailable wardrobe item."
		return response
	case errors.Is(err, accountapp.ErrAuthentication):
		return authenticatedSessionError(ctx, h.secureCookie)
	default:
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
}

func wearEventFields(request WearEventFieldsRequest) weareventapp.WearEventInput {
	items := make([]outfitplanapp.OutfitSelection, 0, len(request.Items))
	for _, item := range request.Items {
		items = append(items, outfitplanapp.OutfitSelection{ItemID: item.ItemID, Revision: item.Revision})
	}
	confirmations := make([]weareventapp.WearEventCandidate, 0, len(request.DuplicateConfirmations))
	for _, candidate := range request.DuplicateConfirmations {
		confirmations = append(confirmations, weareventapp.WearEventCandidate{ID: candidate.ID, Revision: candidate.Revision})
	}
	return weareventapp.WearEventInput{LocalDate: request.LocalDate, TimeZone: request.TimeZone, Completeness: request.Completeness, ContextSummary: request.ContextSummary, Items: items, LaundryItemIDs: request.LaundryItemIDs, ConfirmedUnavailableIDs: request.ConfirmedUnavailableIDs, SourcePlanID: request.SourcePlanID, SourcePlanRevision: request.SourcePlanRevision, SourceKind: request.SourceKind, DuplicateConfirmations: confirmations}
}

func wearEventResponse(event weareventapp.WearEvent) WearEventResponse {
	items := make([]OutfitPlanItemResponse, 0, len(event.Items))
	for _, item := range event.Items {
		response := OutfitPlanItemResponse{Ordinal: item.Ordinal}
		if item.Content != nil {
			response.Content = &OutfitPlanItemContentResponse{ItemID: item.Content.ItemID, ItemRevision: item.Content.ItemRevision, Name: item.Content.Name, Category: item.Content.Category, Availability: item.Content.Availability, Attributes: wardrobeAttributesResponse(item.Content.Attributes)}
		}
		items = append(items, response)
	}
	return WearEventResponse{ID: event.ID, LocalDate: event.LocalDate, TimeZone: event.TimeZone, Completeness: event.Completeness, ContextSummary: event.ContextSummary, SourcePlanID: event.SourcePlanID, SourcePlanRevision: event.SourcePlanRevision, SourceKind: event.SourceKind, Revision: event.Revision, CreatedAt: event.CreatedAt, UpdatedAt: event.UpdatedAt, Items: items}
}
