package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	diaryapp "github.com/StephenQiu30/then-server/backend/internal/application/diary"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DiaryRepository struct{ database *gorm.DB }

func NewDiaryRepository(database *gorm.DB) *DiaryRepository {
	return &DiaryRepository{database: database}
}

type diaryEntryRecord struct {
	OwnerID           string                  `gorm:"column:owner_id;type:uuid;primaryKey;index:diary_entries_owner_order_idx,priority:1"`
	ID                string                  `gorm:"column:id;type:uuid;primaryKey"`
	LocalDate         time.Time               `gorm:"column:local_date;type:date;not null;index:diary_entries_owner_order_idx,priority:2,sort:desc"`
	TimeZone          string                  `gorm:"column:time_zone;type:text;not null;check:diary_entries_time_zone_check,char_length(time_zone) BETWEEN 1 AND 255"`
	Title             *string                 `gorm:"column:title;type:text;check:diary_entries_title_check,title IS NULL OR (title = btrim(title) AND char_length(title) BETWEEN 1 AND 80)"`
	Body              *string                 `gorm:"column:body;type:text;check:diary_entries_body_check,body IS NULL OR (body = btrim(body) AND char_length(body) BETWEEN 1 AND 5000)"`
	Mood              *string                 `gorm:"column:mood;type:text;check:diary_entries_mood_check,mood IS NULL OR (mood = btrim(mood) AND char_length(mood) BETWEEN 1 AND 40)"`
	Occasion          *string                 `gorm:"column:occasion;type:text;check:diary_entries_occasion_check,occasion IS NULL OR (occasion = btrim(occasion) AND char_length(occasion) BETWEEN 1 AND 40)"`
	PlanID            *string                 `gorm:"column:plan_id;type:uuid;index:diary_entries_owner_plan_idx,priority:2"`
	WearEventID       *string                 `gorm:"column:wear_event_id;type:uuid;index:diary_entries_owner_wear_idx,priority:2"`
	CreateFingerprint string                  `gorm:"column:create_fingerprint;type:char(64);not null;check:diary_entries_fingerprint_check,char_length(create_fingerprint) = 64"`
	Revision          int                     `gorm:"column:revision;not null;check:diary_entries_revision_check,revision >= 1"`
	CreatedAt         time.Time               `gorm:"column:created_at;type:timestamptz;not null;index:diary_entries_owner_order_idx,priority:3,sort:desc"`
	UpdatedAt         time.Time               `gorm:"column:updated_at;type:timestamptz;not null;check:diary_entries_timestamps_check,updated_at >= created_at"`
	Plan              *outfitPlanRecord       `gorm:"foreignKey:PlanID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:SET NULL"`
	WearEvent         *wearEventRecord        `gorm:"foreignKey:WearEventID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:SET NULL"`
	Media             []diaryEntryMediaRecord `gorm:"foreignKey:OwnerID,EntryID;references:OwnerID,ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
}

func (diaryEntryRecord) TableName() string { return "diary_entries" }

type diaryEntryMediaRecord struct {
	OwnerID string           `gorm:"column:owner_id;type:uuid;primaryKey;uniqueIndex:diary_entry_media_unique,priority:1;index:diary_entry_media_owner_media_idx,priority:1"`
	EntryID string           `gorm:"column:entry_id;type:uuid;primaryKey;uniqueIndex:diary_entry_media_unique,priority:2"`
	Ordinal int              `gorm:"column:ordinal;primaryKey;check:diary_entry_media_ordinal_check,ordinal >= 0 AND ordinal < 9"`
	MediaID string           `gorm:"column:media_id;type:uuid;not null;uniqueIndex:diary_entry_media_unique,priority:3;index:diary_entry_media_owner_media_idx,priority:2"`
	Media   mediaAssetRecord `gorm:"foreignKey:MediaID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
}

func (diaryEntryMediaRecord) TableName() string { return "diary_entry_media" }

type diaryEntryDeletionRecord struct {
	OwnerID   string    `gorm:"column:owner_id;type:uuid;primaryKey"`
	ID        string    `gorm:"column:id;type:uuid;primaryKey"`
	DeletedAt time.Time `gorm:"column:deleted_at;type:timestamptz;not null"`
}

func (diaryEntryDeletionRecord) TableName() string { return "diary_entry_deletions" }

func (r *DiaryRepository) CreateDiaryEntry(ctx context.Context, ownerID, entryID string, input diaryapp.DiaryEntryInput, at time.Time) (diaryapp.DiaryEntry, error) {
	var result diaryapp.DiaryEntry
	fingerprint := diaryFingerprint(input)
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureSyncSeed(tx, ownerID, at); err != nil {
			return err
		}
		existing, err := readDiaryEntry(tx, ownerID, entryID)
		if err == nil {
			var record diaryEntryRecord
			if err := tx.Select("create_fingerprint").Where("owner_id = ? AND id = ?", ownerID, entryID).First(&record).Error; err != nil {
				return err
			}
			if record.CreateFingerprint != fingerprint {
				return diaryapp.ErrDiaryConflict
			}
			result = existing
			return nil
		}
		if !errors.Is(err, diaryapp.ErrDiaryNotFound) {
			return err
		}
		var deleted int64
		if err := tx.Model(&diaryEntryDeletionRecord{}).Where("owner_id = ? AND id = ?", ownerID, entryID).Count(&deleted).Error; err != nil {
			return err
		}
		if deleted != 0 {
			return diaryapp.ErrDiaryConflict
		}
		if err := validateDiaryReferences(tx, ownerID, input); err != nil {
			return err
		}
		date, _ := time.Parse("2006-01-02", input.LocalDate)
		record := diaryEntryRecord{OwnerID: ownerID, ID: entryID, LocalDate: date, TimeZone: input.TimeZone, Title: input.Title, Body: input.Body, Mood: input.Mood, Occasion: input.Occasion, PlanID: input.PlanID, WearEventID: input.WearEventID, CreateFingerprint: fingerprint, Revision: 1, CreatedAt: at, UpdatedAt: at}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Omit("Plan", "WearEvent", "Media").Create(&record)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			return diaryapp.ErrDiaryConflict
		}
		media := diaryMediaRecords(ownerID, entryID, input.MediaIDs)
		if len(media) > 0 {
			if err := tx.Omit("Media").Create(&media).Error; err != nil {
				return err
			}
		}
		result = diaryFromRecord(record, input.MediaIDs)
		return appendSyncChanges(tx, ownerID, at, syncUpsert("diary_entry", entryID, record.Revision))
	})
	if err != nil {
		return diaryapp.DiaryEntry{}, diaryWriteError(err)
	}
	return result, nil
}

func (r *DiaryRepository) ListDiaryEntries(ctx context.Context, ownerID string, limit int, afterID, dateFrom, dateTo *string) (diaryapp.DiaryEntryPage, error) {
	query := r.database.WithContext(ctx).Where("owner_id = ?", ownerID)
	if dateFrom != nil {
		query = query.Where("local_date >= ?", *dateFrom)
	}
	if dateTo != nil {
		query = query.Where("local_date <= ?", *dateTo)
	}
	if afterID != nil {
		var cursor diaryEntryRecord
		if err := r.database.WithContext(ctx).Select("id", "local_date", "created_at").Where("owner_id = ? AND id = ?", ownerID, *afterID).First(&cursor).Error; err != nil {
			return diaryapp.DiaryEntryPage{}, diaryLookupError(err)
		}
		query = query.Where("local_date < ? OR (local_date = ? AND created_at < ?) OR (local_date = ? AND created_at = ? AND id < ?)", cursor.LocalDate, cursor.LocalDate, cursor.CreatedAt, cursor.LocalDate, cursor.CreatedAt, cursor.ID)
	}
	var records []diaryEntryRecord
	if err := query.Order("local_date DESC").Order("created_at DESC").Order("id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return diaryapp.DiaryEntryPage{}, diaryapp.ErrDiaryUnavailable
	}
	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}
	mediaByEntry, err := readDiaryMediaForEntries(r.database.WithContext(ctx), ownerID, records)
	if err != nil {
		return diaryapp.DiaryEntryPage{}, diaryapp.ErrDiaryUnavailable
	}
	page := diaryapp.DiaryEntryPage{Entries: make([]diaryapp.DiaryEntry, 0, len(records))}
	for _, record := range records {
		page.Entries = append(page.Entries, diaryFromRecord(record, mediaByEntry[record.ID]))
	}
	if hasMore && len(page.Entries) > 0 {
		last := page.Entries[len(page.Entries)-1].ID
		page.NextAfterID = &last
	}
	return page, nil
}

func (r *DiaryRepository) GetDiaryEntry(ctx context.Context, ownerID, entryID string) (diaryapp.DiaryEntry, error) {
	entry, err := readDiaryEntry(r.database.WithContext(ctx), ownerID, entryID)
	return entry, diaryLookupError(err)
}

func (r *DiaryRepository) UpdateDiaryEntry(ctx context.Context, ownerID, entryID string, expectedRevision int, input diaryapp.DiaryEntryInput, at time.Time) (diaryapp.DiaryEntry, error) {
	var result diaryapp.DiaryEntry
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureSyncSeed(tx, ownerID, at); err != nil {
			return err
		}
		var record diaryEntryRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, entryID).First(&record).Error; err != nil {
			return err
		}
		if record.Revision != expectedRevision {
			return diaryapp.ErrDiaryConflict
		}
		if err := validateDiaryReferences(tx, ownerID, input); err != nil {
			return err
		}
		date, _ := time.Parse("2006-01-02", input.LocalDate)
		updates := map[string]any{"local_date": date, "time_zone": input.TimeZone, "title": input.Title, "body": input.Body, "mood": input.Mood, "occasion": input.Occasion, "plan_id": input.PlanID, "wear_event_id": input.WearEventID, "revision": record.Revision + 1, "updated_at": at}
		updated := tx.Model(&diaryEntryRecord{}).Where("owner_id = ? AND id = ? AND revision = ?", ownerID, entryID, record.Revision).Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return diaryapp.ErrDiaryConflict
		}
		if err := tx.Where("owner_id = ? AND entry_id = ?", ownerID, entryID).Delete(&diaryEntryMediaRecord{}).Error; err != nil {
			return err
		}
		media := diaryMediaRecords(ownerID, entryID, input.MediaIDs)
		if len(media) > 0 {
			if err := tx.Omit("Media").Create(&media).Error; err != nil {
				return err
			}
		}
		record.LocalDate, record.TimeZone, record.Title, record.Body = date, input.TimeZone, input.Title, input.Body
		record.Mood, record.Occasion, record.PlanID, record.WearEventID = input.Mood, input.Occasion, input.PlanID, input.WearEventID
		record.Revision, record.UpdatedAt = record.Revision+1, at
		result = diaryFromRecord(record, input.MediaIDs)
		return appendSyncChanges(tx, ownerID, at, syncUpsert("diary_entry", entryID, record.Revision))
	})
	if err != nil {
		return diaryapp.DiaryEntry{}, diaryWriteError(err)
	}
	return result, nil
}

func (r *DiaryRepository) DiaryDeletionImpact(ctx context.Context, ownerID, entryID string) (diaryapp.DiaryDeletionImpact, error) {
	entry, err := readDiaryEntry(r.database.WithContext(ctx), ownerID, entryID)
	if err != nil {
		return diaryapp.DiaryDeletionImpact{}, diaryLookupError(err)
	}
	var publishedPostCount int64
	if err := r.database.WithContext(ctx).Model(&postRecord{}).Where("source_diary_owner_id = ? AND source_diary_id = ? AND state = ?", ownerID, entryID, "published").Count(&publishedPostCount).Error; err != nil {
		return diaryapp.DiaryDeletionImpact{}, diaryapp.ErrDiaryUnavailable
	}
	return diaryapp.DiaryDeletionImpact{EntryID: entry.ID, Revision: entry.Revision, MediaCount: len(entry.MediaIDs), PublishedPostCount: int(publishedPostCount), MediaRetained: true}, nil
}

func (r *DiaryRepository) DeleteDiaryEntry(ctx context.Context, ownerID, entryID string, expectedRevision int, at time.Time) error {
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureSyncSeed(tx, ownerID, at); err != nil {
			return err
		}
		var record diaryEntryRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, entryID).First(&record).Error; err != nil {
			return err
		}
		if record.Revision != expectedRevision {
			return diaryapp.ErrDiaryConflict
		}
		if err := tx.Model(&postRecord{}).Where("source_diary_owner_id = ? AND source_diary_id = ?", ownerID, entryID).Updates(map[string]any{"source_diary_owner_id": nil, "source_diary_id": nil}).Error; err != nil {
			return err
		}
		if err := tx.Where("owner_id = ? AND entry_id = ?", ownerID, entryID).Delete(&diaryEntryMediaRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("owner_id = ? AND id = ?", ownerID, entryID).Delete(&diaryEntryRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&diaryEntryDeletionRecord{OwnerID: ownerID, ID: entryID, DeletedAt: at}).Error; err != nil {
			return err
		}
		return appendSyncChanges(tx, ownerID, at, syncDelete("diary_entry", entryID, nil))
	})
	return diaryWriteError(err)
}

func (r *DiaryRepository) CalendarMonth(ctx context.Context, ownerID, month string) (diaryapp.CalendarMonth, error) {
	start, _ := time.Parse("2006-01", month)
	end := start.AddDate(0, 1, 0)
	type countRow struct {
		LocalDate time.Time `gorm:"column:local_date"`
		Count     int       `gorm:"column:count"`
	}
	counts := make(map[string]*diaryapp.CalendarDay)
	queries := []struct {
		table string
		apply func(*diaryapp.CalendarDay, int)
		where string
	}{
		{"outfit_plans", func(day *diaryapp.CalendarDay, count int) { day.PlanCount = count }, " AND status <> 'cancelled'"},
		{"wear_events", func(day *diaryapp.CalendarDay, count int) { day.WearEventCount = count }, ""},
		{"diary_entries", func(day *diaryapp.CalendarDay, count int) { day.DiaryCount = count }, ""},
	}
	for _, query := range queries {
		var rows []countRow
		statement := "SELECT local_date, COUNT(*)::int AS count FROM " + query.table + " WHERE owner_id = ? AND local_date >= ? AND local_date < ?" + query.where + " GROUP BY local_date ORDER BY local_date"
		if err := r.database.WithContext(ctx).Raw(statement, ownerID, start, end).Scan(&rows).Error; err != nil {
			return diaryapp.CalendarMonth{}, diaryapp.ErrDiaryUnavailable
		}
		for _, row := range rows {
			date := row.LocalDate.Format("2006-01-02")
			day := counts[date]
			if day == nil {
				day = &diaryapp.CalendarDay{LocalDate: date}
				counts[date] = day
			}
			query.apply(day, row.Count)
		}
	}
	result := diaryapp.CalendarMonth{Month: month, Days: make([]diaryapp.CalendarDay, 0, len(counts))}
	for date := start; date.Before(end); date = date.AddDate(0, 0, 1) {
		if day := counts[date.Format("2006-01-02")]; day != nil {
			result.Days = append(result.Days, *day)
		}
	}
	return result, nil
}

func validateDiaryReferences(tx *gorm.DB, ownerID string, input diaryapp.DiaryEntryInput) error {
	if input.PlanID != nil {
		var count int64
		if err := tx.Model(&outfitPlanRecord{}).Where("owner_id = ? AND id = ?", ownerID, *input.PlanID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return diaryapp.ErrDiaryNotFound
		}
	}
	if input.WearEventID != nil {
		var event wearEventRecord
		if err := tx.Select("local_date").Where("owner_id = ? AND id = ?", ownerID, *input.WearEventID).First(&event).Error; err != nil {
			return err
		}
		if event.LocalDate.Format("2006-01-02") != input.LocalDate {
			return diaryapp.ErrDiaryConflict
		}
	}
	if len(input.MediaIDs) > 0 {
		var count int64
		if err := tx.Model(&mediaAssetRecord{}).Where("owner_id = ? AND id IN ? AND purpose = ? AND category = ? AND status = ?", ownerID, input.MediaIDs, mediaapp.MediaPurposeDiaryImage, mediaapp.MediaCategoryOrdinaryImage, string(mediaapp.MediaReady)).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(input.MediaIDs)) {
			return diaryapp.ErrDiaryConflict
		}
	}
	return nil
}

func readDiaryEntry(database *gorm.DB, ownerID, entryID string) (diaryapp.DiaryEntry, error) {
	var record diaryEntryRecord
	if err := database.Where("owner_id = ? AND id = ?", ownerID, entryID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return diaryapp.DiaryEntry{}, diaryapp.ErrDiaryNotFound
		}
		return diaryapp.DiaryEntry{}, diaryapp.ErrDiaryUnavailable
	}
	media, err := readDiaryMediaForEntries(database, ownerID, []diaryEntryRecord{record})
	if err != nil {
		return diaryapp.DiaryEntry{}, diaryapp.ErrDiaryUnavailable
	}
	return diaryFromRecord(record, media[record.ID]), nil
}

func readDiaryMediaForEntries(database *gorm.DB, ownerID string, entries []diaryEntryRecord) (map[string][]string, error) {
	result := make(map[string][]string, len(entries))
	if len(entries) == 0 {
		return result, nil
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.ID)
	}
	var records []diaryEntryMediaRecord
	if err := database.Where("owner_id = ? AND entry_id IN ?", ownerID, ids).Order("entry_id ASC").Order("ordinal ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	for _, record := range records {
		result[record.EntryID] = append(result[record.EntryID], record.MediaID)
	}
	return result, nil
}

func diaryMediaRecords(ownerID, entryID string, mediaIDs []string) []diaryEntryMediaRecord {
	records := make([]diaryEntryMediaRecord, 0, len(mediaIDs))
	for ordinal, mediaID := range mediaIDs {
		records = append(records, diaryEntryMediaRecord{OwnerID: ownerID, EntryID: entryID, Ordinal: ordinal, MediaID: mediaID})
	}
	return records
}

func diaryFromRecord(record diaryEntryRecord, mediaIDs []string) diaryapp.DiaryEntry {
	return diaryapp.DiaryEntry{ID: record.ID, LocalDate: record.LocalDate.Format("2006-01-02"), TimeZone: record.TimeZone, Title: record.Title, Body: record.Body, Mood: record.Mood, Occasion: record.Occasion, PlanID: record.PlanID, WearEventID: record.WearEventID, MediaIDs: append([]string(nil), mediaIDs...), Revision: record.Revision, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}

func diaryFingerprint(input diaryapp.DiaryEntryInput) string {
	encoded, _ := json.Marshal(input)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func diaryLookupError(err error) error {
	if err == nil || errors.Is(err, diaryapp.ErrDiaryNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return diaryapp.ErrDiaryNotFound
	}
	return diaryapp.ErrDiaryUnavailable
}

func diaryWriteError(err error) error {
	if err == nil {
		return nil
	}
	for _, expected := range []error{diaryapp.ErrDiaryNotFound, diaryapp.ErrDiaryConflict, diaryapp.ErrInvalidDiaryInput} {
		if errors.Is(err, expected) {
			return expected
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return diaryapp.ErrDiaryNotFound
	}
	return diaryapp.ErrDiaryUnavailable
}
