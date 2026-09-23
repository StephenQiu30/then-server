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
	"testing"
	"time"
)

func TestBuildArchiveManifestAndFixedEntries(t *testing.T) {
	at := time.Date(2026, 9, 23, 6, 0, 0, 0, time.UTC)
	rows := json.RawMessage(`[{"id":"synthetic","time_zone":"Asia/Shanghai"}]`)
	archive, counts, omissions, err := BuildArchive([]Dataset{{Name: "wardrobe", Rows: rows, Count: 1}}, at)
	if err != nil {
		t.Fatal(err)
	}
	if counts["wardrobe"] != 1 || len(omissions) == 0 {
		t.Fatal("manifest scope missing")
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 2 || reader.File[0].Name != "wardrobe.json" || reader.File[1].Name != "manifest.json" {
		t.Fatal("unexpected ZIP entries")
	}
	content, err := reader.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(content)
	content.Close()
	if err != nil || !bytes.Equal(data, rows) {
		t.Fatal("structured payload changed")
	}
	content, err = reader.File[1].Open()
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := io.ReadAll(content)
	content.Close()
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		SchemaVersion int            `json:"schema_version"`
		Files         []manifestFile `json:"files"`
		Omissions     []string       `json:"omissions"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(rows)
	if manifest.SchemaVersion != 1 || len(manifest.Files) != 1 || manifest.Files[0].SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatal("manifest digest mismatch")
	}
}

type failingArchiveRepository struct {
	Repository
	failed bool
}

func (r *failingArchiveRepository) Claim(context.Context, time.Time) (Job, bool, error) {
	return Job{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400a11", OwnerID: "owner", Mode: ModeStructured, Status: StatusPreparing, Attempts: 1}, true, nil
}
func (r *failingArchiveRepository) Snapshot(context.Context, string) ([]Dataset, error) {
	return []Dataset{{Name: "account", Rows: json.RawMessage(`[]`), Count: 0}}, nil
}
func (r *failingArchiveRepository) Fail(context.Context, Job, time.Time) error {
	r.failed = true
	return nil
}

type failingArchiveStore struct {
	ObjectStore
	deleted bool
}

func (s *failingArchiveStore) PutArchive(context.Context, string, io.Reader, int64) (string, error) {
	return "", ErrUnavailable
}
func (s *failingArchiveStore) DeleteArchive(context.Context, string) error {
	s.deleted = true
	return nil
}

func TestArchiveWriteFailureSchedulesRetryAndCleanup(t *testing.T) {
	repository := &failingArchiveRepository{}
	objects := &failingArchiveStore{}
	service := &Service{repository: repository, objects: objects, now: time.Now}
	claimed, err := service.ProcessNext(context.Background())
	if err != nil || !claimed || !repository.failed || !objects.deleted {
		t.Fatal("failed object write did not schedule retry and cleanup")
	}
}

func TestBuildArchiveRejectsUnapprovedNames(t *testing.T) {
	_, _, _, err := BuildArchive([]Dataset{{Name: "../credentials", Rows: json.RawMessage(`[]`), Count: 0}}, time.Now())
	if !errors.Is(err, ErrInvalidDataset) {
		t.Fatal("unapproved ZIP entry was accepted")
	}
}

func TestBuildArchiveRejectsDuplicateOrNonArrayData(t *testing.T) {
	for _, datasets := range [][]Dataset{
		{{Name: "account", Rows: json.RawMessage(`[]`)}, {Name: "account", Rows: json.RawMessage(`[]`)}},
		{{Name: "account", Rows: json.RawMessage(`{"id":"synthetic"}`)}},
	} {
		if _, _, _, err := BuildArchive(datasets, time.Now()); !errors.Is(err, ErrInvalidDataset) {
			t.Fatal("invalid archive structure accepted")
		}
	}
}
