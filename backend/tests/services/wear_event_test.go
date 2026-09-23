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
	weareventapp "github.com/StephenQiu30/then-server/backend/internal/application/wearevent"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestWearEventPersistenceLifecycle(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base, err := gorm.Open(postgres.Open(environment.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open wear event test database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create wear event test schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := base.WithContext(cleanup).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("wear event schema cleanup failed")
		}
	})
	databaseURL, err := url.Parse(environment.databaseURL)
	serviceOK(t, "parse wear event test database", err)
	query := databaseURL.Query()
	query.Set("search_path", schema)
	databaseURL.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open isolated wear event schema", err)
	serviceOK(t, "migrate wear event schema", store.Migrate(ctx, database))

	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	serviceOK(t, "construct wear event account service", err)
	owner, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "wear-owner@example.test", DisplayName: "Owner", Password: "correct-password-owner"})
	serviceOK(t, "register wear event owner", err)
	other, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "wear-other@example.test", DisplayName: "Other", Password: "correct-password-other"})
	serviceOK(t, "register other wear event owner", err)
	wardrobe, err := wardrobeapp.NewWardrobeService(accounts, store.NewWardrobeRepository(database))
	serviceOK(t, "construct wear event wardrobe service", err)
	outfits, err := outfitplanapp.NewOutfitPlanService(accounts, store.NewOutfitPlanRepository(database))
	serviceOK(t, "construct wear event outfit service", err)
	wear, err := weareventapp.NewWearEventService(accounts, store.NewWearEventRepository(database))
	serviceOK(t, "construct wear event service", err)

	shirt, err := wardrobe.CreateWardrobeItem(ctx, owner.Token, wardrobeapp.CreateWardrobeItemInput{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400f01", Name: "Blue Shirt", Category: wardrobeapp.WardrobeTop, Availability: wardrobeapp.WardrobeWearable, Source: wardrobeapp.WardrobeSourceWardrobe})
	serviceOK(t, "create wear event wardrobe item", err)
	location, err := time.LoadLocation("Asia/Shanghai")
	serviceOK(t, "load wear event timezone", err)
	today := time.Now().In(location).Format("2006-01-02")
	planID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400f02"
	plan, err := outfits.CreateOutfitPlan(ctx, owner.Token, planID, outfitplanapp.OutfitPlanInput{LocalDate: today, TimeZone: "Asia/Shanghai", Items: []outfitplanapp.OutfitSelection{{ItemID: shirt.ID, Revision: shirt.Revision}}})
	serviceOK(t, "create source outfit plan", err)
	notWorn, err := outfits.MarkOutfitPlanNotWorn(ctx, owner.Token, planID, plan.Revision)
	serviceOK(t, "mark source plan not worn", err)
	if notWorn.Status != outfitplanapp.OutfitPlanNotWorn || notWorn.Revision != plan.Revision+1 {
		t.Fatal("not-worn transition did not persist")
	}
	plan, err = outfits.RestoreOutfitPlan(ctx, owner.Token, planID, notWorn.Revision)
	serviceOK(t, "restore source plan", err)
	if plan.Status != outfitplanapp.OutfitPlanActive || plan.Revision != notWorn.Revision+1 {
		t.Fatal("restore transition did not return the plan to active")
	}
	eventID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400f03"
	sourceRevision := plan.Revision
	input := weareventapp.WearEventInput{LocalDate: today, TimeZone: "Asia/Shanghai", Completeness: weareventapp.WearEventComplete, Items: []outfitplanapp.OutfitSelection{{ItemID: shirt.ID, Revision: shirt.Revision}}, LaundryItemIDs: []string{shirt.ID}, SourcePlanID: &planID, SourcePlanRevision: &sourceRevision, SourceKind: weareventapp.WearEventFollowedPlan}
	created, err := wear.CreateWearEvent(ctx, owner.Token, eventID, input)
	serviceOK(t, "create planned wear event", err)
	if created.Revision != 1 || len(created.Items) != 1 || created.Items[0].Content == nil || created.Items[0].Content.Name != "Blue Shirt" {
		t.Fatal("wear event did not preserve its server snapshot")
	}
	repeated, err := wear.CreateWearEvent(ctx, owner.Token, eventID, input)
	serviceOK(t, "repeat idempotent wear event create", err)
	if repeated.Revision != created.Revision || !repeated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatal("idempotent wear event create changed persisted facts")
	}
	changedCommand := input
	changedCommand.LaundryItemIDs = nil
	if _, err := wear.CreateWearEvent(ctx, owner.Token, eventID, changedCommand); !errors.Is(err, weareventapp.ErrWearEventConflict) {
		t.Fatal("same wear event ID accepted a create command with different side effects")
	}
	if _, err := wear.GetWearEvent(ctx, other.Token, eventID); !errors.Is(err, weareventapp.ErrWearEventNotFound) {
		t.Fatal("cross-owner wear event lookup did not return not-found")
	}
	completed, err := outfits.GetOutfitPlan(ctx, owner.Token, planID)
	serviceOK(t, "read completed source plan", err)
	if completed.Status != outfitplanapp.OutfitPlanCompleted || completed.Revision != plan.Revision+1 {
		t.Fatal("wear event did not complete its source plan")
	}
	currentShirt, err := wardrobe.GetWardrobeItem(ctx, owner.Token, shirt.ID)
	serviceOK(t, "read laundry wardrobe item", err)
	if currentShirt.Availability != wardrobeapp.WardrobeLaundry || currentShirt.Revision != shirt.Revision+1 {
		t.Fatal("wear event laundry selection was not committed atomically")
	}

	secondID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400f04"
	secondInput := weareventapp.WearEventInput{LocalDate: today, TimeZone: "Asia/Shanghai", Completeness: weareventapp.WearEventPartial, Items: []outfitplanapp.OutfitSelection{{ItemID: currentShirt.ID, Revision: currentShirt.Revision}}, ConfirmedUnavailableIDs: []string{currentShirt.ID}, SourceKind: weareventapp.WearEventUnplanned}
	if _, err := wear.CreateWearEvent(ctx, owner.Token, secondID, secondInput); !errors.Is(err, weareventapp.ErrWearEventDuplicate) {
		t.Fatal("similar same-day wear event did not require explicit confirmation")
	} else {
		var duplicate *weareventapp.WearEventDuplicateError
		if !errors.As(err, &duplicate) || len(duplicate.Candidates) != 1 || duplicate.Candidates[0].ID != eventID {
			t.Fatal("duplicate response lost the current candidate revision")
		}
		secondInput.DuplicateConfirmations = duplicate.Candidates
	}
	second, err := wear.CreateWearEvent(ctx, owner.Token, secondID, secondInput)
	serviceOK(t, "confirm similar wear event", err)
	page, err := wear.ListWearEvents(ctx, owner.Token, 1, nil, &today)
	serviceOK(t, "list first wear event page", err)
	if len(page.Events) != 1 || page.NextAfterID == nil {
		t.Fatal("wear event pagination did not return a continuation")
	}
	next, err := wear.ListWearEvents(ctx, owner.Token, 1, page.NextAfterID, &today)
	serviceOK(t, "list second wear event page", err)
	if len(next.Events) != 1 || next.Events[0].ID == page.Events[0].ID {
		t.Fatal("wear event pagination duplicated or omitted history")
	}

	serviceOK(t, "delete planned wear event", wear.DeleteWearEvent(ctx, owner.Token, eventID, created.Revision))
	activeAgain, err := outfits.GetOutfitPlan(ctx, owner.Token, planID)
	serviceOK(t, "read restored active plan", err)
	if activeAgain.Status != outfitplanapp.OutfitPlanActive || activeAgain.Revision != completed.Revision+1 {
		t.Fatal("deleting the last linked wear event did not restore the plan")
	}
	if _, err := wear.CreateWearEvent(ctx, owner.Token, eventID, input); !errors.Is(err, weareventapp.ErrWearEventConflict) {
		t.Fatal("wear event tombstone allowed late recreation")
	}

	impact, err := wardrobe.GetWardrobeDeletionImpact(ctx, owner.Token, currentShirt.ID)
	serviceOK(t, "read wear-aware wardrobe deletion impact", err)
	if impact.AffectedPlanCount != 1 || impact.AffectedWearEventCount != 1 {
		t.Fatalf("wear-aware deletion impact mismatch: plans=%d events=%d", impact.AffectedPlanCount, impact.AffectedWearEventCount)
	}
	serviceOK(t, "redact wardrobe history", wardrobe.DeleteWardrobeItem(ctx, owner.Token, currentShirt.ID, currentShirt.Revision, wardrobeapp.WardrobeHistoryRedactSnapshots, impact.ExpectedImpact))
	redacted, err := wear.GetWearEvent(ctx, owner.Token, second.ID)
	serviceOK(t, "read redacted wear event", err)
	if redacted.Revision != second.Revision+1 || redacted.Items[0].Content != nil {
		t.Fatal("wardrobe deletion did not redact and revise wear history")
	}

	_, err = accounts.DeleteCurrentUser(ctx, owner.Token)
	serviceOK(t, "delete wear event owner", err)
	for _, table := range []string{"wear_events", "wear_event_items", "wear_event_deletions", "wear_feedback", "wear_feedback_mutations"} {
		var count int64
		serviceOK(t, "count account-owned "+table, database.WithContext(ctx).Table(table).Where("owner_id = ?", owner.User.ID).Count(&count).Error)
		if count != 0 {
			t.Fatalf("account cascade left %d rows in %s", count, table)
		}
	}
}
