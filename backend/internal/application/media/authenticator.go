package media

import (
	"context"

	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
)

type PrivacyAuthenticator interface {
	CurrentUser(context.Context, string) (accountapp.User, error)
}
