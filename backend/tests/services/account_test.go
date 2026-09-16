//go:build services

package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	store "github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	privacyapp "github.com/StephenQiu30/then-server/backend/internal/application/privacy"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAccountPersistenceLifecycle(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	base, err := gorm.Open(postgres.Open(environment.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open account test database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create account test schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := base.WithContext(cleanup).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("account schema cleanup failed")
		}
	})

	databaseURL, err := url.Parse(environment.databaseURL)
	serviceOK(t, "parse account test database", err)
	query := databaseURL.Query()
	query.Set("search_path", schema)
	databaseURL.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open isolated account schema", err)
	serviceOK(t, "migrate account schema", store.Migrate(ctx, database))

	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	serviceOK(t, "construct account service", err)
	first, err := accounts.Register(ctx, accountapp.RegisterAccountInput{
		Email: " FIRST@Example.Test ", DisplayName: "First User", Password: "correct-password-one",
	})
	serviceOK(t, "register first account", err)
	if first.User.Email != "first@example.test" {
		t.Fatal("persisted email was not normalized")
	}
	privacy, err := privacyapp.NewPrivacyService(accounts, store.NewPrivacyRepository(database))
	serviceOK(t, "construct privacy service", err)
	declaration, err := privacy.CurrentSelfAdultDeclaration(ctx, first.Token)
	serviceOK(t, "read initial declaration", err)
	if declaration.PolicyVersion != privacyapp.CurrentSelfAdultPolicyVersion || declaration.Confirmed || declaration.ConfirmedAt != nil || declaration.WithdrawnAt != nil {
		t.Fatal("new account did not expose an explicit unconfirmed current policy")
	}
	declaration, err = privacy.ConfirmSelfAdultDeclaration(ctx, first.Token, privacyapp.ConfirmSelfAdultDeclarationInput{
		PolicyVersion: privacyapp.CurrentSelfAdultPolicyVersion, ConfirmsSelfAndAdult: true,
	})
	serviceOK(t, "confirm current declaration", err)
	if !declaration.Confirmed || declaration.ConfirmedAt == nil || declaration.WithdrawnAt != nil {
		t.Fatal("confirmed declaration was not persisted")
	}
	firstConfirmation := *declaration.ConfirmedAt
	declaration, err = privacy.ConfirmSelfAdultDeclaration(ctx, first.Token, privacyapp.ConfirmSelfAdultDeclarationInput{
		PolicyVersion: privacyapp.CurrentSelfAdultPolicyVersion, ConfirmsSelfAndAdult: true,
	})
	serviceOK(t, "repeat current declaration", err)
	if declaration.ConfirmedAt == nil || !declaration.ConfirmedAt.Equal(firstConfirmation) {
		t.Fatal("repeated declaration changed its confirmation time")
	}
	declaration, err = privacy.WithdrawSelfAdultDeclaration(ctx, first.Token)
	serviceOK(t, "withdraw current declaration", err)
	if declaration.Confirmed || declaration.WithdrawnAt == nil {
		t.Fatal("withdrawn declaration remained active")
	}
	firstWithdrawal := *declaration.WithdrawnAt
	declaration, err = privacy.WithdrawSelfAdultDeclaration(ctx, first.Token)
	serviceOK(t, "repeat declaration withdrawal", err)
	if declaration.WithdrawnAt == nil || !declaration.WithdrawnAt.Equal(firstWithdrawal) {
		t.Fatal("repeated withdrawal changed its withdrawal time")
	}
	declaration, err = privacy.ConfirmSelfAdultDeclaration(ctx, first.Token, privacyapp.ConfirmSelfAdultDeclarationInput{
		PolicyVersion: privacyapp.CurrentSelfAdultPolicyVersion, ConfirmsSelfAndAdult: true,
	})
	serviceOK(t, "reconfirm withdrawn declaration", err)
	if !declaration.Confirmed || declaration.ConfirmedAt == nil || !declaration.ConfirmedAt.After(firstConfirmation) || declaration.WithdrawnAt != nil {
		t.Fatal("reconfirmation did not replace the withdrawn state")
	}
	if _, err := accounts.Register(ctx, accountapp.RegisterAccountInput{
		Email: "first@example.test", DisplayName: "Duplicate", Password: "correct-password-two",
	}); !errors.Is(err, accountapp.ErrEmailConflict) {
		t.Fatal("duplicate email did not return the stable conflict")
	}
	second, err := accounts.Register(ctx, accountapp.RegisterAccountInput{
		Email: "second@example.test", DisplayName: "Second User", Password: "correct-password-two",
	})
	serviceOK(t, "register second account", err)
	firstBio := "  记录日常穿搭与轻量生活。  "
	firstProfile, err := accounts.PutCurrentProfile(ctx, first.Token, accountapp.PutProfileInput{
		Handle: " First_Style ", Bio: &firstBio, ExpectedRevision: 0,
	})
	serviceOK(t, "create first public profile", err)
	if firstProfile.Handle != "first_style" || firstProfile.Bio == nil || *firstProfile.Bio != "记录日常穿搭与轻量生活。" || firstProfile.Revision != 1 {
		t.Fatal("created profile differs from the normalized contract")
	}
	publicProfile, err := accounts.PublicProfile(ctx, "FIRST_STYLE")
	serviceOK(t, "read normalized public profile", err)
	if publicProfile.Handle != firstProfile.Handle || publicProfile.DisplayName != first.User.DisplayName {
		t.Fatal("public profile did not join the current display name")
	}
	if _, err := accounts.PutCurrentProfile(ctx, second.Token, accountapp.PutProfileInput{
		Handle: firstProfile.Handle, ExpectedRevision: 0,
	}); !errors.Is(err, accountapp.ErrHandleConflict) {
		t.Fatalf("duplicate profile handle error=%v", err)
	}
	if _, err := accounts.PutCurrentProfile(ctx, first.Token, accountapp.PutProfileInput{
		Handle: firstProfile.Handle, ExpectedRevision: 0,
	}); !errors.Is(err, accountapp.ErrProfileConflict) {
		t.Fatalf("stale profile revision error=%v", err)
	}
	firstProfile, err = accounts.PutCurrentProfile(ctx, first.Token, accountapp.PutProfileInput{
		Handle: firstProfile.Handle, ExpectedRevision: firstProfile.Revision,
	})
	serviceOK(t, "update first public profile", err)
	if firstProfile.Revision != 2 || firstProfile.Bio != nil {
		t.Fatal("profile update did not advance revision and clear bio")
	}
	secondProfile, err := accounts.PutCurrentProfile(ctx, second.Token, accountapp.PutProfileInput{
		Handle: "second_style", ExpectedRevision: 0,
	})
	serviceOK(t, "create second public profile", err)
	if secondProfile.Revision != 1 {
		t.Fatal("second profile did not start at revision one")
	}
	if err := database.WithContext(ctx).Exec("UPDATE users SET role = 'owner' WHERE id = ?", second.User.ID).Error; err == nil {
		t.Fatal("database accepted an account role outside the closed set")
	}
	serviceOK(t, "suspend second account", database.WithContext(ctx).Exec("UPDATE users SET status = 'suspended' WHERE id = ?", second.User.ID).Error)
	if _, err := accounts.Login(ctx, accountapp.CreateSessionInput{Email: second.User.Email, Password: "correct-password-two"}); !errors.Is(err, accountapp.ErrAuthentication) {
		t.Fatalf("suspended account login error=%v", err)
	}
	if _, err := accounts.CurrentUser(ctx, second.Token); !errors.Is(err, accountapp.ErrAuthentication) {
		t.Fatalf("suspended account session error=%v", err)
	}
	if _, err := accounts.PublicProfile(ctx, secondProfile.Handle); !errors.Is(err, accountapp.ErrProfileNotFound) {
		t.Fatalf("suspended public profile error=%v", err)
	}
	serviceOK(t, "restore second account", database.WithContext(ctx).Exec("UPDATE users SET status = 'active' WHERE id = ?", second.User.ID).Error)
	const concurrentConfirmations = 16
	confirmationResults := make(chan privacyapp.SelfAdultDeclaration, concurrentConfirmations)
	confirmationErrors := make(chan error, concurrentConfirmations)
	var confirmationGroup sync.WaitGroup
	for range concurrentConfirmations {
		confirmationGroup.Add(1)
		go func() {
			declaration, err := privacy.ConfirmSelfAdultDeclaration(ctx, second.Token, privacyapp.ConfirmSelfAdultDeclarationInput{
				PolicyVersion: privacyapp.CurrentSelfAdultPolicyVersion, ConfirmsSelfAndAdult: true,
			})
			if err != nil {
				confirmationErrors <- err
			} else {
				confirmationResults <- declaration
			}
			confirmationGroup.Done()
		}()
	}
	confirmationGroup.Wait()
	close(confirmationErrors)
	close(confirmationResults)
	for err := range confirmationErrors {
		t.Fatalf("concurrent declaration failed: %v", err)
	}
	var sharedConfirmation time.Time
	for declaration := range confirmationResults {
		if !declaration.Confirmed || declaration.ConfirmedAt == nil {
			t.Fatal("concurrent declaration did not return the confirmed state")
		}
		if sharedConfirmation.IsZero() {
			sharedConfirmation = *declaration.ConfirmedAt
		} else if !declaration.ConfirmedAt.Equal(sharedConfirmation) {
			t.Fatal("concurrent duplicate declarations did not converge on one confirmation")
		}
	}

	conflictEmail := first.User.Email
	newName := "Should Roll Back"
	if _, err := accounts.UpdateCurrentUser(ctx, second.Token, accountapp.UpdateCurrentUserInput{
		Email: &conflictEmail, DisplayName: &newName, ExpectedRevision: second.User.Revision,
	}); !errors.Is(err, accountapp.ErrEmailConflict) {
		t.Fatal("conflicting update did not return email conflict")
	}
	secondAfterConflict, err := accounts.CurrentUser(ctx, second.Token)
	serviceOK(t, "read account after update conflict", err)
	if secondAfterConflict.DisplayName != "Second User" {
		t.Fatal("email conflict did not roll back the display-name update")
	}

	var credential struct {
		PasswordHash string `gorm:"column:password_hash"`
	}
	serviceOK(t, "inspect protected credential", database.WithContext(ctx).Raw("SELECT password_hash FROM user_credentials WHERE user_id = ?", first.User.ID).Scan(&credential).Error)
	if credential.PasswordHash == "correct-password-one" || !strings.HasPrefix(credential.PasswordHash, "$2") {
		t.Fatal("database contains an unprotected password")
	}
	var tokenLength int
	serviceOK(t, "inspect protected session", database.WithContext(ctx).Raw("SELECT octet_length(token_hash) FROM user_sessions WHERE user_id = ? LIMIT 1", first.User.ID).Scan(&tokenLength).Error)
	if tokenLength != 32 {
		t.Fatal("database session token hash has an invalid length")
	}

	loggedIn, err := accounts.Login(ctx, accountapp.CreateSessionInput{Email: first.User.Email, Password: "correct-password-one"})
	serviceOK(t, "login persisted account", err)
	if err := accounts.DeleteCurrentUser(ctx, loggedIn.Token); err != nil {
		t.Fatal("delete persisted account failed")
	}
	for table, expected := range map[string]int64{"users": 1, "user_credentials": 1, "user_sessions": 1, "self_adult_declarations": 1, "user_profiles": 1} {
		var count int64
		statement := fmt.Sprintf("SELECT count(*) FROM %s", table)
		serviceOK(t, "verify account cascade", database.WithContext(ctx).Raw(statement).Scan(&count).Error)
		if count != expected {
			t.Fatalf("%s count=%d expected=%d", table, count, expected)
		}
	}
}
