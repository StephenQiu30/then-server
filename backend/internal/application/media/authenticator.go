package media

import (
	"context"

	accountapp "github.com/StephenQiu30/wearlog-server/backend/internal/application/account"
)

type PrivacyAuthenticator interface {
	CurrentUser(context.Context, string) (accountapp.User, error)
}
