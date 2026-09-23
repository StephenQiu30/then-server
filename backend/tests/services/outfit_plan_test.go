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
	outfitplanapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitplan"
	wardrobeapp "github.com/StephenQiu30/then-server/backend/internal/application/wardrobe"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOutfitPlanPersistenceLifecycle(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	base, err := gorm.Open(postgres.Open(environment.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open outfit plan test database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create outfit plan test schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := base.WithContext(cleanup).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("outfit plan schema cleanup failed")
		}
	})

	databaseURL, err := url.Parse(environment.databaseURL)
	serviceOK(t, "parse outfit plan test database", err)
	query := databaseURL.Query()
	query.Set("search_path", schema)
	databaseURL.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open isolated outfit plan schema", err)
	serviceOK(t, "migrate outfit plan schema", store.Migrate(ctx, database))

	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	serviceOK(t, "construct outfit plan account service", err)
	owner, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "outfit-owner@example.test", DisplayName: "Owner", Password: "correct-password-owner"})
	serviceOK(t, "register outfit plan owner", err)
	other, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "outfit-other@example.test", DisplayName: "Other", Password: "correct-password-other"})
	serviceOK(t, "register second outfit plan owner", err)
	wardrobe, err := wardrobeapp.NewWardrobeService(accounts, store.NewWardrobeRepository(database))
	serviceOK(t, "construct outfit plan wardrobe service", err)
	outfits, err := outfitplanapp.NewOutfitPlanService(accounts, store.NewOutfitPlanRepository(database))
	serviceOK(t, "construct outfit plan service", err)

	top, err := wardrobe.CreateWardrobeItem(ctx, owner.Token, wardrobeapp.CreateWardrobeItemInput{
		ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400c01", Name: "Blue Shirt", Category: wardrobeapp.WardrobeTop,
		Availability: wardrobeapp.WardrobeWearable, Source: wardrobeapp.WardrobeSourceWardrobe,
		Attributes: wardrobeapp.WardrobeAttributes{FormalityBand: wardrobeTestValue(wardrobeapp.WardrobeFormalitySmartCasual), WarmthBand: wardrobeTestValue(wardrobeapp.WardrobeWarmthLight)},
	})
	serviceOK(t, "create wearable outfit item", err)
	shoes, err := wardrobe.CreateWardrobeItem(ctx, owner.Token, wardrobeapp.CreateWardrobeItemInput{
		ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400c02", Name: "Walking Shoes", Category: wardrobeapp.WardrobeShoes,
		Availability: wardrobeapp.WardrobeLaundry, Source: wardrobeapp.WardrobeSourceQuickAdd,
		Attributes: wardrobeapp.WardrobeAttributes{WalkingUse: wardrobeTestValue(wardrobeapp.WardrobeUseSuitable)},
	})
	serviceOK(t, "create unavailable outfit item", err)

	location, err := time.LoadLocation("Asia/Shanghai")
	serviceOK(t, "load outfit plan timezone", err)
	firstDate := time.Now().In(location).AddDate(0, 0, 1).Format("2006-01-02")
	secondDate := time.Now().In(location).AddDate(0, 0, 2).Format("2006-01-02")
	firstPlanID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400d01"
	summary := "Office day"
	firstInput := outfitplanapp.OutfitPlanInput{
		LocalDate: firstDate, TimeZone: "Asia/Shanghai", ContextSummary: &summary,
		Items:                   []outfitplanapp.OutfitSelection{{ItemID: top.ID, Revision: top.Revision}, {ItemID: shoes.ID, Revision: shoes.Revision}},
		ConfirmedUnavailableIDs: []string{shoes.ID},
	}
	created, err := outfits.CreateOutfitPlan(ctx, owner.Token, firstPlanID, firstInput)
	serviceOK(t, "create outfit plan with confirmed unavailable item", err)
	if created.Revision != 1 || created.Status != outfitplanapp.OutfitPlanActive || len(created.Items) != 2 || created.Items[0].Content == nil || created.Items[0].Content.Name != "Blue Shirt" || created.Items[0].Content.Attributes.FormalityBand == nil {
		t.Fatal("created outfit plan did not preserve its ordered server snapshot")
	}
	repeated, err := outfits.CreateOutfitPlan(ctx, owner.Token, firstPlanID, firstInput)
	serviceOK(t, "repeat idempotent outfit plan create", err)
	if repeated.Revision != created.Revision || !repeated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatal("idempotent outfit plan create changed persisted facts")
	}
	changedInput := firstInput
	changedSummary := "Different"
	changedInput.ContextSummary = &changedSummary
	if _, err := outfits.CreateOutfitPlan(ctx, owner.Token, firstPlanID, changedInput); !errors.Is(err, outfitplanapp.ErrOutfitPlanConflict) {
		t.Fatal("same outfit plan ID accepted different content")
	}
	archivedShoes, err := wardrobe.ArchiveWardrobeItem(ctx, owner.Token, shoes.ID, shoes.Revision)
	serviceOK(t, "archive an item after creating a plan snapshot", err)
	if _, err := outfits.CreateOutfitPlan(ctx, owner.Token, "018f1f74-a2d0-7c6d-9c17-4a0ea2400d05", outfitplanapp.OutfitPlanInput{LocalDate: firstDate, TimeZone: "Asia/Shanghai", Items: []outfitplanapp.OutfitSelection{{ItemID: archivedShoes.ID, Revision: archivedShoes.Revision}}, ConfirmedUnavailableIDs: []string{archivedShoes.ID}}); !errors.Is(err, outfitplanapp.ErrOutfitPlanConflict) {
		t.Fatal("archived wardrobe item was accepted into a new outfit plan")
	}
	if created.Items[1].Content == nil || created.Items[1].Content.Availability != wardrobeapp.WardrobeLaundry {
		t.Fatal("archiving a wardrobe item rewrote the existing plan snapshot")
	}
	_, err = wardrobe.RestoreWardrobeItem(ctx, owner.Token, shoes.ID, archivedShoes.Revision)
	serviceOK(t, "restore archived outfit item", err)
	if _, err := outfits.GetOutfitPlan(ctx, other.Token, firstPlanID); !errors.Is(err, outfitplanapp.ErrOutfitPlanNotFound) {
		t.Fatal("cross-owner outfit plan lookup did not return uniform not-found")
	}

	updatedTop, err := wardrobe.UpdateWardrobeItem(ctx, owner.Token, top.ID, top.Revision, wardrobeapp.UpdateWardrobeItemInput{
		Name: "Navy Shirt", Category: wardrobeapp.WardrobeTop, Availability: wardrobeapp.WardrobeWearable,
		Attributes: wardrobeapp.WardrobeAttributes{FormalityBand: wardrobeTestValue(wardrobeapp.WardrobeFormalityFormal)},
	})
	serviceOK(t, "update wardrobe after outfit snapshot", err)
	restarted, err := outfitplanapp.NewOutfitPlanService(accounts, store.NewOutfitPlanRepository(database))
	serviceOK(t, "reconstruct outfit plan service", err)
	snapshotted, err := restarted.GetOutfitPlan(ctx, owner.Token, firstPlanID)
	serviceOK(t, "read outfit plan after service reconstruction", err)
	if snapshotted.Items[0].Content == nil || snapshotted.Items[0].Content.Name != "Blue Shirt" || snapshotted.Items[0].Content.ItemRevision != top.Revision {
		t.Fatal("wardrobe update rewrote an existing outfit snapshot")
	}
	staleUpdate := outfitplanapp.OutfitPlanInput{LocalDate: secondDate, TimeZone: "Asia/Shanghai", Items: []outfitplanapp.OutfitSelection{{ItemID: top.ID, Revision: top.Revision}}}
	if _, err := outfits.UpdateOutfitPlan(ctx, owner.Token, firstPlanID, created.Revision, staleUpdate); !errors.Is(err, outfitplanapp.ErrOutfitPlanConflict) {
		t.Fatal("outfit plan update accepted a stale wardrobe revision")
	}
	updatedSummary := "Dinner"
	currentUpdate := outfitplanapp.OutfitPlanInput{LocalDate: secondDate, TimeZone: "Asia/Shanghai", ContextSummary: &updatedSummary, Items: []outfitplanapp.OutfitSelection{{ItemID: top.ID, Revision: updatedTop.Revision}}}
	updatedPlan, err := outfits.UpdateOutfitPlan(ctx, owner.Token, firstPlanID, created.Revision, currentUpdate)
	serviceOK(t, "update outfit plan with current wardrobe revision", err)
	if updatedPlan.Revision != 2 || updatedPlan.LocalDate != secondDate || len(updatedPlan.Items) != 1 || updatedPlan.Items[0].Content == nil || updatedPlan.Items[0].Content.Name != "Navy Shirt" {
		t.Fatal("outfit plan update missed replacement or revision semantics")
	}

	secondPlanID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400d02"
	secondInput := outfitplanapp.OutfitPlanInput{LocalDate: secondDate, TimeZone: "Asia/Shanghai", Items: []outfitplanapp.OutfitSelection{{ItemID: top.ID, Revision: updatedTop.Revision}}}
	secondPlan, err := outfits.CreateOutfitPlan(ctx, owner.Token, secondPlanID, secondInput)
	serviceOK(t, "create second outfit plan", err)
	firstPage, err := outfits.ListOutfitPlans(ctx, owner.Token, 1, nil, nil)
	serviceOK(t, "list first outfit plan page", err)
	if len(firstPage.Plans) != 1 || firstPage.NextAfterID == nil {
		t.Fatal("first outfit plan page did not return a bounded continuation")
	}
	secondPage, err := outfits.ListOutfitPlans(ctx, owner.Token, 1, firstPage.NextAfterID, nil)
	serviceOK(t, "list second outfit plan page", err)
	if len(secondPage.Plans) != 1 || secondPage.NextAfterID != nil || secondPage.Plans[0].ID == firstPage.Plans[0].ID {
		t.Fatal("outfit plan pagination duplicated or omitted a plan")
	}
	filtered, err := outfits.ListOutfitPlans(ctx, owner.Token, 10, nil, &secondDate)
	serviceOK(t, "filter outfit plans by local date", err)
	if len(filtered.Plans) != 2 || filtered.Plans[0].ID == filtered.Plans[1].ID {
		t.Fatal("outfit plan local-date filter lost same-day plans")
	}
	if _, err := outfits.ListOutfitPlans(ctx, other.Token, 1, firstPage.NextAfterID, nil); !errors.Is(err, outfitplanapp.ErrOutfitPlanNotFound) {
		t.Fatal("cross-owner outfit cursor leaked a different result")
	}

	cancelled, err := outfits.CancelOutfitPlan(ctx, owner.Token, firstPlanID, updatedPlan.Revision)
	serviceOK(t, "cancel current outfit plan", err)
	if cancelled.Status != outfitplanapp.OutfitPlanCancelled || cancelled.Revision != 3 || len(cancelled.Items) != 1 {
		t.Fatal("cancelled outfit plan lost its snapshot or revision")
	}
	if _, err := outfits.UpdateOutfitPlan(ctx, owner.Token, firstPlanID, cancelled.Revision, currentUpdate); !errors.Is(err, outfitplanapp.ErrOutfitPlanConflict) {
		t.Fatal("cancelled outfit plan remained editable")
	}
	serviceOK(t, "delete cancelled outfit plan", outfits.DeleteOutfitPlan(ctx, owner.Token, firstPlanID, cancelled.Revision))
	if _, err := outfits.GetOutfitPlan(ctx, owner.Token, firstPlanID); !errors.Is(err, outfitplanapp.ErrOutfitPlanNotFound) {
		t.Fatal("deleted outfit plan remained readable")
	}
	if _, err := outfits.CreateOutfitPlan(ctx, owner.Token, firstPlanID, currentUpdate); !errors.Is(err, outfitplanapp.ErrOutfitPlanConflict) {
		t.Fatal("outfit plan tombstone allowed a late recreate")
	}

	staleImpact, err := wardrobe.GetWardrobeDeletionImpact(ctx, owner.Token, top.ID)
	serviceOK(t, "read initial wardrobe deletion impact", err)
	if staleImpact.AffectedPlanCount != 1 {
		t.Fatal("wardrobe deletion impact missed an affected outfit plan")
	}
	thirdPlanID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400d03"
	thirdPlan, err := outfits.CreateOutfitPlan(ctx, owner.Token, thirdPlanID, outfitplanapp.OutfitPlanInput{LocalDate: secondDate, TimeZone: "Asia/Shanghai", Items: []outfitplanapp.OutfitSelection{{ItemID: top.ID, Revision: updatedTop.Revision}}})
	serviceOK(t, "create plan after deletion impact preview", err)
	if err := wardrobe.DeleteWardrobeItem(ctx, owner.Token, top.ID, updatedTop.Revision, wardrobeapp.WardrobeHistoryRedactSnapshots, staleImpact.ExpectedImpact); !errors.Is(err, wardrobeapp.ErrWardrobeConflict) {
		t.Fatal("wardrobe deletion accepted a stale impact digest")
	}
	freshImpact, err := wardrobe.GetWardrobeDeletionImpact(ctx, owner.Token, top.ID)
	serviceOK(t, "refresh wardrobe deletion impact", err)
	if freshImpact.AffectedPlanCount != 2 {
		t.Fatal("refreshed wardrobe deletion impact did not include both plans")
	}
	serviceOK(t, "delete wardrobe item and redact snapshots", wardrobe.DeleteWardrobeItem(ctx, owner.Token, top.ID, updatedTop.Revision, wardrobeapp.WardrobeHistoryRedactSnapshots, freshImpact.ExpectedImpact))
	for _, expected := range []struct {
		id       string
		revision int
	}{{secondPlan.ID, secondPlan.Revision + 1}, {thirdPlan.ID, thirdPlan.Revision + 1}} {
		plan, err := outfits.GetOutfitPlan(ctx, owner.Token, expected.id)
		serviceOK(t, "read redacted outfit plan", err)
		if plan.Revision != expected.revision || len(plan.Items) != 1 || plan.Items[0].Content != nil {
			t.Fatal("wardrobe redaction did not clear the snapshot and advance plan revision")
		}
		assertSyncLatest(t, database, owner.User.ID, "outfit_plan", expected.id, "upsert", &plan.Revision)
	}

	bag, err := wardrobe.CreateWardrobeItem(ctx, owner.Token, wardrobeapp.CreateWardrobeItemInput{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400c03", Name: "Canvas Bag", Category: wardrobeapp.WardrobeBag, Availability: wardrobeapp.WardrobeWearable, Source: wardrobeapp.WardrobeSourceWardrobe})
	serviceOK(t, "create deletion-policy wardrobe item", err)
	fourthPlanID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400d04"
	fourthInput := outfitplanapp.OutfitPlanInput{LocalDate: firstDate, TimeZone: "Asia/Shanghai", Items: []outfitplanapp.OutfitSelection{{ItemID: bag.ID, Revision: bag.Revision}}}
	_, err = outfits.CreateOutfitPlan(ctx, owner.Token, fourthPlanID, fourthInput)
	serviceOK(t, "create deletion-policy outfit plan", err)
	bagImpact, err := wardrobe.GetWardrobeDeletionImpact(ctx, owner.Token, bag.ID)
	serviceOK(t, "read deletion-policy wardrobe impact", err)
	serviceOK(t, "delete wardrobe item and affected history", wardrobe.DeleteWardrobeItem(ctx, owner.Token, bag.ID, bag.Revision, wardrobeapp.WardrobeHistoryDeleteAffectedHistory, bagImpact.ExpectedImpact))
	if _, err := outfits.GetOutfitPlan(ctx, owner.Token, fourthPlanID); !errors.Is(err, outfitplanapp.ErrOutfitPlanNotFound) {
		t.Fatal("delete-affected-plans policy left an affected plan readable")
	}
	assertSyncLatest(t, database, owner.User.ID, "outfit_plan", fourthPlanID, "delete", nil)
	if _, err := outfits.CreateOutfitPlan(ctx, owner.Token, fourthPlanID, fourthInput); !errors.Is(err, outfitplanapp.ErrOutfitPlanConflict) {
		t.Fatal("delete-affected-plans policy omitted the plan tombstone")
	}
	if err := database.WithContext(ctx).Exec("UPDATE outfit_plans SET status = 'unknown' WHERE owner_id = ? AND id = ?", owner.User.ID, secondPlanID).Error; err == nil {
		t.Fatal("PostgreSQL accepted an invalid outfit plan status")
	}

	_, err = accounts.DeleteCurrentUser(ctx, owner.Token)
	serviceOK(t, "delete outfit plan owner", err)
	for _, table := range []string{"outfit_plans", "outfit_plan_items", "outfit_plan_deletions", "wardrobe_items"} {
		var count int64
		serviceOK(t, "count account-owned "+table, database.WithContext(ctx).Table(table).Where("owner_id = ?", owner.User.ID).Count(&count).Error)
		if count != 0 {
			t.Fatalf("account cascade left %d rows in %s", count, table)
		}
	}
}
