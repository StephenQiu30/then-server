package httpapi

import (
	"context"
	"errors"
	"net/http"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	diaryapp "github.com/StephenQiu30/then-server/backend/internal/application/diary"
)

type DiaryHTTPService interface {
	Create(context.Context, string, string, diaryapp.DiaryEntryInput) (diaryapp.DiaryEntry, error)
	List(context.Context, string, int, *string, *string, *string) (diaryapp.DiaryEntryPage, error)
	Get(context.Context, string, string) (diaryapp.DiaryEntry, error)
	Update(context.Context, string, string, int, diaryapp.DiaryEntryInput) (diaryapp.DiaryEntry, error)
	DeletionImpact(context.Context, string, string) (diaryapp.DiaryDeletionImpact, error)
	Delete(context.Context, string, string, int) error
	Calendar(context.Context, string, string) (diaryapp.CalendarMonth, error)
}

type DiaryHandler struct {
	service      DiaryHTTPService
	secureCookie bool
}

func NewDiaryHandler(service DiaryHTTPService, secureCookie bool) *DiaryHandler {
	return &DiaryHandler{service: service, secureCookie: secureCookie}
}

func (h *DiaryHandler) create(ctx context.Context, input *createDiaryEntryInput) (*diaryEntryOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	entry, err := h.service.Create(ctx, input.Session, input.Body.ID, diaryInput(input.Body.DiaryEntryRequest))
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &diaryEntryOutput{RequestID: requestID(ctx), Body: diaryResponse(entry)}, nil
}

func (h *DiaryHandler) list(ctx context.Context, input *listDiaryEntriesInput) (*diaryEntryPageOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	page, err := h.service.List(ctx, input.Session, input.Limit, optionalString(input.AfterID), optionalString(input.DateFrom), optionalString(input.DateTo))
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	entries := make([]DiaryEntryResponse, 0, len(page.Entries))
	for _, entry := range page.Entries {
		entries = append(entries, diaryResponse(entry))
	}
	return &diaryEntryPageOutput{RequestID: requestID(ctx), Body: DiaryEntryPageResponse{Entries: entries, NextAfterID: page.NextAfterID}}, nil
}

func (h *DiaryHandler) get(ctx context.Context, input *diaryEntryInput) (*diaryEntryOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	entry, err := h.service.Get(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &diaryEntryOutput{RequestID: requestID(ctx), Body: diaryResponse(entry)}, nil
}

func (h *DiaryHandler) update(ctx context.Context, input *updateDiaryEntryInput) (*diaryEntryOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	entry, err := h.service.Update(ctx, input.Session, input.ID, input.Body.ExpectedRevision, diaryInput(input.Body.DiaryEntryRequest))
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &diaryEntryOutput{RequestID: requestID(ctx), Body: diaryResponse(entry)}, nil
}

func (h *DiaryHandler) deletionImpact(ctx context.Context, input *diaryEntryInput) (*diaryDeletionImpactOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	impact, err := h.service.DeletionImpact(ctx, input.Session, input.ID)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &diaryDeletionImpactOutput{RequestID: requestID(ctx), Body: DiaryDeletionImpactResponse{EntryID: impact.EntryID, Revision: impact.Revision, MediaCount: impact.MediaCount, PublishedPostCount: impact.PublishedPostCount, MediaRetained: impact.MediaRetained}}, nil
}

func (h *DiaryHandler) delete(ctx context.Context, input *deleteDiaryEntryInput) (*deleteDiaryEntryOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	if err := h.service.Delete(ctx, input.Session, input.ID, input.ExpectedRevision); err != nil {
		return nil, h.mapError(ctx, err)
	}
	return &deleteDiaryEntryOutput{RequestID: requestID(ctx)}, nil
}

func (h *DiaryHandler) calendar(ctx context.Context, input *calendarInput) (*calendarOutput, error) {
	if err := h.available(ctx, input.Session); err != nil {
		return nil, err
	}
	month, err := h.service.Calendar(ctx, input.Session, input.Month)
	if err != nil {
		return nil, h.mapError(ctx, err)
	}
	days := make([]CalendarDayResponse, 0, len(month.Days))
	for _, day := range month.Days {
		days = append(days, CalendarDayResponse{LocalDate: day.LocalDate, PlanCount: day.PlanCount, WearEventCount: day.WearEventCount, DiaryCount: day.DiaryCount})
	}
	return &calendarOutput{RequestID: requestID(ctx), Body: CalendarMonthResponse{Month: month.Month, Days: days}}, nil
}

func (h *DiaryHandler) available(ctx context.Context, session string) error {
	if h == nil || h.service == nil {
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
	if session == "" {
		return newErrorResponse(http.StatusUnauthorized, requestID(ctx))
	}
	return nil
}

func (h *DiaryHandler) mapError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, accountapp.ErrAuthentication):
		return authenticatedSessionError(ctx, h.secureCookie)
	case errors.Is(err, diaryapp.ErrInvalidDiaryInput):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, diaryapp.ErrDiaryNotFound):
		return newErrorResponse(http.StatusNotFound, requestID(ctx))
	case errors.Is(err, diaryapp.ErrDiaryConflict):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Request conflicts with current diary state."
		return response
	default:
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
}

func diaryInput(request DiaryEntryRequest) diaryapp.DiaryEntryInput {
	return diaryapp.DiaryEntryInput{LocalDate: request.LocalDate, TimeZone: request.TimeZone, Title: request.Title, Body: request.Body, Mood: request.Mood, Occasion: request.Occasion, PlanID: request.PlanID, WearEventID: request.WearEventID, MediaIDs: append([]string(nil), request.MediaIDs...)}
}

func diaryResponse(entry diaryapp.DiaryEntry) DiaryEntryResponse {
	return DiaryEntryResponse{ID: entry.ID, LocalDate: entry.LocalDate, TimeZone: entry.TimeZone, Title: entry.Title, Body: entry.Body, Mood: entry.Mood, Occasion: entry.Occasion, PlanID: entry.PlanID, WearEventID: entry.WearEventID, MediaIDs: append([]string(nil), entry.MediaIDs...), Revision: entry.Revision, CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
