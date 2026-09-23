package httpapi

import "time"

type SessionResponse struct {
	ID        string    `json:"id" format:"uuid"`
	CreatedAt time.Time `json:"created_at" format:"date-time"`
	ExpiresAt time.Time `json:"expires_at" format:"date-time"`
	Current   bool      `json:"current"`
}

type SessionPageResponse struct {
	Items      []SessionResponse `json:"items" maxItems:"100"`
	NextOffset *int              `json:"next_offset"`
}

type listSessionsInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Limit   int    `query:"limit" default:"50" minimum:"1" maximum:"100"`
	Offset  int    `query:"offset" default:"0" minimum:"0" maximum:"10000"`
}

type revokeSessionInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"session_id" format:"uuid"`
}

type sessionPageOutput struct {
	RequestID    string              `header:"X-Request-ID"`
	CacheControl string              `header:"Cache-Control"`
	Body         SessionPageResponse `json:"body"`
}

type revokedSessionOutput struct {
	RequestID    string `header:"X-Request-ID"`
	CacheControl string `header:"Cache-Control"`
	SetCookie    string `header:"Set-Cookie"`
}
