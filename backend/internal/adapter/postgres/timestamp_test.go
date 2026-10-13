package postgres

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestPersistedResponsesMatchPostgreSQLTimestampPrecision(t *testing.T) {
	input := time.Date(2026, 10, 13, 14, 0, 0, 123456789, time.FixedZone("input", 8*60*60))
	codec := pgtype.NewMap()
	encoded, err := codec.Encode(pgtype.TimestamptzOID, pgtype.BinaryFormatCode, input, nil)
	if err != nil {
		t.Fatal(err)
	}
	var stored time.Time
	if err := codec.Scan(pgtype.TimestamptzOID, pgtype.BinaryFormatCode, encoded, &stored); err != nil {
		t.Fatal(err)
	}

	declaration := selfAdultDeclarationFromRecord(selfAdultDeclarationRecord{ConfirmedAt: input, WithdrawnAt: &input})
	consent := consentFromRecord(consentRecord{AgreedAt: input, WithdrawnAt: &input})
	wardrobe := wardrobeFromRecord(wardrobeItemRecord{CreatedAt: input, UpdatedAt: input, ArchivedAt: &input})
	diary := diaryFromRecord(diaryEntryRecord{CreatedAt: input, UpdatedAt: input}, nil)
	plan := outfitFromRecord(outfitPlanRecord{CreatedAt: input, UpdatedAt: input}, nil)
	wear := wearEventFromRecord(wearEventRecord{CreatedAt: input, UpdatedAt: input}, nil)
	cases := map[string][]time.Time{
		"declaration": {*declaration.ConfirmedAt, *declaration.WithdrawnAt},
		"consent":     {consent.AgreedAt, *consent.WithdrawnAt},
		"wardrobe":    {wardrobe.CreatedAt, wardrobe.UpdatedAt, *wardrobe.ArchivedAt},
		"diary":       {diary.CreatedAt, diary.UpdatedAt},
		"plan":        {plan.CreatedAt, plan.UpdatedAt},
		"wear":        {wear.CreatedAt, wear.UpdatedAt},
	}
	for name, timestamps := range cases {
		t.Run(name, func(t *testing.T) {
			for _, got := range timestamps {
				if !got.Equal(stored) {
					t.Errorf("first response timestamp %s differs from persisted replay %s", got.Format(time.RFC3339Nano), stored.Format(time.RFC3339Nano))
				}
			}
		})
	}
}
