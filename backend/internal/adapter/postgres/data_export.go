package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	exportapp "github.com/StephenQiu30/then-server/backend/internal/application/dataexport"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DataExportRepository struct{ database *gorm.DB }

func NewDataExportRepository(database *gorm.DB) *DataExportRepository {
	return &DataExportRepository{database: database}
}

// No user foreign key: the record must survive physical account deletion until
// its private object versions have been removed.
type dataExportRecord struct {
	ID                string     `gorm:"column:id;type:uuid;primaryKey"`
	OwnerID           string     `gorm:"column:owner_id;type:uuid;not null;index:data_exports_owner_idx"`
	Mode              string     `gorm:"column:mode;type:text;not null;check:data_exports_mode_check,mode = 'structured'"`
	Status            string     `gorm:"column:status;type:text;not null;index:data_exports_status_idx;check:data_exports_status_check,status IN ('preparing','ready','failed','expired','revoked')"`
	Counts            []byte     `gorm:"column:counts;type:jsonb;not null"`
	Omissions         []byte     `gorm:"column:omissions;type:jsonb;not null"`
	ObjectKey         string     `gorm:"column:object_key;type:text;not null;default:''"`
	ObjectVersion     string     `gorm:"column:object_version;type:text;not null;default:''"`
	Attempts          int        `gorm:"column:attempts;not null;default:0;check:data_exports_attempts_check,attempts >= 0"`
	LeaseUntil        *time.Time `gorm:"column:lease_until;type:timestamptz"`
	CleanupLeaseUntil *time.Time `gorm:"column:cleanup_lease_until;type:timestamptz"`
	CleanedAt         *time.Time `gorm:"column:cleaned_at;type:timestamptz;index:data_exports_cleaned_idx"`
	CreatedAt         time.Time  `gorm:"column:created_at;type:timestamptz;not null"`
	CompletedAt       *time.Time `gorm:"column:completed_at;type:timestamptz"`
	ExpiresAt         time.Time  `gorm:"column:expires_at;type:timestamptz;not null;index:data_exports_expiry_idx"`
	RevokedAt         *time.Time `gorm:"column:revoked_at;type:timestamptz"`
}

func (dataExportRecord) TableName() string { return "data_exports" }

func exportFromRecord(record dataExportRecord) (exportapp.Job, error) {
	job := exportapp.Job{ID: record.ID, OwnerID: record.OwnerID, Mode: record.Mode, Status: record.Status, ObjectKey: record.ObjectKey, ObjectVersion: record.ObjectVersion, Attempts: record.Attempts, CreatedAt: record.CreatedAt, CompletedAt: record.CompletedAt, ExpiresAt: record.ExpiresAt, RevokedAt: record.RevokedAt}
	if err := json.Unmarshal(record.Counts, &job.Counts); err != nil {
		return exportapp.Job{}, exportapp.ErrUnavailable
	}
	if err := json.Unmarshal(record.Omissions, &job.Omissions); err != nil {
		return exportapp.Job{}, exportapp.ErrUnavailable
	}
	return job, nil
}

func (r *DataExportRepository) Create(ctx context.Context, job exportapp.Job) error {
	counts, _ := json.Marshal(job.Counts)
	omissions, _ := json.Marshal(job.Omissions)
	return r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "status").Where("id = ?", job.OwnerID).Take(&user).Error; err != nil || user.Status != "active" {
			return exportapp.ErrUnavailable
		}
		return tx.Create(&dataExportRecord{ID: job.ID, OwnerID: job.OwnerID, Mode: job.Mode, Status: job.Status, Counts: counts, Omissions: omissions, CreatedAt: job.CreatedAt, ExpiresAt: job.ExpiresAt}).Error
	})
}

func (r *DataExportRepository) Get(ctx context.Context, ownerID, id string) (exportapp.Job, error) {
	var record dataExportRecord
	if err := r.database.WithContext(ctx).Table("data_exports AS e").Select("e.*").Joins("JOIN users AS u ON u.id = e.owner_id AND u.status = 'active'").Where("e.owner_id = ? AND e.id = ?", ownerID, id).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return exportapp.Job{}, exportapp.ErrNotFound
		}
		return exportapp.Job{}, exportapp.ErrUnavailable
	}
	return exportFromRecord(record)
}

func (r *DataExportRepository) Revoke(ctx context.Context, ownerID, id string, at time.Time) error {
	return r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "status").Where("id = ?", ownerID).Take(&user).Error; err != nil || user.Status != "active" {
			return exportapp.ErrNotFound
		}
		var record dataExportRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_id = ?", id, ownerID).Take(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return exportapp.ErrNotFound
			}
			return exportapp.ErrUnavailable
		}
		if record.Status == exportapp.StatusRevoked {
			return nil
		}
		return tx.Model(&dataExportRecord{}).Where("id = ?", id).Updates(map[string]any{"status": exportapp.StatusRevoked, "revoked_at": at}).Error
	})
}

func (r *DataExportRepository) Claim(ctx context.Context, at time.Time) (exportapp.Job, bool, error) {
	var record dataExportRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&dataExportRecord{}).Where("status = ? AND attempts >= 3 AND lease_until < ?", exportapp.StatusPreparing, at).Updates(map[string]any{"status": exportapp.StatusFailed, "lease_until": nil}).Error; err != nil {
			return err
		}
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND expires_at > ? AND attempts < 3 AND (lease_until IS NULL OR lease_until < ?)", exportapp.StatusPreparing, at, at).
			Order("created_at ASC").Take(&record)
		if query.Error != nil {
			return query.Error
		}
		lease := at.Add(2 * time.Minute)
		return tx.Model(&dataExportRecord{}).Where("id = ?", record.ID).Updates(map[string]any{"attempts": record.Attempts + 1, "lease_until": lease}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return exportapp.Job{}, false, nil
	}
	if err != nil {
		return exportapp.Job{}, false, exportapp.ErrUnavailable
	}
	job, err := exportFromRecord(record)
	job.Attempts++
	return job, true, err
}

func (r *DataExportRepository) Complete(ctx context.Context, job exportapp.Job, counts map[string]int, omissions []string, key, version string, at time.Time) (bool, error) {
	countsJSON, _ := json.Marshal(counts)
	omissionsJSON, _ := json.Marshal(omissions)
	var accepted bool
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "status").Where("id = ?", job.OwnerID).Take(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if user.Status != "active" {
			return nil
		}
		var record dataExportRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_id = ?", job.ID, job.OwnerID).Take(&record).Error; err != nil {
			return err
		}
		if record.Status != exportapp.StatusPreparing || record.Attempts != job.Attempts || !at.Before(record.ExpiresAt) {
			return nil
		}
		if err := tx.Model(&dataExportRecord{}).Where("id = ?", job.ID).Updates(map[string]any{"status": exportapp.StatusReady, "counts": countsJSON, "omissions": omissionsJSON, "object_key": key, "object_version": version, "completed_at": at, "lease_until": nil}).Error; err != nil {
			return err
		}
		accepted = true
		return nil
	})
	if err != nil {
		return false, exportapp.ErrUnavailable
	}
	return accepted, nil
}

func (r *DataExportRepository) Fail(ctx context.Context, job exportapp.Job, at time.Time) error {
	status := exportapp.StatusPreparing
	if job.Attempts >= 3 {
		status = exportapp.StatusFailed
	}
	return r.database.WithContext(ctx).Model(&dataExportRecord{}).
		Where("id = ? AND status = ? AND attempts = ?", job.ID, exportapp.StatusPreparing, job.Attempts).
		Updates(map[string]any{"status": status, "lease_until": at.Add(10 * time.Second), "omissions": []byte(`["preparation_failed"]`)}).Error
}

func (r *DataExportRepository) ClaimCleanup(ctx context.Context, at time.Time) (exportapp.Job, bool, error) {
	var record dataExportRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("cleaned_at IS NULL AND (expires_at <= ? OR status = ?) AND (lease_until IS NULL OR lease_until < ?) AND (cleanup_lease_until IS NULL OR cleanup_lease_until < ?)", at, exportapp.StatusRevoked, at, at).
			Order("created_at ASC").Take(&record)
		if query.Error != nil {
			return query.Error
		}
		return tx.Model(&dataExportRecord{}).Where("id = ?", record.ID).Updates(map[string]any{"status": gorm.Expr("CASE WHEN status = 'revoked' THEN 'revoked' ELSE 'expired' END"), "cleanup_lease_until": at.Add(time.Minute)}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return exportapp.Job{}, false, nil
	}
	if err != nil {
		return exportapp.Job{}, false, exportapp.ErrUnavailable
	}
	job, err := exportFromRecord(record)
	return job, true, err
}

func (r *DataExportRepository) FinishCleanup(ctx context.Context, id string, at time.Time) error {
	return r.database.WithContext(ctx).Model(&dataExportRecord{}).Where("id = ? AND (status = ? OR status = ?)", id, exportapp.StatusExpired, exportapp.StatusRevoked).
		Updates(map[string]any{"object_key": "", "object_version": "", "cleanup_lease_until": nil, "lease_until": nil, "cleaned_at": at}).Error
}

func (r *DataExportRepository) Purge(ctx context.Context, before time.Time) error {
	return r.database.WithContext(ctx).Where("cleaned_at < ?", before).Delete(&dataExportRecord{}).Error
}
