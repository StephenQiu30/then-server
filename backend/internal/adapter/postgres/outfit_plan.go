package postgres

import (
	"context"
	"errors"
	"slices"
	"time"

	outfitplanapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitplan"
	wardrobeapp "github.com/StephenQiu30/then-server/backend/internal/application/wardrobe"
	weareventapp "github.com/StephenQiu30/then-server/backend/internal/application/wearevent"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OutfitPlanRepository struct{ database *gorm.DB }

func NewOutfitPlanRepository(database *gorm.DB) *OutfitPlanRepository {
	return &OutfitPlanRepository{database: database}
}

type outfitPlanRecord struct {
	OwnerID        string                 `gorm:"column:owner_id;type:uuid;primaryKey;index:outfit_plans_owner_order_idx,priority:1"`
	ID             string                 `gorm:"column:id;type:uuid;primaryKey;uniqueIndex:outfit_plans_id_unique"`
	LocalDate      time.Time              `gorm:"column:local_date;type:date;not null;index:outfit_plans_owner_order_idx,priority:2,sort:desc"`
	TimeZone       string                 `gorm:"column:time_zone;type:text;not null;check:outfit_plans_time_zone_check,char_length(time_zone) BETWEEN 1 AND 255"`
	ContextSummary *string                `gorm:"column:context_summary;type:text;check:outfit_plans_context_summary_check,context_summary IS NULL OR (context_summary = btrim(context_summary) AND char_length(context_summary) BETWEEN 1 AND 120)"`
	Status         string                 `gorm:"column:status;type:text;not null;check:outfit_plans_status_check,status IN ('active','completed','not_worn','cancelled')"`
	Revision       int                    `gorm:"column:revision;not null;check:outfit_plans_revision_check,revision >= 1"`
	CreatedAt      time.Time              `gorm:"column:created_at;type:timestamptz;not null;index:outfit_plans_owner_order_idx,priority:3,sort:desc"`
	UpdatedAt      time.Time              `gorm:"column:updated_at;type:timestamptz;not null;check:outfit_plans_timestamps_check,updated_at >= created_at"`
	Items          []outfitPlanItemRecord `gorm:"foreignKey:OwnerID,PlanID;references:OwnerID,ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
}

func (outfitPlanRecord) TableName() string { return "outfit_plans" }

type outfitPlanItemRecord struct {
	OwnerID        string  `gorm:"column:owner_id;type:uuid;primaryKey;index:outfit_plan_items_owner_wardrobe_idx,priority:1"`
	PlanID         string  `gorm:"column:plan_id;type:uuid;primaryKey;index:outfit_plan_items_owner_wardrobe_idx,priority:3"`
	Ordinal        int     `gorm:"column:ordinal;primaryKey;check:outfit_plan_items_ordinal_check,ordinal >= 0 AND ordinal < 20"`
	WardrobeItemID *string `gorm:"column:wardrobe_item_id;type:uuid;index:outfit_plan_items_owner_wardrobe_idx,priority:2"`
	ItemRevision   *int    `gorm:"column:item_revision"`
	Name           *string `gorm:"column:name;type:text"`
	Category       *string `gorm:"column:category;type:text"`
	Availability   *string `gorm:"column:availability;type:text"`
	FormalityBand  *string `gorm:"column:formality_band;type:text;check:outfit_plan_items_formality_check,formality_band IN ('casual','smart_casual','formal')"`
	WarmthBand     *string `gorm:"column:warmth_band;type:text;check:outfit_plan_items_warmth_check,warmth_band IN ('light','medium','warm')"`
	RainUse        *string `gorm:"column:rain_use;type:text;check:outfit_plan_items_rain_check,rain_use IN ('suitable','unsuitable')"`
	WalkingUse     *string `gorm:"column:walking_use;type:text;check:outfit_plan_items_walking_check,walking_use IN ('suitable','unsuitable')"`
	Redacted       bool    `gorm:"column:redacted;not null;check:outfit_plan_items_content_check,(redacted AND wardrobe_item_id IS NULL AND item_revision IS NULL AND name IS NULL AND category IS NULL AND availability IS NULL AND formality_band IS NULL AND warmth_band IS NULL AND rain_use IS NULL AND walking_use IS NULL) OR (NOT redacted AND wardrobe_item_id IS NOT NULL AND item_revision >= 1 AND name IS NOT NULL AND category IN ('top','bottom','one_piece','outerwear','shoes','bag','accessory') AND availability IN ('wearable','laundry','lent_out','packed'))"`
}

func (outfitPlanItemRecord) TableName() string { return "outfit_plan_items" }

type outfitPlanDeletionRecord struct {
	OwnerID   string    `gorm:"column:owner_id;type:uuid;primaryKey"`
	ID        string    `gorm:"column:id;type:uuid;primaryKey"`
	DeletedAt time.Time `gorm:"column:deleted_at;type:timestamptz;not null"`
}

func (outfitPlanDeletionRecord) TableName() string { return "outfit_plan_deletions" }

func (r *OutfitPlanRepository) CreateOutfitPlan(ctx context.Context, ownerID, planID string, input outfitplanapp.OutfitPlanInput, at time.Time) (outfitplanapp.OutfitPlan, error) {
	var result outfitplanapp.OutfitPlan
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureSyncSeed(tx, ownerID, at); err != nil {
			return err
		}
		existing, err := readOutfitPlan(tx, ownerID, planID)
		if err == nil {
			if !outfitPlanMatchesInput(existing, input) {
				return outfitplanapp.ErrOutfitPlanConflict
			}
			result = existing
			return nil
		}
		if !errors.Is(err, outfitplanapp.ErrOutfitPlanNotFound) {
			return err
		}
		var deleted int64
		if err := tx.Model(&outfitPlanDeletionRecord{}).Where("owner_id = ? AND id = ?", ownerID, planID).Count(&deleted).Error; err != nil {
			return err
		}
		if deleted != 0 {
			return outfitplanapp.ErrOutfitPlanConflict
		}
		items, err := snapshotOutfitItems(tx, ownerID, planID, input)
		if err != nil {
			return err
		}
		date, _ := time.Parse("2006-01-02", input.LocalDate)
		record := outfitPlanRecord{OwnerID: ownerID, ID: planID, LocalDate: date, TimeZone: input.TimeZone, ContextSummary: input.ContextSummary, Status: string(outfitplanapp.OutfitPlanActive), Revision: 1, CreatedAt: at, UpdatedAt: at}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Omit("Items").Create(&record)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			existing, err := readOutfitPlan(tx, ownerID, planID)
			if err != nil || !outfitPlanMatchesInput(existing, input) {
				return outfitplanapp.ErrOutfitPlanConflict
			}
			result = existing
			return nil
		}
		if err := tx.Create(&items).Error; err != nil {
			return err
		}
		result = outfitFromRecord(record, items)
		return appendSyncChanges(tx, ownerID, at, syncUpsert("outfit_plan", planID, record.Revision))
	})
	if err != nil {
		return outfitplanapp.OutfitPlan{}, outfitPlanWriteError(err)
	}
	return result, nil
}

func (r *OutfitPlanRepository) ListOutfitPlans(ctx context.Context, ownerID string, limit int, afterID, localDate *string) (outfitplanapp.OutfitPlanPage, error) {
	query := r.database.WithContext(ctx).Where("owner_id = ?", ownerID)
	if localDate != nil {
		query = query.Where("local_date = ?", *localDate)
	}
	if afterID != nil {
		var cursor outfitPlanRecord
		cursorQuery := r.database.WithContext(ctx).Select("id", "local_date", "created_at").Where("owner_id = ? AND id = ?", ownerID, *afterID)
		if localDate != nil {
			cursorQuery = cursorQuery.Where("local_date = ?", *localDate)
		}
		if err := cursorQuery.First(&cursor).Error; err != nil {
			return outfitplanapp.OutfitPlanPage{}, outfitPlanLookupError(err)
		}
		query = query.Where("local_date < ? OR (local_date = ? AND created_at < ?) OR (local_date = ? AND created_at = ? AND id > ?)", cursor.LocalDate, cursor.LocalDate, cursor.CreatedAt, cursor.LocalDate, cursor.CreatedAt, cursor.ID)
	}
	var records []outfitPlanRecord
	if err := query.Order("local_date DESC").Order("created_at DESC").Order("id ASC").Limit(limit + 1).Find(&records).Error; err != nil {
		return outfitplanapp.OutfitPlanPage{}, outfitplanapp.ErrOutfitPlanUnavailable
	}
	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}
	itemsByPlan, err := readOutfitItemsForPlans(r.database.WithContext(ctx), ownerID, records)
	if err != nil {
		return outfitplanapp.OutfitPlanPage{}, outfitplanapp.ErrOutfitPlanUnavailable
	}
	page := outfitplanapp.OutfitPlanPage{Plans: make([]outfitplanapp.OutfitPlan, 0, len(records))}
	for _, record := range records {
		page.Plans = append(page.Plans, outfitFromRecord(record, itemsByPlan[record.ID]))
	}
	if hasMore && len(page.Plans) != 0 {
		last := page.Plans[len(page.Plans)-1].ID
		page.NextAfterID = &last
	}
	return page, nil
}

func (r *OutfitPlanRepository) GetOutfitPlan(ctx context.Context, ownerID, planID string) (outfitplanapp.OutfitPlan, error) {
	plan, err := readOutfitPlan(r.database.WithContext(ctx), ownerID, planID)
	if err != nil {
		return outfitplanapp.OutfitPlan{}, outfitPlanLookupError(err)
	}
	return plan, nil
}

func (r *OutfitPlanRepository) UpdateOutfitPlan(ctx context.Context, ownerID, planID string, expectedRevision int, input outfitplanapp.OutfitPlanInput, at time.Time) (outfitplanapp.OutfitPlan, error) {
	var result outfitplanapp.OutfitPlan
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureSyncSeed(tx, ownerID, at); err != nil {
			return err
		}
		var record outfitPlanRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, planID).First(&record).Error; err != nil {
			return err
		}
		if record.Revision != expectedRevision || record.Status != string(outfitplanapp.OutfitPlanActive) || record.TimeZone != input.TimeZone {
			return outfitplanapp.ErrOutfitPlanConflict
		}
		date, _ := time.Parse("2006-01-02", input.LocalDate)
		location, err := time.LoadLocation(input.TimeZone)
		if err != nil {
			return outfitplanapp.ErrInvalidOutfitPlanInput
		}
		local := at.In(location)
		today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
		if date.Before(today) && !sameDate(date, record.LocalDate) {
			return outfitplanapp.ErrInvalidOutfitPlanInput
		}
		items, err := snapshotOutfitItems(tx, ownerID, planID, input)
		if err != nil {
			return err
		}
		if err := tx.Where("owner_id = ? AND plan_id = ?", ownerID, planID).Delete(&outfitPlanItemRecord{}).Error; err != nil {
			return err
		}
		if at.Before(record.UpdatedAt) {
			at = record.UpdatedAt
		}
		record.LocalDate, record.ContextSummary, record.Revision, record.UpdatedAt = date, input.ContextSummary, record.Revision+1, at
		updated := tx.Model(&outfitPlanRecord{}).Where("owner_id = ? AND id = ? AND revision = ?", ownerID, planID, expectedRevision).Updates(map[string]any{"local_date": record.LocalDate, "context_summary": record.ContextSummary, "revision": record.Revision, "updated_at": record.UpdatedAt})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return outfitplanapp.ErrOutfitPlanConflict
		}
		if err := tx.Create(&items).Error; err != nil {
			return err
		}
		result = outfitFromRecord(record, items)
		return appendSyncChanges(tx, ownerID, at, syncUpsert("outfit_plan", planID, record.Revision))
	})
	if err != nil {
		return outfitplanapp.OutfitPlan{}, outfitPlanWriteError(err)
	}
	return result, nil
}

func (r *OutfitPlanRepository) CancelOutfitPlan(ctx context.Context, ownerID, planID string, expectedRevision int, at time.Time) (outfitplanapp.OutfitPlan, error) {
	var result outfitplanapp.OutfitPlan
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureSyncSeed(tx, ownerID, at); err != nil {
			return err
		}
		var record outfitPlanRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, planID).First(&record).Error; err != nil {
			return err
		}
		if record.Revision != expectedRevision || record.Status != string(outfitplanapp.OutfitPlanActive) {
			return outfitplanapp.ErrOutfitPlanConflict
		}
		if at.Before(record.UpdatedAt) {
			at = record.UpdatedAt
		}
		record.Status, record.Revision, record.UpdatedAt = string(outfitplanapp.OutfitPlanCancelled), record.Revision+1, at
		if err := tx.Model(&outfitPlanRecord{}).Where("owner_id = ? AND id = ? AND revision = ?", ownerID, planID, expectedRevision).Updates(map[string]any{"status": record.Status, "revision": record.Revision, "updated_at": record.UpdatedAt}).Error; err != nil {
			return err
		}
		itemsByPlan, err := readOutfitItemsForPlans(tx, ownerID, []outfitPlanRecord{record})
		if err != nil {
			return err
		}
		result = outfitFromRecord(record, itemsByPlan[planID])
		return appendSyncChanges(tx, ownerID, at, syncUpsert("outfit_plan", planID, record.Revision))
	})
	if err != nil {
		return outfitplanapp.OutfitPlan{}, outfitPlanWriteError(err)
	}
	return result, nil
}

func (r *OutfitPlanRepository) MarkOutfitPlanNotWorn(ctx context.Context, ownerID, planID string, expectedRevision int, at time.Time) (outfitplanapp.OutfitPlan, error) {
	return r.transitionOutfitPlan(ctx, ownerID, planID, expectedRevision, outfitplanapp.OutfitPlanActive, outfitplanapp.OutfitPlanNotWorn, at)
}

func (r *OutfitPlanRepository) RestoreOutfitPlan(ctx context.Context, ownerID, planID string, expectedRevision int, at time.Time) (outfitplanapp.OutfitPlan, error) {
	return r.transitionOutfitPlan(ctx, ownerID, planID, expectedRevision, outfitplanapp.OutfitPlanNotWorn, outfitplanapp.OutfitPlanActive, at)
}

func (r *OutfitPlanRepository) transitionOutfitPlan(ctx context.Context, ownerID, planID string, expectedRevision int, from, to outfitplanapp.OutfitPlanStatus, at time.Time) (outfitplanapp.OutfitPlan, error) {
	var result outfitplanapp.OutfitPlan
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureSyncSeed(tx, ownerID, at); err != nil {
			return err
		}
		var record outfitPlanRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, planID).First(&record).Error; err != nil {
			return err
		}
		if record.Revision != expectedRevision || record.Status != string(from) {
			return outfitplanapp.ErrOutfitPlanConflict
		}
		var linked int64
		if err := tx.Model(&wearEventRecord{}).Where("owner_id = ? AND source_plan_id = ?", ownerID, planID).Count(&linked).Error; err != nil {
			return err
		}
		if linked != 0 {
			return outfitplanapp.ErrOutfitPlanConflict
		}
		if at.Before(record.UpdatedAt) {
			at = record.UpdatedAt
		}
		record.Status, record.Revision, record.UpdatedAt = string(to), record.Revision+1, at
		updated := tx.Model(&outfitPlanRecord{}).Where("owner_id = ? AND id = ? AND revision = ?", ownerID, planID, expectedRevision).Updates(map[string]any{"status": record.Status, "revision": record.Revision, "updated_at": record.UpdatedAt})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return outfitplanapp.ErrOutfitPlanConflict
		}
		items, err := readOutfitItemsForPlans(tx, ownerID, []outfitPlanRecord{record})
		if err != nil {
			return err
		}
		result = outfitFromRecord(record, items[planID])
		return appendSyncChanges(tx, ownerID, at, syncUpsert("outfit_plan", planID, record.Revision))
	})
	if err != nil {
		return outfitplanapp.OutfitPlan{}, outfitPlanWriteError(err)
	}
	return result, nil
}

func (r *OutfitPlanRepository) DeleteOutfitPlan(ctx context.Context, ownerID, planID string, expectedRevision int, at time.Time) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureSyncSeed(tx, ownerID, at); err != nil {
			return err
		}
		var record outfitPlanRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("owner_id", "id", "revision").Where("owner_id = ? AND id = ?", ownerID, planID).First(&record).Error; err != nil {
			return err
		}
		if record.Revision != expectedRevision {
			return outfitplanapp.ErrOutfitPlanConflict
		}
		if err := createOutfitPlanTombstone(tx, ownerID, planID, at); err != nil {
			return err
		}
		if err := unlinkWearEventsFromPlan(tx, ownerID, planID, at); err != nil {
			return err
		}
		if err := unlinkDiaryReference(tx, ownerID, "plan_id", planID, at); err != nil {
			return err
		}
		if err := tx.Where("owner_id = ? AND id = ?", ownerID, planID).Delete(&outfitPlanRecord{}).Error; err != nil {
			return err
		}
		return appendSyncChanges(tx, ownerID, at, syncDelete("outfit_plan", planID, nil))
	})
	return outfitPlanWriteError(err)
}

func unlinkWearEventsFromPlan(tx *gorm.DB, ownerID, planID string, at time.Time) error {
	var events []wearEventRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND source_plan_id = ?", ownerID, planID).Find(&events).Error; err != nil {
		return err
	}
	for _, event := range events {
		updatedAt := at
		if updatedAt.Before(event.UpdatedAt) {
			updatedAt = event.UpdatedAt
		}
		if err := tx.Model(&wearEventRecord{}).Where("owner_id = ? AND id = ? AND revision = ?", ownerID, event.ID, event.Revision).Updates(map[string]any{"source_plan_id": nil, "source_plan_revision": nil, "source_kind": string(weareventapp.WearEventUnplanned), "revision": event.Revision + 1, "updated_at": updatedAt}).Error; err != nil {
			return err
		}
		if err := appendSyncChanges(tx, ownerID, at, syncUpsert("wear_event", event.ID, event.Revision+1)); err != nil {
			return err
		}
	}
	return nil
}

func snapshotOutfitItems(tx *gorm.DB, ownerID, planID string, input outfitplanapp.OutfitPlanInput) ([]outfitPlanItemRecord, error) {
	ids := make([]string, len(input.Items))
	for index, item := range input.Items {
		ids[index] = item.ItemID
	}
	var wardrobe []wardrobeItemRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id IN ?", ownerID, ids).Find(&wardrobe).Error; err != nil {
		return nil, err
	}
	if len(wardrobe) != len(ids) {
		return nil, outfitplanapp.ErrOutfitPlanConflict
	}
	byID := make(map[string]wardrobeItemRecord, len(wardrobe))
	for _, item := range wardrobe {
		byID[item.ID] = item
	}
	confirmed := make(map[string]bool, len(input.ConfirmedUnavailableIDs))
	for _, itemID := range input.ConfirmedUnavailableIDs {
		confirmed[itemID] = true
	}
	items := make([]outfitPlanItemRecord, 0, len(input.Items))
	for ordinal, selected := range input.Items {
		item := byID[selected.ItemID]
		if item.Revision != selected.Revision {
			return nil, outfitplanapp.ErrOutfitPlanConflict
		}
		if item.ArchivedAt != nil {
			return nil, outfitplanapp.ErrOutfitPlanConflict
		}
		unavailable := item.Availability != string(wardrobeapp.WardrobeWearable)
		if unavailable && !confirmed[item.ID] {
			return nil, outfitplanapp.ErrOutfitItemsUnavailable
		}
		if !unavailable && confirmed[item.ID] {
			return nil, outfitplanapp.ErrOutfitPlanConflict
		}
		itemID, revision, name, category, availability := item.ID, item.Revision, item.Name, item.Category, item.Availability
		items = append(items, outfitPlanItemRecord{OwnerID: ownerID, PlanID: planID, Ordinal: ordinal, WardrobeItemID: &itemID, ItemRevision: &revision, Name: &name, Category: &category, Availability: &availability, FormalityBand: copyString(item.FormalityBand), WarmthBand: copyString(item.WarmthBand), RainUse: copyString(item.RainUse), WalkingUse: copyString(item.WalkingUse)})
	}
	return items, nil
}

func readOutfitPlan(tx *gorm.DB, ownerID, planID string) (outfitplanapp.OutfitPlan, error) {
	var record outfitPlanRecord
	if err := tx.Where("owner_id = ? AND id = ?", ownerID, planID).First(&record).Error; err != nil {
		return outfitplanapp.OutfitPlan{}, outfitPlanLookupError(err)
	}
	itemsByPlan, err := readOutfitItemsForPlans(tx, ownerID, []outfitPlanRecord{record})
	if err != nil {
		return outfitplanapp.OutfitPlan{}, outfitplanapp.ErrOutfitPlanUnavailable
	}
	return outfitFromRecord(record, itemsByPlan[planID]), nil
}

func readOutfitItemsForPlans(tx *gorm.DB, ownerID string, plans []outfitPlanRecord) (map[string][]outfitPlanItemRecord, error) {
	result := make(map[string][]outfitPlanItemRecord, len(plans))
	if len(plans) == 0 {
		return result, nil
	}
	ids := make([]string, len(plans))
	for index, plan := range plans {
		ids[index] = plan.ID
	}
	var items []outfitPlanItemRecord
	if err := tx.Where("owner_id = ? AND plan_id IN ?", ownerID, ids).Order("plan_id ASC").Order("ordinal ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		result[item.PlanID] = append(result[item.PlanID], item)
	}
	return result, nil
}

func outfitFromRecord(record outfitPlanRecord, items []outfitPlanItemRecord) outfitplanapp.OutfitPlan {
	result := outfitplanapp.OutfitPlan{OwnerID: record.OwnerID, ID: record.ID, LocalDate: record.LocalDate.Format("2006-01-02"), TimeZone: record.TimeZone, ContextSummary: copyString(record.ContextSummary), Status: outfitplanapp.OutfitPlanStatus(record.Status), Revision: record.Revision, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, Items: make([]outfitplanapp.OutfitPlanItemSnapshot, 0, len(items))}
	for _, item := range items {
		snapshot := outfitplanapp.OutfitPlanItemSnapshot{Ordinal: item.Ordinal}
		if !item.Redacted && item.WardrobeItemID != nil && item.ItemRevision != nil && item.Name != nil && item.Category != nil && item.Availability != nil {
			snapshot.Content = &outfitplanapp.OutfitItemContent{ItemID: *item.WardrobeItemID, ItemRevision: *item.ItemRevision, Name: *item.Name, Category: wardrobeapp.WardrobeCategory(*item.Category), Availability: wardrobeapp.WardrobeAvailability(*item.Availability), Attributes: wardrobeapp.WardrobeAttributes{FormalityBand: typedPointer[wardrobeapp.WardrobeFormalityBand](item.FormalityBand), WarmthBand: typedPointer[wardrobeapp.WardrobeWarmthBand](item.WarmthBand), RainUse: typedPointer[wardrobeapp.WardrobeUseSuitability](item.RainUse), WalkingUse: typedPointer[wardrobeapp.WardrobeUseSuitability](item.WalkingUse)}}
		}
		result.Items = append(result.Items, snapshot)
	}
	return result
}

func outfitPlanMatchesInput(plan outfitplanapp.OutfitPlan, input outfitplanapp.OutfitPlanInput) bool {
	if plan.Status != outfitplanapp.OutfitPlanActive || plan.LocalDate != input.LocalDate || plan.TimeZone != input.TimeZone || !stringsEqual(plan.ContextSummary, input.ContextSummary) || len(plan.Items) != len(input.Items) {
		return false
	}
	confirmed := make(map[string]bool, len(input.ConfirmedUnavailableIDs))
	for _, itemID := range input.ConfirmedUnavailableIDs {
		confirmed[itemID] = true
	}
	for index, selected := range input.Items {
		content := plan.Items[index].Content
		if content == nil || content.ItemID != selected.ItemID || content.ItemRevision != selected.Revision || confirmed[selected.ItemID] != (content.Availability != wardrobeapp.WardrobeWearable) {
			return false
		}
	}
	return true
}

func createOutfitPlanTombstone(tx *gorm.DB, ownerID, planID string, at time.Time) error {
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&outfitPlanDeletionRecord{OwnerID: ownerID, ID: planID, DeletedAt: at}).Error
}

func sameDate(left, right time.Time) bool {
	return left.Format("2006-01-02") == right.Format("2006-01-02")
}

func copyString(value *string) *string {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func outfitPlanLookupError(err error) error {
	if errors.Is(err, outfitplanapp.ErrOutfitPlanNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		return outfitplanapp.ErrOutfitPlanNotFound
	}
	return outfitplanapp.ErrOutfitPlanUnavailable
}

func outfitPlanWriteError(err error) error {
	if err == nil {
		return nil
	}
	for _, known := range []error{outfitplanapp.ErrInvalidOutfitPlanInput, outfitplanapp.ErrOutfitPlanNotFound, outfitplanapp.ErrOutfitPlanConflict, outfitplanapp.ErrOutfitItemsUnavailable} {
		if errors.Is(err, known) {
			return known
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return outfitplanapp.ErrOutfitPlanNotFound
	}
	return outfitplanapp.ErrOutfitPlanUnavailable
}

func sortedPlanIDs(plans []outfitPlanRecord) []string {
	ids := make([]string, len(plans))
	for index, plan := range plans {
		ids[index] = plan.ID
	}
	slices.Sort(ids)
	return ids
}
