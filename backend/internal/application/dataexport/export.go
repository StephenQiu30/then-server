// Package dataexport owns private account export jobs and their archive contract.
package dataexport

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sort"
	"strconv"
	"time"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	"github.com/google/uuid"
)

const (
	ModeStructured    = "structured"
	StatusPreparing   = "preparing"
	StatusReady       = "ready"
	StatusFailed      = "failed"
	StatusExpired     = "expired"
	StatusRevoked     = "revoked"
	Lifetime          = 24 * time.Hour
	MetadataRetention = 7 * 24 * time.Hour
	maxArchiveBytes   = 32 << 20
)

var (
	ErrInvalidMode    = errors.New("invalid export mode")
	ErrNotFound       = errors.New("export not found")
	ErrNotReady       = errors.New("export not ready")
	ErrUnavailable    = errors.New("export unavailable")
	ErrTooLarge       = errors.New("export too large")
	ErrInvalidDataset = errors.New("invalid export dataset")
)

type Job struct {
	ID            string
	OwnerID       string
	Mode          string
	Status        string
	Counts        map[string]int
	Omissions     []string
	ObjectKey     string
	ObjectVersion string
	CreatedAt     time.Time
	CompletedAt   *time.Time
	ExpiresAt     time.Time
	RevokedAt     *time.Time
	Attempts      int
}

type Dataset struct {
	Name   string
	Rows   json.RawMessage
	Count  int
	Fields []string
}

type Repository interface {
	Create(context.Context, Job) error
	Get(context.Context, string, string) (Job, error)
	Revoke(context.Context, string, string, time.Time) error
	Claim(context.Context, time.Time) (Job, bool, error)
	Snapshot(context.Context, string) ([]Dataset, error)
	Complete(context.Context, Job, map[string]int, []string, string, string, time.Time) (bool, error)
	Fail(context.Context, Job, time.Time) error
	ClaimCleanup(context.Context, time.Time) (Job, bool, error)
	FinishCleanup(context.Context, string, time.Time) error
	Purge(context.Context, time.Time) error
}

type ObjectStore interface {
	PutArchive(context.Context, string, io.Reader, int64) (string, error)
	OpenArchive(context.Context, string, string) (io.ReadCloser, error)
	DeleteArchive(context.Context, string) error
}

type Accounts interface {
	CurrentUser(context.Context, string) (accountapp.User, error)
	Reauthenticate(context.Context, string, string) (accountapp.User, error)
}

type Service struct {
	accounts   Accounts
	repository Repository
	objects    ObjectStore
	now        func() time.Time
}

func New(accounts Accounts, repository Repository, objects ObjectStore) (*Service, error) {
	if accounts == nil || repository == nil || objects == nil {
		return nil, ErrUnavailable
	}
	return &Service{accounts: accounts, repository: repository, objects: objects, now: time.Now}, nil
}

func (s *Service) Create(ctx context.Context, token, password, mode string) (Job, error) {
	if mode != ModeStructured {
		return Job{}, ErrInvalidMode
	}
	user, err := s.accounts.Reauthenticate(ctx, token, password)
	if err != nil {
		return Job{}, err
	}
	now := s.now().UTC()
	job := Job{ID: uuid.NewString(), OwnerID: user.ID, Mode: mode, Status: StatusPreparing, Counts: map[string]int{}, Omissions: []string{}, CreatedAt: now, ExpiresAt: now.Add(Lifetime)}
	if err := s.repository.Create(ctx, job); err != nil {
		return Job{}, err
	}
	return job, nil
}

func (s *Service) Get(ctx context.Context, token, id string) (Job, error) {
	user, err := s.accounts.CurrentUser(ctx, token)
	if err != nil {
		return Job{}, err
	}
	job, err := s.repository.Get(ctx, user.ID, id)
	if err != nil {
		return Job{}, err
	}
	if !s.now().Before(job.ExpiresAt) && job.Status != StatusRevoked {
		job.Status = StatusExpired
	}
	return job, nil
}

func (s *Service) Open(ctx context.Context, token, id string) (io.ReadCloser, error) {
	job, err := s.Get(ctx, token, id)
	if err != nil {
		return nil, err
	}
	if job.Status != StatusReady || job.ObjectKey == "" || job.ObjectVersion == "" {
		return nil, ErrNotReady
	}
	reader, err := s.objects.OpenArchive(ctx, job.ObjectKey, job.ObjectVersion)
	if err != nil {
		return nil, ErrUnavailable
	}
	return reader, nil
}

func (s *Service) Revoke(ctx context.Context, token, id string) error {
	user, err := s.accounts.CurrentUser(ctx, token)
	if err != nil {
		return err
	}
	return s.repository.Revoke(ctx, user.ID, id, s.now().UTC())
}

func (s *Service) ProcessNext(ctx context.Context) (bool, error) {
	job, found, err := s.repository.Claim(ctx, s.now().UTC())
	if err != nil || !found {
		return found, err
	}
	work, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	datasets, err := s.repository.Snapshot(work, job.OwnerID)
	if err != nil {
		return true, s.repository.Fail(ctx, job, s.now().UTC())
	}
	archive, counts, omissions, err := BuildArchive(datasets, s.now().UTC())
	if err != nil {
		return true, s.repository.Fail(ctx, job, s.now().UTC())
	}
	key := archiveKey(job.ID, job.Attempts)
	version, err := s.objects.PutArchive(work, key, bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		_ = s.objects.DeleteArchive(ctx, key)
		return true, s.repository.Fail(ctx, job, s.now().UTC())
	}
	accepted, err := s.repository.Complete(work, job, counts, omissions, key, version, s.now().UTC())
	if err != nil || !accepted {
		if cleanupErr := s.objects.DeleteArchive(ctx, key); cleanupErr != nil {
			return true, ErrUnavailable
		}
	}
	return true, err
}

func (s *Service) CleanupNext(ctx context.Context) (bool, error) {
	job, found, err := s.repository.ClaimCleanup(ctx, s.now().UTC())
	if err != nil || !found {
		return found, err
	}
	for attempt := 1; attempt <= job.Attempts; attempt++ {
		if err := s.objects.DeleteArchive(ctx, archiveKey(job.ID, attempt)); err != nil {
			return true, ErrUnavailable
		}
	}
	return true, s.repository.FinishCleanup(ctx, job.ID, s.now().UTC())
}

func archiveKey(id string, attempt int) string {
	return "exports/" + id + "/" + strconv.Itoa(attempt) + ".zip"
}

func (s *Service) Run(ctx context.Context, log *slog.Logger) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var nextPurge time.Time
	for {
		if now := s.now().UTC(); !now.Before(nextPurge) {
			if err := s.repository.Purge(ctx, now.Add(-MetadataRetention)); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				log.WarnContext(ctx, "data_export_metadata_retry")
			} else {
				nextPurge = now.Add(time.Hour)
			}
		}
		if _, err := s.CleanupNext(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.WarnContext(ctx, "data_export_cleanup_retry")
		}
		if _, err := s.ProcessNext(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.WarnContext(ctx, "data_export_processing_retry")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

type manifestFile struct {
	Name   string   `json:"name"`
	Count  int      `json:"count"`
	SHA256 string   `json:"sha256"`
	Fields []string `json:"fields"`
}

func BuildArchive(datasets []Dataset, at time.Time) ([]byte, map[string]int, []string, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	counts := make(map[string]int, len(datasets))
	files := make([]manifestFile, 0, len(datasets))
	seen := make(map[string]bool, len(datasets))
	sort.Slice(datasets, func(i, j int) bool { return datasets[i].Name < datasets[j].Name })
	for _, dataset := range datasets {
		trimmed := bytes.TrimSpace(dataset.Rows)
		if !validDatasetName(dataset.Name) || seen[dataset.Name] || dataset.Count < 0 || !json.Valid(trimmed) || len(trimmed) == 0 || trimmed[0] != '[' {
			writer.Close()
			return nil, nil, nil, ErrInvalidDataset
		}
		seen[dataset.Name] = true
		if len(dataset.Rows) > maxArchiveBytes || len(buffer.Bytes()) > maxArchiveBytes {
			writer.Close()
			return nil, nil, nil, ErrTooLarge
		}
		name := dataset.Name + ".json"
		entry, err := writer.Create(name)
		if err != nil {
			return nil, nil, nil, ErrUnavailable
		}
		if _, err := entry.Write(dataset.Rows); err != nil {
			return nil, nil, nil, ErrUnavailable
		}
		if len(buffer.Bytes()) > maxArchiveBytes {
			writer.Close()
			return nil, nil, nil, ErrTooLarge
		}
		digest := sha256.Sum256(dataset.Rows)
		files = append(files, manifestFile{Name: name, Count: dataset.Count, SHA256: hex.EncodeToString(digest[:]), Fields: dataset.Fields})
		counts[dataset.Name] = dataset.Count
	}
	omissions := []string{"device_only_data", "cloud_generation_not_enabled", "media_requires_with_media", "credentials_sessions_and_mail_challenges_excluded", "internal_moderation_and_queue_records_excluded", "third_party_private_content_excluded"}
	manifest, err := json.Marshal(struct {
		SchemaVersion int               `json:"schema_version"`
		ExportedAt    time.Time         `json:"exported_at"`
		FieldNotes    map[string]string `json:"field_notes"`
		Files         []manifestFile    `json:"files"`
		Omissions     []string          `json:"omissions"`
	}{1, at.UTC(), map[string]string{
		"id":           "Stable record UUID.",
		"*_id":         "UUID reference to another record; the referenced record may no longer be available.",
		"*_at":         "UTC RFC3339 timestamp, or null when the event has not occurred.",
		"local_date":   "Civil date interpreted using that record's time_zone.",
		"time_zone":    "Original IANA time zone; it is not replaced by the export machine's zone.",
		"revision":     "Record version at the export snapshot.",
		"redacted":     "True when historical item details were removed by a deletion rule.",
		"status/state": "State at the export snapshot, not a promise about later changes.",
	}, files, omissions})
	if err != nil {
		return nil, nil, nil, ErrUnavailable
	}
	entry, err := writer.Create("manifest.json")
	if err != nil {
		return nil, nil, nil, ErrUnavailable
	}
	if _, err := entry.Write(manifest); err != nil {
		writer.Close()
		return nil, nil, nil, ErrUnavailable
	}
	if err := writer.Close(); err != nil {
		return nil, nil, nil, ErrUnavailable
	}
	if buffer.Len() > maxArchiveBytes {
		return nil, nil, nil, ErrTooLarge
	}
	return buffer.Bytes(), counts, omissions, nil
}

func validDatasetName(name string) bool {
	switch name {
	case "account", "profile", "wardrobe", "outfit_plans", "outfit_plan_items", "wear_events", "wear_event_items", "wear_feedback", "diary", "diary_media", "posts", "post_revisions", "post_revision_media", "post_revision_tags", "post_likes", "post_bookmarks", "follows", "blocks", "comments", "appeals", "reports", "consents", "media", "media_deletions", "adult_declarations", "outfit_plan_deletions", "wear_event_deletions", "diary_deletions", "exports":
		return true
	default:
		return false
	}
}
