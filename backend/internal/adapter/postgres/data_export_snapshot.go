package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	exportapp "github.com/StephenQiu30/then-server/backend/internal/application/dataexport"
	"gorm.io/gorm"
)

// Every projection is a fixed allowlist. Keys, credentials, moderation reasons,
// fingerprints and private object locations never enter the archive.
var exportQueries = []struct {
	name, projection, source, predicate string
}{
	{"account", "u.id::text AS sort_key, u.id, c.email, u.display_name, u.status, u.role, (u.email_verified_at IS NOT NULL) AS email_verified, u.created_at, u.updated_at", "users u JOIN user_credentials c ON c.user_id = u.id", "u.id = ?"},
	{"profile", "p.user_id::text AS sort_key, p.handle, p.bio, p.avatar_media_id, p.revision, p.created_at, p.updated_at", "user_profiles p", "p.user_id = ?"},
	{"wardrobe", "w.id::text AS sort_key, w.id, w.name, w.category, w.availability, w.source, w.formality_band, w.warmth_band, w.rain_use, w.walking_use, w.archived_at, w.revision, w.created_at, w.updated_at", "wardrobe_items w", "w.owner_id = ?"},
	{"outfit_plans", "p.id::text AS sort_key, p.id, p.local_date, p.time_zone, p.context_summary, p.status, p.revision, p.created_at, p.updated_at", "outfit_plans p", "p.owner_id = ?"},
	{"outfit_plan_items", "i.plan_id::text || ':' || lpad(i.ordinal::text, 3, '0') AS sort_key, i.plan_id, i.ordinal, i.wardrobe_item_id, i.item_revision, i.name, i.category, i.availability, i.formality_band, i.warmth_band, i.rain_use, i.walking_use, i.redacted", "outfit_plan_items i", "i.owner_id = ?"},
	{"outfit_plan_deletions", "d.id::text AS sort_key, d.id, d.deleted_at", "outfit_plan_deletions d", "d.owner_id = ?"},
	{"wear_events", "e.id::text AS sort_key, e.id, e.local_date, e.time_zone, e.completeness, e.context_summary, e.source_plan_id, e.source_plan_revision, e.source_kind, e.revision, e.created_at, e.updated_at", "wear_events e", "e.owner_id = ?"},
	{"wear_event_items", "i.event_id::text || ':' || lpad(i.ordinal::text, 3, '0') AS sort_key, i.event_id, i.ordinal, i.wardrobe_item_id, i.item_revision, i.name, i.category, i.availability, i.formality_band, i.warmth_band, i.rain_use, i.walking_use, i.redacted", "wear_event_items i", "i.owner_id = ?"},
	{"wear_event_deletions", "d.id::text AS sort_key, d.id, d.deleted_at", "wear_event_deletions d", "d.owner_id = ?"},
	{"wear_feedback", "f.id::text AS sort_key, f.id, f.event_id, f.thermal_comfort, f.activity_comfort, f.occasion_fit, f.repeat_intent, f.issue_tags, f.note, f.revision, f.created_at, f.updated_at", "wear_feedback f", "f.owner_id = ?"},
	{"diary", "d.id::text AS sort_key, d.id, d.local_date, d.time_zone, d.title, d.body, d.mood, d.occasion, d.plan_id, d.wear_event_id, d.revision, d.created_at, d.updated_at", "diary_entries d", "d.owner_id = ?"},
	{"diary_media", "m.entry_id::text || ':' || lpad(m.ordinal::text, 3, '0') AS sort_key, m.entry_id, m.ordinal, m.media_id", "diary_entry_media m", "m.owner_id = ?"},
	{"diary_deletions", "d.id::text AS sort_key, d.id, d.deleted_at", "diary_entry_deletions d", "d.owner_id = ?"},
	{"posts", "p.id::text AS sort_key, p.id, p.state, p.revision, p.draft_version, p.pending_version, p.published_version, p.source_diary_id, p.published_at, p.withdrawn_at, p.created_at, p.updated_at", "posts p", "p.owner_id = ? AND p.state <> 'deleted'"},
	{"post_revisions", "r.post_id::text || ':' || lpad(r.version::text, 10, '0') AS sort_key, r.post_id, r.version, r.title, r.body, r.review_state, r.submitted_at, r.reviewed_at, r.created_at", "post_revisions r JOIN posts p ON p.id = r.post_id", "p.owner_id = ? AND p.state <> 'deleted'"},
	{"post_revision_media", "m.post_id::text || ':' || lpad(m.version::text, 10, '0') || ':' || lpad(m.ordinal::text, 3, '0') AS sort_key, m.post_id, m.version, m.ordinal, m.media_id", "post_revision_media m JOIN posts p ON p.id = m.post_id", "p.owner_id = ? AND p.state <> 'deleted'"},
	{"post_revision_tags", "t.post_id::text || ':' || lpad(t.version::text, 10, '0') || ':' || t.tag AS sort_key, t.post_id, t.version, t.tag", "post_revision_tags t JOIN posts p ON p.id = t.post_id", "p.owner_id = ? AND p.state <> 'deleted'"},
	{"post_likes", "l.post_id::text AS sort_key, l.post_id, l.created_at", "post_likes l", "l.user_id = ?"},
	{"post_bookmarks", "b.post_id::text AS sort_key, b.post_id, b.created_at", "post_bookmarks b", "b.user_id = ?"},
	{"follows", "f.followee_id::text AS sort_key, f.followee_id, f.created_at", "user_follows f", "f.follower_id = ?"},
	{"blocks", "b.blocked_id::text AS sort_key, b.blocked_id, b.created_at", "user_blocks b", "b.blocker_id = ?"},
	{"comments", "c.id::text AS sort_key, c.id, c.post_id, c.parent_id, c.body, c.state, c.revision, c.created_at, c.updated_at", "comments c", "c.author_id = ? AND c.state <> 'deleted'"},
	{"appeals", "a.id::text AS sort_key, a.id, a.reason, a.status, a.created_at, a.resolved_at", "moderation_appeals a", "a.appellant_id = ?"},
	{"reports", "r.id::text AS sort_key, r.id, r.post_id, r.target_type, r.comment_id, r.reason_code, r.detail, r.status, r.created_at, r.resolved_at", "content_reports r", "r.reporter_id = ?"},
	{"consents", "c.id::text AS sort_key, c.id, c.purpose, c.category, c.processor, c.region, c.policy_version, c.max_retention_hours, c.training_allowed, c.agreed_at, c.withdrawn_at", "consent_records c", "c.owner_id = ?"},
	{"adult_declarations", "d.policy_version AS sort_key, d.policy_version, d.confirmed_at, d.withdrawn_at", "self_adult_declarations d", "d.user_id = ?"},
	{"media", "m.id::text AS sort_key, m.id, m.purpose, m.category, m.content_type, m.byte_size, m.status, m.stable_reason, m.pixel_width, m.pixel_height, m.created_at, m.updated_at", "media_assets m", "m.owner_id = ?"},
	{"media_deletions", "d.id::text AS sort_key, d.id, d.media_id, d.status, d.read_revoked_at, d.completed_at, d.backup_expires_at, d.created_at, d.updated_at", "deletion_requests d", "d.owner_id = ?"},
	{"exports", "e.id::text AS sort_key, e.id, e.mode, e.status, e.created_at, e.completed_at, e.expires_at, e.revoked_at", "data_exports e", "e.owner_id = ?"},
}

func (r *DataExportRepository) Snapshot(ctx context.Context, ownerID, mode string) ([]exportapp.Dataset, []exportapp.MediaSource, error) {
	var datasets []exportapp.Dataset
	var media []exportapp.MediaSource
	var totalBytes int
	err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userRecord
		if err := tx.Select("id", "status").Where("id = ? AND status = 'active'", ownerID).Take(&user).Error; err != nil {
			return err
		}
		for _, query := range exportQueries {
			// SQL fragments above are compile-time constants; ownerID is always bound.
			statement := fmt.Sprintf("SELECT COALESCE(jsonb_agg(to_jsonb(x) - 'sort_key' ORDER BY x.sort_key), '[]'::jsonb) AS payload FROM (SELECT %s FROM %s WHERE %s ORDER BY sort_key LIMIT 10001) x", query.projection, query.source, query.predicate)
			var payload string
			if err := tx.Raw(statement, ownerID).Scan(&payload).Error; err != nil {
				return err
			}
			var rows []json.RawMessage
			if err := json.Unmarshal([]byte(payload), &rows); err != nil {
				return err
			}
			totalBytes += len(payload)
			if len(rows) > 10000 || totalBytes > 32<<20 {
				return exportapp.ErrTooLarge
			}
			metadata, err := tx.Raw("SELECT " + query.projection + " FROM " + query.source + " WHERE false").Rows()
			if err != nil {
				return err
			}
			fields, err := metadata.Columns()
			metadata.Close()
			if err != nil || len(fields) < 2 || fields[0] != "sort_key" {
				return exportapp.ErrUnavailable
			}
			datasets = append(datasets, exportapp.Dataset{Name: query.name, Rows: json.RawMessage(payload), Count: len(rows), Fields: fields[1:]})
		}
		if mode == exportapp.ModeWithMedia {
			var records []mediaAssetRecord
			if err := tx.Select("id", "raw_object_key", "object_version_id", "byte_size", "sha256", "source_deleted_at").Where("owner_id = ? AND status = 'ready' AND purpose IN ?", ownerID, []string{"avatar_source_preparation", "diary_image", "community_publish", "profile_avatar"}).Order("id ASC").Limit(10001).Find(&records).Error; err != nil {
				return err
			}
			if len(records) > 10000 {
				return exportapp.ErrTooLarge
			}
			for _, record := range records {
				media = append(media, exportapp.MediaSource{ID: record.ID, ObjectKey: record.RawObjectKey, ObjectVersion: record.ObjectVersionID, ByteSize: record.ByteSize, SHA256: record.SHA256, SourceRemoved: record.SourceDeletedAt != nil})
			}
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, nil, exportapp.ErrUnavailable
	}
	return datasets, media, nil
}
