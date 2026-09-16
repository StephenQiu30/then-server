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
	diaryapp "github.com/StephenQiu30/then-server/backend/internal/application/diary"
	outfitplanapp "github.com/StephenQiu30/then-server/backend/internal/application/outfitplan"
	wardrobeapp "github.com/StephenQiu30/then-server/backend/internal/application/wardrobe"
	weareventapp "github.com/StephenQiu30/then-server/backend/internal/application/wearevent"
	"github.com/StephenQiu30/then-server/backend/internal/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestDiaryPersistenceCalendarAndOwnershipLifecycle(t *testing.T) {
	environment := loadServiceEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base, err := gorm.Open(postgres.Open(environment.databaseURL), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open diary database", err)
	schema := strings.ReplaceAll(serviceID(t), "-", "_")
	serviceOK(t, "create diary schema", base.WithContext(ctx).Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := base.WithContext(cleanup).Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error("diary schema cleanup failed")
		}
	})
	databaseURL, err := url.Parse(environment.databaseURL)
	serviceOK(t, "parse diary database", err)
	query := databaseURL.Query()
	query.Set("search_path", schema)
	databaseURL.RawQuery = query.Encode()
	database, err := gorm.Open(postgres.Open(databaseURL.String()), &gorm.Config{Logger: logger.Discard})
	serviceOK(t, "open isolated diary schema", err)
	serviceOK(t, "migrate diary schema", store.Migrate(ctx, database))

	accounts, err := accountapp.NewAccountService(store.NewAccountRepository(database))
	serviceOK(t, "construct diary account service", err)
	owner, err := accounts.Register(ctx, domain.RegisterAccountInput{Email: "diary-owner@example.test", DisplayName: "Owner", Password: "correct-password-owner"})
	serviceOK(t, "register diary owner", err)
	other, err := accounts.Register(ctx, domain.RegisterAccountInput{Email: "diary-other@example.test", DisplayName: "Other", Password: "correct-password-other"})
	serviceOK(t, "register diary other", err)
	wardrobe, err := wardrobeapp.NewWardrobeService(accounts, store.NewWardrobeRepository(database))
	serviceOK(t, "construct diary wardrobe service", err)
	outfits, err := outfitplanapp.NewOutfitPlanService(accounts, store.NewOutfitPlanRepository(database))
	serviceOK(t, "construct diary outfit service", err)
	wear, err := weareventapp.NewWearEventService(accounts, store.NewWearEventRepository(database))
	serviceOK(t, "construct diary wear service", err)
	diaries, err := diaryapp.NewService(accounts, store.NewDiaryRepository(database))
	serviceOK(t, "construct diary service", err)

	location, err := time.LoadLocation("Asia/Shanghai")
	serviceOK(t, "load diary timezone", err)
	today := time.Now().In(location).Format("2006-01-02")
	tomorrow := time.Now().In(location).AddDate(0, 0, 1).Format("2006-01-02")
	body := "今天的穿搭"
	if _, err := diaries.Create(ctx, owner.Token, "10000000-0000-4000-8000-000000000001", domain.DiaryEntryInput{LocalDate: tomorrow, TimeZone: "Asia/Shanghai", Body: &body}); !errors.Is(err, domain.ErrInvalidDiaryInput) {
		t.Fatalf("future diary error=%v", err)
	}

	item, err := wardrobe.CreateWardrobeItem(ctx, owner.Token, domain.CreateWardrobeItemInput{ID: "10000000-0000-4000-8000-000000000010", Name: "Diary Shirt", Category: domain.WardrobeTop, Availability: domain.WardrobeWearable, Source: domain.WardrobeSourceWardrobe})
	serviceOK(t, "create diary wardrobe item", err)
	planID := "10000000-0000-4000-8000-000000000011"
	_, err = outfits.CreateOutfitPlan(ctx, owner.Token, planID, domain.OutfitPlanInput{LocalDate: today, TimeZone: "Asia/Shanghai", Items: []domain.OutfitSelection{{ItemID: item.ID, Revision: item.Revision}}})
	serviceOK(t, "create diary linked plan", err)
	eventID := "10000000-0000-4000-8000-000000000012"
	event, err := wear.CreateWearEvent(ctx, owner.Token, eventID, domain.WearEventInput{LocalDate: today, TimeZone: "Asia/Shanghai", Completeness: domain.WearEventComplete, Items: []domain.OutfitSelection{{ItemID: item.ID, Revision: item.Revision}}, SourceKind: domain.WearEventUnplanned})
	serviceOK(t, "create diary linked wear event", err)

	mediaRepository := store.NewMediaRepository(database)
	media, err := mediaRepository.CreateMedia(ctx, owner.User.ID, domain.CreateMediaUploadInput{Purpose: domain.MediaPurposeDiaryImage, ContentType: domain.MediaContentTypeJPEG, ByteSize: 128, SHA256: strings.Repeat("a", 64)}, time.Now().UTC())
	serviceOK(t, "create ordinary diary media", err)
	media, err = mediaRepository.CompleteMedia(ctx, owner.User.ID, media.ID, "test-version", domain.ObjectFact{ContentType: domain.MediaContentTypeJPEG, ByteSize: 128, SHA256: strings.Repeat("a", 64)}, time.Now().UTC())
	serviceOK(t, "complete ordinary diary media", err)
	media, process, err := mediaRepository.BeginMediaCheck(ctx, media.ID, time.Now().UTC())
	serviceOK(t, "begin ordinary diary media check", err)
	if !process || media.Purpose != domain.MediaPurposeDiaryImage || media.ConsentID != "" {
		t.Fatal("ordinary diary media reused person-photo semantics")
	}
	serviceOK(t, "finish ordinary diary media check", mediaRepository.CompleteMediaCheck(ctx, "10000000-0000-4000-8000-000000000099", media.ID, nil, 10, 10, domain.MediaReady, "ready", time.Now().UTC()))

	firstID := "10000000-0000-4000-8000-000000000020"
	first, err := diaries.Create(ctx, owner.Token, firstID, domain.DiaryEntryInput{LocalDate: today, TimeZone: "Asia/Shanghai", Body: &body})
	serviceOK(t, "create first text diary", err)
	repeated, err := diaries.Create(ctx, owner.Token, firstID, domain.DiaryEntryInput{LocalDate: today, TimeZone: "Asia/Shanghai", Body: &body})
	serviceOK(t, "repeat idempotent diary", err)
	if repeated.Revision != first.Revision || !repeated.CreatedAt.Equal(first.CreatedAt) {
		t.Fatal("idempotent diary create changed facts")
	}
	changed := "不同内容"
	if _, err := diaries.Create(ctx, owner.Token, firstID, domain.DiaryEntryInput{LocalDate: today, TimeZone: "Asia/Shanghai", Body: &changed}); !errors.Is(err, domain.ErrDiaryConflict) {
		t.Fatal("same diary id accepted different content")
	}
	if _, err := diaries.Get(ctx, other.Token, firstID); !errors.Is(err, domain.ErrDiaryNotFound) {
		t.Fatal("cross-owner diary read did not return not-found")
	}

	secondID := "10000000-0000-4000-8000-000000000021"
	second, err := diaries.Create(ctx, owner.Token, secondID, domain.DiaryEntryInput{LocalDate: today, TimeZone: "Asia/Shanghai", PlanID: &planID, WearEventID: &eventID, MediaIDs: []string{media.ID}})
	serviceOK(t, "create linked image diary", err)
	if second.Revision != 1 || len(second.MediaIDs) != 1 || second.PlanID == nil || second.WearEventID == nil {
		t.Fatal("linked diary lost ordered facts")
	}
	if _, err := diaries.Create(ctx, other.Token, "10000000-0000-4000-8000-000000000022", domain.DiaryEntryInput{LocalDate: today, TimeZone: "Asia/Shanghai", Body: &body, PlanID: &planID}); !errors.Is(err, domain.ErrDiaryNotFound) {
		t.Fatalf("cross-owner diary association error=%v", err)
	}

	page, err := diaries.List(ctx, owner.Token, 1, nil, nil, nil)
	serviceOK(t, "list first diary page", err)
	if len(page.Entries) != 1 || page.NextAfterID == nil {
		t.Fatal("diary page did not return continuation")
	}
	next, err := diaries.List(ctx, owner.Token, 1, page.NextAfterID, nil, nil)
	serviceOK(t, "list second diary page", err)
	if len(next.Entries) != 1 || next.Entries[0].ID == page.Entries[0].ID {
		t.Fatal("diary pagination duplicated entry")
	}

	calendar, err := diaries.Calendar(ctx, owner.Token, today[:7])
	serviceOK(t, "read diary calendar", err)
	var day domain.CalendarDay
	for _, candidate := range calendar.Days {
		if candidate.LocalDate == today {
			day = candidate
		}
	}
	if day.PlanCount != 1 || day.WearEventCount != 1 || day.DiaryCount != 2 {
		t.Fatalf("calendar mixed facts: %+v", day)
	}

	updatedBody := "编辑后的记录"
	updated, err := diaries.Update(ctx, owner.Token, secondID, second.Revision, domain.DiaryEntryInput{LocalDate: today, TimeZone: "Asia/Shanghai", Body: &updatedBody, PlanID: &planID, WearEventID: &eventID, MediaIDs: []string{media.ID}})
	serviceOK(t, "update linked diary", err)
	if updated.Revision != 2 || updated.Body == nil || *updated.Body != updatedBody {
		t.Fatal("diary update lost revision or body")
	}
	if _, err := diaries.Update(ctx, owner.Token, secondID, second.Revision, domain.DiaryEntryInput{LocalDate: today, TimeZone: "Asia/Shanghai", Body: &body}); !errors.Is(err, domain.ErrDiaryConflict) {
		t.Fatal("stale diary update succeeded")
	}

	restarted, err := diaryapp.NewService(accounts, store.NewDiaryRepository(database))
	serviceOK(t, "reconstruct diary service", err)
	persisted, err := restarted.Get(ctx, owner.Token, secondID)
	serviceOK(t, "read diary after reconstruction", err)
	if persisted.Revision != updated.Revision || len(persisted.MediaIDs) != 1 {
		t.Fatal("diary did not survive service reconstruction")
	}

	serviceOK(t, "delete linked wear event", wear.DeleteWearEvent(ctx, owner.Token, eventID, event.Revision))
	persisted, err = diaries.Get(ctx, owner.Token, secondID)
	serviceOK(t, "read diary after wear deletion", err)
	if persisted.WearEventID != nil || persisted.PlanID == nil {
		t.Fatal("wear deletion removed or retained wrong diary associations")
	}
	currentPlan, err := outfits.GetOutfitPlan(ctx, owner.Token, planID)
	serviceOK(t, "read plan after wear deletion", err)
	serviceOK(t, "delete linked plan", outfits.DeleteOutfitPlan(ctx, owner.Token, planID, currentPlan.Revision))
	persisted, err = diaries.Get(ctx, owner.Token, secondID)
	serviceOK(t, "read diary after plan deletion", err)
	if persisted.PlanID != nil || persisted.WearEventID != nil {
		t.Fatal("linked facts were not detached from diary")
	}
	impact, err := diaries.DeletionImpact(ctx, owner.Token, secondID)
	serviceOK(t, "read diary deletion impact", err)
	if impact.MediaCount != 1 || !impact.MediaRetained || impact.PublishedPostCount != 0 {
		t.Fatalf("unexpected diary deletion impact: %+v", impact)
	}
	serviceOK(t, "delete diary", diaries.Delete(ctx, owner.Token, secondID, persisted.Revision))
	if _, err := diaries.Create(ctx, owner.Token, secondID, domain.DiaryEntryInput{LocalDate: today, TimeZone: "Asia/Shanghai", Body: &body}); !errors.Is(err, domain.ErrDiaryConflict) {
		t.Fatal("deleted diary was resurrected")
	}
}
