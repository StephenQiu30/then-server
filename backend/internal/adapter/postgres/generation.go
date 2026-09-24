package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GenerationRepository persists the provider-neutral task workflow. It does
// not contain a provider client and therefore cannot spend money by itself.
type GenerationRepository struct{ database *gorm.DB }

func NewGenerationRepository(database *gorm.DB) *GenerationRepository {
	return &GenerationRepository{database: database}
}

type generationJobRecord struct {
	ID                  string     `gorm:"column:id;type:uuid;primaryKey"`
	OwnerID             string     `gorm:"column:owner_id;type:uuid;not null;index:generation_jobs_owner_order_idx,priority:1;uniqueIndex:generation_jobs_owner_purpose_idempotency_unique,priority:1;index:generation_jobs_owner_purpose_dedupe_idx,priority:1"`
	LookID              string     `gorm:"column:look_id;type:uuid;not null"`
	LookRevision        int        `gorm:"column:look_revision;not null;check:generation_jobs_look_revision_check,look_revision >= 1"`
	Purpose             string     `gorm:"column:purpose;type:text;not null;uniqueIndex:generation_jobs_owner_purpose_idempotency_unique,priority:2;index:generation_jobs_owner_purpose_dedupe_idx,priority:2;check:generation_jobs_purpose_check,purpose IN ('image','model')"`
	Provider            string     `gorm:"column:provider;type:text;not null;check:generation_jobs_provider_check,char_length(provider) BETWEEN 1 AND 96"`
	Model               string     `gorm:"column:model;type:text;not null;check:generation_jobs_model_check,char_length(model) BETWEEN 1 AND 128"`
	Parameters          []byte     `gorm:"column:parameters;type:jsonb;not null"`
	Inputs              []byte     `gorm:"column:inputs;type:jsonb;not null"`
	Consent             []byte     `gorm:"column:consent;type:jsonb;not null"`
	Currency            string     `gorm:"column:currency;type:text;not null;default:''"`
	EstimatedMinorUnits int64      `gorm:"column:estimated_minor_units;not null;check:generation_jobs_estimated_minor_units_check,estimated_minor_units >= 0"`
	ReservedQuotaUnits  int        `gorm:"column:reserved_quota_units;not null;check:generation_jobs_reserved_quota_check,reserved_quota_units >= 0"`
	IdempotencyKeyHash  string     `gorm:"column:idempotency_key_hash;type:char(64);not null;uniqueIndex:generation_jobs_owner_purpose_idempotency_unique,priority:3"`
	DedupeKey           string     `gorm:"column:dedupe_key;type:char(64);not null;index:generation_jobs_owner_purpose_dedupe_idx,priority:3"`
	Status              string     `gorm:"column:status;type:text;not null;index:generation_jobs_owner_status_idx,priority:2;check:generation_jobs_status_check,status IN ('queued','running','validating','succeeded','failed','canceled','expired')"`
	StatusRevision      int        `gorm:"column:status_revision;not null;check:generation_jobs_status_revision_check,status_revision >= 1"`
	SubmissionState     string     `gorm:"column:submission_state;type:text;not null;check:generation_jobs_submission_state_check,submission_state IN ('not_started','in_flight','unknown','accepted')"`
	SubmissionAttempt   int        `gorm:"column:submission_attempt;not null;check:generation_jobs_submission_attempt_check,submission_attempt >= 0"`
	SubmissionStartedAt *time.Time `gorm:"column:submission_started_at;type:timestamptz"`
	SubmissionUnknownAt *time.Time `gorm:"column:submission_unknown_at;type:timestamptz"`
	NextAttemptAt       *time.Time `gorm:"column:next_attempt_at;type:timestamptz;index:generation_jobs_next_attempt_idx"`
	CancelRequestedAt   *time.Time `gorm:"column:cancel_requested_at;type:timestamptz"`
	AccessRevokedAt     *time.Time `gorm:"column:access_revoked_at;type:timestamptz;index:generation_jobs_access_revoked_idx"`
	ExternalTaskID      string     `gorm:"column:external_task_id;type:text;not null;default:''"`
	ResultAssetID       string     `gorm:"column:result_asset_id;type:text"`
	FailureCode         string     `gorm:"column:failure_code;type:text;not null;default:''"`
	LeaseOwner          string     `gorm:"column:lease_owner;type:text;not null;default:''"`
	FencingToken        uint64     `gorm:"column:fencing_token;not null;default:0"`
	LeaseAttempt        int        `gorm:"column:lease_attempt;not null;default:0;check:generation_jobs_lease_attempt_check,lease_attempt >= 0"`
	LeaseUntil          *time.Time `gorm:"column:lease_until;type:timestamptz"`
	CreatedAt           time.Time  `gorm:"column:created_at;type:timestamptz;not null;index:generation_jobs_owner_order_idx,priority:2,sort:desc"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;type:timestamptz;not null;check:generation_jobs_timestamps_check,updated_at >= created_at"`
}

func (generationJobRecord) TableName() string { return "generation_jobs" }

type generationQuotaReservationRecord struct {
	ID                  string    `gorm:"column:id;type:uuid;primaryKey"`
	TaskID              string    `gorm:"column:task_id;type:uuid;not null;uniqueIndex:generation_quota_task_unique"`
	OwnerID             string    `gorm:"column:owner_id;type:uuid;not null;index:generation_quota_owner_state_idx,priority:1"`
	Purpose             string    `gorm:"column:purpose;type:text;not null;check:generation_quota_purpose_check,purpose IN ('image','model')"`
	Currency            string    `gorm:"column:currency;type:text;not null;default:''"`
	ReservedQuotaUnits  int       `gorm:"column:reserved_quota_units;not null;check:generation_quota_units_check,reserved_quota_units >= 0"`
	EstimatedMinorUnits int64     `gorm:"column:estimated_minor_units;not null;check:generation_quota_minor_units_check,estimated_minor_units >= 0"`
	State               string    `gorm:"column:state;type:text;not null;index:generation_quota_owner_state_idx,priority:2;check:generation_quota_state_check,state IN ('reserved','released','consumed')"`
	StateRevision       int       `gorm:"column:state_revision;not null;check:generation_quota_revision_check,state_revision >= 1"`
	CreatedAt           time.Time `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt           time.Time `gorm:"column:updated_at;type:timestamptz;not null;check:generation_quota_timestamps_check,updated_at >= created_at"`
}

func (generationQuotaReservationRecord) TableName() string { return "generation_quota_reservations" }

type generationOutputRecord struct {
	ID                 string    `gorm:"column:id;type:uuid;primaryKey"`
	TaskID             string    `gorm:"column:task_id;type:uuid;not null;uniqueIndex:generation_outputs_task_unique"`
	OwnerID            string    `gorm:"column:owner_id;type:uuid;not null;index:generation_outputs_owner_idx"`
	LookID             string    `gorm:"column:look_id;type:uuid;not null"`
	LookRevision       int       `gorm:"column:look_revision;not null;check:generation_outputs_look_revision_check,look_revision >= 1"`
	Purpose            string    `gorm:"column:purpose;type:text;not null;check:generation_outputs_purpose_check,purpose IN ('image','model')"`
	SourceImageAssetID string    `gorm:"column:source_image_asset_id;type:text"`
	SourceImageSHA256  string    `gorm:"column:source_image_sha256;type:char(64)"`
	ContentType        string    `gorm:"column:content_type;type:text;not null;check:generation_outputs_content_type_check,content_type IN ('image/jpeg','model/gltf-binary')"`
	ByteSize           int64     `gorm:"column:byte_size;not null;check:generation_outputs_byte_size_check,byte_size > 0"`
	SHA256             string    `gorm:"column:sha256;type:char(64);not null"`
	ObjectVersionID    string    `gorm:"column:object_version_id;type:text;not null"`
	PublishedAt        time.Time `gorm:"column:published_at;type:timestamptz;not null"`
}

func (generationOutputRecord) TableName() string { return "generation_outputs" }

func (r *GenerationRepository) Accept(ctx context.Context, input generationapp.CreateInput, policy generationapp.AdmissionPolicy, reservationID, outboxID string, at time.Time) (generationapp.AcceptanceResult, error) {
	var result generationapp.AcceptanceResult
	if r == nil || r.database == nil {
		return result, generationapp.ErrGenerationUnavailable
	}
	if _, _, err := generationapp.Identity(input); err != nil {
		return result, err
	}
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ?", input.OwnerID, "active").First(&user).Error; err != nil {
			return generationLookupError(err)
		}
		idempotencyHash, dedupeKey, err := generationapp.Identity(input)
		if err != nil {
			return err
		}
		var records []generationJobRecord
		if err := tx.Where("owner_id = ? AND purpose = ? AND (idempotency_key_hash = ? OR dedupe_key = ?)", input.OwnerID, input.Purpose, idempotencyHash, dedupeKey).
			Order("created_at DESC").Order("id ASC").Find(&records).Error; err != nil {
			return err
		}
		existing := make([]generationapp.Task, 0, len(records))
		for _, record := range records {
			task, err := generationTaskFromRecord(record)
			if err != nil {
				return err
			}
			existing = append(existing, task)
		}
		usage, err := generationUsage(tx, input.OwnerID)
		if err != nil {
			return err
		}
		accepted, err := generationapp.PrepareAcceptance(policy, usage, existing, input, reservationID, outboxID, at)
		if err != nil {
			return err
		}
		if accepted.Reused {
			view, err := r.readTaskView(tx, accepted.Task)
			if err != nil {
				return err
			}
			acceptedResult := accepted
			result = acceptedResult
			result.Task = view.Task
			result.Reservation = view.Reservation
			return nil
		}
		if err := requireGenerationImageSource(tx, accepted.Task); err != nil {
			return err
		}
		job, err := generationJobRecordFromTask(accepted.Task)
		if err != nil {
			return err
		}
		if err := tx.Create(&job).Error; err != nil {
			return err
		}
		if accepted.Reservation != nil {
			reservation, err := generationReservationRecordFromDomain(*accepted.Reservation)
			if err != nil {
				return err
			}
			if err := tx.Create(&reservation).Error; err != nil {
				return err
			}
		}
		payload, err := json.Marshal(struct {
			TaskID   string `json:"task_id"`
			Revision int    `json:"revision"`
			Purpose  string `json:"purpose"`
		}{TaskID: accepted.Task.ID, Revision: accepted.Task.StatusRevision, Purpose: string(accepted.Task.Purpose)})
		if err != nil {
			return err
		}
		if err := tx.Create(&outboxEventRecord{ID: accepted.Event.ID, EventType: accepted.Event.EventType, AggregateID: accepted.Event.AggregateID, Payload: payload, CreatedAt: accepted.Event.CreatedAt}).Error; err != nil {
			return err
		}
		var persisted generationJobRecord
		if err := tx.Where("owner_id = ? AND id = ?", input.OwnerID, accepted.Task.ID).First(&persisted).Error; err != nil {
			return err
		}
		persistedTask, err := generationTaskFromRecord(persisted)
		if err != nil {
			return err
		}
		view, err := r.readTaskView(tx, persistedTask)
		if err != nil {
			return err
		}
		result = accepted
		result.Task = view.Task
		result.Reservation = view.Reservation
		result.Event = accepted.Event
		return nil
	})
	if err != nil {
		return generationapp.AcceptanceResult{}, generationGenerationError(err)
	}
	return result, nil
}

// requireGenerationImageSource keeps model admission tied to a confirmed
// image output instead of trusting a client-supplied asset ID and digest. The
// source task is locked in share mode so a concurrent source cleanup cannot
// revoke it between validation and model task insertion.
func requireGenerationImageSource(tx *gorm.DB, task generationapp.Task) error {
	if task.Purpose != generationapp.PurposeModel {
		return nil
	}
	var output generationOutputRecord
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where(
		"id = ? AND owner_id = ? AND look_id = ? AND look_revision = ? AND purpose = ? AND content_type = ? AND sha256 = ?",
		task.Inputs.ImageAssetID,
		task.OwnerID,
		task.LookID,
		task.LookRevision,
		string(generationapp.PurposeImage),
		generationapp.OutputContentTypeJPEG,
		task.Inputs.ImageSHA256,
	).First(&output).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return generationapp.ErrGenerationSourceUnavailable
		}
		return err
	}
	var source generationJobRecord
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where(
		"id = ? AND owner_id = ? AND status = ? AND access_revoked_at IS NULL AND result_asset_id = ?",
		output.TaskID,
		task.OwnerID,
		string(generationapp.StatusSucceeded),
		output.ID,
	).First(&source).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return generationapp.ErrGenerationSourceUnavailable
		}
		return err
	}
	sourceTask, err := generationTaskFromRecord(source)
	if err != nil {
		return err
	}
	if _, err := generationOutputFromRecord(output, sourceTask); err != nil {
		return generationapp.ErrGenerationSourceUnavailable
	}
	return nil
}

func (r *GenerationRepository) Get(ctx context.Context, ownerID, taskID string) (generationapp.TaskView, error) {
	if r == nil || r.database == nil {
		return generationapp.TaskView{}, generationapp.ErrGenerationUnavailable
	}
	var record generationJobRecord
	if err := r.database.WithContext(ctx).Where("owner_id = ? AND id = ?", ownerID, taskID).First(&record).Error; err != nil {
		return generationapp.TaskView{}, generationLookupError(err)
	}
	task, err := generationTaskFromRecord(record)
	if err != nil {
		return generationapp.TaskView{}, generationGenerationError(err)
	}
	return r.readTaskView(r.database.WithContext(ctx), task)
}

func (r *GenerationRepository) List(ctx context.Context, ownerID string, limit int, afterID *string) (generationapp.TaskPage, error) {
	if r == nil || r.database == nil {
		return generationapp.TaskPage{}, generationapp.ErrGenerationUnavailable
	}
	query := r.database.WithContext(ctx).Where("owner_id = ?", ownerID)
	if afterID != nil {
		var cursor generationJobRecord
		if err := r.database.WithContext(ctx).Where("owner_id = ? AND id = ?", ownerID, *afterID).First(&cursor).Error; err != nil {
			return generationapp.TaskPage{}, generationLookupError(err)
		}
		query = query.Where("created_at < ? OR (created_at = ? AND id > ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	var records []generationJobRecord
	if err := query.Order("created_at DESC").Order("id ASC").Limit(limit + 1).Find(&records).Error; err != nil {
		return generationapp.TaskPage{}, generationapp.ErrGenerationUnavailable
	}
	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}
	page := generationapp.TaskPage{Items: make([]generationapp.TaskView, 0, len(records))}
	for _, record := range records {
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return generationapp.TaskPage{}, generationGenerationError(err)
		}
		view, err := r.readTaskView(r.database.WithContext(ctx), task)
		if err != nil {
			return generationapp.TaskPage{}, err
		}
		page.Items = append(page.Items, view)
	}
	if hasMore && len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1].Task.ID
		page.NextAfterID = &last
	}
	return page, nil
}

func (r *GenerationRepository) RequestCancel(ctx context.Context, ownerID, taskID string, at time.Time) (generationapp.TaskView, error) {
	if r == nil || r.database == nil {
		return generationapp.TaskView{}, generationapp.ErrGenerationUnavailable
	}
	var view generationapp.TaskView
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record generationJobRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, taskID).First(&record).Error; err != nil {
			return generationLookupError(err)
		}
		task, err := generationTaskFromRecord(record)
		if err != nil {
			return err
		}
		oldRevision := task.StatusRevision
		if err := task.RequestCancel(at); err != nil {
			return err
		}
		if task.StatusRevision != oldRevision {
			updates := map[string]any{"cancel_requested_at": task.CancelRequestedAt, "access_revoked_at": task.AccessRevokedAt, "status_revision": task.StatusRevision, "updated_at": task.UpdatedAt}
			updated := tx.Model(&generationJobRecord{}).Where("owner_id = ? AND id = ? AND status_revision = ?", ownerID, taskID, oldRevision).Updates(updates)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return generationapp.ErrGenerationUnavailable
			}
		}
		var persisted generationJobRecord
		if err := tx.Where("owner_id = ? AND id = ?", ownerID, taskID).First(&persisted).Error; err != nil {
			return err
		}
		persistedTask, err := generationTaskFromRecord(persisted)
		if err != nil {
			return err
		}
		view, err = r.readTaskView(tx, persistedTask)
		return err
	})
	if err != nil {
		return generationapp.TaskView{}, generationGenerationError(err)
	}
	return view, nil
}

type generationUsageSnapshot struct {
	ActiveTasks        int
	ReservedQuotaUnits int
	ReservedMinorUnits int64
}

func generationUsage(tx *gorm.DB, ownerID string) (generationapp.AdmissionUsage, error) {
	var active int64
	if err := tx.Model(&generationJobRecord{}).Where("owner_id = ? AND status IN ?", ownerID, []string{"queued", "running", "validating"}).Count(&active).Error; err != nil {
		return generationapp.AdmissionUsage{}, err
	}
	var reserved generationUsageSnapshot
	if err := tx.Model(&generationQuotaReservationRecord{}).Select("COALESCE(SUM(reserved_quota_units), 0) AS reserved_quota_units, COALESCE(SUM(estimated_minor_units), 0) AS reserved_minor_units").Where("owner_id = ? AND state = ?", ownerID, string(generationapp.ReservationReserved)).Scan(&reserved).Error; err != nil {
		return generationapp.AdmissionUsage{}, err
	}
	return generationapp.AdmissionUsage{ActiveTasks: int(active), ReservedQuotaUnits: reserved.ReservedQuotaUnits, ReservedMinorUnits: reserved.ReservedMinorUnits}, nil
}

func (r *GenerationRepository) readTaskView(database *gorm.DB, task generationapp.Task) (generationapp.TaskView, error) {
	view := generationapp.TaskView{Task: task}
	var reservation generationQuotaReservationRecord
	if err := database.Where("task_id = ?", task.ID).First(&reservation).Error; err == nil {
		domain, err := generationReservationFromRecord(reservation)
		if err != nil {
			return generationapp.TaskView{}, err
		}
		view.Reservation = &domain
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return generationapp.TaskView{}, generationapp.ErrGenerationUnavailable
	}
	var output generationOutputRecord
	if err := database.Where("task_id = ?", task.ID).First(&output).Error; err == nil {
		if task.AccessRevokedAt == nil {
			domain, err := generationOutputFromRecord(output, task)
			if err != nil {
				return generationapp.TaskView{}, err
			}
			view.Asset = &domain
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return generationapp.TaskView{}, generationapp.ErrGenerationUnavailable
	}
	var cleanup generationCleanupRequestRecord
	if err := database.
		Where("task_id = ? AND scope IN ?", task.ID, []string{string(generationapp.CleanupScopeTask), string(generationapp.CleanupScopeSource), string(generationapp.CleanupScopeAccount)}).
		Order("CASE scope WHEN 'task' THEN 1 WHEN 'source' THEN 2 WHEN 'account' THEN 3 ELSE 4 END").
		Order("created_at DESC").
		First(&cleanup).Error; err == nil {
		domain, err := generationCleanupFromRecord(cleanup)
		if err != nil {
			return generationapp.TaskView{}, err
		}
		view.Cleanup = &domain
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return generationapp.TaskView{}, generationapp.ErrGenerationUnavailable
	}
	return view, nil
}

func generationJobRecordFromTask(task generationapp.Task) (generationJobRecord, error) {
	if err := task.Validate(); err != nil {
		return generationJobRecord{}, err
	}
	inputs, err := json.Marshal(task.Inputs)
	if err != nil {
		return generationJobRecord{}, generationapp.ErrInvalidGenerationState
	}
	consent, err := json.Marshal(task.Consent)
	if err != nil {
		return generationJobRecord{}, generationapp.ErrInvalidGenerationState
	}
	return generationJobRecord{ID: task.ID, OwnerID: task.OwnerID, LookID: task.LookID, LookRevision: task.LookRevision, Purpose: string(task.Purpose), Provider: task.Provider, Model: task.Model, Parameters: append([]byte(nil), task.Parameters...), Inputs: inputs, Consent: consent, Currency: task.Cost.Currency, EstimatedMinorUnits: task.Cost.EstimatedMinorUnits, ReservedQuotaUnits: task.Cost.ReservedQuotaUnits, IdempotencyKeyHash: task.IdempotencyKeyHash, DedupeKey: task.DedupeKey, Status: string(task.Status), StatusRevision: task.StatusRevision, SubmissionState: string(task.SubmissionState), SubmissionAttempt: task.SubmissionAttempt, SubmissionStartedAt: task.SubmissionStartedAt, SubmissionUnknownAt: task.SubmissionUnknownAt, NextAttemptAt: task.NextAttemptAt, CancelRequestedAt: task.CancelRequestedAt, AccessRevokedAt: task.AccessRevokedAt, ExternalTaskID: task.ExternalTaskID, ResultAssetID: task.ResultAssetID, FailureCode: task.FailureCode, LeaseOwner: task.LeaseOwner, FencingToken: task.FencingToken, LeaseAttempt: task.LeaseAttempt, LeaseUntil: task.LeaseUntil, CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt}, nil
}

func generationTaskFromRecord(record generationJobRecord) (generationapp.Task, error) {
	var inputs generationapp.InputSnapshot
	var consent generationapp.ConsentReceipt
	parameters, err := generationapp.CanonicalParameters(record.Parameters)
	if err != nil {
		return generationapp.Task{}, generationapp.ErrInvalidGenerationState
	}
	if err := json.Unmarshal(record.Inputs, &inputs); err != nil {
		return generationapp.Task{}, generationapp.ErrInvalidGenerationState
	}
	if err := json.Unmarshal(record.Consent, &consent); err != nil {
		return generationapp.Task{}, generationapp.ErrInvalidGenerationState
	}
	task := generationapp.Task{ID: record.ID, OwnerID: record.OwnerID, LookID: record.LookID, LookRevision: record.LookRevision, Purpose: generationapp.Purpose(record.Purpose), Provider: record.Provider, Model: record.Model, Parameters: parameters, Inputs: inputs, Consent: consent, Cost: generationapp.CostEstimate{Currency: record.Currency, EstimatedMinorUnits: record.EstimatedMinorUnits, ReservedQuotaUnits: record.ReservedQuotaUnits}, IdempotencyKeyHash: record.IdempotencyKeyHash, DedupeKey: record.DedupeKey, Status: generationapp.Status(record.Status), StatusRevision: record.StatusRevision, SubmissionState: generationapp.SubmissionState(record.SubmissionState), SubmissionAttempt: record.SubmissionAttempt, SubmissionStartedAt: record.SubmissionStartedAt, SubmissionUnknownAt: record.SubmissionUnknownAt, NextAttemptAt: record.NextAttemptAt, CancelRequestedAt: record.CancelRequestedAt, AccessRevokedAt: record.AccessRevokedAt, ExternalTaskID: record.ExternalTaskID, ResultAssetID: record.ResultAssetID, FailureCode: record.FailureCode, LeaseOwner: record.LeaseOwner, FencingToken: record.FencingToken, LeaseAttempt: record.LeaseAttempt, LeaseUntil: record.LeaseUntil, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
	if err := task.Validate(); err != nil {
		return generationapp.Task{}, err
	}
	return task, nil
}

func generationReservationRecordFromDomain(reservation generationapp.QuotaReservation) (generationQuotaReservationRecord, error) {
	if err := reservation.Validate(); err != nil {
		return generationQuotaReservationRecord{}, err
	}
	return generationQuotaReservationRecord{ID: reservation.ID, TaskID: reservation.TaskID, OwnerID: reservation.OwnerID, Purpose: string(reservation.Purpose), Currency: reservation.Currency, ReservedQuotaUnits: reservation.ReservedQuotaUnits, EstimatedMinorUnits: reservation.EstimatedMinorUnits, State: string(reservation.State), StateRevision: reservation.StateRevision, CreatedAt: reservation.CreatedAt, UpdatedAt: reservation.UpdatedAt}, nil
}

func generationReservationFromRecord(record generationQuotaReservationRecord) (generationapp.QuotaReservation, error) {
	reservation := generationapp.QuotaReservation{ID: record.ID, TaskID: record.TaskID, OwnerID: record.OwnerID, Purpose: generationapp.Purpose(record.Purpose), Currency: record.Currency, ReservedQuotaUnits: record.ReservedQuotaUnits, EstimatedMinorUnits: record.EstimatedMinorUnits, State: generationapp.ReservationState(record.State), StateRevision: record.StateRevision, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
	if err := reservation.Validate(); err != nil {
		return generationapp.QuotaReservation{}, err
	}
	return reservation, nil
}

func generationOutputFromRecord(record generationOutputRecord, task generationapp.Task) (generationapp.OutputAsset, error) {
	asset := generationapp.OutputAsset{ID: record.ID, Lineage: generationapp.OutputLineage{TaskID: record.TaskID, OwnerID: record.OwnerID, LookID: record.LookID, LookRevision: record.LookRevision, Purpose: generationapp.Purpose(record.Purpose), SourceImageAssetID: record.SourceImageAssetID, SourceImageSHA256: strings.TrimSpace(record.SourceImageSHA256)}, ContentType: record.ContentType, ByteSize: record.ByteSize, SHA256: record.SHA256, ObjectVersionID: record.ObjectVersionID, PublishedAt: record.PublishedAt}
	if err := asset.ValidateFor(task); err != nil {
		return generationapp.OutputAsset{}, err
	}
	return asset, nil
}

func generationLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return generationapp.ErrGenerationNotFound
	}
	return generationapp.ErrGenerationUnavailable
}

func generationGenerationError(err error) error {
	if errors.Is(err, generationapp.ErrGenerationNotFound) || errors.Is(err, generationapp.ErrGenerationUnavailable) || errors.Is(err, generationapp.ErrInvalidGenerationInput) || errors.Is(err, generationapp.ErrGenerationDisabled) || errors.Is(err, generationapp.ErrGenerationQuotaExceeded) || errors.Is(err, generationapp.ErrGenerationBudgetExceeded) || errors.Is(err, generationapp.ErrGenerationConcurrency) || errors.Is(err, generationapp.ErrGenerationCurrency) || errors.Is(err, generationapp.ErrGenerationIdempotencyConflict) || errors.Is(err, generationapp.ErrGenerationNotCancellable) || errors.Is(err, generationapp.ErrGenerationRetryNotReady) || errors.Is(err, generationapp.ErrGenerationRetryExhausted) || errors.Is(err, generationapp.ErrGenerationLeaseHeld) || errors.Is(err, generationapp.ErrGenerationLeaseExpired) || errors.Is(err, generationapp.ErrGenerationLeaseConflict) || errors.Is(err, generationapp.ErrInvalidGenerationLease) || errors.Is(err, generationapp.ErrInvalidGenerationCleanup) || errors.Is(err, generationapp.ErrGenerationCleanupNotReady) || errors.Is(err, generationapp.ErrGenerationCleanupInProgress) || errors.Is(err, generationapp.ErrGenerationCleanupClaim) || errors.Is(err, generationapp.ErrGenerationCleanupExhausted) || errors.Is(err, generationapp.ErrGenerationSourceUnavailable) {
		return err
	}
	return generationapp.ErrGenerationUnavailable
}
