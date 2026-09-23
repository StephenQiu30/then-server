package httpapi

import (
	"context"
	"errors"
	"net/http"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	feedbackapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitfeedback"
)

type FeedbackHTTPService interface {
	Get(context.Context, string, string) (feedbackapp.Feedback, error)
	Save(context.Context, string, string, string, string, *int, feedbackapp.Input) (feedbackapp.Feedback, error)
	Delete(context.Context, string, string, string, string, int) error
	Statistics(context.Context, string, string, string) (feedbackapp.Statistics, error)
}

type FeedbackHandler struct {
	service      FeedbackHTTPService
	secureCookie bool
}

func NewFeedbackHandler(service FeedbackHTTPService, secureCookie bool) *FeedbackHandler {
	return &FeedbackHandler{service: service, secureCookie: secureCookie}
}

func (h *FeedbackHandler) available(ctx context.Context, token string) error {
	if h == nil || h.service == nil {
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if token == "" {
		return newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	return nil
}
func (h *FeedbackHandler) get(ctx context.Context, input *feedbackInput) (*feedbackOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	value, err := h.service.Get(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &feedbackOutput{RequestID: requestID(ctx), Body: feedbackResponse(value)}, nil
}
func (h *FeedbackHandler) save(ctx context.Context, input *saveFeedbackInput) (*feedbackOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	b := input.Body
	value, err := h.service.Save(ctx, input.Session, input.ID, b.FeedbackID, b.MutationID, b.ExpectedRevision, feedbackapp.Input{ThermalComfort: b.ThermalComfort, ActivityComfort: b.ActivityComfort, OccasionFit: b.OccasionFit, RepeatIntent: b.RepeatIntent, IssueTags: b.IssueTags, Note: b.Note})
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &feedbackOutput{RequestID: requestID(ctx), Body: feedbackResponse(value)}, nil
}
func (h *FeedbackHandler) delete(ctx context.Context, input *deleteFeedbackInput) (*deleteWearEventOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	if err := h.service.Delete(ctx, input.Session, input.ID, input.FeedbackID, input.MutationID, input.ExpectedRevision); err != nil {
		return nil, h.error(ctx, err)
	}
	return &deleteWearEventOutput{RequestID: requestID(ctx)}, nil
}
func (h *FeedbackHandler) statistics(ctx context.Context, input *wearStatisticsInput) (*wearStatisticsOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	value, err := h.service.Statistics(ctx, input.Session, input.From, input.To)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	uses := make([]WearItemUseResponse, 0, len(value.ItemUses))
	for _, use := range value.ItemUses {
		uses = append(uses, WearItemUseResponse{ItemID: use.ItemID, Count: use.Count})
	}
	return &wearStatisticsOutput{RequestID: requestID(ctx), Body: WearStatisticsResponse{From: value.From, To: value.To, WearDays: value.WearDays, WearEvents: value.WearEvents, ItemUses: uses}}, nil
}
func (h *FeedbackHandler) error(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, feedbackapp.ErrInvalid):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, feedbackapp.ErrNotFound):
		return newErrorResponse(http.StatusNotFound, requestID(ctx))
	case errors.Is(err, feedbackapp.ErrConflict):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Wear feedback changed."
		return response
	case errors.Is(err, accountapp.ErrAuthentication):
		return authenticatedSessionError(ctx, h.secureCookie)
	default:
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
}
func feedbackResponse(value feedbackapp.Feedback) FeedbackResponse {
	return FeedbackResponse{ID: value.ID, WearEventID: value.WearEventID, FeedbackFieldsRequest: FeedbackFieldsRequest{ThermalComfort: value.Input.ThermalComfort, ActivityComfort: value.Input.ActivityComfort, OccasionFit: value.Input.OccasionFit, RepeatIntent: value.Input.RepeatIntent, IssueTags: value.Input.IssueTags, Note: value.Input.Note}, Revision: value.Revision, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
