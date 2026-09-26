package generation

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidGenerationOutput = errors.New("invalid generation output")

// OutputExpired is true at the configured publication deadline. Zero means
// the caller has no retention policy, as in provider-neutral unit tests.
func OutputExpired(publishedAt time.Time, retention time.Duration, at time.Time) bool {
	return retention > 0 && !at.Before(publishedAt.Add(retention))
}

const (
	OutputContentTypeJPEG               = "image/jpeg"
	OutputContentTypeGLB                = "model/gltf-binary"
	MaxGenerationImageOutputBytes int64 = 12 * 1024 * 1024
	MaxGenerationModelOutputBytes int64 = 10 * 1024 * 1024
)

// OutputFact is the verified private object-store fact supplied by a worker.
// The private key is persisted for deletion, but is never part of an HTTP DTO.
type OutputFact struct {
	ObjectKey       string
	ContentType     string
	ByteSize        int64
	SHA256          string
	ObjectVersionID string
}

// OutputLineage keeps the minimum private ancestry needed to prevent a late
// result from being attached to another Look revision or owner. Model output
// additionally records the exact confirmed image asset and digest it used.
type OutputLineage struct {
	TaskID             string
	OwnerID            string
	LookID             string
	LookRevision       int
	Purpose            Purpose
	SourceImageAssetID string
	SourceImageSHA256  string
}

// OutputAsset is an immutable, ready result published by a generation task.
// Deletion and visibility transitions are separate lifecycle facts owned by
// the later DELETE-01 repository; this value only records a validated result.
type OutputAsset struct {
	ID              string
	Lineage         OutputLineage
	ContentType     string
	ByteSize        int64
	SHA256          string
	ObjectKey       string
	ObjectVersionID string
	PublishedAt     time.Time
}

// ValidateFor checks a persisted output against its task lineage. The task
// state is supplied separately because output rows are immutable facts while
// the task moves through validating and terminal states.
func (a OutputAsset) ValidateFor(task Task) error {
	if !task.validTaskFacts() || !validOutputAsset(task, a) {
		return ErrInvalidGenerationOutput
	}
	return nil
}

// ValidatePersistedFor permits a blank key on records created before private
// output keys were tracked. Such records remain readable but cannot be newly
// published or physically cleaned without a verified key.
func (a OutputAsset) ValidatePersistedFor(task Task) error {
	if !task.validTaskFacts() || !validPersistedOutputAsset(task, a) {
		return ErrInvalidGenerationOutput
	}
	return nil
}

// NewOutputAsset binds a validated object version to the task that produced
// it. A task must be in validating state so the caller can commit this asset
// and the succeeding task transition in one short database transaction.
func NewOutputAsset(id string, task Task, fact OutputFact, at time.Time) (OutputAsset, error) {
	if !task.validTaskCoreFacts() || !validID(id) || !validID(task.ID) || !validID(task.OwnerID) || !validID(task.LookID) ||
		task.LookRevision < 1 || !task.Purpose.valid() || task.Status != StatusValidating || task.AccessRevokedAt != nil ||
		!validOutputFact(task.Purpose, fact) || !validOutputObjectKey(task, fact.ObjectKey) || at.IsZero() {
		return OutputAsset{}, ErrInvalidGenerationOutput
	}
	if !task.UpdatedAt.IsZero() && at.Before(task.UpdatedAt) {
		return OutputAsset{}, ErrInvalidGenerationOutput
	}
	at = at.UTC()
	lineage := OutputLineage{
		TaskID:       task.ID,
		OwnerID:      task.OwnerID,
		LookID:       task.LookID,
		LookRevision: task.LookRevision,
		Purpose:      task.Purpose,
	}
	if task.Purpose == PurposeModel {
		lineage.SourceImageAssetID = task.Inputs.ImageAssetID
		lineage.SourceImageSHA256 = task.Inputs.ImageSHA256
	}
	return OutputAsset{
		ID:              id,
		Lineage:         lineage,
		ContentType:     fact.ContentType,
		ByteSize:        fact.ByteSize,
		SHA256:          fact.SHA256,
		ObjectKey:       fact.ObjectKey,
		ObjectVersionID: fact.ObjectVersionID,
		PublishedAt:     at,
	}, nil
}

// OutputObjectKey returns the task-bound private destination for a validated
// generation result. IDs must be single safe path segments before composing
// the object key.
func OutputObjectKey(task Task) (string, error) {
	if !validObjectKeySegment(task.OwnerID) || !validObjectKeySegment(task.ID) || !task.Purpose.valid() {
		return "", ErrInvalidGenerationOutput
	}
	extension := "jpg"
	if task.Purpose == PurposeModel {
		extension = "glb"
	}
	return "owners/" + task.OwnerID + "/generation/" + task.ID + "/output." + extension, nil
}

func validOutputObjectKey(task Task, key string) bool {
	expected, err := OutputObjectKey(task)
	return err == nil && key == expected
}

func validGenerationOutputObjectKey(key string) bool {
	parts := strings.Split(key, "/")
	return len(parts) == 5 && parts[0] == "owners" && validObjectKeySegment(parts[1]) &&
		parts[2] == "generation" && validObjectKeySegment(parts[3]) &&
		(parts[4] == "output.jpg" || parts[4] == "output.glb")
}

func validGenerationOutputObjectKeyForTask(ownerID, taskID, key string) bool {
	if !validGenerationOutputObjectKey(key) {
		return false
	}
	parts := strings.Split(key, "/")
	return parts[1] == ownerID && parts[3] == taskID
}

func validObjectKeySegment(value string) bool {
	if !validID(value) {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && char != '-' && char != '_' {
			return false
		}
	}
	return true
}

func validOutputFact(purpose Purpose, fact OutputFact) bool {
	if fact.ByteSize < 1 || !sha256Pattern.MatchString(fact.SHA256) || !validToken(fact.ObjectVersionID, 160) {
		return false
	}
	switch purpose {
	case PurposeImage:
		return fact.ContentType == OutputContentTypeJPEG && fact.ByteSize <= MaxGenerationImageOutputBytes
	case PurposeModel:
		return fact.ContentType == OutputContentTypeGLB && fact.ByteSize <= MaxGenerationModelOutputBytes
	default:
		return false
	}
}
