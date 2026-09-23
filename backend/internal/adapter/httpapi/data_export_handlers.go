package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	exportapp "github.com/StephenQiu30/then-server/backend/internal/application/dataexport"
	"github.com/danielgtaylor/huma/v2"
)

type DataExportService interface {
	Create(context.Context, string, string, string) (exportapp.Job, error)
	Get(context.Context, string, string) (exportapp.Job, error)
	Open(context.Context, string, string) (io.ReadCloser, error)
	Revoke(context.Context, string, string) error
}

type DataExportHandler struct {
	service DataExportService
	limiter AuthenticationRateLimiter
}

func NewDataExportHandler(service DataExportService, limiter AuthenticationRateLimiter) *DataExportHandler {
	return &DataExportHandler{service: service, limiter: limiter}
}

func (h *DataExportHandler) create(ctx context.Context, input *createDataExportInput) (*dataExportOutput, error) {
	if h == nil || h.service == nil || h.limiter == nil {
		return nil, notReadyError(ctx)
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	ip, err := directSourceIP(ctx)
	if err != nil {
		return nil, notReadyError(ctx)
	}
	allowed, retryAfter, err := h.limiter.Allow(ctx, "data_export", ip, 5, time.Hour)
	if err != nil {
		return nil, notReadyError(ctx)
	}
	if !allowed {
		return nil, rateLimitError(ctx, retryAfter)
	}
	job, err := h.service.Create(ctx, input.Session, input.Body.Password, input.Body.Mode)
	if err != nil {
		return nil, dataExportError(ctx, err)
	}
	return &dataExportOutput{RequestID: requestID(ctx), Body: dataExportResponse(job)}, nil
}

func (h *DataExportHandler) get(ctx context.Context, input *dataExportInput) (*dataExportOutput, error) {
	if h == nil || h.service == nil {
		return nil, notReadyError(ctx)
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	job, err := h.service.Get(ctx, input.Session, input.ID)
	if err != nil {
		return nil, dataExportError(ctx, err)
	}
	return &dataExportOutput{RequestID: requestID(ctx), Body: dataExportResponse(job)}, nil
}

func (h *DataExportHandler) download(ctx context.Context, input *dataExportInput) (*huma.StreamResponse, error) {
	if h == nil || h.service == nil {
		return nil, notReadyError(ctx)
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	reader, err := h.service.Open(ctx, input.Session, input.ID)
	if err != nil {
		return nil, dataExportError(ctx, err)
	}
	return &huma.StreamResponse{Body: func(stream huma.Context) {
		defer reader.Close()
		stream.SetHeader("Content-Type", "application/zip")
		stream.SetHeader("Content-Disposition", "attachment; filename=then-export.zip")
		_, _ = io.Copy(stream.BodyWriter(), reader)
	}}, nil
}

func (h *DataExportHandler) revoke(ctx context.Context, input *dataExportInput) (*emptyDataExportOutput, error) {
	if h == nil || h.service == nil {
		return nil, notReadyError(ctx)
	}
	if input.Session == "" {
		return nil, newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	if err := h.service.Revoke(ctx, input.Session, input.ID); err != nil {
		return nil, dataExportError(ctx, err)
	}
	return &emptyDataExportOutput{RequestID: requestID(ctx)}, nil
}

func dataExportResponse(job exportapp.Job) DataExportResponse {
	return DataExportResponse{
		ID: job.ID, Mode: job.Mode, Status: job.Status,
		Counts: job.Counts, Omissions: job.Omissions,
		CreatedAt: job.CreatedAt, CompletedAt: job.CompletedAt, ExpiresAt: job.ExpiresAt,
	}
}

func dataExportError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, accountapp.ErrAuthentication):
		return newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	case errors.Is(err, exportapp.ErrInvalidMode):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, exportapp.ErrNotFound):
		return newErrorResponse(http.StatusNotFound, requestID(ctx))
	case errors.Is(err, exportapp.ErrNotReady):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "EXPORT_NOT_READY", "Export is not available for download."
		return response
	default:
		return notReadyError(ctx)
	}
}
