package vaultapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"vault-manager/internal/domain"
)

type membershipFixture struct {
	mu          sync.Mutex
	groups      map[string]map[string]interface{}
	failGroup   string
	writeCounts map[string]int
}

func newMembershipFixture() *membershipFixture {
	return &membershipFixture{
		groups: map[string]map[string]interface{}{
			"group-a": {
				"id":                "group-a",
				"name":              "alpha",
				"type":              "internal",
				"policies":          []string{},
				"member_entity_ids": []string{"entity-1"},
				"member_group_ids":  []string{},
				"parent_group_ids":  []string{},
				"metadata":          map[string]interface{}{},
				"modify_index":      1,
				"last_update_time":  "2026-01-01T00:00:00Z",
			},
			"group-b": {
				"id":                "group-b",
				"name":              "beta",
				"type":              "internal",
				"policies":          []string{},
				"member_entity_ids": []string{},
				"member_group_ids":  []string{},
				"parent_group_ids":  []string{},
				"metadata":          map[string]interface{}{},
				"modify_index":      1,
				"last_update_time":  "2026-01-01T00:00:00Z",
			},
		},
		writeCounts: make(map[string]int),
	}
}

func (f *membershipFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/identity/group/id" && r.URL.Query().Get("list") == "true":
		writeJSON(w, map[string]interface{}{"data": map[string]interface{}{
			"keys": []string{"group-a", "group-b"},
			"key_info": map[string]interface{}{
				"group-a": map[string]interface{}{"name": "alpha"},
				"group-b": map[string]interface{}{"name": "beta"},
			},
		}})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/identity/group/id/"):
		groupID := strings.TrimPrefix(r.URL.Path, "/v1/identity/group/id/")
		group, ok := f.groups[groupID]
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, map[string]interface{}{"data": group})
	case r.Method == http.MethodGet && r.URL.Path == "/v1/identity/entity/id/entity-1":
		writeJSON(w, map[string]interface{}{"data": f.entityData()})
	case r.Method == http.MethodGet && r.URL.Path == "/v1/sys/policies/acl" && r.URL.Query().Get("list") == "true":
		writeJSON(w, map[string]interface{}{"data": map[string]interface{}{"keys": []string{"default"}}})
	case (r.Method == http.MethodPut || r.Method == http.MethodPost) && strings.HasPrefix(r.URL.Path, "/v1/identity/group/id/"):
		groupID := strings.TrimPrefix(r.URL.Path, "/v1/identity/group/id/")
		f.writeCounts[groupID]++
		if groupID == f.failGroup {
			http.Error(w, `{"errors":["injected failure"]}`, http.StatusBadRequest)
			return
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		group := f.groups[groupID]
		group["name"] = payload["name"]
		group["type"] = payload["type"]
		group["policies"] = payload["policies"]
		group["member_entity_ids"] = payload["member_entity_ids"]
		group["member_group_ids"] = payload["member_group_ids"]
		group["metadata"] = payload["metadata"]
		group["modify_index"] = f.writeCounts[groupID] + 1
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (f *membershipFixture) entityData() map[string]interface{} {
	directGroupIDs := make([]string, 0)
	for groupID, group := range f.groups {
		if containsString(stringSlice(group["member_entity_ids"]), "entity-1") {
			directGroupIDs = append(directGroupIDs, groupID)
		}
	}
	return map[string]interface{}{
		"id":                  "entity-1",
		"name":                "alice",
		"disabled":            false,
		"policies":            []string{},
		"direct_group_ids":    sortedStrings(directGroupIDs),
		"inherited_group_ids": []string{},
		"aliases":             []interface{}{},
		"metadata":            map[string]interface{}{},
		"modify_index":        1,
		"creation_time":       "2026-01-01T00:00:00Z",
		"last_update_time":    "2026-01-01T00:00:00Z",
	}
}

func TestUserMembershipSaveMovesBetweenGroups(t *testing.T) {
	fixture := newMembershipFixture()
	server := httptest.NewServer(fixture)
	defer server.Close()

	client := newTestClient(t, server.URL)
	cacheMembershipUser(client)
	item := domain.Item{ID: "entity:entity-1", Name: "alice"}
	editor, err := client.LoadEditor(context.Background(), domain.SectionUsers, item, "edit")
	if err != nil {
		t.Fatalf("LoadEditor() error = %v", err)
	}
	if !reflect.DeepEqual(editor.Warnings, []string{
		"Group membership updates multiple Vault groups sequentially and is not atomic.",
	}) {
		t.Fatalf("warnings = %#v", editor.Warnings)
	}
	setEditorField(&editor, "groups", "beta")

	preview, err := client.SaveEditor(context.Background(), editor)
	if err != nil {
		t.Fatalf("SaveEditor() error = %v", err)
	}
	if preview.Title != "alice" || rowValue(preview.Rows, "Direct groups") != "beta" {
		t.Fatalf("preview does not show saved membership: %#v", preview)
	}

	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if !reflect.DeepEqual(fixture.writeCounts, map[string]int{"group-a": 1, "group-b": 1}) {
		t.Fatalf("write counts = %#v", fixture.writeCounts)
	}
	if containsString(stringSlice(fixture.groups["group-a"]["member_entity_ids"]), "entity-1") {
		t.Fatal("alice was not removed from alpha")
	}
	if !containsString(stringSlice(fixture.groups["group-b"]["member_entity_ids"]), "entity-1") {
		t.Fatal("alice was not added to beta")
	}
}

func TestUserMembershipReportsPartialSave(t *testing.T) {
	fixture := newMembershipFixture()
	fixture.failGroup = "group-b"
	server := httptest.NewServer(fixture)
	defer server.Close()

	client := newTestClient(t, server.URL)
	cacheMembershipUser(client)
	item := domain.Item{ID: "entity:entity-1", Name: "alice"}
	editor, err := client.LoadEditor(context.Background(), domain.SectionUsers, item, "edit")
	if err != nil {
		t.Fatalf("LoadEditor() error = %v", err)
	}
	setEditorField(&editor, "groups", "beta")

	preview, err := client.SaveEditor(context.Background(), editor)
	var partial *domain.PartialSaveError
	if !errors.As(err, &partial) {
		t.Fatalf("SaveEditor() error = %v, want PartialSaveError", err)
	}
	if !reflect.DeepEqual(partial.Applied, []string{"remove alice from alpha"}) {
		t.Fatalf("applied = %#v", partial.Applied)
	}
	if !strings.Contains(partial.Failed, "update group beta") {
		t.Fatalf("failed = %q", partial.Failed)
	}
	if len(partial.Pending) != 0 {
		t.Fatalf("pending = %#v", partial.Pending)
	}
	if preview.Title != "alice" || rowValue(preview.Rows, "Direct groups") != "—" {
		t.Fatalf("partial preview does not reflect Vault state: %#v", preview)
	}

	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if !reflect.DeepEqual(fixture.writeCounts, map[string]int{"group-a": 1, "group-b": 1}) {
		t.Fatalf("write counts = %#v", fixture.writeCounts)
	}
}

func TestUserMembershipConflictWritesNothing(t *testing.T) {
	fixture := newMembershipFixture()
	server := httptest.NewServer(fixture)
	defer server.Close()

	client := newTestClient(t, server.URL)
	cacheMembershipUser(client)
	item := domain.Item{ID: "entity:entity-1", Name: "alice"}
	editor, err := client.LoadEditor(context.Background(), domain.SectionUsers, item, "edit")
	if err != nil {
		t.Fatalf("LoadEditor() error = %v", err)
	}
	setEditorField(&editor, "groups", "beta")

	fixture.mu.Lock()
	fixture.groups["group-a"]["modify_index"] = 2
	fixture.mu.Unlock()

	_, err = client.SaveEditor(context.Background(), editor)
	if !errorsIs(err, errConflict) {
		t.Fatalf("SaveEditor() error = %v, want conflict", err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if len(fixture.writeCounts) != 0 {
		t.Fatalf("conflicting save wrote requests: %#v", fixture.writeCounts)
	}
}

func TestUserEditorIncludesOptionalDeploymentNotice(t *testing.T) {
	fixture := newMembershipFixture()
	server := httptest.NewServer(fixture)
	defer server.Close()

	client := newTestClient(t, server.URL)
	client.membershipNotice = "Membership is managed externally."
	cacheMembershipUser(client)
	item := domain.Item{ID: "entity:entity-1", Name: "alice"}
	editor, err := client.LoadEditor(context.Background(), domain.SectionUsers, item, "edit")
	if err != nil {
		t.Fatalf("LoadEditor() error = %v", err)
	}
	if !containsString(editor.Warnings, "Membership is managed externally.") {
		t.Fatalf("warnings = %#v", editor.Warnings)
	}
}

func cacheMembershipUser(client *Client) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.userRefs["entity:entity-1"] = userRef{EntityID: "entity-1"}
}

func rowValue(rows []domain.Row, label string) string {
	for _, row := range rows {
		if row.Label == label {
			return row.Value
		}
	}
	return ""
}
