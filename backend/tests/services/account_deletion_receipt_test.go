//go:build services

package services

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	store "github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	mediaapp "github.com/StephenQiu30/then-server/backend/internal/application/media"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAccountDeletionReceiptAfterSessionRevocation(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	base, err := gorm.Open(postgres.Open(environment.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open receipt database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create receipt schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if err := base.WithContext(cleanup).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("receipt schema cleanup failed")
		}
	})
	databaseURL, err := url.Parse(environment.databaseURL)
	serviceOK(t, "parse receipt database URL", err)
	query := databaseURL.Query()
	query.Set("search_path", schema)
	databaseURL.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open receipt schema", err)
	serviceOK(t, "migrate receipt schema", store.Migrate(ctx, database))
	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	serviceOK(t, "construct receipt accounts", err)
	first, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "receipt-first@example.test", DisplayName: "First", Password: "first-password-2026"})
	serviceOK(t, "register first receipt owner", err)
	second, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "receipt-second@example.test", DisplayName: "Second", Password: "second-password-2026"})
	serviceOK(t, "register second receipt owner", err)
	completed, err := accounts.DeleteCurrentUser(ctx, first.Token)
	serviceOK(t, "delete owner without media", err)
	if completed.Status != accountapp.AccountDeletionComplete || len(completed.ReceiptToken) != 43 || !completed.ReceiptExpiresAt.After(completed.RequestedAt) {
		t.Fatal("one-time receipt was not issued")
	}
	if _, err := accounts.CurrentUser(ctx, first.Token); !errors.Is(err, accountapp.ErrAuthentication) {
		t.Fatal("deleted account session remained valid")
	}
	var stored struct{ ReceiptHash []byte }
	serviceOK(t, "inspect receipt digest", database.WithContext(ctx).Raw("SELECT receipt_hash FROM account_deletion_requests WHERE id = ?", completed.ID).Scan(&stored).Error)
	if len(stored.ReceiptHash) != 32 || string(stored.ReceiptHash) == completed.ReceiptToken {
		t.Fatal("raw receipt credential persisted")
	}
	if _, err := accounts.GetDeletionReceipt(ctx, completed.ID, second.Token); !errors.Is(err, accountapp.ErrDeletionReceiptNotFound) {
		t.Fatal("other session token read account receipt")
	}
	if _, err := accounts.GetDeletionReceipt(ctx, uuid.NewString(), completed.ReceiptToken); !errors.Is(err, accountapp.ErrDeletionReceiptNotFound) {
		t.Fatal("receipt token read another request")
	}
	view, err := accounts.GetDeletionReceipt(ctx, completed.ID, completed.ReceiptToken)
	serviceOK(t, "read completed receipt without session", err)
	if view.Status != accountapp.AccountDeletionComplete || view.Phase != "complete" || !view.AccessClosed || view.RemainingMediaCount != 0 || view.ReceiptToken != "" {
		t.Fatal("completed receipt disclosed wrong progress or repeated token")
	}
	serviceOK(t, "revoke completed receipt", accounts.RevokeDeletionReceipt(ctx, completed.ID, completed.ReceiptToken))
	if _, err := accounts.GetDeletionReceipt(ctx, completed.ID, completed.ReceiptToken); !errors.Is(err, accountapp.ErrDeletionReceiptNotFound) {
		t.Fatal("revoked receipt remained readable")
	}
	serviceOK(t, "expire completed receipt", database.WithContext(ctx).Exec("UPDATE account_deletion_requests SET requested_at = ?, receipt_expires_at = ? WHERE id = ?", time.Now().Add(-8*24*time.Hour), time.Now().Add(-time.Second), completed.ID).Error)
	serviceOK(t, "purge completed receipt", accounts.PurgeDeletionReceipts(ctx))
	var count int64
	serviceOK(t, "count purged completed receipt", database.WithContext(ctx).Table("account_deletion_requests").Where("id = ?", completed.ID).Count(&count).Error)
	if count != 0 {
		t.Fatal("expired completed receipt was retained")
	}

	mediaRepository := store.NewMediaRepository(database)
	priorMedia, err := mediaRepository.CreateMedia(ctx, second.User.ID, mediaapp.CreateMediaUploadInput{Purpose: mediaapp.MediaPurposeDiaryImage, ContentType: mediaapp.MediaContentTypeJPEG, ByteSize: 8, SHA256: strings.Repeat("c", 64)}, time.Now().UTC())
	serviceOK(t, "create prior individual media", err)
	_, err = mediaRepository.DeleteMedia(ctx, second.User.ID, priorMedia.ID, time.Now().UTC())
	serviceOK(t, "delete prior individual media", err)
	serviceOK(t, "complete prior individual deletion", mediaRepository.CompleteDeletion(ctx, uuid.NewString(), priorMedia.ID, time.Now().UTC()))
	serviceOK(t, "record unrelated prior retry", database.WithContext(ctx).Exec("UPDATE deletion_requests SET attempts = 2 WHERE media_id = ?", priorMedia.ID).Error)
	media, err := mediaRepository.CreateMedia(ctx, second.User.ID, mediaapp.CreateMediaUploadInput{Purpose: mediaapp.MediaPurposeDiaryImage, ContentType: mediaapp.MediaContentTypeJPEG, ByteSize: 8, SHA256: strings.Repeat("a", 64)}, time.Now().UTC())
	serviceOK(t, "create unfinished synthetic media", err)
	pending, err := accounts.DeleteCurrentUser(ctx, second.Token)
	serviceOK(t, "delete owner with pending media", err)
	if pending.Status != accountapp.AccountDeletionPending || pending.MediaCount != 1 || pending.RemainingMediaCount != 1 {
		t.Fatal("pending receipt missed media cleanup")
	}
	view, err = accounts.GetDeletionReceipt(ctx, pending.ID, pending.ReceiptToken)
	serviceOK(t, "read pending receipt", err)
	if view.Phase != "media_cleanup" || view.RemainingMediaCount != 1 || !view.AccessClosed || view.RetryObserved {
		t.Fatal("pending cleanup was not reported")
	}
	serviceOK(t, "record observed retry", database.WithContext(ctx).Exec("UPDATE deletion_requests SET attempts = 2, updated_at = ? WHERE media_id = ?", time.Now().UTC(), media.ID).Error)
	view, err = accounts.GetDeletionReceipt(ctx, pending.ID, pending.ReceiptToken)
	serviceOK(t, "read retry evidence", err)
	if !view.RetryObserved || view.Phase != "media_cleanup_retry_observed" {
		t.Fatal("retry evidence was not reported")
	}
	serviceOK(t, "expire pending receipt", database.WithContext(ctx).Exec("UPDATE account_deletion_requests SET requested_at = ?, receipt_expires_at = ? WHERE id = ?", time.Now().Add(-8*24*time.Hour), time.Now().Add(-time.Second), pending.ID).Error)
	if _, err := accounts.GetDeletionReceipt(ctx, pending.ID, pending.ReceiptToken); !errors.Is(err, accountapp.ErrDeletionReceiptNotFound) {
		t.Fatal("expired pending receipt remained readable")
	}
	serviceOK(t, "purge expired pending credential", accounts.PurgeDeletionReceipts(ctx))
	serviceOK(t, "inspect pending internal record", database.WithContext(ctx).Raw("SELECT count(*) FROM account_deletion_requests WHERE id = ? AND receipt_hash IS NULL AND status = 'pending'", pending.ID).Scan(&count).Error)
	if count != 1 {
		t.Fatal("pending deletion lost its internal progress record")
	}
	serviceOK(t, "complete empty synthetic media deletion", mediaRepository.CompleteDeletion(ctx, uuid.NewString(), media.ID, time.Now().UTC()))
	serviceOK(t, "purge finished expired receipt", accounts.PurgeDeletionReceipts(ctx))
	serviceOK(t, "count finished expired receipt", database.WithContext(ctx).Table("account_deletion_requests").Where("id = ?", pending.ID).Count(&count).Error)
	if count != 0 {
		t.Fatal("finished expired receipt was retained")
	}
	third, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "receipt-third@example.test", DisplayName: "Third", Password: "third-password-2026"})
	serviceOK(t, "register third receipt owner", err)
	thirdMedia, err := mediaRepository.CreateMedia(ctx, third.User.ID, mediaapp.CreateMediaUploadInput{Purpose: mediaapp.MediaPurposeDiaryImage, ContentType: mediaapp.MediaContentTypeJPEG, ByteSize: 8, SHA256: strings.Repeat("b", 64)}, time.Now().UTC())
	serviceOK(t, "create third synthetic media", err)
	transition, err := accounts.DeleteCurrentUser(ctx, third.Token)
	serviceOK(t, "begin visible deletion transition", err)
	if transition.Status != accountapp.AccountDeletionPending {
		t.Fatal("deletion transition skipped pending state")
	}
	serviceOK(t, "record third retry", database.WithContext(ctx).Exec("UPDATE deletion_requests SET attempts = 2 WHERE media_id = ?", thirdMedia.ID).Error)
	serviceOK(t, "finish visible deletion transition", mediaRepository.CompleteDeletion(ctx, uuid.NewString(), thirdMedia.ID, time.Now().UTC()))
	view, err = accounts.GetDeletionReceipt(ctx, transition.ID, transition.ReceiptToken)
	serviceOK(t, "read completed deletion transition", err)
	if view.Status != accountapp.AccountDeletionComplete || view.Phase != "complete" || view.RemainingMediaCount != 0 || view.CompletedAt == nil || !view.RetryObserved {
		t.Fatal("receipt did not advance to complete after media cleanup")
	}
}
