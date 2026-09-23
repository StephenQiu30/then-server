package postgres

import (
	"errors"
	"slices"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type syncPositionRecord struct {
	OwnerID string `gorm:"column:owner_id;type:uuid;primaryKey"`
	Seq     int64  `gorm:"column:seq;type:bigint;not null;default:0;check:sync_positions_seq_check,seq >= 0"`
	Seeded  bool   `gorm:"column:seeded;not null;default:false"`
}

func (syncPositionRecord) TableName() string { return "sync_positions" }

type syncChangeRecord struct {
	OwnerID   string    `gorm:"column:owner_id;type:uuid;primaryKey"`
	Seq       int64     `gorm:"column:seq;type:bigint;primaryKey;check:sync_changes_seq_check,seq >= 1"`
	Kind      string    `gorm:"column:kind;type:text;not null;check:sync_changes_kind_check,kind IN ('wardrobe_item','outfit_plan','wear_event','wear_feedback','diary_entry')"`
	EntityID  string    `gorm:"column:entity_id;type:uuid;not null"`
	Action    string    `gorm:"column:action;type:text;not null;check:sync_changes_action_check,action IN ('upsert','delete')"`
	Revision  *int      `gorm:"column:revision;check:sync_changes_revision_check,revision IS NULL OR revision >= 1"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null"`
}

func (syncChangeRecord) TableName() string { return "sync_changes" }

type pendingSyncChange struct {
	Kind     string
	EntityID string
	Action   string
	Revision *int
}

func syncUpsert(kind, id string, revision int) pendingSyncChange {
	return pendingSyncChange{Kind: kind, EntityID: id, Action: "upsert", Revision: &revision}
}

func syncDelete(kind, id string, revision *int) pendingSyncChange {
	return pendingSyncChange{Kind: kind, EntityID: id, Action: "delete", Revision: revision}
}

// ensureSyncSeed serializes all writes for one owner before reading the initial
// snapshot. It runs inside the caller's business transaction.
func ensureSyncSeed(tx *gorm.DB, ownerID string, at time.Time) error {
	var user userRecord
	if err := tx.Clauses(clause.Locking{Strength: "KEY SHARE"}).Select("id").Where("id = ? AND status = 'active'", ownerID).Take(&user).Error; err != nil {
		return err
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&syncPositionRecord{OwnerID: ownerID}).Error; err != nil {
		return err
	}
	var position syncPositionRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ?", ownerID).Take(&position).Error; err != nil {
		return err
	}
	if position.Seeded {
		return nil
	}
	var changes []pendingSyncChange
	for _, source := range []struct{ table, idColumn, kind string }{
		{"wardrobe_items", "id", "wardrobe_item"},
		{"outfit_plans", "id", "outfit_plan"},
		{"wear_events", "id", "wear_event"},
		{"wear_feedback", "event_id", "wear_feedback"},
		{"diary_entries", "id", "diary_entry"},
	} {
		var rows []struct {
			ID       string `gorm:"column:id"`
			Revision int    `gorm:"column:revision"`
		}
		if err := tx.Raw("SELECT "+source.idColumn+" AS id, revision FROM "+source.table+" WHERE owner_id = ?", ownerID).Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			changes = append(changes, syncUpsert(source.kind, row.ID, row.Revision))
		}
	}
	for _, source := range []struct{ table, kind string }{
		{"wardrobe_item_deletions", "wardrobe_item"},
		{"outfit_plan_deletions", "outfit_plan"},
		{"wear_event_deletions", "wear_event"},
		{"diary_entry_deletions", "diary_entry"},
	} {
		var rows []struct {
			ID string `gorm:"column:id"`
		}
		if err := tx.Raw("SELECT id FROM "+source.table+" WHERE owner_id = ?", ownerID).Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			changes = append(changes, syncDelete(source.kind, row.ID, nil))
		}
	}
	slices.SortFunc(changes, func(a, b pendingSyncChange) int {
		if a.Kind != b.Kind {
			if a.Kind < b.Kind {
				return -1
			}
			return 1
		}
		if a.EntityID < b.EntityID {
			return -1
		}
		if a.EntityID > b.EntityID {
			return 1
		}
		return 0
	})
	if err := writeSyncChanges(tx, ownerID, position.Seq, changes, at); err != nil {
		return err
	}
	return tx.Model(&syncPositionRecord{}).Where("owner_id = ?", ownerID).Update("seeded", true).Error
}

func appendSyncChanges(tx *gorm.DB, ownerID string, at time.Time, changes ...pendingSyncChange) error {
	if len(changes) == 0 {
		return nil
	}
	var position syncPositionRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND seeded = true", ownerID).Take(&position).Error; err != nil {
		return err
	}
	return writeSyncChanges(tx, ownerID, position.Seq, changes, at)
}

func writeSyncChanges(tx *gorm.DB, ownerID string, previous int64, changes []pendingSyncChange, at time.Time) error {
	if len(changes) == 0 {
		return nil
	}
	records := make([]syncChangeRecord, 0, len(changes))
	for i, change := range changes {
		records = append(records, syncChangeRecord{OwnerID: ownerID, Seq: previous + int64(i) + 1, Kind: change.Kind, EntityID: change.EntityID, Action: change.Action, Revision: change.Revision, CreatedAt: at})
	}
	if err := tx.Create(&records).Error; err != nil {
		return err
	}
	updated := tx.Model(&syncPositionRecord{}).Where("owner_id = ? AND seq = ?", ownerID, previous).Update("seq", previous+int64(len(records)))
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return errors.New("sync position changed during transaction")
	}
	return nil
}

func unlinkDiaryReference(tx *gorm.DB, ownerID, column, id string, at time.Time) error {
	if column != "plan_id" && column != "wear_event_id" {
		return errors.New("unsupported diary reference")
	}
	var entries []diaryEntryRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND "+column+" = ?", ownerID, id).Order("id ASC").Find(&entries).Error; err != nil {
		return err
	}
	for _, entry := range entries {
		updatedAt := at
		if updatedAt.Before(entry.UpdatedAt) {
			updatedAt = entry.UpdatedAt
		}
		updated := tx.Model(&diaryEntryRecord{}).Where("owner_id = ? AND id = ? AND revision = ?", ownerID, entry.ID, entry.Revision).Updates(map[string]any{column: nil, "revision": entry.Revision + 1, "updated_at": updatedAt})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errors.New("diary reference changed during transaction")
		}
		if err := appendSyncChanges(tx, ownerID, at, syncUpsert("diary_entry", entry.ID, entry.Revision+1)); err != nil {
			return err
		}
	}
	return nil
}

func deleteLinkedWearRecords(tx *gorm.DB, ownerID, eventID string, at time.Time) error {
	if err := unlinkDiaryReference(tx, ownerID, "wear_event_id", eventID, at); err != nil {
		return err
	}
	var feedback wearFeedbackRecord
	err := tx.Select("event_id").Where("owner_id = ? AND event_id = ?", ownerID, eventID).Take(&feedback).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return appendSyncChanges(tx, ownerID, at, syncDelete("wear_feedback", eventID, nil))
}
