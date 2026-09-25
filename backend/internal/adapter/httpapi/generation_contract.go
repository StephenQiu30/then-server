package httpapi

import (
	"time"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
)

type GenerationInputReferenceRequest struct {
	MediaID  string `json:"media_id" format:"uuid"`
	Role     string `json:"role" enum:"person,garment,look_image"`
	Ordinal  int    `json:"ordinal" minimum:"0" maximum:"3"`
	Revision int    `json:"revision" minimum:"1"`
	SHA256   string `json:"sha256" pattern:"^[a-f0-9]{64}$"`
}

type GenerationConsentRequest struct {
	ID            string    `json:"id" format:"uuid"`
	Purpose       string    `json:"purpose" enum:"image,model"`
	PolicyVersion string    `json:"policy_version" minLength:"1" maxLength:"96"`
	AcceptedAt    time.Time `json:"accepted_at" format:"date-time"`
}

type CreateGenerationJobRequest struct {
	IdempotencyKey string                            `json:"idempotency_key" minLength:"1" maxLength:"256"`
	LookID         string                            `json:"look_id" format:"uuid"`
	LookRevision   int                               `json:"look_revision" minimum:"1"`
	Purpose        string                            `json:"purpose" enum:"image,model"`
	Provider       string                            `json:"provider" minLength:"1" maxLength:"96"`
	Model          string                            `json:"model" minLength:"1" maxLength:"128"`
	Parameters     map[string]any                    `json:"parameters,omitempty"`
	Inputs         []GenerationInputReferenceRequest `json:"inputs" minItems:"1" maxItems:"4"`
	ImageAssetID   string                            `json:"image_asset_id,omitempty" format:"uuid"`
	ImageSHA256    string                            `json:"image_sha256,omitempty" pattern:"^[a-f0-9]{64}$"`
	Consent        GenerationConsentRequest          `json:"consent"`
}

type GenerationOutputResponse struct {
	ID              string    `json:"id" format:"uuid"`
	ContentType     string    `json:"content_type" enum:"image/jpeg,model/gltf-binary"`
	ByteSize        int64     `json:"byte_size" minimum:"1"`
	SHA256          string    `json:"sha256" pattern:"^[a-f0-9]{64}$"`
	ObjectVersionID string    `json:"object_version_id"`
	PublishedAt     time.Time `json:"published_at" format:"date-time"`
}

type GenerationOutputAccessResponse struct {
	URL         string    `json:"url" format:"uri"`
	ExpiresAt   time.Time `json:"expires_at" format:"date-time"`
	ContentType string    `json:"content_type" enum:"image/jpeg,model/gltf-binary"`
	ByteSize    int64     `json:"byte_size" minimum:"1"`
	SHA256      string    `json:"sha256" pattern:"^[a-f0-9]{64}$"`
}

type GenerationReservationResponse struct {
	ID                  string                         `json:"id" format:"uuid"`
	State               generationapp.ReservationState `json:"state" enum:"reserved,released,consumed"`
	ReservedQuotaUnits  int                            `json:"reserved_quota_units" minimum:"0"`
	EstimatedMinorUnits int64                          `json:"estimated_minor_units" minimum:"0"`
	Currency            string                         `json:"currency,omitempty"`
	StateRevision       int                            `json:"state_revision" minimum:"1"`
	CreatedAt           time.Time                      `json:"created_at" format:"date-time"`
	UpdatedAt           time.Time                      `json:"updated_at" format:"date-time"`
}

type GenerationJobResponse struct {
	ID                string                         `json:"id" format:"uuid"`
	LookID            string                         `json:"look_id" format:"uuid"`
	LookRevision      int                            `json:"look_revision" minimum:"1"`
	Purpose           generationapp.Purpose          `json:"purpose" enum:"image,model"`
	Provider          string                         `json:"provider"`
	Model             string                         `json:"model"`
	Status            generationapp.Status           `json:"status" enum:"queued,running,validating,succeeded,failed,canceled,expired"`
	StatusRevision    int                            `json:"status_revision" minimum:"1"`
	SubmissionState   generationapp.SubmissionState  `json:"submission_state" enum:"not_started,in_flight,unknown,accepted"`
	SubmissionAttempt int                            `json:"submission_attempt" minimum:"0"`
	ExternalTaskID    string                         `json:"external_task_id,omitempty"`
	ResultAssetID     string                         `json:"result_asset_id,omitempty" format:"uuid"`
	FailureCode       string                         `json:"failure_code,omitempty"`
	CancelRequestedAt *time.Time                     `json:"cancel_requested_at,omitempty" format:"date-time"`
	AccessRevokedAt   *time.Time                     `json:"access_revoked_at,omitempty" format:"date-time"`
	Reservation       *GenerationReservationResponse `json:"reservation,omitempty"`
	Output            *GenerationOutputResponse      `json:"output,omitempty"`
	Cleanup           *GenerationCleanupResponse     `json:"cleanup,omitempty"`
	Reused            bool                           `json:"reused,omitempty"`
	Match             generationapp.RequestMatch     `json:"match,omitempty" enum:"none,idempotent_replay,content_dedupe"`
	CreatedAt         time.Time                      `json:"created_at" format:"date-time"`
	UpdatedAt         time.Time                      `json:"updated_at" format:"date-time"`
}

type GenerationCleanupResponse struct {
	ID              string                      `json:"id" format:"uuid"`
	Status          generationapp.CleanupStatus `json:"status" enum:"pending,running,complete,failed"`
	AccessRevokedAt time.Time                   `json:"access_revoked_at" format:"date-time"`
	CompletedAt     *time.Time                  `json:"completed_at,omitempty" format:"date-time"`
	Attempts        int                         `json:"attempts" minimum:"0"`
	NextAttemptAt   *time.Time                  `json:"next_attempt_at,omitempty" format:"date-time"`
}

type GenerationJobPageResponse struct {
	Jobs        []GenerationJobResponse `json:"jobs" maxItems:"100"`
	NextAfterID *string                 `json:"next_after_id,omitempty" format:"uuid"`
}

type createGenerationJobInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Body    CreateGenerationJobRequest
}

type listGenerationJobsInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	Limit   int    `query:"limit" default:"20" minimum:"1" maximum:"100"`
	AfterID string `query:"after_id" format:"uuid" required:"false"`
}

type generationJobInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"job_id" format:"uuid"`
}

type generationJobOutput struct {
	RequestID string                `header:"X-Request-ID"`
	Body      GenerationJobResponse `json:"body"`
}

type generationOutputAccessOutput struct {
	RequestID string                         `header:"X-Request-ID"`
	Body      GenerationOutputAccessResponse `json:"body"`
}

type generationJobPageOutput struct {
	RequestID string                    `header:"X-Request-ID"`
	Body      GenerationJobPageResponse `json:"body"`
}

type ListUnknownSubmissionsRequest struct {
	Session string `cookie:"then_session" hidden:"true"`
	Limit   int    `query:"limit" default:"20" minimum:"1" maximum:"100"`
	AfterID string `query:"after_id" format:"uuid" required:"false"`
}

type UnknownSubmissionResponse struct {
	ID                string                `json:"id" format:"uuid"`
	Purpose           generationapp.Purpose `json:"purpose" enum:"image,model"`
	Provider          string                `json:"provider"`
	Model             string                `json:"model"`
	StatusRevision    int                   `json:"status_revision" minimum:"1"`
	SubmissionAttempt int                   `json:"submission_attempt" minimum:"1"`
	UnknownAt         time.Time             `json:"unknown_at" format:"date-time"`
	CancelRequestedAt *time.Time            `json:"cancel_requested_at,omitempty" format:"date-time"`
	AccessRevokedAt   *time.Time            `json:"access_revoked_at,omitempty" format:"date-time"`
	CreatedAt         time.Time             `json:"created_at" format:"date-time"`
	UpdatedAt         time.Time             `json:"updated_at" format:"date-time"`
}

type UnknownSubmissionPageResponse struct {
	Jobs        []UnknownSubmissionResponse `json:"jobs" maxItems:"100"`
	NextAfterID *string                     `json:"next_after_id,omitempty" format:"uuid"`
}

type unknownSubmissionPageOutput struct {
	RequestID string                        `header:"X-Request-ID"`
	Body      UnknownSubmissionPageResponse `json:"body"`
}

type ReconcileUnknownSubmissionRequest struct {
	ExpectedRevision  int                                  `json:"expected_revision" minimum:"1"`
	Decision          generationapp.SubmissionDecision     `json:"decision" enum:"accepted,not_accepted"`
	ExternalTaskID    string                               `json:"external_task_id,omitempty" maxLength:"256"`
	EvidenceType      generationapp.SubmissionEvidenceType `json:"evidence_type" enum:"provider_console,provider_query,support_case"`
	EvidenceReference string                               `json:"evidence_reference" pattern:"^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$"`
}

type reconcileUnknownSubmissionInput struct {
	Session string `cookie:"then_session" hidden:"true"`
	ID      string `path:"job_id" format:"uuid"`
	Body    ReconcileUnknownSubmissionRequest
}

type SubmissionReconciliationResponse struct {
	AuditID    string                           `json:"audit_id" format:"uuid"`
	Decision   generationapp.SubmissionDecision `json:"decision" enum:"accepted,not_accepted"`
	RecordedAt time.Time                        `json:"recorded_at" format:"date-time"`
	Job        GenerationJobResponse            `json:"job"`
}

type submissionReconciliationOutput struct {
	RequestID string                           `header:"X-Request-ID"`
	Body      SubmissionReconciliationResponse `json:"body"`
}
