package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
	"github.com/danielgtaylor/huma/v2"
)

type DiaryHTTPService interface {
	Create(context.Context, string, string, domain.DiaryEntryInput) (domain.DiaryEntry, error)
	List(context.Context, string, int, *string, *string, *string) (domain.DiaryEntryPage, error)
	Get(context.Context, string, string) (domain.DiaryEntry, error)
	Update(context.Context, string, string, int, domain.DiaryEntryInput) (domain.DiaryEntry, error)
	DeletionImpact(context.Context, string, string) (domain.DiaryDeletionImpact, error)
	Delete(context.Context, string, string, int) error
	Calendar(context.Context, string, string) (domain.CalendarMonth, error)
}

type DiaryHandler struct {
	service      DiaryHTTPService
	secureCookie bool
}

func NewDiaryHandler(service DiaryHTTPService, secureCookie bool) *DiaryHandler {
	return &DiaryHandler{service: service, secureCookie: secureCookie}
}

type DiaryEntryRequest struct {
	LocalDate   string   `json:"local_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	TimeZone    string   `json:"time_zone" minLength:"1" maxLength:"255"`
	Title       *string  `json:"title,omitempty" maxLength:"80"`
	Body        *string  `json:"body,omitempty" maxLength:"5000"`
	Mood        *string  `json:"mood,omitempty" maxLength:"40"`
	Occasion    *string  `json:"occasion,omitempty" maxLength:"40"`
	PlanID      *string  `json:"plan_id,omitempty" format:"uuid"`
	WearEventID *string  `json:"wear_event_id,omitempty" format:"uuid"`
	MediaIDs    []string `json:"media_ids" maxItems:"9"`
}

type CreateDiaryEntryRequest struct {
	ID string `json:"id" format:"uuid"`
	DiaryEntryRequest
}

type UpdateDiaryEntryRequest struct {
	ExpectedRevision int `json:"expected_revision" minimum:"1"`
	DiaryEntryRequest
}

type DiaryEntryResponse struct {
	ID          string    `json:"id" format:"uuid"`
	LocalDate   string    `json:"local_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	TimeZone    string    `json:"time_zone" minLength:"1" maxLength:"255"`
	Title       *string   `json:"title,omitempty" maxLength:"80"`
	Body        *string   `json:"body,omitempty" maxLength:"5000"`
	Mood        *string   `json:"mood,omitempty" maxLength:"40"`
	Occasion    *string   `json:"occasion,omitempty" maxLength:"40"`
	PlanID      *string   `json:"plan_id,omitempty" format:"uuid"`
	WearEventID *string   `json:"wear_event_id,omitempty" format:"uuid"`
	MediaIDs    []string  `json:"media_ids" maxItems:"9"`
	Revision    int       `json:"revision" minimum:"1"`
	CreatedAt   time.Time `json:"created_at" format:"date-time"`
	UpdatedAt   time.Time `json:"updated_at" format:"date-time"`
}

type DiaryEntryPageResponse struct {
	Entries     []DiaryEntryResponse `json:"entries" maxItems:"50"`
	NextAfterID *string              `json:"next_after_id,omitempty" format:"uuid"`
}

type DiaryDeletionImpactResponse struct {
	EntryID            string `json:"entry_id" format:"uuid"`
	Revision           int    `json:"revision" minimum:"1"`
	MediaCount         int    `json:"media_count" minimum:"0" maximum:"9"`
	PublishedPostCount int    `json:"published_post_count" minimum:"0"`
	MediaRetained      bool   `json:"media_retained"`
}

type CalendarDayResponse struct {
	LocalDate      string `json:"local_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	PlanCount      int    `json:"plan_count" minimum:"0"`
	WearEventCount int    `json:"wear_event_count" minimum:"0"`
	DiaryCount     int    `json:"diary_count" minimum:"0"`
}

type CalendarMonthResponse struct {
	Month string                `json:"month" pattern:"^[0-9]{4}-[0-9]{2}$"`
	Days  []CalendarDayResponse `json:"days" maxItems:"31"`
}

type createDiaryEntryInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    CreateDiaryEntryRequest
}
type listDiaryEntriesInput struct {
	Session  string `cookie:"then_session" hidden:"true"`
	Limit    int    `query:"limit" default:"20" minimum:"1" maximum:"50"`
	AfterID  string `query:"after_id" format:"uuid" required:"false"`
	DateFrom string `query:"date_from" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$" required:"false"`
	DateTo   string `query:"date_to" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$" required:"false"`
}
type diaryEntryInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"entry_id" format:"uuid"`
}
type updateDiaryEntryInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"entry_id" format:"uuid"`
	Body    UpdateDiaryEntryRequest
}
type deleteDiaryEntryInput struct {
	Session          string `cookie:"then_session" hidden:"true"`
	ID               string `path:"entry_id" format:"uuid"`
	ExpectedRevision int    `query:"expected_revision" minimum:"1"`
}
type calendarInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Month   string `query:"month" pattern:"^[0-9]{4}-[0-9]{2}$"`
}

type diaryEntryOutput struct {
	RequestID string             `header:"X-Request-ID"`
	Body      DiaryEntryResponse `json:"body"`
}
type diaryEntryPageOutput struct {
	RequestID string                 `header:"X-Request-ID"`
	Body      DiaryEntryPageResponse `json:"body"`
}
type diaryDeletionImpactOutput struct {
	RequestID string                      `header:"X-Request-ID"`
	Body      DiaryDeletionImpactResponse `json:"body"`
}
type calendarOutput struct {
	RequestID string                `header:"X-Request-ID"`
	Body      CalendarMonthResponse `json:"body"`
}
type deleteDiaryEntryOutput struct {
	RequestID string `header:"X-Request-ID"`
}

func registerDiaryOperations(api huma.API, handler *DiaryHandler) {
	common := []int{http.StatusUnauthorized, http.StatusBadRequest, http.StatusNotFound, http.StatusConflict, http.StatusInternalServerError}
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "createDiaryEntry", Method: http.MethodPost, Path: "/diary-entries", Tags: []string{"Diary"}, Summary: "创建本人私人穿搭日记", DefaultStatus: http.StatusCreated, MaxBodyBytes: 32 * 1024, Errors: common}), handler.create)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "listDiaryEntries", Method: http.MethodGet, Path: "/diary-entries", Tags: []string{"Diary"}, Summary: "稳定分页列出本人日记", Errors: common}), handler.list)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getDiaryEntry", Method: http.MethodGet, Path: "/diary-entries/{entry_id}", Tags: []string{"Diary"}, Summary: "读取本人私人日记", Errors: common}), handler.get)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "updateDiaryEntry", Method: http.MethodPut, Path: "/diary-entries/{entry_id}", Tags: []string{"Diary"}, Summary: "按 revision 完整更新本人日记", MaxBodyBytes: 32 * 1024, Errors: common}), handler.update)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getDiaryEntryDeletionImpact", Method: http.MethodGet, Path: "/diary-entries/{entry_id}/deletion-impact", Tags: []string{"Diary"}, Summary: "查看日记删除影响", Errors: common}), handler.deletionImpact)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "deleteDiaryEntry", Method: http.MethodDelete, Path: "/diary-entries/{entry_id}", Tags: []string{"Diary"}, Summary: "永久删除日记并保留独立事实", DefaultStatus: http.StatusNoContent, Errors: common}), handler.delete)
	huma.Register(api, authenticatedOperation(huma.Operation{OperationID: "getCalendarMonth", Method: http.MethodGet, Path: "/calendar", Tags: []string{"Diary"}, Summary: "按原本地日期聚合计划、实际穿着和日记", Errors: common}), handler.calendar)
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
	case errors.Is(err, domain.ErrAuthentication):
		return authenticatedSessionError(ctx, h.secureCookie)
	case errors.Is(err, domain.ErrInvalidDiaryInput):
		return newErrorResponse(http.StatusBadRequest, requestID(ctx))
	case errors.Is(err, domain.ErrDiaryNotFound):
		return newErrorResponse(http.StatusNotFound, requestID(ctx))
	case errors.Is(err, domain.ErrDiaryConflict):
		response := newErrorResponse(http.StatusConflict, requestID(ctx))
		response.Code, response.Message = "CONFLICT", "Request conflicts with current diary state."
		return response
	default:
		return newErrorResponse(http.StatusInternalServerError, requestID(ctx))
	}
}

func diaryInput(request DiaryEntryRequest) domain.DiaryEntryInput {
	return domain.DiaryEntryInput{LocalDate: request.LocalDate, TimeZone: request.TimeZone, Title: request.Title, Body: request.Body, Mood: request.Mood, Occasion: request.Occasion, PlanID: request.PlanID, WearEventID: request.WearEventID, MediaIDs: append([]string(nil), request.MediaIDs...)}
}

func diaryResponse(entry domain.DiaryEntry) DiaryEntryResponse {
	return DiaryEntryResponse{ID: entry.ID, LocalDate: entry.LocalDate, TimeZone: entry.TimeZone, Title: entry.Title, Body: entry.Body, Mood: entry.Mood, Occasion: entry.Occasion, PlanID: entry.PlanID, WearEventID: entry.WearEventID, MediaIDs: append([]string(nil), entry.MediaIDs...), Revision: entry.Revision, CreatedAt: entry.CreatedAt, UpdatedAt: entry.UpdatedAt}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
