//go:build services

package services

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	store "github.com/StephenQiu30/then-server/backend/internal/adapter/postgres"
	accountapp "github.com/StephenQiu30/then-server/backend/internal/application/account"
	syncapp "github.com/StephenQiu30/then-server/backend/internal/application/syncchange"
	wardrobeapp "github.com/StephenQiu30/then-server/backend/internal/application/wardrobe"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSyncChangesPersistence(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	base, err := gorm.Open(postgres.Open(environment.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open sync database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create sync schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		serviceOK(t, "drop sync schema", base.WithContext(cleanup).Exec("DROP SCHEMA "+schema+" CASCADE").Error)
	})
	databaseURL, err := url.Parse(environment.databaseURL)
	serviceOK(t, "parse sync database URL", err)
	query := databaseURL.Query()
	query.Set("search_path", schema)
	databaseURL.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open sync schema", err)
	serviceOK(t, "migrate sync schema", store.Migrate(ctx, database))
	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	serviceOK(t, "construct sync accounts", err)
	wardrobe, err := wardrobeapp.NewWardrobeService(accounts, store.NewWardrobeRepository(database))
	serviceOK(t, "construct sync wardrobe", err)
	syncService, err := syncapp.New(accounts, store.NewSyncRepository(database))
	serviceOK(t, "construct sync service", err)
	first, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "sync-first@example.test", DisplayName: "First", Password: "correct-password-first"})
	serviceOK(t, "register first sync owner", err)
	second, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "sync-second@example.test", DisplayName: "Second", Password: "correct-password-second"})
	serviceOK(t, "register second sync owner", err)
	initial, err := syncService.List(ctx, first.Token, "", 1)
	serviceOK(t, "read empty sync page", err)
	if len(initial.Changes) != 0 || initial.HasMore || initial.NextCursor == "" {
		t.Fatal("empty sync page has incorrect cursor or content")
	}
	id := uuid.NewString()
	input := wardrobeapp.CreateWardrobeItemInput{ID: id, Name: "Jacket", Category: wardrobeapp.WardrobeOuterwear, Availability: wardrobeapp.WardrobeWearable, Source: wardrobeapp.WardrobeSourceWardrobe}
	item, err := wardrobe.CreateWardrobeItem(ctx, first.Token, input)
	serviceOK(t, "create synced wardrobe item", err)
	_, err = wardrobe.CreateWardrobeItem(ctx, first.Token, input)
	serviceOK(t, "repeat synced wardrobe create", err)
	updated, err := wardrobe.UpdateWardrobeItem(ctx, first.Token, id, item.Revision, wardrobeapp.UpdateWardrobeItemInput{Name: "Warm Jacket", Category: wardrobeapp.WardrobeOuterwear, Availability: wardrobeapp.WardrobeWearable})
	serviceOK(t, "update synced wardrobe item", err)
	page, err := syncService.List(ctx, first.Token, initial.NextCursor, 1)
	serviceOK(t, "read first incremental page", err)
	if len(page.Changes) != 1 || page.Changes[0].EntityID != id || page.Changes[0].Seq != 1 || !page.HasMore {
		t.Fatalf("first incremental page=%+v", page)
	}
	if _, err := syncService.List(ctx, second.Token, page.NextCursor, 1); !errors.Is(err, syncapp.ErrInvalidCursor) {
		t.Fatal("cross-owner sync cursor was accepted")
	}
	if _, err := syncService.List(ctx, first.Token, "broken", 1); !errors.Is(err, syncapp.ErrInvalidCursor) {
		t.Fatal("broken sync cursor was accepted")
	}
	future := base64.RawURLEncoding.EncodeToString([]byte("v1:" + first.User.ID + ":999"))
	if _, err := syncService.List(ctx, first.Token, future, 1); !errors.Is(err, syncapp.ErrInvalidCursor) {
		t.Fatal("future sync cursor was accepted")
	}
	page, err = syncService.List(ctx, first.Token, page.NextCursor, 1)
	serviceOK(t, "read second incremental page", err)
	if len(page.Changes) != 1 || page.Changes[0].Seq != 2 || page.Changes[0].Revision == nil || *page.Changes[0].Revision != updated.Revision || page.HasMore {
		t.Fatalf("second incremental page=%+v", page)
	}
	otherClient, err := syncService.List(ctx, first.Token, initial.NextCursor, 10)
	serviceOK(t, "read independent same-owner cursor", err)
	if len(otherClient.Changes) != 2 || otherClient.Changes[0].Seq != 1 || otherClient.Changes[1].Seq != 2 {
		t.Fatalf("independent cursor lost committed changes: %+v", otherClient)
	}
	impact, err := wardrobe.GetWardrobeDeletionImpact(ctx, first.Token, id)
	serviceOK(t, "read sync deletion impact", err)
	serviceOK(t, "delete synced wardrobe item", wardrobe.DeleteWardrobeItem(ctx, first.Token, id, updated.Revision, wardrobeapp.WardrobeHistoryRedactSnapshots, impact.ExpectedImpact))
	if _, err := wardrobe.CreateWardrobeItem(ctx, first.Token, input); !errors.Is(err, wardrobeapp.ErrWardrobeConflict) {
		t.Fatal("deleted wardrobe ID was recreated")
	}
	page, err = syncService.List(ctx, first.Token, page.NextCursor, 10)
	serviceOK(t, "read deletion", err)
	if len(page.Changes) != 1 || page.Changes[0].Action != "delete" || page.Changes[0].Seq != 3 || page.HasMore {
		t.Fatalf("deletion page=%+v", page)
	}
	var wg sync.WaitGroup
	errorsByWorker := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := wardrobe.CreateWardrobeItem(ctx, first.Token, wardrobeapp.CreateWardrobeItemInput{ID: uuid.NewString(), Name: "Shirt", Category: wardrobeapp.WardrobeTop, Availability: wardrobeapp.WardrobeWearable, Source: wardrobeapp.WardrobeSourceWardrobe})
			errorsByWorker <- err
		}()
	}
	wg.Wait()
	close(errorsByWorker)
	for err := range errorsByWorker {
		serviceOK(t, "concurrent sync write", err)
	}
	page, err = syncService.List(ctx, first.Token, page.NextCursor, 10)
	serviceOK(t, "read concurrent writes", err)
	if len(page.Changes) != 4 || page.HasMore {
		t.Fatalf("concurrent sync page=%+v", page)
	}
	for index, change := range page.Changes {
		if change.Seq != int64(index+4) {
			t.Fatalf("sync sequence skipped or duplicated: %+v", page.Changes)
		}
	}
	_, err = accounts.DeleteCurrentUser(ctx, first.Token)
	serviceOK(t, "delete sync owner", err)
	var count int64
	serviceOK(t, "count deleted sync changes", database.Raw("SELECT count(*) FROM sync_changes WHERE owner_id = ?", first.User.ID).Scan(&count).Error)
	if count != 0 {
		t.Fatal("deleted owner retained sync history")
	}
	_, err = syncService.List(ctx, first.Token, page.NextCursor, 10)
	if !errors.Is(err, accountapp.ErrAuthentication) {
		t.Fatalf("deleted account could still sync: %v", err)
	}
	secondPage, err := syncService.List(ctx, second.Token, "", 10)
	serviceOK(t, "read unaffected second owner", err)
	if len(secondPage.Changes) != 0 {
		t.Fatal("first owner changes leaked to second owner")
	}
	legacyID := uuid.NewString()
	_, err = wardrobe.CreateWardrobeItem(ctx, second.Token, wardrobeapp.CreateWardrobeItemInput{ID: legacyID, Name: "Legacy Coat", Category: wardrobeapp.WardrobeOuterwear, Availability: wardrobeapp.WardrobeWearable, Source: wardrobeapp.WardrobeSourceWardrobe})
	serviceOK(t, "create legacy seed fixture", err)
	serviceOK(t, "clear seed fixture change", database.Exec("DELETE FROM sync_changes WHERE owner_id = ?", second.User.ID).Error)
	serviceOK(t, "reset seed fixture position", database.Exec("UPDATE sync_positions SET seq = 0, seeded = false WHERE owner_id = ?", second.User.ID).Error)
	seeded, err := syncService.List(ctx, second.Token, "", 10)
	serviceOK(t, "seed preexisting records", err)
	if len(seeded.Changes) != 1 || seeded.Changes[0].EntityID != legacyID || seeded.Changes[0].Seq != 1 {
		t.Fatalf("initial seed omitted legacy record: %+v", seeded)
	}
	repeatedSeed, err := syncService.List(ctx, second.Token, seeded.NextCursor, 10)
	serviceOK(t, "repeat seeded read", err)
	if len(repeatedSeed.Changes) != 0 || repeatedSeed.HasMore {
		t.Fatal("initial seed was repeated")
	}
}

func assertSyncLatest(t *testing.T, database *gorm.DB, ownerID, kind, entityID, action string, revision *int) {
	t.Helper()
	var row struct {
		Action   string
		Revision *int
	}
	serviceOK(t, "read latest sync change", database.Raw("SELECT action, revision FROM sync_changes WHERE owner_id = ? AND kind = ? AND entity_id = ? ORDER BY seq DESC LIMIT 1", ownerID, kind, entityID).Scan(&row).Error)
	if row.Action != action || revision != nil && (row.Revision == nil || *row.Revision != *revision) || revision == nil && row.Revision != nil {
		t.Fatalf("latest sync change for %s/%s = %+v, want %s/%v", kind, entityID, row, action, revision)
	}
}
