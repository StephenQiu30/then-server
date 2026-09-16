package outfitplan

import (
	"context"

	"github.com/StephenQiu30/then-server/backend/internal/domain"
)

type PrivacyAuthenticator interface {
	CurrentUser(context.Context, string) (domain.User, error)
}
