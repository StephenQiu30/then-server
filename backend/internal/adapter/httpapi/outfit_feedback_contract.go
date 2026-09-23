package httpapi

import "time"

type FeedbackFieldsRequest struct {
	ThermalComfort  *string  `json:"thermal_comfort,omitempty" enum:"cold,comfortable,hot"`
	ActivityComfort *string  `json:"activity_comfort,omitempty" enum:"uncomfortable,okay,comfortable"`
	OccasionFit     *string  `json:"occasion_fit,omitempty" enum:"tooCasual,right,tooFormal"`
	RepeatIntent    *string  `json:"repeat_intent,omitempty" enum:"yes,unsure,no"`
	IssueTags       []string `json:"issue_tags" maxItems:"5"`
	Note            *string  `json:"note,omitempty" maxLength:"240"`
}

type SaveFeedbackRequest struct {
	FeedbackID       string `json:"feedback_id" format:"uuid"`
	MutationID       string `json:"mutation_id" format:"uuid"`
	ExpectedRevision *int   `json:"expected_revision,omitempty" minimum:"1"`
	FeedbackFieldsRequest
}

type FeedbackResponse struct {
	ID          string `json:"id" format:"uuid"`
	WearEventID string `json:"wear_event_id" format:"uuid"`
	FeedbackFieldsRequest
	Revision  int       `json:"revision" minimum:"1"`
	CreatedAt time.Time `json:"created_at" format:"date-time"`
	UpdatedAt time.Time `json:"updated_at" format:"date-time"`
}

type WearItemUseResponse struct {
	ItemID string `json:"item_id" format:"uuid"`
	Count  int    `json:"count" minimum:"1"`
}
type WearStatisticsResponse struct {
	From       string                `json:"from" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	To         string                `json:"to" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	WearDays   int                   `json:"wear_days" minimum:"0"`
	WearEvents int                   `json:"wear_events" minimum:"0"`
	ItemUses   []WearItemUseResponse `json:"item_uses"`
}

type feedbackInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"wear_event_id" format:"uuid"`
}
type saveFeedbackInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"wear_event_id" format:"uuid"`
	Body    SaveFeedbackRequest
}
type deleteFeedbackInput struct {
	Session          string `cookie:"then_session" hidden:"true"`
	ID               string `path:"wear_event_id" format:"uuid"`
	FeedbackID       string `query:"feedback_id" format:"uuid"`
	MutationID       string `query:"mutation_id" format:"uuid"`
	ExpectedRevision int    `query:"expected_revision" minimum:"1"`
}
type wearStatisticsInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	From    string `query:"from" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
	To      string `query:"to" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"`
}
type feedbackOutput struct {
	RequestID string           `header:"X-Request-ID"`
	Body      FeedbackResponse `json:"body"`
}
type wearStatisticsOutput struct {
	RequestID string                 `header:"X-Request-ID"`
	Body      WearStatisticsResponse `json:"body"`
}
