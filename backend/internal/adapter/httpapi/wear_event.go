package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	outfitplanapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitplan"
	weareventapp "github.com/StephenQiu30/then-server/backend/internal/application/wearevent"

	"github.com/danielgtaylor/huma/v2"
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

func registerWearEventOperations(api huma.API, handler *WearEventHandler) {
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createWearEvent", Method: http.MethodPost, Path: "/wear-events", Tags: []string{"Wear Events"}, Summary: "记录本人实际穿着并生成服务端快照", DefaultStatus: http.StatusCreated, MaxBodyBytes: 20 * 1024, Errors: wearEventErrors(http.StatusBadRequest, http.StatusConflict)}), handler.create)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "listWearEvents", Method: http.MethodGet, Path: "/wear-events", Tags: []string{"Wear Events"}, Summary: "按本地日期稳定分页列出本人实际穿着", Errors: wearEventErrors(http.StatusBadRequest, http.StatusNotFound)}), handler.list)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getWearEvent", Method: http.MethodGet, Path: "/wear-events/{wear_event_id}", Tags: []string{"Wear Events"}, Summary: "读取本人实际穿着与快照", Errors: wearEventErrors(http.StatusBadRequest, http.StatusNotFound)}), handler.get)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "updateWearEvent", Method: http.MethodPut, Path: "/wear-events/{wear_event_id}", Tags: []string{"Wear Events"}, Summary: "按 revision 纠正实际穿着", MaxBodyBytes: 20 * 1024, Errors: wearEventErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.update)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "deleteWearEvent", Method: http.MethodDelete, Path: "/wear-events/{wear_event_id}", Tags: []string{"Wear Events"}, Summary: "永久删除实际穿着并重算计划状态", DefaultStatus: http.StatusNoContent, Errors: wearEventErrors(http.StatusBadRequest, http.StatusNotFound, http.StatusConflict)}), handler.delete)
}

func wearEventErrors(statuses ...int) []int {
	return append([]int{http.StatusUnauthorized, http.StatusInternalServerError}, statuses...)
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
