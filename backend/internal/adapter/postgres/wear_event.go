package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	outfitplanapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitplan"
	wardrobeapp "github.com/StephenQiu30/then-server/backend/internal/application/wardrobe"
	weareventapp "github.com/StephenQiu30/then-server/backend/internal/application/wearevent"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WearEventRepository struct{ database *gorm.DB }

func NewWearEventRepository(database *gorm.DB) *WearEventRepository {
	return &WearEventRepository{database: database}
}

type wearEventRecord struct {
	OwnerID            string                       `gorm:"column:owner_id;type:uuid;primaryKey;index:wear_events_owner_order_idx,priority:1"`
	ID                 string                       `gorm:"column:id;type:uuid;primaryKey;uniqueIndex:wear_events_id_unique"`
	LocalDate          time.Time                    `gorm:"column:local_date;type:date;not null;index:wear_events_owner_order_idx,priority:2,sort:desc"`
	TimeZone           string                       `gorm:"column:time_zone;type:text;not null;check:wear_events_time_zone_check,char_length(time_zone) BETWEEN 1 AND 255"`
	Completeness       string                       `gorm:"column:completeness;type:text;not null;check:wear_events_completeness_check,completeness IN ('partial','complete')"`
	ContextSummary     *string                      `gorm:"column:context_summary;type:text;check:wear_events_context_summary_check,context_summary IS NULL OR (context_summary = btrim(context_summary) AND char_length(context_summary) BETWEEN 1 AND 120)"`
	SourcePlanID       *string                      `gorm:"column:source_plan_id;type:uuid;index:wear_events_owner_source_plan_idx,priority:2"`
	SourcePlanRevision *int                         `gorm:"column:source_plan_revision"`
	SourceKind         string                       `gorm:"column:source_kind;type:text;not null;check:wear_events_source_check,(source_kind = 'unplanned' AND source_plan_id IS NULL AND source_plan_revision IS NULL) OR (source_kind IN ('followed_plan','changed_plan','different_outfit') AND source_plan_id IS NOT NULL AND source_plan_revision >= 1)"`
	CreateFingerprint  string                       `gorm:"column:create_fingerprint;type:char(64);not null;check:wear_events_create_fingerprint_check,char_length(create_fingerprint) = 64"`
	Revision           int                          `gorm:"column:revision;not null;check:wear_events_revision_check,revision >= 1"`
	CreatedAt          time.Time                    `gorm:"column:created_at;type:timestamptz;not null;index:wear_events_owner_order_idx,priority:3,sort:desc"`
	UpdatedAt          time.Time                    `gorm:"column:updated_at;type:timestamptz;not null;check:wear_events_timestamps_check,updated_at >= created_at"`
	Items              []wearEventItemRecord        `gorm:"foreignKey:OwnerID,EventID;references:OwnerID,ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	Feedback           *wearFeedbackRecord          `gorm:"foreignKey:OwnerID,EventID;references:OwnerID,ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
	FeedbackMutations  []wearFeedbackMutationRecord `gorm:"foreignKey:OwnerID,EventID;references:OwnerID,ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
}

func (wearEventRecord) TableName() string { return "wear_events" }

type wearEventItemRecord struct {
	OwnerID        string  `gorm:"column:owner_id;type:uuid;primaryKey;index:wear_event_items_owner_wardrobe_idx,priority:1"`
	EventID        string  `gorm:"column:event_id;type:uuid;primaryKey;index:wear_event_items_owner_wardrobe_idx,priority:3"`
	Ordinal        int     `gorm:"column:ordinal;primaryKey;check:wear_event_items_ordinal_check,ordinal >= 0 AND ordinal < 20"`
	WardrobeItemID *string `gorm:"column:wardrobe_item_id;type:uuid;index:wear_event_items_owner_wardrobe_idx,priority:2"`
	ItemRevision   *int    `gorm:"column:item_revision"`
	Name           *string `gorm:"column:name;type:text"`
	Category       *string `gorm:"column:category;type:text"`
	Availability   *string `gorm:"column:availability;type:text"`
	FormalityBand  *string `gorm:"column:formality_band;type:text;check:wear_event_items_formality_check,formality_band IN ('casual','smart_casual','formal')"`
	WarmthBand     *string `gorm:"column:warmth_band;type:text;check:wear_event_items_warmth_check,warmth_band IN ('light','medium','warm')"`
	RainUse        *string `gorm:"column:rain_use;type:text;check:wear_event_items_rain_check,rain_use IN ('suitable','unsuitable')"`
	WalkingUse     *string `gorm:"column:walking_use;type:text;check:wear_event_items_walking_check,walking_use IN ('suitable','unsuitable')"`
	Redacted       bool    `gorm:"column:redacted;not null;check:wear_event_items_content_check,(redacted AND wardrobe_item_id IS NULL AND item_revision IS NULL AND name IS NULL AND category IS NULL AND availability IS NULL AND formality_band IS NULL AND warmth_band IS NULL AND rain_use IS NULL AND walking_use IS NULL) OR (NOT redacted AND wardrobe_item_id IS NOT NULL AND item_revision >= 1 AND name IS NOT NULL AND category IN ('top','bottom','one_piece','outerwear','shoes','bag','accessory') AND availability IN ('wearable','laundry','lent_out','packed'))"`
}

func (wearEventItemRecord) TableName() string { return "wear_event_items" }

type wearEventDeletionRecord struct {
	OwnerID   string    `gorm:"column:owner_id;type:uuid;primaryKey"`
	ID        string    `gorm:"column:id;type:uuid;primaryKey"`
	DeletedAt time.Time `gorm:"column:deleted_at;type:timestamptz;not null"`
}

func (wearEventDeletionRecord) TableName() string { return "wear_event_deletions" }

func (r *WearEventRepository) CreateWearEvent(ctx context.Context, ownerID, eventID string, input weareventapp.WearEventInput, at time.Time) (weareventapp.WearEvent, error) {
	var result weareventapp.WearEvent
	fingerprint := wearEventCreateFingerprint(input)
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureSyncSeed(tx, ownerID, at); err != nil {
			return err
		}
		var existingRecord wearEventRecord
		err := tx.Where("owner_id = ? AND id = ?", ownerID, eventID).First(&existingRecord).Error
		if err == nil {
			if existingRecord.CreateFingerprint != fingerprint {
				return weareventapp.ErrWearEventConflict
			}
			result, err = readWearEvent(tx, ownerID, eventID)
			return err
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var deleted int64
		if err := tx.Model(&wearEventDeletionRecord{}).Where("owner_id = ? AND id = ?", ownerID, eventID).Count(&deleted).Error; err != nil {
			return err
		}
		if deleted != 0 {
			return weareventapp.ErrWearEventConflict
		}
		result, err = writeWearEvent(tx, ownerID, eventID, nil, input, fingerprint, at)
		return err
	})
	if err != nil {
		return weareventapp.WearEvent{}, wearEventWriteError(err)
	}
	return result, nil
}

func (r *WearEventRepository) ListWearEvents(ctx context.Context, ownerID string, limit int, afterID, localDate *string) (weareventapp.WearEventPage, error) {
	query := r.database.WithContext(ctx).Where("owner_id = ?", ownerID)
	if localDate != nil {
		query = query.Where("local_date = ?", *localDate)
	}
	if afterID != nil {
		var cursor wearEventRecord
		cursorQuery := r.database.WithContext(ctx).Select("id", "local_date", "created_at").Where("owner_id = ? AND id = ?", ownerID, *afterID)
		if localDate != nil {
			cursorQuery = cursorQuery.Where("local_date = ?", *localDate)
		}
		if err := cursorQuery.First(&cursor).Error; err != nil {
			return weareventapp.WearEventPage{}, wearEventLookupError(err)
		}
		query = query.Where("local_date < ? OR (local_date = ? AND created_at < ?) OR (local_date = ? AND created_at = ? AND id > ?)", cursor.LocalDate, cursor.LocalDate, cursor.CreatedAt, cursor.LocalDate, cursor.CreatedAt, cursor.ID)
	}
	var records []wearEventRecord
	if err := query.Order("local_date DESC").Order("created_at DESC").Order("id ASC").Limit(limit + 1).Find(&records).Error; err != nil {
		return weareventapp.WearEventPage{}, weareventapp.ErrWearEventServiceUnavailable
	}
	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}
	items, err := readWearItemsForEvents(r.database.WithContext(ctx), ownerID, records)
	if err != nil {
		return weareventapp.WearEventPage{}, weareventapp.ErrWearEventServiceUnavailable
	}
	page := weareventapp.WearEventPage{Events: make([]weareventapp.WearEvent, 0, len(records))}
	for _, record := range records {
		page.Events = append(page.Events, wearEventFromRecord(record, items[record.ID]))
	}
	if hasMore && len(page.Events) > 0 {
		last := page.Events[len(page.Events)-1].ID
		page.NextAfterID = &last
	}
	return page, nil
}

func (r *WearEventRepository) GetWearEvent(ctx context.Context, ownerID, eventID string) (weareventapp.WearEvent, error) {
	return readWearEvent(r.database.WithContext(ctx), ownerID, eventID)
}

func (r *WearEventRepository) UpdateWearEvent(ctx context.Context, ownerID, eventID string, expectedRevision int, input weareventapp.WearEventInput, at time.Time) (weareventapp.WearEvent, error) {
	var result weareventapp.WearEvent
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureSyncSeed(tx, ownerID, at); err != nil {
			return err
		}
		var record wearEventRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, eventID).First(&record).Error; err != nil {
			return err
		}
		if record.Revision != expectedRevision || record.TimeZone != input.TimeZone {
			return weareventapp.ErrWearEventConflict
		}
		old, err := readWearEvent(tx, ownerID, eventID)
		if err != nil {
			return err
		}
		result, err = writeWearEvent(tx, ownerID, eventID, &old, input, record.CreateFingerprint, at)
		return err
	})
	if err != nil {
		return weareventapp.WearEvent{}, wearEventWriteError(err)
	}
	return result, nil
}

func (r *WearEventRepository) DeleteWearEvent(ctx context.Context, ownerID, eventID string, expectedRevision int, at time.Time) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureSyncSeed(tx, ownerID, at); err != nil {
			return err
		}
		var record wearEventRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, eventID).First(&record).Error; err != nil {
			return err
		}
		if record.Revision != expectedRevision {
			return weareventapp.ErrWearEventConflict
		}
		if err := createWearEventTombstone(tx, ownerID, eventID, at); err != nil {
			return err
		}
		if err := deleteLinkedWearRecords(tx, ownerID, eventID, at); err != nil {
			return err
		}
		if err := tx.Where("owner_id = ? AND id = ?", ownerID, eventID).Delete(&wearEventRecord{}).Error; err != nil {
			return err
		}
		if err := appendSyncChanges(tx, ownerID, at, syncDelete("wear_event", eventID, nil)); err != nil {
			return err
		}
		if record.SourcePlanID != nil {
			return recomputeOutfitPlanStatus(tx, ownerID, *record.SourcePlanID, at)
		}
		return nil
	})
	return wearEventWriteError(err)
}

func writeWearEvent(tx *gorm.DB, ownerID, eventID string, old *weareventapp.WearEvent, input weareventapp.WearEventInput, createFingerprint string, at time.Time) (weareventapp.WearEvent, error) {
	candidates, err := duplicateWearEvents(tx, ownerID, eventID, input.LocalDate, input.Items)
	if err != nil {
		return weareventapp.WearEvent{}, err
	}
	if !sameWearCandidates(candidates, input.DuplicateConfirmations) {
		return weareventapp.WearEvent{}, &weareventapp.WearEventDuplicateError{Candidates: candidates}
	}
	if err := validateWearSourcePlan(tx, ownerID, old, input); err != nil {
		return weareventapp.WearEvent{}, err
	}
	items, wardrobe, err := snapshotWearItems(tx, ownerID, eventID, input)
	if err != nil {
		return weareventapp.WearEvent{}, err
	}
	date, _ := time.Parse("2006-01-02", input.LocalDate)
	revision := 1
	createdAt := at
	if old != nil {
		revision = old.Revision + 1
		createdAt = old.CreatedAt
		if at.Before(old.UpdatedAt) {
			at = old.UpdatedAt
		}
		if err := tx.Where("owner_id = ? AND event_id = ?", ownerID, eventID).Delete(&wearEventItemRecord{}).Error; err != nil {
			return weareventapp.WearEvent{}, err
		}
	}
	record := wearEventRecord{OwnerID: ownerID, ID: eventID, LocalDate: date, TimeZone: input.TimeZone, Completeness: string(input.Completeness), ContextSummary: input.ContextSummary, SourcePlanID: input.SourcePlanID, SourcePlanRevision: input.SourcePlanRevision, SourceKind: string(input.SourceKind), CreateFingerprint: createFingerprint, Revision: revision, CreatedAt: createdAt, UpdatedAt: at}
	if old == nil {
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Omit("Items").Create(&record)
		if created.Error != nil {
			return weareventapp.WearEvent{}, created.Error
		}
		if created.RowsAffected == 0 {
			var existing wearEventRecord
			if err := tx.Where("owner_id = ? AND id = ?", ownerID, eventID).First(&existing).Error; err != nil || existing.CreateFingerprint != createFingerprint {
				return weareventapp.WearEvent{}, weareventapp.ErrWearEventConflict
			}
			return readWearEvent(tx, ownerID, eventID)
		}
	} else {
		updated := tx.Model(&wearEventRecord{}).Where("owner_id = ? AND id = ? AND revision = ?", ownerID, eventID, old.Revision).Updates(map[string]any{"local_date": date, "completeness": record.Completeness, "context_summary": record.ContextSummary, "source_plan_id": record.SourcePlanID, "source_plan_revision": record.SourcePlanRevision, "source_kind": record.SourceKind, "revision": revision, "updated_at": at})
		if updated.Error != nil {
			return weareventapp.WearEvent{}, updated.Error
		}
		if updated.RowsAffected != 1 {
			return weareventapp.WearEvent{}, weareventapp.ErrWearEventConflict
		}
	}
	if err := tx.Create(&items).Error; err != nil {
		return weareventapp.WearEvent{}, err
	}
	if err := appendSyncChanges(tx, ownerID, at, syncUpsert("wear_event", eventID, revision)); err != nil {
		return weareventapp.WearEvent{}, err
	}
	if err := applyLaundrySelection(tx, wardrobe, input.LaundryItemIDs, at); err != nil {
		return weareventapp.WearEvent{}, err
	}
	planIDs := map[string]bool{}
	if old != nil && old.SourcePlanID != nil {
		planIDs[*old.SourcePlanID] = true
	}
	if input.SourcePlanID != nil {
		planIDs[*input.SourcePlanID] = true
	}
	for planID := range planIDs {
		if err := recomputeOutfitPlanStatus(tx, ownerID, planID, at); err != nil {
			return weareventapp.WearEvent{}, err
		}
	}
	return wearEventFromRecord(record, items), nil
}

func validateWearSourcePlan(tx *gorm.DB, ownerID string, old *weareventapp.WearEvent, input weareventapp.WearEventInput) error {
	if input.SourcePlanID == nil {
		return nil
	}
	var plan outfitPlanRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, *input.SourcePlanID).First(&plan).Error; err != nil {
		return weareventapp.ErrWearEventConflict
	}
	preserves := old != nil && old.SourcePlanID != nil && *old.SourcePlanID == *input.SourcePlanID && old.SourcePlanRevision != nil && *old.SourcePlanRevision == *input.SourcePlanRevision
	if !preserves && plan.Revision != *input.SourcePlanRevision {
		return weareventapp.ErrWearEventConflict
	}
	if plan.Status != string(outfitplanapp.OutfitPlanActive) && plan.Status != string(outfitplanapp.OutfitPlanCompleted) {
		return weareventapp.ErrWearEventConflict
	}
	return nil
}

func snapshotWearItems(tx *gorm.DB, ownerID, eventID string, input weareventapp.WearEventInput) ([]wearEventItemRecord, map[string]wardrobeItemRecord, error) {
	ids := make([]string, len(input.Items))
	for index, item := range input.Items {
		ids[index] = item.ItemID
	}
	var records []wardrobeItemRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id IN ?", ownerID, ids).Find(&records).Error; err != nil {
		return nil, nil, err
	}
	if len(records) != len(ids) {
		return nil, nil, weareventapp.ErrWearEventConflict
	}
	byID := make(map[string]wardrobeItemRecord, len(records))
	for _, item := range records {
		byID[item.ID] = item
	}
	confirmed := stringSet(input.ConfirmedUnavailableIDs)
	items := make([]wearEventItemRecord, 0, len(input.Items))
	for ordinal, selected := range input.Items {
		item, exists := byID[selected.ItemID]
		if !exists || item.Revision != selected.Revision {
			return nil, nil, weareventapp.ErrWearEventConflict
		}
		unavailable := item.Availability != string(wardrobeapp.WardrobeWearable)
		if unavailable != confirmed[item.ID] {
			if unavailable {
				return nil, nil, weareventapp.ErrWearEventItemsUnavailable
			}
			return nil, nil, weareventapp.ErrWearEventConflict
		}
		itemID, revision, name, category, availability := item.ID, item.Revision, item.Name, item.Category, item.Availability
		items = append(items, wearEventItemRecord{OwnerID: ownerID, EventID: eventID, Ordinal: ordinal, WardrobeItemID: &itemID, ItemRevision: &revision, Name: &name, Category: &category, Availability: &availability, FormalityBand: copyString(item.FormalityBand), WarmthBand: copyString(item.WarmthBand), RainUse: copyString(item.RainUse), WalkingUse: copyString(item.WalkingUse)})
	}
	return items, byID, nil
}

func applyLaundrySelection(tx *gorm.DB, wardrobe map[string]wardrobeItemRecord, selected []string, at time.Time) error {
	for _, itemID := range selected {
		item := wardrobe[itemID]
		if item.Availability == string(wardrobeapp.WardrobeLaundry) {
			continue
		}
		updatedAt := at
		if updatedAt.Before(item.UpdatedAt) {
			updatedAt = item.UpdatedAt
		}
		updated := tx.Model(&wardrobeItemRecord{}).Where("owner_id = ? AND id = ? AND revision = ?", item.OwnerID, item.ID, item.Revision).Updates(map[string]any{"availability": string(wardrobeapp.WardrobeLaundry), "revision": item.Revision + 1, "updated_at": updatedAt})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return weareventapp.ErrWearEventConflict
		}
		if err := appendSyncChanges(tx, item.OwnerID, at, syncUpsert("wardrobe_item", item.ID, item.Revision+1)); err != nil {
			return err
		}
	}
	return nil
}

func duplicateWearEvents(tx *gorm.DB, ownerID, eventID, localDate string, selected []outfitplanapp.OutfitSelection) ([]weareventapp.WearEventCandidate, error) {
	var records []wearEventRecord
	if err := tx.Select("owner_id", "id", "revision").Where("owner_id = ? AND local_date = ? AND id <> ?", ownerID, localDate, eventID).Order("id ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	selectedIDs := map[string]bool{}
	for _, item := range selected {
		selectedIDs[item.ItemID] = true
	}
	result := []weareventapp.WearEventCandidate{}
	for _, record := range records {
		var items []wearEventItemRecord
		if err := tx.Select("wardrobe_item_id").Where("owner_id = ? AND event_id = ? AND wardrobe_item_id IS NOT NULL", ownerID, record.ID).Find(&items).Error; err != nil {
			return nil, err
		}
		other := map[string]bool{}
		for _, item := range items {
			if item.WardrobeItemID != nil {
				other[*item.WardrobeItemID] = true
			}
		}
		intersection, union := 0, len(selectedIDs)
		for id := range other {
			if selectedIDs[id] {
				intersection++
			} else {
				union++
			}
		}
		if union > 0 && float64(intersection)/float64(union) >= 0.8 {
			result = append(result, weareventapp.WearEventCandidate{ID: record.ID, Revision: record.Revision})
		}
	}
	return result, nil
}

func recomputeOutfitPlanStatus(tx *gorm.DB, ownerID, planID string, at time.Time) error {
	var plan outfitPlanRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, planID).First(&plan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if plan.Status == string(outfitplanapp.OutfitPlanCancelled) || plan.Status == string(outfitplanapp.OutfitPlanNotWorn) {
		return nil
	}
	var count int64
	if err := tx.Model(&wearEventRecord{}).Where("owner_id = ? AND source_plan_id = ?", ownerID, planID).Count(&count).Error; err != nil {
		return err
	}
	desired := string(outfitplanapp.OutfitPlanActive)
	if count > 0 {
		desired = string(outfitplanapp.OutfitPlanCompleted)
	}
	if plan.Status == desired {
		return nil
	}
	if at.Before(plan.UpdatedAt) {
		at = plan.UpdatedAt
	}
	if err := tx.Model(&outfitPlanRecord{}).Where("owner_id = ? AND id = ? AND revision = ?", ownerID, planID, plan.Revision).Updates(map[string]any{"status": desired, "revision": plan.Revision + 1, "updated_at": at}).Error; err != nil {
		return err
	}
	return appendSyncChanges(tx, ownerID, at, syncUpsert("outfit_plan", planID, plan.Revision+1))
}

func readWearEvent(tx *gorm.DB, ownerID, eventID string) (weareventapp.WearEvent, error) {
	var record wearEventRecord
	if err := tx.Where("owner_id = ? AND id = ?", ownerID, eventID).First(&record).Error; err != nil {
		return weareventapp.WearEvent{}, wearEventLookupError(err)
	}
	items, err := readWearItemsForEvents(tx, ownerID, []wearEventRecord{record})
	if err != nil {
		return weareventapp.WearEvent{}, weareventapp.ErrWearEventServiceUnavailable
	}
	return wearEventFromRecord(record, items[eventID]), nil
}

func readWearItemsForEvents(tx *gorm.DB, ownerID string, events []wearEventRecord) (map[string][]wearEventItemRecord, error) {
	result := make(map[string][]wearEventItemRecord, len(events))
	if len(events) == 0 {
		return result, nil
	}
	ids := make([]string, len(events))
	for index, event := range events {
		ids[index] = event.ID
	}
	var items []wearEventItemRecord
	if err := tx.Where("owner_id = ? AND event_id IN ?", ownerID, ids).Order("event_id ASC").Order("ordinal ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	for _, item := range items {
		result[item.EventID] = append(result[item.EventID], item)
	}
	return result, nil
}

func wearEventFromRecord(record wearEventRecord, items []wearEventItemRecord) weareventapp.WearEvent {
	result := weareventapp.WearEvent{ID: record.ID, OwnerID: record.OwnerID, LocalDate: record.LocalDate.Format("2006-01-02"), TimeZone: record.TimeZone, Completeness: weareventapp.WearEventCompleteness(record.Completeness), ContextSummary: copyString(record.ContextSummary), SourcePlanID: copyString(record.SourcePlanID), SourcePlanRevision: copyInt(record.SourcePlanRevision), SourceKind: weareventapp.WearEventSourceKind(record.SourceKind), Revision: record.Revision, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, Items: make([]outfitplanapp.OutfitPlanItemSnapshot, 0, len(items))}
	for _, item := range items {
		snapshot := outfitplanapp.OutfitPlanItemSnapshot{Ordinal: item.Ordinal}
		if !item.Redacted && item.WardrobeItemID != nil && item.ItemRevision != nil && item.Name != nil && item.Category != nil && item.Availability != nil {
			snapshot.Content = &outfitplanapp.OutfitItemContent{ItemID: *item.WardrobeItemID, ItemRevision: *item.ItemRevision, Name: *item.Name, Category: wardrobeapp.WardrobeCategory(*item.Category), Availability: wardrobeapp.WardrobeAvailability(*item.Availability), Attributes: wardrobeapp.WardrobeAttributes{FormalityBand: typedPointer[wardrobeapp.WardrobeFormalityBand](item.FormalityBand), WarmthBand: typedPointer[wardrobeapp.WardrobeWarmthBand](item.WarmthBand), RainUse: typedPointer[wardrobeapp.WardrobeUseSuitability](item.RainUse), WalkingUse: typedPointer[wardrobeapp.WardrobeUseSuitability](item.WalkingUse)}}
		}
		result.Items = append(result.Items, snapshot)
	}
	return result
}

func wearEventCreateFingerprint(input weareventapp.WearEventInput) string {
	payload := struct {
		LocalDate               string
		TimeZone                string
		Completeness            weareventapp.WearEventCompleteness
		ContextSummary          *string
		Items                   []outfitplanapp.OutfitSelection
		LaundryItemIDs          []string
		ConfirmedUnavailableIDs []string
		SourcePlanID            *string
		SourcePlanRevision      *int
		SourceKind              weareventapp.WearEventSourceKind
		DuplicateConfirmations  []weareventapp.WearEventCandidate
	}{
		LocalDate:               input.LocalDate,
		TimeZone:                input.TimeZone,
		Completeness:            input.Completeness,
		ContextSummary:          input.ContextSummary,
		Items:                   input.Items,
		LaundryItemIDs:          append([]string(nil), input.LaundryItemIDs...),
		ConfirmedUnavailableIDs: append([]string(nil), input.ConfirmedUnavailableIDs...),
		SourcePlanID:            input.SourcePlanID,
		SourcePlanRevision:      input.SourcePlanRevision,
		SourceKind:              input.SourceKind,
		DuplicateConfirmations:  append([]weareventapp.WearEventCandidate(nil), input.DuplicateConfirmations...),
	}
	sort.Strings(payload.LaundryItemIDs)
	sort.Strings(payload.ConfirmedUnavailableIDs)
	sort.Slice(payload.DuplicateConfirmations, func(i, j int) bool {
		return payload.DuplicateConfirmations[i].ID < payload.DuplicateConfirmations[j].ID
	})
	encoded, _ := json.Marshal(payload)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func sameWearCandidates(left, right []weareventapp.WearEventCandidate) bool {
	if len(left) != len(right) {
		return false
	}
	l, r := append([]weareventapp.WearEventCandidate(nil), left...), append([]weareventapp.WearEventCandidate(nil), right...)
	sort.Slice(l, func(i, j int) bool { return l[i].ID < l[j].ID })
	sort.Slice(r, func(i, j int) bool { return r[i].ID < r[j].ID })
	for index := range l {
		if l[index] != r[index] {
			return false
		}
	}
	return true
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func copyInt(value *int) *int {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func createWearEventTombstone(tx *gorm.DB, ownerID, eventID string, at time.Time) error {
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&wearEventDeletionRecord{OwnerID: ownerID, ID: eventID, DeletedAt: at}).Error
}

func wearEventLookupError(err error) error {
	if errors.Is(err, weareventapp.ErrWearEventNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		return weareventapp.ErrWearEventNotFound
	}
	return weareventapp.ErrWearEventServiceUnavailable
}

func wearEventWriteError(err error) error {
	if err == nil {
		return nil
	}
	for _, known := range []error{weareventapp.ErrInvalidWearEventInput, weareventapp.ErrWearEventNotFound, weareventapp.ErrWearEventConflict, weareventapp.ErrWearEventDuplicate, weareventapp.ErrWearEventItemsUnavailable} {
		if errors.Is(err, known) {
			return err
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return weareventapp.ErrWearEventNotFound
	}
	return weareventapp.ErrWearEventServiceUnavailable
}
