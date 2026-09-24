package httpapi

import "time"

type accountDeletionReceiptInput struct {
	ID string `path:"id" format:"uuid"`
}

type AccountDeletionReceiptResponse struct {
	ID                       string     `json:"id" format:"uuid"`
	Status                   string     `json:"status" enum:"pending,complete"`
	Phase                    string     `json:"phase" enum:"media_cleanup,media_cleanup_retry_observed,generation_cleanup,complete"`
	AccessClosed             bool       `json:"access_closed"`
	MediaCount               int        `json:"media_count" minimum:"0"`
	RemainingMediaCount      int        `json:"remaining_media_count" minimum:"0"`
	GenerationCount          int        `json:"generation_count" minimum:"0"`
	RemainingGenerationCount int        `json:"remaining_generation_count" minimum:"0"`
	RetryObserved            bool       `json:"retry_observed"`
	RequestedAt              time.Time  `json:"requested_at" format:"date-time"`
	UpdatedAt                time.Time  `json:"updated_at" format:"date-time"`
	CompletedAt              *time.Time `json:"completed_at,omitempty" format:"date-time"`
	ReceiptExpiresAt         time.Time  `json:"receipt_expires_at" format:"date-time"`
}

type accountDeletionReceiptOutput struct {
	RequestID    string                         `header:"X-Request-ID" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$"`
	CacheControl string                         `header:"Cache-Control"`
	Body         AccountDeletionReceiptResponse `json:"body"`
}

type emptyAccountDeletionReceiptOutput struct {
	RequestID    string `header:"X-Request-ID" minLength:"26" maxLength:"64" pattern:"^[A-Za-z0-9]+$"`
	CacheControl string `header:"Cache-Control"`
}
