//go:build services

package tests

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/StephenQiu30/then-server/backend/internal/model"
	"github.com/StephenQiu30/then-server/backend/internal/repository"
	"github.com/StephenQiu30/then-server/backend/internal/service"
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
	migration, err := os.ReadFile(filepath.Join("..", "migrations", "20260914170000_accounts.sql"))
	serviceOK(t, "read account migration", err)
	serviceOK(t, "apply account migration", database.WithContext(ctx).Exec(string(migration)).Error)

	accounts, err := service.NewAccountService(repository.NewAccountRepository(database))
	serviceOK(t, "construct account service", err)
	first, err := accounts.Register(ctx, model.RegisterAccountInput{
		Email: " FIRST@Example.Test ", DisplayName: "First User", Password: "correct-password-one",
	})
	serviceOK(t, "register first account", err)
	if first.User.Email != "first@example.test" {
		t.Fatal("persisted email was not normalized")
	}
	if _, err := accounts.Register(ctx, model.RegisterAccountInput{
		Email: "first@example.test", DisplayName: "Duplicate", Password: "correct-password-two",
	}); !errors.Is(err, model.ErrEmailConflict) {
		t.Fatal("duplicate email did not return the stable conflict")
	}
	second, err := accounts.Register(ctx, model.RegisterAccountInput{
		Email: "second@example.test", DisplayName: "Second User", Password: "correct-password-two",
	})
	serviceOK(t, "register second account", err)

	conflictEmail := first.User.Email
	newName := "Should Roll Back"
	if _, err := accounts.UpdateCurrentUser(ctx, second.Token, model.UpdateCurrentUserInput{Email: &conflictEmail, DisplayName: &newName}); !errors.Is(err, model.ErrEmailConflict) {
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

	loggedIn, err := accounts.Login(ctx, model.CreateSessionInput{Email: first.User.Email, Password: "correct-password-one"})
	serviceOK(t, "login persisted account", err)
	if err := accounts.DeleteCurrentUser(ctx, loggedIn.Token); err != nil {
		t.Fatal("delete persisted account failed")
	}
	for table, expected := range map[string]int64{"users": 1, "user_credentials": 1, "user_sessions": 1} {
		var count int64
		statement := fmt.Sprintf("SELECT count(*) FROM %s", table)
		serviceOK(t, "verify account cascade", database.WithContext(ctx).Raw(statement).Scan(&count).Error)
		if count != expected {
			t.Fatalf("%s count=%d expected=%d", table, count, expected)
		}
	}
}
