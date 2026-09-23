package httpapi

import (
	"context"
	"errors"
	"net/http"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	wardrobeapp "github.com/StephenQiu30/then-server/backend/internal/application/wardrobe"
)

type WardrobeHTTPService interface {
	CreateWardrobeItem(context.Context, string, wardrobeapp.CreateWardrobeItemInput) (wardrobeapp.WardrobeItem, error)
	ListWardrobeItems(context.Context, string, int, *string, wardrobeapp.WardrobeListFilter) (wardrobeapp.WardrobePage, error)
	GetWardrobeItem(context.Context, string, string) (wardrobeapp.WardrobeItem, error)
	UpdateWardrobeItem(context.Context, string, string, int, wardrobeapp.UpdateWardrobeItemInput) (wardrobeapp.WardrobeItem, error)
	ArchiveWardrobeItem(context.Context, string, string, int) (wardrobeapp.WardrobeItem, error)
	RestoreWardrobeItem(context.Context, string, string, int) (wardrobeapp.WardrobeItem, error)
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
	var availability *wardrobeapp.WardrobeAvailability
	if input.Availability != "" {
		availability = &input.Availability
	}
	page, err := h.service.ListWardrobeItems(ctx, input.Session, input.Limit, afterID, wardrobeapp.WardrobeListFilter{Lifecycle: input.Lifecycle, Availability: availability})
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

func (h *WardrobeHandler) archive(ctx context.Context, input *transitionWardrobeItemInput) (*wardrobeItemOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	item, err := h.service.ArchiveWardrobeItem(ctx, input.Session, input.ID, input.Body.ExpectedRevision)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &wardrobeItemOutput{RequestID: requestID(ctx), Body: wardrobeResponse(item)}, nil
}

func (h *WardrobeHandler) restore(ctx context.Context, input *transitionWardrobeItemInput) (*wardrobeItemOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	item, err := h.service.RestoreWardrobeItem(ctx, input.Session, input.ID, input.Body.ExpectedRevision)
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
	lifecycle := wardrobeapp.WardrobeActive
	if item.ArchivedAt != nil {
		lifecycle = wardrobeapp.WardrobeArchived
	}
	return WardrobeItemResponse{ID: item.ID, Name: item.Name, Category: item.Category, Availability: item.Availability, Source: item.Source, Attributes: wardrobeAttributesResponse(item.Attributes), Lifecycle: lifecycle, ArchivedAt: item.ArchivedAt, Revision: item.Revision, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
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
