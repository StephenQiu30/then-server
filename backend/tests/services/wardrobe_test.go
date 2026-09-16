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
	wardrobeapp "github.com/StephenQiu30/then-server/backend/internal/application/wardrobe"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestWardrobePersistenceLifecycle(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	base, err := gorm.Open(postgres.Open(environment.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open wardrobe test database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create wardrobe test schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := base.WithContext(cleanup).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("wardrobe schema cleanup failed")
		}
	})

	databaseURL, err := url.Parse(environment.databaseURL)
	serviceOK(t, "parse wardrobe test database", err)
	query := databaseURL.Query()
	query.Set("search_path", schema)
	databaseURL.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open isolated wardrobe schema", err)
	serviceOK(t, "migrate wardrobe schema", store.Migrate(ctx, database))

	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	serviceOK(t, "construct wardrobe account service", err)
	first, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "wardrobe-first@example.test", DisplayName: "First", Password: "correct-password-first"})
	serviceOK(t, "register first wardrobe owner", err)
	second, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "wardrobe-second@example.test", DisplayName: "Second", Password: "correct-password-second"})
	serviceOK(t, "register second wardrobe owner", err)
	wardrobe, err := wardrobeapp.NewWardrobeService(accounts, store.NewWardrobeRepository(database))
	serviceOK(t, "construct wardrobe service", err)

	sharedID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400b11"
	input := wardrobeapp.CreateWardrobeItemInput{ID: sharedID, Name: "Blue Shirt", Category: wardrobeapp.WardrobeTop, Availability: wardrobeapp.WardrobeWearable, Source: wardrobeapp.WardrobeSourceWardrobe, Attributes: wardrobeapp.WardrobeAttributes{FormalityBand: wardrobeTestValue(wardrobeapp.WardrobeFormalitySmartCasual), WarmthBand: wardrobeTestValue(wardrobeapp.WardrobeWarmthLight), RainUse: wardrobeTestValue(wardrobeapp.WardrobeUseSuitable)}}
	created, err := wardrobe.CreateWardrobeItem(ctx, first.Token, input)
	serviceOK(t, "create first wardrobe item", err)
	if created.Attributes.FormalityBand == nil || *created.Attributes.FormalityBand != wardrobeapp.WardrobeFormalitySmartCasual || created.Attributes.WalkingUse != nil {
		t.Fatal("created wardrobe attributes did not preserve known and unknown values")
	}
	repeated, err := wardrobe.CreateWardrobeItem(ctx, first.Token, input)
	serviceOK(t, "repeat idempotent wardrobe create", err)
	if repeated.Revision != created.Revision || !repeated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatal("idempotent create changed the stored wardrobe item")
	}
	conflict := input
	conflict.Attributes.RainUse = wardrobeTestValue(wardrobeapp.WardrobeUseUnsuitable)
	if _, err := wardrobe.CreateWardrobeItem(ctx, first.Token, conflict); !errors.Is(err, wardrobeapp.ErrWardrobeConflict) {
		t.Fatal("same owner and ID accepted different attribute content")
	}
	if err := database.WithContext(ctx).Exec("UPDATE wardrobe_items SET warmth_band = 'boiling' WHERE owner_id = ? AND id = ?", first.User.ID, sharedID).Error; err == nil {
		t.Fatal("PostgreSQL accepted an attribute outside the CHECK constraint")
	}
	if _, err := wardrobe.CreateWardrobeItem(ctx, second.Token, input); err != nil {
		t.Fatal("different owners could not reuse a client-generated item ID")
	}

	privateID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400b12"
	private, err := wardrobe.CreateWardrobeItem(ctx, first.Token, wardrobeapp.CreateWardrobeItemInput{ID: privateID, Name: "Black Shoes", Category: wardrobeapp.WardrobeShoes, Availability: wardrobeapp.WardrobePacked, Source: wardrobeapp.WardrobeSourceQuickAdd})
	serviceOK(t, "create owner-only wardrobe item", err)
	thirdID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400b13"
	_, err = wardrobe.CreateWardrobeItem(ctx, first.Token, wardrobeapp.CreateWardrobeItemInput{ID: thirdID, Name: "Rain Coat", Category: wardrobeapp.WardrobeOuterwear, Availability: wardrobeapp.WardrobeLaundry, Source: wardrobeapp.WardrobeSourceWardrobe})
	serviceOK(t, "create pagination wardrobe item", err)
	if _, err := wardrobe.GetWardrobeItem(ctx, second.Token, privateID); !errors.Is(err, wardrobeapp.ErrWardrobeNotFound) {
		t.Fatal("cross-owner wardrobe lookup did not return the uniform not-found result")
	}

	firstPage, err := wardrobe.ListWardrobeItems(ctx, first.Token, 2, nil)
	serviceOK(t, "list first wardrobe page", err)
	if len(firstPage.Items) != 2 || firstPage.NextAfterID == nil {
		t.Fatal("first wardrobe page did not return a bounded continuation")
	}
	secondPage, err := wardrobe.ListWardrobeItems(ctx, first.Token, 2, firstPage.NextAfterID)
	serviceOK(t, "list second wardrobe page", err)
	if len(secondPage.Items) != 1 || secondPage.NextAfterID != nil || secondPage.Items[0].ID == firstPage.Items[0].ID || secondPage.Items[0].ID == firstPage.Items[1].ID {
		t.Fatal("wardrobe pagination duplicated or omitted an item")
	}
	if _, err := wardrobe.ListWardrobeItems(ctx, second.Token, 2, firstPage.NextAfterID); !errors.Is(err, wardrobeapp.ErrWardrobeNotFound) {
		t.Fatal("cross-owner wardrobe cursor leaked a different result")
	}

	updated, err := wardrobe.UpdateWardrobeItem(ctx, first.Token, private.ID, private.Revision, wardrobeapp.UpdateWardrobeItemInput{Name: "Black Walking Shoes", Category: wardrobeapp.WardrobeShoes, Availability: wardrobeapp.WardrobeWearable, Attributes: wardrobeapp.WardrobeAttributes{WarmthBand: wardrobeTestValue(wardrobeapp.WardrobeWarmthMedium), WalkingUse: wardrobeTestValue(wardrobeapp.WardrobeUseSuitable)}})
	serviceOK(t, "update current wardrobe revision", err)
	if updated.Revision != private.Revision+1 || updated.Source != wardrobeapp.WardrobeSourceQuickAdd || updated.Name != "Black Walking Shoes" || updated.Attributes.FormalityBand != nil || updated.Attributes.WarmthBand == nil || *updated.Attributes.WarmthBand != wardrobeapp.WardrobeWarmthMedium {
		t.Fatal("wardrobe update changed immutable facts or missed revision increment")
	}
	restarted, err := wardrobeapp.NewWardrobeService(accounts, store.NewWardrobeRepository(database))
	serviceOK(t, "reconstruct wardrobe service", err)
	reloaded, err := restarted.GetWardrobeItem(ctx, first.Token, private.ID)
	serviceOK(t, "read attributes after repository reconstruction", err)
	if reloaded.Attributes.WalkingUse == nil || *reloaded.Attributes.WalkingUse != wardrobeapp.WardrobeUseSuitable || reloaded.Attributes.RainUse != nil {
		t.Fatal("wardrobe attributes did not survive a fresh repository read")
	}
	if _, err := wardrobe.UpdateWardrobeItem(ctx, first.Token, private.ID, private.Revision, wardrobeapp.UpdateWardrobeItemInput{Name: "Stale", Category: wardrobeapp.WardrobeShoes, Availability: wardrobeapp.WardrobeLaundry}); !errors.Is(err, wardrobeapp.ErrWardrobeConflict) {
		t.Fatal("stale wardrobe update was accepted")
	}
	impact, err := wardrobe.GetWardrobeDeletionImpact(ctx, first.Token, private.ID)
	serviceOK(t, "read wardrobe deletion impact", err)
	if err := wardrobe.DeleteWardrobeItem(ctx, first.Token, private.ID, private.Revision, wardrobeapp.WardrobeHistoryRedactSnapshots, impact.ExpectedImpact); !errors.Is(err, wardrobeapp.ErrWardrobeConflict) {
		t.Fatal("stale wardrobe delete was accepted")
	}
	serviceOK(t, "delete current wardrobe revision", wardrobe.DeleteWardrobeItem(ctx, first.Token, private.ID, updated.Revision, wardrobeapp.WardrobeHistoryRedactSnapshots, impact.ExpectedImpact))
	if _, err := wardrobe.GetWardrobeItem(ctx, first.Token, private.ID); !errors.Is(err, wardrobeapp.ErrWardrobeNotFound) {
		t.Fatal("deleted wardrobe item remained readable")
	}

	serviceOK(t, "delete first wardrobe owner", accounts.DeleteCurrentUser(ctx, first.Token))
	var firstCount, secondCount int64
	serviceOK(t, "count first owner wardrobe", database.WithContext(ctx).Raw("SELECT count(*) FROM wardrobe_items WHERE owner_id = ?", first.User.ID).Scan(&firstCount).Error)
	serviceOK(t, "count second owner wardrobe", database.WithContext(ctx).Raw("SELECT count(*) FROM wardrobe_items WHERE owner_id = ?", second.User.ID).Scan(&secondCount).Error)
	if firstCount != 0 || secondCount != 1 {
		t.Fatalf("account cascade crossed owners: first=%d second=%d", firstCount, secondCount)
	}
}

func wardrobeTestValue[T any](value T) *T { return &value }
