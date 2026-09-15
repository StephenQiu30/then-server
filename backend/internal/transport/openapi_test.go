package transport

import (
	"context"
	"encoding/json"
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
			Schemas map[string]struct {
				Properties map[string]struct {
					Enum []any `json:"enum"`
				} `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
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
		"deleteSession":                true,
		"getCurrentUser":               true,
		"updateCurrentUser":            true,
		"deleteCurrentUser":            true,
		"getSelfAdultDeclaration":      true,
		"confirmSelfAdultDeclaration":  true,
		"withdrawSelfAdultDeclaration": true,
	}
	for _, path := range spec.Paths {
		for method, operation := range path {
			if method == "parameters" {
				continue
			}
			operations++
			if operation.OperationID == "" || identifiers[operation.OperationID] {
				t.Fatalf("missing or duplicate operationId %q", operation.OperationID)
			}
			identifiers[operation.OperationID] = true
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
	if spec.OpenAPI != "3.1.2" || spec.Info.Version != "0.6.0" || operations != 11 {
		t.Fatalf("unexpected generated contract: openapi=%s api=%s operations=%d", spec.OpenAPI, spec.Info.Version, operations)
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
