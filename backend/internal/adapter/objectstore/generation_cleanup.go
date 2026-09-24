package objectstore

import (
	"context"
	"errors"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
)

// GenerationCleanupExecutor deletes private generation object versions. The
// provider deletion path stays unavailable until an approved provider adapter
// is implemented; in particular, this type never sends a remote request.
type GenerationCleanupExecutor struct {
	store *Store
}

func NewGenerationCleanupExecutor(store *Store) (*GenerationCleanupExecutor, error) {
	if store == nil || store.client == nil {
		return nil, errors.New("generation cleanup object store unavailable")
	}
	return &GenerationCleanupExecutor{store: store}, nil
}

func (e *GenerationCleanupExecutor) DeleteObject(ctx context.Context, target generationapp.CleanupTarget) error {
	if e == nil || e.store == nil || target.Kind != generationapp.CleanupTargetObject || target.ObjectKey == "" || target.ObjectVersionID == "" {
		return &generationapp.CleanupTargetError{Code: "invalid_object_target", Err: generationapp.ErrGenerationCleanupTarget}
	}
	if err := e.store.DeleteVersion(ctx, DerivedBucket, target.ObjectKey, target.ObjectVersionID); err != nil {
		return &generationapp.CleanupTargetError{Code: "object_version_delete_failed", Err: err}
	}
	return nil
}

func (*GenerationCleanupExecutor) DeleteProviderTask(context.Context, generationapp.CleanupTarget) error {
	return &generationapp.CleanupTargetError{Code: "provider_cleanup_unavailable", Err: generationapp.ErrGenerationCleanupTarget}
}
