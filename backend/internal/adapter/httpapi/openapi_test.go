package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"testing"
)

func TestGeneratedOpenAPIContract(t *testing.T) {
	yamlDocument, jsonDocument, err := GeneratedOpenAPI(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(yamlDocument) == 0 {
		t.Fatal("generated YAML contract is empty")
	}
	var spec struct {
		OpenAPI string `json:"openapi"`
		Info    struct {
			Version string `json:"version"`
		} `json:"info"`
		Components struct {
			SecuritySchemes map[string]json.RawMessage `json:"securitySchemes"`
			Schemas         map[string]struct {
				Required   []string `json:"required"`
				Properties map[string]struct {
					Enum []any `json:"enum"`
				} `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
		Paths map[string]map[string]struct {
			OperationID string                `json:"operationId"`
			Security    []map[string][]string `json:"security"`
			Responses   map[string]struct {
				Headers map[string]json.RawMessage `json:"headers"`
			} `json:"responses"`
			Parameters []struct {
				In   string `json:"in"`
				Name string `json:"name"`
			} `json:"parameters"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(jsonDocument, &spec); err != nil {
		t.Fatal(err)
	}
	operations := 0
	identifiers := map[string]bool{}
	protectedOperations := map[string]bool{
		"deleteSession":                   true,
		"getCurrentUser":                  true,
		"listCurrentUserSessions":         true,
		"revokeCurrentUserSession":        true,
		"putProfileAvatar":                true,
		"deleteProfileAvatar":             true,
		"updateCurrentUser":               true,
		"deleteCurrentUser":               true,
		"requestEmailVerification":        true,
		"confirmEmailVerification":        true,
		"getSelfAdultDeclaration":         true,
		"confirmSelfAdultDeclaration":     true,
		"withdrawSelfAdultDeclaration":    true,
		"createConsent":                   true,
		"getConsent":                      true,
		"withdrawConsent":                 true,
		"createMediaUpload":               true,
		"completeMediaUpload":             true,
		"getMedia":                        true,
		"deleteMedia":                     true,
		"getDeletionRequest":              true,
		"createWardrobeItem":              true,
		"listWardrobeItems":               true,
		"getWardrobeItem":                 true,
		"updateWardrobeItem":              true,
		"deleteWardrobeItem":              true,
		"getWardrobeDeletionImpact":       true,
		"createOutfitPlan":                true,
		"listOutfitPlans":                 true,
		"getOutfitPlan":                   true,
		"updateOutfitPlan":                true,
		"cancelOutfitPlan":                true,
		"markOutfitPlanNotWorn":           true,
		"restoreOutfitPlan":               true,
		"deleteOutfitPlan":                true,
		"createWearEvent":                 true,
		"listWearEvents":                  true,
		"getWearEvent":                    true,
		"updateWearEvent":                 true,
		"deleteWearEvent":                 true,
		"getWearFeedback":                 true,
		"saveWearFeedback":                true,
		"deleteWearFeedback":              true,
		"listSyncChanges":                 true,
		"getWearStatistics":               true,
		"createDiaryEntry":                true,
		"listDiaryEntries":                true,
		"getDiaryEntry":                   true,
		"updateDiaryEntry":                true,
		"getDiaryEntryDeletionImpact":     true,
		"deleteDiaryEntry":                true,
		"getCalendarMonth":                true,
		"createPost":                      true,
		"listOwnPosts":                    true,
		"getOwnPost":                      true,
		"updatePost":                      true,
		"submitPost":                      true,
		"withdrawPost":                    true,
		"deletePost":                      true,
		"listPostModerationCandidates":    true,
		"getPostModerationCandidate":      true,
		"getPostModerationImage":          true,
		"decidePostModeration":            true,
		"removePublishedPost":             true,
		"createPostReport":                true,
		"listOwnReports":                  true,
		"listCommunityReports":            true,
		"resolveCommunityReport":          true,
		"listModerationActions":           true,
		"listAdminUsers":                  true,
		"suspendUser":                     true,
		"restoreUser":                     true,
		"likePost":                        true,
		"unlikePost":                      true,
		"bookmarkPost":                    true,
		"unbookmarkPost":                  true,
		"listBookmarks":                   true,
		"followProfile":                   true,
		"unfollowProfile":                 true,
		"blockUser":                       true,
		"unblockUser":                     true,
		"listBlockedUsers":                true,
		"createComment":                   true,
		"deleteComment":                   true,
		"listCommentModerationCandidates": true,
		"decideCommentModeration":         true,
		"removePublishedComment":          true,
		"listNotifications":               true,
		"markNotificationRead":            true,
		"createModerationAppeal":          true,
		"listOwnModerationAppeals":        true,
		"listModerationAppeals":           true,
		"resolveModerationAppeal":         true,
	}
	versionedPath := regexp.MustCompile(`^/v[0-9]+(?:/|$)`)
	for path, item := range spec.Paths {
		if versionedPath.MatchString(path) {
			t.Fatalf("generated contract exposes a path-version prefix: %s", path)
		}
		for method, operation := range item {
			if method == "parameters" {
				continue
			}
			operations++
			if operation.OperationID == "" || identifiers[operation.OperationID] {
				t.Fatalf("missing or duplicate operationId %q", operation.OperationID)
			}
			identifiers[operation.OperationID] = true
			if operation.OperationID == "getAccountDeletionReceipt" || operation.OperationID == "revokeAccountDeletionReceipt" {
				if len(spec.Components.SecuritySchemes["deletionReceiptAuth"]) == 0 || len(operation.Security) != 1 || len(operation.Security[0]) != 1 {
					t.Fatal("deletion receipt operation lacks its dedicated bearer security contract")
				}
				if _, ok := operation.Security[0]["deletionReceiptAuth"]; !ok {
					t.Fatal("deletion receipt operation exposes the wrong authentication scheme")
				}
			}
			if _, exists := operation.Responses["422"]; exists {
				t.Fatal("generated contract exposed the internal validation status 422")
			}
			if protectedOperations[operation.OperationID] {
				unauthorized, exists := operation.Responses["401"]
				if !exists || unauthorized.Headers["Set-Cookie"] == nil {
					t.Fatalf("protected operation %s does not document stale session cleanup", operation.OperationID)
				}
			}
			if operation.OperationID == "registerAccount" || operation.OperationID == "createSession" {
				limited, exists := operation.Responses["429"]
				if !exists || limited.Headers["Retry-After"] == nil {
					t.Fatalf("authentication operation %s does not document rate-limit recovery", operation.OperationID)
				}
				unavailable, exists := operation.Responses["503"]
				if !exists || unavailable.Headers["Retry-After"] == nil {
					t.Fatalf("authentication operation %s does not document limiter unavailability", operation.OperationID)
				}
			}
			for _, parameter := range operation.Parameters {
				if parameter.In == "cookie" && parameter.Name == sessionCookieName {
					t.Fatal("generated client contract exposed the HttpOnly session cookie as a request parameter")
				}
			}
		}
	}
	if spec.OpenAPI != "3.1.2" || spec.Info.Version != "0.26.0" || operations != 124 {
		t.Fatalf("unexpected generated contract: openapi=%s api=%s operations=%d", spec.OpenAPI, spec.Info.Version, operations)
	}
	for _, operationID := range []string{"listCurrentUserSessions", "revokeCurrentUserSession"} {
		if !identifiers[operationID] {
			t.Fatalf("generated contract is missing session operation %s", operationID)
		}
	}
	for _, operationID := range []string{"requestEmailVerification", "confirmEmailVerification", "requestPasswordReset", "confirmPasswordReset"} {
		if !identifiers[operationID] {
			t.Fatalf("generated contract is missing account mail operation %s", operationID)
		}
	}
	for _, operationID := range []string{"getCurrentProfile", "putCurrentProfile", "getPublicProfile", "putProfileAvatar", "deleteProfileAvatar", "getPublicProfileAvatar", "createConsent", "getConsent", "withdrawConsent", "createMediaUpload", "completeMediaUpload", "getMedia", "deleteMedia", "getDeletionRequest", "createWardrobeItem", "listWardrobeItems", "getWardrobeItem", "updateWardrobeItem", "archiveWardrobeItem", "restoreWardrobeItem", "getWardrobeDeletionImpact", "deleteWardrobeItem", "createOutfitPlan", "listOutfitPlans", "getOutfitPlan", "updateOutfitPlan", "cancelOutfitPlan", "markOutfitPlanNotWorn", "restoreOutfitPlan", "deleteOutfitPlan", "createWearEvent", "listWearEvents", "getWearEvent", "updateWearEvent", "deleteWearEvent", "createDiaryEntry", "listDiaryEntries", "getDiaryEntry", "updateDiaryEntry", "getDiaryEntryDeletionImpact", "deleteDiaryEntry", "getCalendarMonth", "createPost", "listOwnPosts", "getOwnPost", "updatePost", "submitPost", "withdrawPost", "deletePost", "getPublicPost", "getPublicPostImage", "listPostModerationCandidates", "getPostModerationCandidate", "getPostModerationImage", "decidePostModeration", "removePublishedPost", "createPostReport", "listOwnReports", "listCommunityReports", "resolveCommunityReport", "listModerationActions", "listAdminUsers", "suspendUser", "restoreUser"} {
		if !identifiers[operationID] {
			t.Fatalf("generated contract is missing %s", operationID)
		}
	}
	for _, operationID := range []string{"listCommunityFeed", "searchCommunityPosts", "listProfilePosts", "listProfileFollowers", "listProfileFollowing", "likePost", "unlikePost", "bookmarkPost", "unbookmarkPost", "listBookmarks", "followProfile", "unfollowProfile", "blockUser", "unblockUser", "listBlockedUsers", "createComment", "listComments", "listCommentReplies", "deleteComment", "listCommentModerationCandidates", "decideCommentModeration", "removePublishedComment", "listNotifications", "markNotificationRead", "createModerationAppeal", "listOwnModerationAppeals", "listModerationAppeals", "resolveModerationAppeal"} {
		if !identifiers[operationID] {
			t.Fatalf("generated contract is missing B4 operation %s", operationID)
		}
	}
	if bytes.Contains(jsonDocument, []byte(`"owner_id"`)) {
		t.Fatal("generated client contract exposed a wardrobe owner field")
	}
	for _, expected := range [][]byte{[]byte(`"smart_casual"`), []byte(`"user_confirmed"`), []byte(`"formality_band"`), []byte(`"walking_use"`)} {
		if !bytes.Contains(jsonDocument, expected) {
			t.Fatalf("generated client contract is missing wardrobe attribute constraint %s", expected)
		}
	}
	requestAttributes, exists := spec.Components.Schemas["WardrobeAttributesRequest"]
	if !exists {
		t.Fatal("generated client contract is missing WardrobeAttributesRequest")
	}
	if _, acceptsSource := requestAttributes.Properties["source"]; acceptsSource {
		t.Fatal("generated wardrobe attribute request accepts a client-provided source")
	}
	for _, schemaName := range []string{"CreateWardrobeItemRequest", "UpdateWardrobeItemRequest"} {
		schema, exists := spec.Components.Schemas[schemaName]
		if !exists || !slices.Contains(schema.Required, "attributes") {
			t.Fatalf("generated %s does not require the attributes object", schemaName)
		}
	}
	selection, exists := spec.Components.Schemas["OutfitSelectionRequest"]
	if !exists || len(selection.Properties) != 2 || selection.Properties["item_id"].Enum != nil || selection.Properties["revision"].Enum != nil {
		t.Fatal("generated outfit selection request does not expose only item_id and revision")
	}
	for _, forbidden := range []string{"name", "category", "availability", "attributes"} {
		if _, exists := selection.Properties[forbidden]; exists {
			t.Fatalf("generated outfit selection request accepts client snapshot field %s", forbidden)
		}
	}
	confirmationConstraintFound := false
	for _, schema := range spec.Components.Schemas {
		if property, ok := schema.Properties["confirms_self_and_adult"]; ok {
			confirmationConstraintFound = len(property.Enum) == 1 && property.Enum[0] == true
			break
		}
	}
	if !confirmationConstraintFound {
		t.Fatal("generated contract does not require confirms_self_and_adult to be true")
	}
}
