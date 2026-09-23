package httpapi

type syncChangesInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	After   string `query:"after" required:"false" maxLength:"128"`
	Limit   int    `query:"limit" default:"50" minimum:"1" maximum:"100"`
}

type SyncChangeResponse struct {
	Seq      int64  `json:"seq" minimum:"1"`
	Kind     string `json:"kind" enum:"wardrobe_item,outfit_plan,wear_event,wear_feedback,diary_entry"`
	EntityID string `json:"entity_id" format:"uuid"`
	Action   string `json:"action" enum:"upsert,delete"`
	Revision *int   `json:"revision"`
}

type SyncChangesResponse struct {
	Changes    []SyncChangeResponse `json:"changes" maxItems:"100"`
	NextCursor string               `json:"next_cursor"`
	HasMore    bool                 `json:"has_more"`
}

type syncChangesOutput struct {
	RequestID string `header:"X-Request-ID"`
	Body      SyncChangesResponse
}
