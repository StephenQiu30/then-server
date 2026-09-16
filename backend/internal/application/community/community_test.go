package community

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizePostInputAppliesPublicContract(t *testing.T) {
	title, body := "  Autumn Look  ", "  line one\nline two  "
	input := PostContentInput{
		Title:    &title,
		Body:     &body,
		MediaIDs: []string{"10000000-0000-4000-8000-000000000001"},
		Tags:     []string{" Street ", "OOTD"},
	}
	normalized, ok := normalizePostInput("10000000-0000-4000-8000-000000000010", input)
	if !ok || normalized.Title == nil || *normalized.Title != "Autumn Look" || normalized.Body == nil || *normalized.Body != "line one\nline two" {
		t.Fatalf("unexpected normalized content: %+v", normalized)
	}
	if !reflect.DeepEqual(normalized.Tags, []string{"ootd", "street"}) {
		t.Fatalf("tags were not normalized deterministically: %#v", normalized.Tags)
	}
}

func TestNormalizePostInputRejectsInvalidOrPrivateLookingShapes(t *testing.T) {
	body := "valid"
	cases := []PostContentInput{
		{},
		{Body: &body, MediaIDs: []string{"not-a-uuid"}},
		{Body: &body, Tags: []string{"same", " SAME "}},
		{Body: &body, Tags: []string{strings.Repeat("x", 21)}},
	}
	for index, input := range cases {
		if _, ok := normalizePostInput("10000000-0000-4000-8000-000000000010", input); ok {
			t.Fatalf("case %d unexpectedly passed", index)
		}
	}
}

func TestCommunityReasonCodesAreBounded(t *testing.T) {
	if !validReason("policy_violation") || validReason("Policy") || validReason(strings.Repeat("x", 41)) {
		t.Fatal("moderation reason contract changed")
	}
	for _, reason := range []string{"spam", "harassment", "sexual", "violence", "misinformation", "other"} {
		if !validReportReason(reason) {
			t.Fatalf("supported report reason %q rejected", reason)
		}
	}
}

func TestSocialInputNormalization(t *testing.T) {
	query, ok := normalizeSearchQuery("  秋日 穿搭  ")
	if !ok || query == nil || *query != "秋日 穿搭" {
		t.Fatalf("search query normalization failed: %v %v", query, ok)
	}
	tag, ok := normalizeSearchTag("  OOTD  ")
	if !ok || tag == nil || *tag != "ootd" {
		t.Fatalf("search tag normalization failed: %v %v", tag, ok)
	}
	comment, ok := normalizeCommentBody("  第一行\n第二行  ")
	if !ok || comment != "第一行\n第二行" {
		t.Fatalf("comment normalization failed: %q %v", comment, ok)
	}
	for _, invalid := range []string{"x", strings.Repeat("x", 81), "valid\u0000control"} {
		if _, ok := normalizeSearchQuery(invalid); ok {
			t.Fatalf("invalid search query passed: %q", invalid)
		}
	}
	if _, ok := normalizeCommentBody(strings.Repeat("评", 501)); ok {
		t.Fatal("oversized comment passed")
	}
}
