package generationfixture

import (
	"context"
	"errors"

	generationapp "github.com/StephenQiu30/then-server/backend/internal/application/generation"
)

// ObjectDeleter removes exact private output versions. The fixture provider
// itself retains no remote task state or media.
type ObjectDeleter interface {
	DeleteObject(context.Context, generationapp.CleanupTarget) error
}

type CleanupExecutor struct {
	objects ObjectDeleter
}

var _ generationapp.CleanupExecutor = (*CleanupExecutor)(nil)

func NewCleanupExecutor(objects ObjectDeleter) (*CleanupExecutor, error) {
	if objects == nil {
		return nil, errors.New("fixture cleanup object deleter unavailable")
	}
	return &CleanupExecutor{objects: objects}, nil
}

func (e *CleanupExecutor) DeleteObject(ctx context.Context, target generationapp.CleanupTarget) error {
	if e == nil || e.objects == nil {
		return errors.New("fixture cleanup object deleter unavailable")
	}
	return e.objects.DeleteObject(ctx, target)
}

func (e *CleanupExecutor) DeleteProviderTask(ctx context.Context, target generationapp.CleanupTarget) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if e == nil || target.Validate() != nil || target.Kind != generationapp.CleanupTargetProvider || !validFixtureTaskID(target.ID) {
		return &generationapp.CleanupTargetError{Code: "provider_cleanup_unavailable", Err: generationapp.ErrGenerationCleanupTarget}
	}
	// A fixture task is reproducible from its namespaced identity; no provider
	// task or external copy exists to delete. Object versions are separate targets.
	return nil
}
