package domain

import "testing"

func TestMediaStatusOnlyMovesForward(t *testing.T) {
	allowed := map[MediaStatus][]MediaStatus{
		MediaPendingUpload: {MediaUploaded, MediaDeleting},
		MediaUploaded:      {MediaChecking, MediaDeleting},
		MediaChecking:      {MediaReady, MediaRejected, MediaDeleting},
		MediaReady:         {MediaDeleting},
		MediaRejected:      {MediaDeleting},
		MediaDeleting:      {MediaDeleted},
		MediaDeleted:       {},
	}
	for from, targets := range allowed {
		for _, to := range targets {
			if !from.CanTransitionTo(to) {
				t.Fatalf("expected transition %s -> %s", from, to)
			}
		}
	}
	for _, transition := range [][2]MediaStatus{
		{MediaPendingUpload, MediaReady},
		{MediaUploaded, MediaReady},
		{MediaReady, MediaUploaded},
		{MediaDeleting, MediaReady},
		{MediaDeleted, MediaDeleting},
	} {
		if transition[0].CanTransitionTo(transition[1]) {
			t.Fatalf("unexpected transition %s -> %s", transition[0], transition[1])
		}
	}
}

func TestPhotoUploadContractIsFixed(t *testing.T) {
	if MediaPurposeAvatarSourcePreparation != "avatar_source_preparation" || MediaCategoryPersonPhoto != "person_photo" {
		t.Fatal("media purpose or category drifted")
	}
	if MediaContentTypeJPEG != "image/jpeg" || MaxPersonPhotoBytes != 12*1024*1024 || MaxPersonPhotoPixels != 24_000_000 {
		t.Fatal("photo input limits drifted")
	}
	if UploadIntentLifetime.Minutes() != 10 || UnfinishedUploadLifetime.Hours() != 24 {
		t.Fatal("media retention timing drifted")
	}
}
