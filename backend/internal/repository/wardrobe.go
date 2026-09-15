package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
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
	OwnerID       string    `gorm:"column:owner_id;type:uuid;primaryKey;index:wardrobe_items_owner_order_idx,priority:1"`
	ID            string    `gorm:"column:id;type:uuid;primaryKey"`
	Name          string    `gorm:"column:name;type:text;not null;check:wardrobe_items_name_check,name = btrim(name) AND char_length(name) BETWEEN 1 AND 80"`
	Category      string    `gorm:"column:category;type:text;not null;check:wardrobe_items_category_check,category IN ('top','bottom','one_piece','outerwear','shoes','bag','accessory')"`
	Availability  string    `gorm:"column:availability;type:text;not null;check:wardrobe_items_availability_check,availability IN ('wearable','laundry','lent_out','packed')"`
	Source        string    `gorm:"column:source;type:text;not null;check:wardrobe_items_source_check,source IN ('wardrobe','quick_add')"`
	FormalityBand *string   `gorm:"column:formality_band;type:text;check:wardrobe_items_formality_band_check,formality_band IN ('casual','smart_casual','formal')"`
	WarmthBand    *string   `gorm:"column:warmth_band;type:text;check:wardrobe_items_warmth_band_check,warmth_band IN ('light','medium','warm')"`
	RainUse       *string   `gorm:"column:rain_use;type:text;check:wardrobe_items_rain_use_check,rain_use IN ('suitable','unsuitable')"`
	WalkingUse    *string   `gorm:"column:walking_use;type:text;check:wardrobe_items_walking_use_check,walking_use IN ('suitable','unsuitable')"`
	Revision      int       `gorm:"column:revision;not null;check:wardrobe_items_revision_check,revision >= 1"`
	CreatedAt     time.Time `gorm:"column:created_at;type:timestamptz;not null;index:wardrobe_items_owner_order_idx,priority:2,sort:desc"`
	UpdatedAt     time.Time `gorm:"column:updated_at;type:timestamptz;not null;check:wardrobe_items_timestamps_check,updated_at >= created_at"`
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
		if existing.Name != item.Name || existing.Category != string(item.Category) || existing.Availability != string(item.Availability) || existing.Source != string(item.Source) || !wardrobeRecordAttributesEqual(existing, item.Attributes) {
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
		record.FormalityBand = stringPointer(input.Attributes.FormalityBand)
		record.WarmthBand = stringPointer(input.Attributes.WarmthBand)
		record.RainUse = stringPointer(input.Attributes.RainUse)
		record.WalkingUse = stringPointer(input.Attributes.WalkingUse)
		record.Revision++
		if at.Before(record.UpdatedAt) {
			at = record.UpdatedAt
		}
		record.UpdatedAt = at
		return tx.Model(&wardrobeItemRecord{}).Where("owner_id = ? AND id = ? AND revision = ?", ownerID, itemID, expectedRevision).Updates(map[string]any{
			"name": record.Name, "category": record.Category, "availability": record.Availability,
			"formality_band": record.FormalityBand, "warmth_band": record.WarmthBand, "rain_use": record.RainUse, "walking_use": record.WalkingUse,
			"revision": record.Revision, "updated_at": record.UpdatedAt,
		}).Error
	})
	if err != nil {
		return model.WardrobeItem{}, wardrobeWriteError(err)
	}
	return wardrobeFromRecord(record), nil
}

func (r *WardrobeRepository) GetWardrobeDeletionImpact(ctx context.Context, ownerID, itemID string) (model.WardrobeDeletionImpact, error) {
	var item wardrobeItemRecord
	if err := r.database.WithContext(ctx).Select("owner_id", "id").Where("owner_id = ? AND id = ?", ownerID, itemID).First(&item).Error; err != nil {
		return model.WardrobeDeletionImpact{}, wardrobeLookupError(err)
	}
	plans, err := wardrobeAffectedPlans(r.database.WithContext(ctx), ownerID, itemID)
	if err != nil {
		return model.WardrobeDeletionImpact{}, model.ErrWardrobeUnavailable
	}
	return model.WardrobeDeletionImpact{AffectedPlanCount: len(plans), ExpectedImpact: wardrobeImpactDigest(plans)}, nil
}

func (r *WardrobeRepository) DeleteWardrobeItem(ctx context.Context, ownerID, itemID string, expectedRevision int, policy model.WardrobeHistoryPolicy, expectedImpact string, at time.Time) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		initialPlans, err := wardrobeAffectedPlans(tx, ownerID, itemID)
		if err != nil {
			return err
		}
		for _, planID := range sortedPlanIDs(initialPlans) {
			var locked outfitPlanRecord
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("owner_id", "id", "revision", "updated_at").Where("owner_id = ? AND id = ?", ownerID, planID).First(&locked).Error; err != nil {
				return model.ErrWardrobeConflict
			}
		}
		var record wardrobeItemRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("owner_id", "id", "revision").Where("owner_id = ? AND id = ?", ownerID, itemID).First(&record).Error; err != nil {
			return err
		}
		if record.Revision != expectedRevision {
			return model.ErrWardrobeConflict
		}
		plans, err := wardrobeAffectedPlans(tx, ownerID, itemID)
		if err != nil {
			return err
		}
		if wardrobeImpactDigest(plans) != expectedImpact {
			return model.ErrWardrobeConflict
		}
		switch policy {
		case model.WardrobeHistoryRedactSnapshots:
			for _, plan := range plans {
				if err := tx.Model(&outfitPlanItemRecord{}).Where("owner_id = ? AND plan_id = ? AND wardrobe_item_id = ?", ownerID, plan.ID, itemID).Updates(map[string]any{"wardrobe_item_id": nil, "item_revision": nil, "name": nil, "category": nil, "availability": nil, "formality_band": nil, "warmth_band": nil, "rain_use": nil, "walking_use": nil, "redacted": true}).Error; err != nil {
					return err
				}
				updatedAt := at
				if updatedAt.Before(plan.UpdatedAt) {
					updatedAt = plan.UpdatedAt
				}
				if err := tx.Model(&outfitPlanRecord{}).Where("owner_id = ? AND id = ? AND revision = ?", ownerID, plan.ID, plan.Revision).Updates(map[string]any{"revision": plan.Revision + 1, "updated_at": updatedAt}).Error; err != nil {
					return err
				}
			}
		case model.WardrobeHistoryDeleteAffectedPlans:
			for _, plan := range plans {
				if err := createOutfitPlanTombstone(tx, ownerID, plan.ID, at); err != nil {
					return err
				}
				if err := tx.Where("owner_id = ? AND id = ?", ownerID, plan.ID).Delete(&outfitPlanRecord{}).Error; err != nil {
					return err
				}
			}
		default:
			return model.ErrInvalidWardrobeInput
		}
		return tx.Where("owner_id = ? AND id = ? AND revision = ?", ownerID, itemID, expectedRevision).Delete(&wardrobeItemRecord{}).Error
	})
	return wardrobeWriteError(err)
}

func wardrobeAffectedPlans(tx *gorm.DB, ownerID, itemID string) ([]outfitPlanRecord, error) {
	var plans []outfitPlanRecord
	err := tx.Distinct("outfit_plans.owner_id", "outfit_plans.id", "outfit_plans.revision", "outfit_plans.updated_at").
		Table("outfit_plans").Joins("JOIN outfit_plan_items ON outfit_plan_items.owner_id = outfit_plans.owner_id AND outfit_plan_items.plan_id = outfit_plans.id").
		Where("outfit_plans.owner_id = ? AND outfit_plan_items.wardrobe_item_id = ?", ownerID, itemID).Order("outfit_plans.id ASC").Scan(&plans).Error
	return plans, err
}

func wardrobeImpactDigest(plans []outfitPlanRecord) string {
	var canonical strings.Builder
	for _, plan := range plans {
		canonical.WriteString(plan.ID)
		canonical.WriteByte(':')
		canonical.WriteString(strconv.Itoa(plan.Revision))
		canonical.WriteByte('\n')
	}
	digest := sha256.Sum256([]byte(canonical.String()))
	return hex.EncodeToString(digest[:])
}

func wardrobeRecord(item model.WardrobeItem) wardrobeItemRecord {
	return wardrobeItemRecord{OwnerID: item.OwnerID, ID: item.ID, Name: item.Name, Category: string(item.Category), Availability: string(item.Availability), Source: string(item.Source), FormalityBand: stringPointer(item.Attributes.FormalityBand), WarmthBand: stringPointer(item.Attributes.WarmthBand), RainUse: stringPointer(item.Attributes.RainUse), WalkingUse: stringPointer(item.Attributes.WalkingUse), Revision: item.Revision, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func wardrobeFromRecord(record wardrobeItemRecord) model.WardrobeItem {
	return model.WardrobeItem{OwnerID: record.OwnerID, ID: record.ID, Name: record.Name, Category: model.WardrobeCategory(record.Category), Availability: model.WardrobeAvailability(record.Availability), Source: model.WardrobeSource(record.Source), Attributes: model.WardrobeAttributes{FormalityBand: typedPointer[model.WardrobeFormalityBand](record.FormalityBand), WarmthBand: typedPointer[model.WardrobeWarmthBand](record.WarmthBand), RainUse: typedPointer[model.WardrobeUseSuitability](record.RainUse), WalkingUse: typedPointer[model.WardrobeUseSuitability](record.WalkingUse)}, Revision: record.Revision, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}

func wardrobeRecordAttributesEqual(record wardrobeItemRecord, attributes model.WardrobeAttributes) bool {
	return stringsEqual(record.FormalityBand, stringPointer(attributes.FormalityBand)) &&
		stringsEqual(record.WarmthBand, stringPointer(attributes.WarmthBand)) &&
		stringsEqual(record.RainUse, stringPointer(attributes.RainUse)) &&
		stringsEqual(record.WalkingUse, stringPointer(attributes.WalkingUse))
}

func stringsEqual(left, right *string) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

func stringPointer[T ~string](value *T) *string {
	if value == nil {
		return nil
	}
	result := string(*value)
	return &result
}

func typedPointer[T ~string](value *string) *T {
	if value == nil {
		return nil
	}
	result := T(*value)
	return &result
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
	case errors.Is(err, model.ErrInvalidWardrobeInput):
		return model.ErrInvalidWardrobeInput
	case errors.Is(err, gorm.ErrRecordNotFound):
		return model.ErrWardrobeNotFound
	default:
		return model.ErrWardrobeUnavailable
	}
}
