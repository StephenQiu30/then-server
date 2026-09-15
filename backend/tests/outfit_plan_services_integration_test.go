//go:build services

package tests

import (
	"context"
	"errors"
	"net/url"
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
	serviceOK(t, "migrate outfit plan schema", repository.Migrate(ctx, database))

	accounts, err := service.NewAccountService(repository.NewAccountRepository(database))
	serviceOK(t, "construct outfit plan account service", err)
	owner, err := accounts.Register(ctx, model.RegisterAccountInput{Email: "outfit-owner@example.test", DisplayName: "Owner", Password: "correct-password-owner"})
	serviceOK(t, "register outfit plan owner", err)
	other, err := accounts.Register(ctx, model.RegisterAccountInput{Email: "outfit-other@example.test", DisplayName: "Other", Password: "correct-password-other"})
	serviceOK(t, "register second outfit plan owner", err)
	wardrobe, err := service.NewWardrobeService(accounts, repository.NewWardrobeRepository(database))
	serviceOK(t, "construct outfit plan wardrobe service", err)
	outfits, err := service.NewOutfitPlanService(accounts, repository.NewOutfitPlanRepository(database))
	serviceOK(t, "construct outfit plan service", err)

	top, err := wardrobe.CreateWardrobeItem(ctx, owner.Token, model.CreateWardrobeItemInput{
		ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400c01", Name: "Blue Shirt", Category: model.WardrobeTop,
		Availability: model.WardrobeWearable, Source: model.WardrobeSourceWardrobe,
		Attributes: model.WardrobeAttributes{FormalityBand: wardrobeTestValue(model.WardrobeFormalitySmartCasual), WarmthBand: wardrobeTestValue(model.WardrobeWarmthLight)},
	})
	serviceOK(t, "create wearable outfit item", err)
	shoes, err := wardrobe.CreateWardrobeItem(ctx, owner.Token, model.CreateWardrobeItemInput{
		ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400c02", Name: "Walking Shoes", Category: model.WardrobeShoes,
		Availability: model.WardrobeLaundry, Source: model.WardrobeSourceQuickAdd,
		Attributes: model.WardrobeAttributes{WalkingUse: wardrobeTestValue(model.WardrobeUseSuitable)},
	})
	serviceOK(t, "create unavailable outfit item", err)

	location, err := time.LoadLocation("Asia/Shanghai")
	serviceOK(t, "load outfit plan timezone", err)
	firstDate := time.Now().In(location).AddDate(0, 0, 1).Format("2006-01-02")
	secondDate := time.Now().In(location).AddDate(0, 0, 2).Format("2006-01-02")
	firstPlanID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400d01"
	summary := "Office day"
	firstInput := model.OutfitPlanInput{
		LocalDate: firstDate, TimeZone: "Asia/Shanghai", ContextSummary: &summary,
		Items:                   []model.OutfitSelection{{ItemID: top.ID, Revision: top.Revision}, {ItemID: shoes.ID, Revision: shoes.Revision}},
		ConfirmedUnavailableIDs: []string{shoes.ID},
	}
	created, err := outfits.CreateOutfitPlan(ctx, owner.Token, firstPlanID, firstInput)
	serviceOK(t, "create outfit plan with confirmed unavailable item", err)
	if created.Revision != 1 || created.Status != model.OutfitPlanActive || len(created.Items) != 2 || created.Items[0].Content == nil || created.Items[0].Content.Name != "Blue Shirt" || created.Items[0].Content.Attributes.FormalityBand == nil {
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
	if _, err := outfits.CreateOutfitPlan(ctx, owner.Token, firstPlanID, changedInput); !errors.Is(err, model.ErrOutfitPlanConflict) {
		t.Fatal("same outfit plan ID accepted different content")
	}
	if _, err := outfits.GetOutfitPlan(ctx, other.Token, firstPlanID); !errors.Is(err, model.ErrOutfitPlanNotFound) {
		t.Fatal("cross-owner outfit plan lookup did not return uniform not-found")
	}

	updatedTop, err := wardrobe.UpdateWardrobeItem(ctx, owner.Token, top.ID, top.Revision, model.UpdateWardrobeItemInput{
		Name: "Navy Shirt", Category: model.WardrobeTop, Availability: model.WardrobeWearable,
		Attributes: model.WardrobeAttributes{FormalityBand: wardrobeTestValue(model.WardrobeFormalityFormal)},
	})
	serviceOK(t, "update wardrobe after outfit snapshot", err)
	restarted, err := service.NewOutfitPlanService(accounts, repository.NewOutfitPlanRepository(database))
	serviceOK(t, "reconstruct outfit plan service", err)
	snapshotted, err := restarted.GetOutfitPlan(ctx, owner.Token, firstPlanID)
	serviceOK(t, "read outfit plan after service reconstruction", err)
	if snapshotted.Items[0].Content == nil || snapshotted.Items[0].Content.Name != "Blue Shirt" || snapshotted.Items[0].Content.ItemRevision != top.Revision {
		t.Fatal("wardrobe update rewrote an existing outfit snapshot")
	}
	staleUpdate := model.OutfitPlanInput{LocalDate: secondDate, TimeZone: "Asia/Shanghai", Items: []model.OutfitSelection{{ItemID: top.ID, Revision: top.Revision}}}
	if _, err := outfits.UpdateOutfitPlan(ctx, owner.Token, firstPlanID, created.Revision, staleUpdate); !errors.Is(err, model.ErrOutfitPlanConflict) {
		t.Fatal("outfit plan update accepted a stale wardrobe revision")
	}
	updatedSummary := "Dinner"
	currentUpdate := model.OutfitPlanInput{LocalDate: secondDate, TimeZone: "Asia/Shanghai", ContextSummary: &updatedSummary, Items: []model.OutfitSelection{{ItemID: top.ID, Revision: updatedTop.Revision}}}
	updatedPlan, err := outfits.UpdateOutfitPlan(ctx, owner.Token, firstPlanID, created.Revision, currentUpdate)
	serviceOK(t, "update outfit plan with current wardrobe revision", err)
	if updatedPlan.Revision != 2 || updatedPlan.LocalDate != secondDate || len(updatedPlan.Items) != 1 || updatedPlan.Items[0].Content == nil || updatedPlan.Items[0].Content.Name != "Navy Shirt" {
		t.Fatal("outfit plan update missed replacement or revision semantics")
	}

	secondPlanID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400d02"
	secondInput := model.OutfitPlanInput{LocalDate: secondDate, TimeZone: "Asia/Shanghai", Items: []model.OutfitSelection{{ItemID: top.ID, Revision: updatedTop.Revision}}}
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
	if _, err := outfits.ListOutfitPlans(ctx, other.Token, 1, firstPage.NextAfterID, nil); !errors.Is(err, model.ErrOutfitPlanNotFound) {
		t.Fatal("cross-owner outfit cursor leaked a different result")
	}

	cancelled, err := outfits.CancelOutfitPlan(ctx, owner.Token, firstPlanID, updatedPlan.Revision)
	serviceOK(t, "cancel current outfit plan", err)
	if cancelled.Status != model.OutfitPlanCancelled || cancelled.Revision != 3 || len(cancelled.Items) != 1 {
		t.Fatal("cancelled outfit plan lost its snapshot or revision")
	}
	if _, err := outfits.UpdateOutfitPlan(ctx, owner.Token, firstPlanID, cancelled.Revision, currentUpdate); !errors.Is(err, model.ErrOutfitPlanConflict) {
		t.Fatal("cancelled outfit plan remained editable")
	}
	serviceOK(t, "delete cancelled outfit plan", outfits.DeleteOutfitPlan(ctx, owner.Token, firstPlanID, cancelled.Revision))
	if _, err := outfits.GetOutfitPlan(ctx, owner.Token, firstPlanID); !errors.Is(err, model.ErrOutfitPlanNotFound) {
		t.Fatal("deleted outfit plan remained readable")
	}
	if _, err := outfits.CreateOutfitPlan(ctx, owner.Token, firstPlanID, currentUpdate); !errors.Is(err, model.ErrOutfitPlanConflict) {
		t.Fatal("outfit plan tombstone allowed a late recreate")
	}

	staleImpact, err := wardrobe.GetWardrobeDeletionImpact(ctx, owner.Token, top.ID)
	serviceOK(t, "read initial wardrobe deletion impact", err)
	if staleImpact.AffectedPlanCount != 1 {
		t.Fatal("wardrobe deletion impact missed an affected outfit plan")
	}
	thirdPlanID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400d03"
	thirdPlan, err := outfits.CreateOutfitPlan(ctx, owner.Token, thirdPlanID, model.OutfitPlanInput{LocalDate: secondDate, TimeZone: "Asia/Shanghai", Items: []model.OutfitSelection{{ItemID: top.ID, Revision: updatedTop.Revision}}})
	serviceOK(t, "create plan after deletion impact preview", err)
	if err := wardrobe.DeleteWardrobeItem(ctx, owner.Token, top.ID, updatedTop.Revision, model.WardrobeHistoryRedactSnapshots, staleImpact.ExpectedImpact); !errors.Is(err, model.ErrWardrobeConflict) {
		t.Fatal("wardrobe deletion accepted a stale impact digest")
	}
	freshImpact, err := wardrobe.GetWardrobeDeletionImpact(ctx, owner.Token, top.ID)
	serviceOK(t, "refresh wardrobe deletion impact", err)
	if freshImpact.AffectedPlanCount != 2 {
		t.Fatal("refreshed wardrobe deletion impact did not include both plans")
	}
	serviceOK(t, "delete wardrobe item and redact snapshots", wardrobe.DeleteWardrobeItem(ctx, owner.Token, top.ID, updatedTop.Revision, model.WardrobeHistoryRedactSnapshots, freshImpact.ExpectedImpact))
	for _, expected := range []struct {
		id       string
		revision int
	}{{secondPlan.ID, secondPlan.Revision + 1}, {thirdPlan.ID, thirdPlan.Revision + 1}} {
		plan, err := outfits.GetOutfitPlan(ctx, owner.Token, expected.id)
		serviceOK(t, "read redacted outfit plan", err)
		if plan.Revision != expected.revision || len(plan.Items) != 1 || plan.Items[0].Content != nil {
			t.Fatal("wardrobe redaction did not clear the snapshot and advance plan revision")
		}
	}

	bag, err := wardrobe.CreateWardrobeItem(ctx, owner.Token, model.CreateWardrobeItemInput{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400c03", Name: "Canvas Bag", Category: model.WardrobeBag, Availability: model.WardrobeWearable, Source: model.WardrobeSourceWardrobe})
	serviceOK(t, "create deletion-policy wardrobe item", err)
	fourthPlanID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400d04"
	fourthInput := model.OutfitPlanInput{LocalDate: firstDate, TimeZone: "Asia/Shanghai", Items: []model.OutfitSelection{{ItemID: bag.ID, Revision: bag.Revision}}}
	_, err = outfits.CreateOutfitPlan(ctx, owner.Token, fourthPlanID, fourthInput)
	serviceOK(t, "create deletion-policy outfit plan", err)
	bagImpact, err := wardrobe.GetWardrobeDeletionImpact(ctx, owner.Token, bag.ID)
	serviceOK(t, "read deletion-policy wardrobe impact", err)
	serviceOK(t, "delete wardrobe item and affected history", wardrobe.DeleteWardrobeItem(ctx, owner.Token, bag.ID, bag.Revision, model.WardrobeHistoryDeleteAffectedHistory, bagImpact.ExpectedImpact))
	if _, err := outfits.GetOutfitPlan(ctx, owner.Token, fourthPlanID); !errors.Is(err, model.ErrOutfitPlanNotFound) {
		t.Fatal("delete-affected-plans policy left an affected plan readable")
	}
	if _, err := outfits.CreateOutfitPlan(ctx, owner.Token, fourthPlanID, fourthInput); !errors.Is(err, model.ErrOutfitPlanConflict) {
		t.Fatal("delete-affected-plans policy omitted the plan tombstone")
	}
	if err := database.WithContext(ctx).Exec("UPDATE outfit_plans SET status = 'unknown' WHERE owner_id = ? AND id = ?", owner.User.ID, secondPlanID).Error; err == nil {
		t.Fatal("PostgreSQL accepted an invalid outfit plan status")
	}

	serviceOK(t, "delete outfit plan owner", accounts.DeleteCurrentUser(ctx, owner.Token))
	for _, table := range []string{"outfit_plans", "outfit_plan_items", "outfit_plan_deletions", "wardrobe_items"} {
		var count int64
		serviceOK(t, "count account-owned "+table, database.WithContext(ctx).Table(table).Where("owner_id = ?", owner.User.ID).Count(&count).Error)
		if count != 0 {
			t.Fatalf("account cascade left %d rows in %s", count, table)
		}
	}
}
