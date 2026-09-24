package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	"github.com/danielgtaylor/huma/v2"
)

type GenerationHTTPService interface {
	Create(context.Context, string, generationapp.CreateServiceInput) (generationapp.CreateResult, error)
	Get(context.Context, string, string) (generationapp.TaskView, error)
	List(context.Context, string, int, *string) (generationapp.TaskPage, error)
	Cancel(context.Context, string, string) (generationapp.TaskView, error)
	Delete(context.Context, string, string) (generationapp.DeleteResult, error)
}

type GenerationHandler struct {
	service      GenerationHTTPService
	secureCookie bool
}

func NewGenerationHandler(service GenerationHTTPService, secureCookie bool) *GenerationHandler {
	return &GenerationHandler{service: service, secureCookie: secureCookie}
}

func (h *GenerationHandler) create(ctx context.Context, input *createGenerationJobInput) (*generationJobOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	parameters := []byte("{}")
	if input.Body.Parameters != nil {
		encoded, err := json.Marshal(input.Body.Parameters)
		if err != nil {
			return nil, newErrorResponse(http.StatusBadRequest, requestID(ctx))
		}
		parameters = encoded
	}
	references := make([]generationapp.InputReference, 0, len(input.Body.Inputs))
	for _, reference := range input.Body.Inputs {
		references = append(references, generationapp.InputReference{MediaID: reference.MediaID, Role: generationapp.InputRole(reference.Role), Ordinal: reference.Ordinal, Revision: reference.Revision, SHA256: reference.SHA256})
	}
	cost := generationapp.CostEstimate{}
	if input.Body.Cost != nil {
		cost = generationapp.CostEstimate{Currency: input.Body.Cost.Currency, EstimatedMinorUnits: input.Body.Cost.EstimatedMinorUnits, ReservedQuotaUnits: input.Body.Cost.ReservedQuotaUnits}
	}
	result, err := h.service.Create(ctx, input.Session, generationapp.CreateServiceInput{IdempotencyKey: input.Body.IdempotencyKey, LookID: input.Body.LookID, LookRevision: input.Body.LookRevision, Purpose: generationapp.Purpose(input.Body.Purpose), Provider: input.Body.Provider, Model: input.Body.Model, Parameters: parameters, Inputs: generationapp.InputSnapshot{LookID: input.Body.LookID, LookRevision: input.Body.LookRevision, References: references, ImageAssetID: input.Body.ImageAssetID, ImageSHA256: input.Body.ImageSHA256}, Consent: generationapp.ConsentReceipt{ID: input.Body.Consent.ID, Purpose: generationapp.Purpose(input.Body.Consent.Purpose), PolicyVersion: input.Body.Consent.PolicyVersion, AcceptedAt: input.Body.Consent.AcceptedAt}, Cost: cost})
	if err != nil {
		return nil, h.error(ctx, err)
	}
	response := generationJobResponse(result.View)
	response.Reused = result.Reused
	response.Match = result.Match
	return &generationJobOutput{RequestID: requestID(ctx), Body: response}, nil
}

func (h *GenerationHandler) list(ctx context.Context, input *listGenerationJobsInput) (*generationJobPageOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	var after *string
	if input.AfterID != "" {
		after = &input.AfterID
	}
	page, err := h.service.List(ctx, input.Session, input.Limit, after)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	jobs := make([]GenerationJobResponse, 0, len(page.Items))
	for _, item := range page.Items {
		jobs = append(jobs, generationJobResponse(item))
	}
	return &generationJobPageOutput{RequestID: requestID(ctx), Body: GenerationJobPageResponse{Jobs: jobs, NextAfterID: page.NextAfterID}}, nil
}

func (h *GenerationHandler) get(ctx context.Context, input *generationJobInput) (*generationJobOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	view, err := h.service.Get(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &generationJobOutput{RequestID: requestID(ctx), Body: generationJobResponse(view)}, nil
}

func (h *GenerationHandler) cancel(ctx context.Context, input *generationJobInput) (*generationJobOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	view, err := h.service.Cancel(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	return &generationJobOutput{RequestID: requestID(ctx), Body: generationJobResponse(view)}, nil
}

func (h *GenerationHandler) delete(ctx context.Context, input *generationJobInput) (*generationJobOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	result, err := h.service.Delete(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.error(ctx, err)
	}
	response := generationJobResponse(result.View)
	response.Cleanup = generationCleanupResponse(result.Cleanup)
	return &generationJobOutput{RequestID: requestID(ctx), Body: response}, nil
}

func (h *GenerationHandler) available(ctx context.Context, session string) error {
	if h == nil || h.service == nil {
		return newErrorResponse(http.StatusServiceUnavailable, requestID(ctx))
	}
	if session == "" {
		return newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	return nil
}

func (h *GenerationHandler) error(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, accountapp.ErrAuthentication):
		return authenticatedSessionError(ctx, h.secureCookie)
	case errors.Is(err, generationapp.ErrInvalidGenerationInput):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, generationapp.ErrGenerationNotFound):
		return newErrorResponse(http.StatusNotFound, requestID(ctx))
	case errors.Is(err, generationapp.ErrGenerationIdempotencyConflict), errors.Is(err, generationapp.ErrGenerationNotCancellable), errors.Is(err, generationapp.ErrGenerationQuotaExceeded), errors.Is(err, generationapp.ErrGenerationBudgetExceeded), errors.Is(err, generationapp.ErrGenerationConcurrency), errors.Is(err, generationapp.ErrGenerationCurrency), errors.Is(err, generationapp.ErrGenerationCleanupInProgress):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Generation request conflicts with the current task or admission limits."
		return response
	case errors.Is(err, generationapp.ErrGenerationDisabled), errors.Is(err, generationapp.ErrGenerationUnavailable), errors.Is(err, accountapp.ErrAccountUnavailable):
		return huma.ErrorWithHeaders(newErrorResponse(http.StatusServiceUnavailable, requestID(ctx)), http.Header{"Retry-After": []string{"1"}})
	default:
		return huma.ErrorWithHeaders(newErrorResponse(http.StatusServiceUnavailable, requestID(ctx)), http.Header{"Retry-After": []string{"1"}})
	}
}

func generationJobResponse(view generationapp.TaskView) GenerationJobResponse {
	task := view.Task
	externalTaskID, resultAssetID := task.ExternalTaskID, task.ResultAssetID
	if task.AccessRevokedAt != nil {
		externalTaskID, resultAssetID = "", ""
	}
	response := GenerationJobResponse{ID: task.ID, LookID: task.LookID, LookRevision: task.LookRevision, Purpose: task.Purpose, Provider: task.Provider, Model: task.Model, Status: task.Status, StatusRevision: task.StatusRevision, SubmissionState: task.SubmissionState, SubmissionAttempt: task.SubmissionAttempt, ExternalTaskID: externalTaskID, ResultAssetID: resultAssetID, FailureCode: task.FailureCode, CancelRequestedAt: task.CancelRequestedAt, AccessRevokedAt: task.AccessRevokedAt, CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt, Match: generationapp.RequestMatchNone}
	if view.Reservation != nil {
		reservation := view.Reservation
		response.Reservation = &GenerationReservationResponse{ID: reservation.ID, State: reservation.State, ReservedQuotaUnits: reservation.ReservedQuotaUnits, EstimatedMinorUnits: reservation.EstimatedMinorUnits, Currency: reservation.Currency, StateRevision: reservation.StateRevision, CreatedAt: reservation.CreatedAt, UpdatedAt: reservation.UpdatedAt}
	}
	if view.Asset != nil {
		asset := view.Asset
		response.Output = &GenerationOutputResponse{ID: asset.ID, ContentType: asset.ContentType, ByteSize: asset.ByteSize, SHA256: asset.SHA256, ObjectVersionID: asset.ObjectVersionID, PublishedAt: asset.PublishedAt}
	}
	return response
}

func generationCleanupResponse(cleanup generationapp.CleanupRequest) *GenerationCleanupResponse {
	return &GenerationCleanupResponse{ID: cleanup.ID, Status: cleanup.Status, AccessRevokedAt: cleanup.AccessRevokedAt, CompletedAt: cleanup.CompletedAt, Attempts: cleanup.Attempts, NextAttemptAt: cleanup.NextAttemptAt}
}
