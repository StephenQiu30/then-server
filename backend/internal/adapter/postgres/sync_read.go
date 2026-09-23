package postgres

import (
	"context"
	"time"

	syncapp "github.com/StephenQiu30/then-server/backend/internal/application/syncchange"
	"gorm.io/gorm"
)

type SyncRepository struct{ database *gorm.DB }

func NewSyncRepository(database *gorm.DB) *SyncRepository {
	return &SyncRepository{database: database}
}

func (r *SyncRepository) List(ctx context.Context, ownerID string, after int64, limit int) ([]syncapp.Change, int64, error) {
	changes := make([]syncapp.Change, 0)
	var watermark int64
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureSyncSeed(tx, ownerID, time.Now().UTC()); err != nil {
			return err
		}
		var position syncPositionRecord
		if err := tx.Where("owner_id = ?", ownerID).Take(&position).Error; err != nil {
			return err
		}
		watermark = position.Seq
		if after > watermark {
			return nil
		}
		var records []syncChangeRecord
		if err := tx.Where("owner_id = ? AND seq > ? AND seq <= ?", ownerID, after, watermark).Order("seq ASC").Limit(limit).Find(&records).Error; err != nil {
			return err
		}
		for _, record := range records {
			changes = append(changes, syncapp.Change{Seq: record.Seq, Kind: record.Kind, EntityID: record.EntityID, Action: record.Action, Revision: record.Revision})
		}
		return nil
	})
	return changes, watermark, err
}
