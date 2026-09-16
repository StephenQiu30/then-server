package account

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPutCurrentProfileNormalizesPublicFields(t *testing.T) {
	bio := "  记录日常穿搭。  "
	repository := &accountRepositoryStub{
		sessionUser: User{ID: "user-id", Revision: 1},
		profile:     PublicProfile{Handle: "then_style", Revision: 1},
	}
	service := newAccountServiceForTest(t, repository)
	profile, err := service.PutCurrentProfile(context.Background(), strings.Repeat("K", encodedTokenLength), PutProfileInput{
		Handle: " Then_Style ", Bio: &bio, ExpectedRevision: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Handle != "then_style" || repository.profileInput.Handle != "then_style" || repository.profileInput.Bio == nil || *repository.profileInput.Bio != "记录日常穿搭。" {
		t.Fatal("profile fields were not normalized before persistence")
	}
}

func TestPutCurrentProfileClearsBlankBio(t *testing.T) {
	bio := "  "
	repository := &accountRepositoryStub{
		sessionUser: User{ID: "user-id", Revision: 1},
		profile:     PublicProfile{Handle: "then_style", Revision: 2},
	}
	service := newAccountServiceForTest(t, repository)
	if _, err := service.PutCurrentProfile(context.Background(), strings.Repeat("K", encodedTokenLength), PutProfileInput{
		Handle: "then_style", Bio: &bio, ExpectedRevision: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if repository.profileInput.Bio != nil {
		t.Fatal("blank public bio was not cleared")
	}
}

func TestProfileValidationStopsBeforeRepositoryWrite(t *testing.T) {
	tooLong := strings.Repeat("穿", 301)
	for _, input := range []PutProfileInput{
		{Handle: "a!", ExpectedRevision: 0},
		{Handle: "valid_handle", Bio: &tooLong, ExpectedRevision: 0},
		{Handle: "valid_handle", ExpectedRevision: -1},
	} {
		repository := &accountRepositoryStub{sessionUser: User{ID: "user-id", Revision: 1}}
		service := newAccountServiceForTest(t, repository)
		if _, err := service.PutCurrentProfile(context.Background(), strings.Repeat("K", encodedTokenLength), input); !errors.Is(err, ErrInvalidProfileInput) {
			t.Fatalf("invalid profile error=%v", err)
		}
		if repository.profileInput.Handle != "" {
			t.Fatal("invalid profile reached the repository write")
		}
	}
}

func TestPublicProfileInvalidHandleUsesNotFoundBoundary(t *testing.T) {
	service := newAccountServiceForTest(t, new(accountRepositoryStub))
	if _, err := service.PublicProfile(context.Background(), "not valid"); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("invalid public handle error=%v", err)
	}
}
