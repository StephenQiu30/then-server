package httpapi

import "time"

type createDataExportInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    struct {
		Password string `json:"password" format:"password" minLength:"1" maxLength:"72"`
		Mode     string `json:"mode" enum:"structured,with_media"`
	}
}

type dataExportInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"id" format:"uuid"`
}

type DataExportResponse struct {
	ID          string         `json:"id" format:"uuid"`
	Mode        string         `json:"mode" enum:"structured,with_media"`
	Status      string         `json:"status" enum:"preparing,ready,partial,failed,expired,revoked"`
	Counts      map[string]int `json:"counts"`
	Omissions   []string       `json:"omissions"`
	CreatedAt   time.Time      `json:"created_at" format:"date-time"`
	CompletedAt *time.Time     `json:"completed_at,omitempty" format:"date-time"`
	ExpiresAt   time.Time      `json:"expires_at" format:"date-time"`
}

type dataExportOutput struct {
	RequestID string             `header:"X-Request-ID" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$"`
	Body      DataExportResponse `json:"body"`
}

type emptyDataExportOutput struct {
	RequestID string `header:"X-Request-ID" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$"`
}
