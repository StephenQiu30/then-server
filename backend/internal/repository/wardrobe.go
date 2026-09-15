package repository

import (
	"context"
	"errors"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WardrobeRepository struct{ database *gorm.DB }

func NewWardrobeRepository(database *gorm.DB) *WardrobeRepository {
	return &WardrobeRepository{database: database}
}

type wardrobeItemRecord struct {
	OwnerID      string    `gorm:"column:owner_id;type:uuid;primaryKey;index:wardrobe_items_owner_order_idx,priority:1"`
	ID           string    `gorm:"column:id;type:uuid;primaryKey"`
	Name         string    `gorm:"column:name;type:text;not null;check:wardrobe_items_name_check,name = btrim(name) AND char_length(name) BETWEEN 1 AND 80"`
	Category     string    `gorm:"column:category;type:text;not null;check:wardrobe_items_category_check,category IN ('top','bottom','one_piece','outerwear','shoes','bag','accessory')"`
	Availability string    `gorm:"column:availability;type:text;not null;check:wardrobe_items_availability_check,availability IN ('wearable','laundry','lent_out','packed')"`
	Source       string    `gorm:"column:source;type:text;not null;check:wardrobe_items_source_check,source IN ('wardrobe','quick_add')"`
	Revision     int       `gorm:"column:revision;not null;check:wardrobe_items_revision_check,revision >= 1"`
	CreatedAt    time.Time `gorm:"column:created_at;type:timestamptz;not null;index:wardrobe_items_owner_order_idx,priority:2,sort:desc"`
	UpdatedAt    time.Time `gorm:"column:updated_at;type:timestamptz;not null;check:wardrobe_items_timestamps_check,updated_at >= created_at"`
}

func (wardrobeItemRecord) TableName() string { return "wardrobe_items" }

func (r *WardrobeRepository) CreateWardrobeItem(ctx context.Context, item model.WardrobeItem) (model.WardrobeItem, error) {
	record := wardrobeRecord(item)
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			return nil
		}
		var existing wardrobeItemRecord
		if err := tx.Where("owner_id = ? AND id = ?", item.OwnerID, item.ID).First(&existing).Error; err != nil {
			return err
		}
		if existing.Name != item.Name || existing.Category != string(item.Category) || existing.Availability != string(item.Availability) || existing.Source != string(item.Source) {
			return model.ErrWardrobeConflict
		}
		record = existing
		return nil
	})
	if err != nil {
		return model.WardrobeItem{}, wardrobeWriteError(err)
	}
	return wardrobeFromRecord(record), nil
}

func (r *WardrobeRepository) ListWardrobeItems(ctx context.Context, ownerID string, limit int, afterID *string) (model.WardrobePage, error) {
	query := r.database.WithContext(ctx).Where("owner_id = ?", ownerID)
	if afterID != nil {
		var cursor wardrobeItemRecord
		if err := r.database.WithContext(ctx).Select("id", "created_at").Where("owner_id = ? AND id = ?", ownerID, *afterID).First(&cursor).Error; err != nil {
			return model.WardrobePage{}, wardrobeLookupError(err)
		}
		query = query.Where("created_at < ? OR (created_at = ? AND id > ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	var records []wardrobeItemRecord
	if err := query.Order("created_at DESC").Order("id ASC").Limit(limit + 1).Find(&records).Error; err != nil {
		return model.WardrobePage{}, model.ErrWardrobeUnavailable
	}
	page := model.WardrobePage{Items: make([]model.WardrobeItem, 0, min(limit, len(records)))}
	for index, record := range records {
		if index == limit {
			last := page.Items[len(page.Items)-1].ID
			page.NextAfterID = &last
			break
		}
		page.Items = append(page.Items, wardrobeFromRecord(record))
	}
	return page, nil
}

func (r *WardrobeRepository) GetWardrobeItem(ctx context.Context, ownerID, itemID string) (model.WardrobeItem, error) {
	var record wardrobeItemRecord
	if err := r.database.WithContext(ctx).Where("owner_id = ? AND id = ?", ownerID, itemID).First(&record).Error; err != nil {
		return model.WardrobeItem{}, wardrobeLookupError(err)
	}
	return wardrobeFromRecord(record), nil
}

func (r *WardrobeRepository) UpdateWardrobeItem(ctx context.Context, ownerID, itemID string, expectedRevision int, input model.UpdateWardrobeItemInput, at time.Time) (model.WardrobeItem, error) {
	var record wardrobeItemRecord
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, itemID).First(&record).Error; err != nil {
			return err
		}
		if record.Revision != expectedRevision {
			return model.ErrWardrobeConflict
		}
		record.Name, record.Category, record.Availability = input.Name, string(input.Category), string(input.Availability)
		record.Revision++
		if at.Before(record.UpdatedAt) {
			at = record.UpdatedAt
		}
		record.UpdatedAt = at
		return tx.Model(&wardrobeItemRecord{}).Where("owner_id = ? AND id = ? AND revision = ?", ownerID, itemID, expectedRevision).Updates(map[string]any{
			"name": record.Name, "category": record.Category, "availability": record.Availability,
			"revision": record.Revision, "updated_at": record.UpdatedAt,
		}).Error
	})
	if err != nil {
		return model.WardrobeItem{}, wardrobeWriteError(err)
	}
	return wardrobeFromRecord(record), nil
}

func (r *WardrobeRepository) DeleteWardrobeItem(ctx context.Context, ownerID, itemID string, expectedRevision int) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record wardrobeItemRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("owner_id", "id", "revision").Where("owner_id = ? AND id = ?", ownerID, itemID).First(&record).Error; err != nil {
			return err
		}
		if record.Revision != expectedRevision {
			return model.ErrWardrobeConflict
		}
		return tx.Where("owner_id = ? AND id = ? AND revision = ?", ownerID, itemID, expectedRevision).Delete(&wardrobeItemRecord{}).Error
	})
	return wardrobeWriteError(err)
}

func wardrobeRecord(item model.WardrobeItem) wardrobeItemRecord {
	return wardrobeItemRecord{OwnerID: item.OwnerID, ID: item.ID, Name: item.Name, Category: string(item.Category), Availability: string(item.Availability), Source: string(item.Source), Revision: item.Revision, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func wardrobeFromRecord(record wardrobeItemRecord) model.WardrobeItem {
	return model.WardrobeItem{OwnerID: record.OwnerID, ID: record.ID, Name: record.Name, Category: model.WardrobeCategory(record.Category), Availability: model.WardrobeAvailability(record.Availability), Source: model.WardrobeSource(record.Source), Revision: record.Revision, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}

func wardrobeLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ErrWardrobeNotFound
	}
	return model.ErrWardrobeUnavailable
}

func wardrobeWriteError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, model.ErrWardrobeConflict):
		return model.ErrWardrobeConflict
	case errors.Is(err, gorm.ErrRecordNotFound):
		return model.ErrWardrobeNotFound
	default:
		return model.ErrWardrobeUnavailable
	}
}
