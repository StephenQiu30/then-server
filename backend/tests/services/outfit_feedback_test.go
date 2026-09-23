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
	feedbackapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitfeedback"
	outfitplanapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitplan"
	wardrobeapp "github.com/StephenQiu30/then-server/backend/internal/application/wardrobe"
	weareventapp "github.com/StephenQiu30/then-server/backend/internal/application/wearevent"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOutfitFeedbackPersistenceAndStatistics(t *testing.T) {
	env := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	base, err := gorm.Open(postgres.Open(env.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open feedback test database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create feedback schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := base.WithContext(cleanup).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("feedback schema cleanup failed")
		}
	})
	u, err := url.Parse(env.databaseURL)
	serviceOK(t, "parse database URL", err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open feedback schema", err)
	serviceOK(t, "migrate feedback schema", store.Migrate(ctx, db))
	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(db))
	serviceOK(t, "construct accounts", err)
	a, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "feedback-a@example.test", DisplayName: "A", Password: "feedback-password-a"})
	serviceOK(t, "register A", err)
	b, err := accounts.Register(ctx, accountapp.RegisterAccountInput{Email: "feedback-b@example.test", DisplayName: "B", Password: "feedback-password-b"})
	serviceOK(t, "register B", err)
	wardrobe, err := wardrobeapp.NewWardrobeService(accounts, store.NewWardrobeRepository(db))
	serviceOK(t, "construct wardrobe", err)
	shirt, err := wardrobe.CreateWardrobeItem(ctx, a.Token, wardrobeapp.CreateWardrobeItemInput{ID: "018f1f74-a2d0-7c6d-9c17-4a0ea2400b01", Name: "Shirt", Category: wardrobeapp.WardrobeTop, Availability: wardrobeapp.WardrobeWearable, Source: wardrobeapp.WardrobeSourceWardrobe})
	serviceOK(t, "create shirt", err)
	wear, err := weareventapp.NewWearEventService(accounts, store.NewWearEventRepository(db))
	serviceOK(t, "construct wear", err)
	feedback, err := feedbackapp.NewService(accounts, store.NewOutfitFeedbackRepository(db))
	serviceOK(t, "construct feedback", err)
	today := time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02")
	eventID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400b02"
	wearInput := weareventapp.WearEventInput{LocalDate: today, TimeZone: "Asia/Shanghai", Completeness: weareventapp.WearEventComplete, Items: []outfitplanapp.OutfitSelection{{ItemID: shirt.ID, Revision: shirt.Revision}}, SourceKind: weareventapp.WearEventUnplanned}
	_, err = wear.CreateWearEvent(ctx, a.Token, eventID, wearInput)
	serviceOK(t, "create wear event", err)
	stats, err := feedback.Statistics(ctx, a.Token, today, today)
	serviceOK(t, "read initial statistics", err)
	if stats.WearEvents != 1 || stats.WearDays != 1 || len(stats.ItemUses) != 1 || stats.ItemUses[0].Count != 1 {
		t.Fatal("actual event is not counted from live facts")
	}
	otherStats, err := feedback.Statistics(ctx, b.Token, today, today)
	serviceOK(t, "read other account statistics", err)
	if otherStats.WearEvents != 0 || otherStats.WearDays != 0 || len(otherStats.ItemUses) != 0 {
		t.Fatal("other account received private statistics")
	}
	if _, err := feedback.Get(ctx, b.Token, eventID); !errors.Is(err, feedbackapp.ErrNotFound) {
		t.Fatal("other account read feedback")
	}
	id := "018f1f74-a2d0-7c6d-9c17-4a0ea2400b03"
	mutation := "018f1f74-a2d0-7c6d-9c17-4a0ea2400b04"
	cold := "cold"
	input := feedbackapp.Input{ThermalComfort: &cold, IssueTags: []string{"rainUnsuitable"}}
	created, err := feedback.Save(ctx, a.Token, eventID, id, mutation, nil, input)
	serviceOK(t, "create feedback", err)
	if created.Revision != 1 || *created.Input.ThermalComfort != cold {
		t.Fatal("feedback was not saved")
	}
	repeated, err := feedback.Save(ctx, a.Token, eventID, id, mutation, nil, input)
	serviceOK(t, "retry feedback", err)
	if repeated.Revision != 1 {
		t.Fatal("feedback retry changed revision")
	}
	changed := "hot"
	if _, err := feedback.Save(ctx, a.Token, eventID, id, mutation, nil, feedbackapp.Input{ThermalComfort: &changed}); !errors.Is(err, feedbackapp.ErrConflict) {
		t.Fatal("same mutation ID accepted changed content")
	}
	stale := 2
	if _, err := feedback.Save(ctx, a.Token, eventID, id, "018f1f74-a2d0-7c6d-9c17-4a0ea2400b05", &stale, feedbackapp.Input{ThermalComfort: &changed}); !errors.Is(err, feedbackapp.ErrConflict) {
		t.Fatal("stale revision saved feedback")
	}
	current := 1
	updated, err := feedback.Save(ctx, a.Token, eventID, id, "018f1f74-a2d0-7c6d-9c17-4a0ea2400b06", &current, feedbackapp.Input{ThermalComfort: &changed})
	serviceOK(t, "correct feedback", err)
	if updated.Revision != 2 || *updated.Input.ThermalComfort != changed {
		t.Fatal("feedback correction did not replace value")
	}
	if err := feedback.Delete(ctx, a.Token, eventID, id, "018f1f74-a2d0-7c6d-9c17-4a0ea2400b07", 1); !errors.Is(err, feedbackapp.ErrConflict) {
		t.Fatal("stale delete removed feedback")
	}
	deleteID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400b08"
	serviceOK(t, "delete feedback", feedback.Delete(ctx, a.Token, eventID, id, deleteID, 2))
	serviceOK(t, "retry feedback delete", feedback.Delete(ctx, a.Token, eventID, id, deleteID, 2))
	if _, err := feedback.Get(ctx, a.Token, eventID); !errors.Is(err, feedbackapp.ErrNotFound) {
		t.Fatal("deleted feedback remained readable")
	}
	if _, err := feedback.Save(ctx, b.Token, eventID, id, "018f1f74-a2d0-7c6d-9c17-4a0ea2400b09", nil, input); !errors.Is(err, feedbackapp.ErrNotFound) {
		t.Fatal("other account wrote feedback")
	}
	if _, err := feedback.Save(ctx, a.Token, eventID, id, "018f1f74-a2d0-7c6d-9c17-4a0ea2400b0a", nil, input); err != nil {
		t.Fatal("new command could not recreate feedback")
	}
	yesterday := time.Now().In(time.FixedZone("CST", 8*3600)).AddDate(0, 0, -1).Format("2006-01-02")
	wearInput.LocalDate = yesterday
	corrected, err := wear.UpdateWearEvent(ctx, a.Token, eventID, 1, wearInput)
	serviceOK(t, "correct wear event date", err)
	if corrected.Revision != 2 {
		t.Fatal("wear event correction did not advance revision")
	}
	stats, err = feedback.Statistics(ctx, a.Token, today, today)
	serviceOK(t, "read corrected day", err)
	if stats.WearEvents != 0 || stats.WearDays != 0 {
		t.Fatal("original day retained corrected event")
	}
	stats, err = feedback.Statistics(ctx, a.Token, yesterday, yesterday)
	serviceOK(t, "read new day", err)
	if stats.WearEvents != 1 || stats.WearDays != 1 {
		t.Fatal("new day did not receive corrected event")
	}
	serviceOK(t, "delete wear event", wear.DeleteWearEvent(ctx, a.Token, eventID, 2))
	if _, err := feedback.Get(ctx, a.Token, eventID); !errors.Is(err, feedbackapp.ErrNotFound) {
		t.Fatal("deleted event retained feedback")
	}
	stats, err = feedback.Statistics(ctx, a.Token, today, today)
	serviceOK(t, "recompute statistics", err)
	if stats.WearEvents != 0 || stats.WearDays != 0 || len(stats.ItemUses) != 0 {
		t.Fatal("deleted event remained in statistics")
	}
	var rows int64
	serviceOK(t, "count feedback receipts", db.Table("wear_feedback_mutations").Where("event_id = ?", eventID).Count(&rows).Error)
	if rows != 0 {
		t.Fatal("deleted event retained mutation receipts")
	}
	accountEventID := "018f1f74-a2d0-7c6d-9c17-4a0ea2400b0b"
	wearInput.LocalDate = today
	_, err = wear.CreateWearEvent(ctx, a.Token, accountEventID, wearInput)
	serviceOK(t, "create account deletion event", err)
	_, err = feedback.Save(ctx, a.Token, accountEventID, "018f1f74-a2d0-7c6d-9c17-4a0ea2400b0c", "018f1f74-a2d0-7c6d-9c17-4a0ea2400b0d", nil, input)
	serviceOK(t, "create account deletion feedback", err)
	_, err = accounts.DeleteCurrentUser(ctx, a.Token)
	serviceOK(t, "delete feedback owner", err)
	for _, table := range []string{"wear_feedback", "wear_feedback_mutations"} {
		serviceOK(t, "count account-owned feedback", db.Table(table).Where("owner_id = ?", a.User.ID).Count(&rows).Error)
		if rows != 0 {
			t.Fatal("account deletion retained feedback data")
		}
	}
}
