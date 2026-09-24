package generation

import (
	"errors"
	"time"
)

var ErrInvalidGenerationOutput = errors.New("invalid generation output")

const (
	OutputContentTypeJPEG = "image/jpeg"
	OutputContentTypeGLB  = "model/gltf-binary"
)

// OutputFact is the verified object-store fact supplied by the worker after
// it has downloaded and validated a provider result. Object keys and provider
// URLs stay in adapters; only the immutable version and content metadata cross
// into the application domain.
type OutputFact struct {
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

// NewOutputAsset binds a validated object version to the task that produced
// it. A task must be in validating state so the caller can commit this asset
// and the succeeding task transition in one short database transaction.
func NewOutputAsset(id string, task Task, fact OutputFact, at time.Time) (OutputAsset, error) {
	if !task.validTaskCoreFacts() || !validID(id) || !validID(task.ID) || !validID(task.OwnerID) || !validID(task.LookID) ||
		task.LookRevision < 1 || !task.Purpose.valid() || task.Status != StatusValidating ||
		!validOutputFact(task.Purpose, fact) || at.IsZero() {
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
		ObjectVersionID: fact.ObjectVersionID,
		PublishedAt:     at,
	}, nil
}

func validOutputFact(purpose Purpose, fact OutputFact) bool {
	if fact.ByteSize < 1 || !sha256Pattern.MatchString(fact.SHA256) || !validToken(fact.ObjectVersionID, 160) {
		return false
	}
	switch purpose {
	case PurposeImage:
		return fact.ContentType == OutputContentTypeJPEG
	case PurposeModel:
		return fact.ContentType == OutputContentTypeGLB
	default:
		return false
	}
}
