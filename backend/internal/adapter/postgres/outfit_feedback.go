package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	feedbackapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitfeedback"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OutfitFeedbackRepository struct{ database *gorm.DB }

func NewOutfitFeedbackRepository(database *gorm.DB) *OutfitFeedbackRepository {
	return &OutfitFeedbackRepository{database: database}
}

type wearFeedbackRecord struct {
	OwnerID         string    `gorm:"column:owner_id;type:uuid;primaryKey"`
	EventID         string    `gorm:"column:event_id;type:uuid;primaryKey"`
	ID              string    `gorm:"column:id;type:uuid;not null;uniqueIndex:wear_feedback_id_unique"`
	ThermalComfort  *string   `gorm:"column:thermal_comfort;type:text;check:wear_feedback_thermal_check,thermal_comfort IN ('cold','comfortable','hot')"`
	ActivityComfort *string   `gorm:"column:activity_comfort;type:text;check:wear_feedback_activity_check,activity_comfort IN ('uncomfortable','okay','comfortable')"`
	OccasionFit     *string   `gorm:"column:occasion_fit;type:text;check:wear_feedback_occasion_check,occasion_fit IN ('tooCasual','right','tooFormal')"`
	RepeatIntent    *string   `gorm:"column:repeat_intent;type:text;check:wear_feedback_repeat_check,repeat_intent IN ('yes','unsure','no')"`
	IssueTags       []string  `gorm:"column:issue_tags;type:jsonb;serializer:json;not null;check:wear_feedback_content_check,jsonb_typeof(issue_tags) = 'array' AND jsonb_array_length(issue_tags) <= 5 AND (thermal_comfort IS NOT NULL OR activity_comfort IS NOT NULL OR occasion_fit IS NOT NULL OR repeat_intent IS NOT NULL OR jsonb_array_length(issue_tags) > 0 OR note IS NOT NULL)"`
	Note            *string   `gorm:"column:note;type:text;check:wear_feedback_note_check,note IS NULL OR (note = btrim(note) AND char_length(note) BETWEEN 1 AND 240)"`
	Revision        int       `gorm:"column:revision;not null;check:wear_feedback_revision_check,revision >= 1"`
	CreatedAt       time.Time `gorm:"column:created_at;type:timestamptz;not null"`
	UpdatedAt       time.Time `gorm:"column:updated_at;type:timestamptz;not null;check:wear_feedback_timestamps_check,updated_at >= created_at"`
}

func (wearFeedbackRecord) TableName() string { return "wear_feedback" }

type wearFeedbackMutationRecord struct {
	OwnerID     string `gorm:"column:owner_id;type:uuid;primaryKey;uniqueIndex:wear_feedback_mutations_owner_id_unique,priority:1"`
	EventID     string `gorm:"column:event_id;type:uuid;primaryKey"`
	ID          string `gorm:"column:id;type:uuid;primaryKey;uniqueIndex:wear_feedback_mutations_owner_id_unique,priority:2"`
	FeedbackID  string `gorm:"column:feedback_id;type:uuid;not null"`
	Operation   string `gorm:"column:operation;type:text;not null;check:wear_feedback_mutations_operation_check,operation IN ('save','delete')"`
	Fingerprint string `gorm:"column:fingerprint;type:char(64);not null;check:wear_feedback_mutations_fingerprint_check,char_length(fingerprint) = 64"`
}

func (wearFeedbackMutationRecord) TableName() string { return "wear_feedback_mutations" }

func feedbackFingerprint(feedbackID, operation string, expected *int, input *feedbackapp.Input) string {
	data, _ := json.Marshal(struct {
		ID        string
		Operation string
		Expected  *int
		Input     *feedbackapp.Input
	}{feedbackID, operation, expected, input})
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func feedbackFromRecord(record wearFeedbackRecord) feedbackapp.Feedback {
	return feedbackapp.Feedback{ID: record.ID, WearEventID: record.EventID, Input: feedbackapp.Input{ThermalComfort: record.ThermalComfort, ActivityComfort: record.ActivityComfort, OccasionFit: record.OccasionFit, RepeatIntent: record.RepeatIntent, IssueTags: record.IssueTags, Note: record.Note}, Revision: record.Revision, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}

func (r *OutfitFeedbackRepository) Get(ctx context.Context, ownerID, eventID string) (feedbackapp.Feedback, error) {
	var event wearEventRecord
	if err := r.database.WithContext(ctx).Select("id").Where("owner_id = ? AND id = ?", ownerID, eventID).First(&event).Error; err != nil {
		return feedbackapp.Feedback{}, feedbackLookupError(err)
	}
	var record wearFeedbackRecord
	if err := r.database.WithContext(ctx).Where("owner_id = ? AND event_id = ?", ownerID, eventID).First(&record).Error; err != nil {
		return feedbackapp.Feedback{}, feedbackLookupError(err)
	}
	return feedbackFromRecord(record), nil
}

func (r *OutfitFeedbackRepository) Save(ctx context.Context, ownerID, eventID, feedbackID, mutationID string, expected *int, input feedbackapp.Input, at time.Time) (feedbackapp.Feedback, error) {
	var result feedbackapp.Feedback
	fingerprint := feedbackFingerprint(feedbackID, "save", expected, &input)
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var event wearEventRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, eventID).First(&event).Error; err != nil {
			return err
		}
		var receipt wearFeedbackMutationRecord
		err := tx.Where("owner_id = ? AND event_id = ? AND id = ?", ownerID, eventID, mutationID).First(&receipt).Error
		if err == nil {
			if receipt.FeedbackID != feedbackID || receipt.Operation != "save" || receipt.Fingerprint != fingerprint {
				return feedbackapp.ErrConflict
			}
			var current wearFeedbackRecord
			if err := tx.Where("owner_id = ? AND event_id = ?", ownerID, eventID).First(&current).Error; err != nil {
				return err
			}
			if current.ID != feedbackID {
				return feedbackapp.ErrConflict
			}
			result = feedbackFromRecord(current)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var current wearFeedbackRecord
		err = tx.Where("owner_id = ? AND event_id = ?", ownerID, eventID).First(&current).Error
		if err == nil {
			if expected == nil || current.ID != feedbackID || current.Revision != *expected {
				return feedbackapp.ErrConflict
			}
			current.ThermalComfort, current.ActivityComfort, current.OccasionFit, current.RepeatIntent = input.ThermalComfort, input.ActivityComfort, input.OccasionFit, input.RepeatIntent
			current.IssueTags, current.Note, current.Revision, current.UpdatedAt = input.IssueTags, input.Note, current.Revision+1, maxTime(at, current.UpdatedAt)
			if err := tx.Model(&wearFeedbackRecord{}).Where("owner_id = ? AND event_id = ?", ownerID, eventID).Select("thermal_comfort", "activity_comfort", "occasion_fit", "repeat_intent", "issue_tags", "note", "revision", "updated_at").Updates(current).Error; err != nil {
				return err
			}
		} else {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if expected != nil {
				return feedbackapp.ErrNotFound
			}
			current = wearFeedbackRecord{OwnerID: ownerID, EventID: eventID, ID: feedbackID, ThermalComfort: input.ThermalComfort, ActivityComfort: input.ActivityComfort, OccasionFit: input.OccasionFit, RepeatIntent: input.RepeatIntent, IssueTags: input.IssueTags, Note: input.Note, Revision: 1, CreatedAt: at, UpdatedAt: at}
			if err := tx.Create(&current).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&wearFeedbackMutationRecord{OwnerID: ownerID, EventID: eventID, ID: mutationID, FeedbackID: feedbackID, Operation: "save", Fingerprint: fingerprint}).Error; err != nil {
			return err
		}
		result = feedbackFromRecord(current)
		return nil
	})
	if err != nil {
		return feedbackapp.Feedback{}, feedbackWriteError(err)
	}
	return result, nil
}

func (r *OutfitFeedbackRepository) Delete(ctx context.Context, ownerID, eventID, feedbackID, mutationID string, expected int) error {
	fingerprint := feedbackFingerprint(feedbackID, "delete", &expected, nil)
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var event wearEventRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND id = ?", ownerID, eventID).First(&event).Error; err != nil {
			return err
		}
		var receipt wearFeedbackMutationRecord
		err := tx.Where("owner_id = ? AND event_id = ? AND id = ?", ownerID, eventID, mutationID).First(&receipt).Error
		if err == nil {
			if receipt.FeedbackID != feedbackID || receipt.Operation != "delete" || receipt.Fingerprint != fingerprint {
				return feedbackapp.ErrConflict
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var current wearFeedbackRecord
		if err := tx.Where("owner_id = ? AND event_id = ?", ownerID, eventID).First(&current).Error; err != nil {
			return err
		}
		if current.ID != feedbackID || current.Revision != expected {
			return feedbackapp.ErrConflict
		}
		if err := tx.Where("owner_id = ? AND event_id = ?", ownerID, eventID).Delete(&wearFeedbackRecord{}).Error; err != nil {
			return err
		}
		return tx.Create(&wearFeedbackMutationRecord{OwnerID: ownerID, EventID: eventID, ID: mutationID, FeedbackID: feedbackID, Operation: "delete", Fingerprint: fingerprint}).Error
	})
	return feedbackWriteError(err)
}

func (r *OutfitFeedbackRepository) Statistics(ctx context.Context, ownerID, from, to string) (feedbackapp.Statistics, error) {
	result := feedbackapp.Statistics{From: from, To: to, ItemUses: []feedbackapp.ItemUse{}}
	base := r.database.WithContext(ctx).Model(&wearEventRecord{}).Where("owner_id = ? AND local_date BETWEEN ? AND ?", ownerID, from, to)
	var totals struct {
		WearEvents int
		WearDays   int
	}
	if err := base.Select("COUNT(*) AS wear_events, COUNT(DISTINCT local_date) AS wear_days").Scan(&totals).Error; err != nil {
		return feedbackapp.Statistics{}, err
	}
	result.WearEvents, result.WearDays = totals.WearEvents, totals.WearDays
	var uses []struct {
		ItemID string
		Count  int
	}
	err := r.database.WithContext(ctx).Table("wear_event_items AS i").Select("i.wardrobe_item_id AS item_id, COUNT(*) AS count").Joins("JOIN wear_events AS e ON e.owner_id = i.owner_id AND e.id = i.event_id").Where("e.owner_id = ? AND e.local_date BETWEEN ? AND ? AND i.wardrobe_item_id IS NOT NULL", ownerID, from, to).Group("i.wardrobe_item_id").Order("i.wardrobe_item_id").Scan(&uses).Error
	if err != nil {
		return feedbackapp.Statistics{}, err
	}
	for _, use := range uses {
		result.ItemUses = append(result.ItemUses, feedbackapp.ItemUse{ItemID: use.ItemID, Count: use.Count})
	}
	return result, nil
}

func feedbackLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return feedbackapp.ErrNotFound
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		return feedbackapp.ErrConflict
	}
	return err
}
func feedbackWriteError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return feedbackapp.ErrNotFound
	}
	return err
}
func maxTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return b
	}
	return a
}
