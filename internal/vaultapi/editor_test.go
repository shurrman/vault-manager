package vaultapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"vault-manager/internal/config"
	"vault-manager/internal/domain"
)

type groupFixture struct {
	mu         sync.Mutex
	modify     int
	group      map[string]interface{}
	writeCount int
	payload    map[string]interface{}
	tokenSeen  bool
}

func newGroupFixture() *groupFixture {
	return &groupFixture{
		modify: 1,
		group: map[string]interface{}{
			"id":                "group-1",
			"name":              "developers",
			"type":              "internal",
			"policies":          []string{"old-policy"},
			"member_entity_ids": []string{},
			"member_group_ids":  []string{},
			"parent_group_ids":  []string{},
			"metadata":          map[string]interface{}{},
			"modify_index":      1,
			"last_update_time":  "2026-01-01T00:00:00Z",
		},
	}
}

func (f *groupFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("X-Vault-Token") == "test-token" {
		f.tokenSeen = true
	}
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/identity/group/id/group-1":
		f.group["modify_index"] = f.modify
		writeJSON(w, map[string]interface{}{"data": f.group})
	case r.Method == http.MethodGet && r.URL.Path == "/v1/identity/group/id" && r.URL.Query().Get("list") == "true":
		writeJSON(w, map[string]interface{}{"data": map[string]interface{}{
			"keys":     []string{"group-1"},
			"key_info": map[string]interface{}{"group-1": map[string]interface{}{"name": "developers"}},
		}})
	case r.Method == http.MethodGet && r.URL.Path == "/v1/identity/entity/id" && r.URL.Query().Get("list") == "true":
		writeJSON(w, map[string]interface{}{"data": map[string]interface{}{
			"keys":     []string{"entity-1"},
			"key_info": map[string]interface{}{"entity-1": map[string]interface{}{"name": "alice"}},
		}})
	case r.Method == http.MethodGet && r.URL.Path == "/v1/sys/policies/acl" && r.URL.Query().Get("list") == "true":
		writeJSON(w, map[string]interface{}{"data": map[string]interface{}{"keys": []string{"old-policy", "new-policy"}}})
	case (r.Method == http.MethodPut || r.Method == http.MethodPost) && r.URL.Path == "/v1/identity/group/id/group-1":
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		f.payload = payload
		f.writeCount++
		f.modify++
		f.group["policies"] = payload["policies"]
		f.group["member_entity_ids"] = payload["member_entity_ids"]
		f.group["member_group_ids"] = payload["member_group_ids"]
		f.group["metadata"] = payload["metadata"]
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func TestGroupEditorResolvesNamesAndPreservesObject(t *testing.T) {
	fixture := newGroupFixture()
	server := httptest.NewServer(fixture)
	defer server.Close()

	client := newTestClient(t, server.URL)
	item := domain.Item{ID: "group-1", Name: "developers"}
	editor, err := client.LoadEditor(context.Background(), domain.SectionGroups, item, "edit")
	if err != nil {
		t.Fatalf("LoadEditor() error = %v", err)
	}
	fields := make(map[string]domain.EditField, len(editor.Fields))
	for _, field := range editor.Fields {
		fields[field.ID] = field
	}
	if fields["policies"].Kind != domain.FieldList || !containsString(fields["policies"].Options, "new-policy") {
		t.Fatalf("policy choices = %#v", fields["policies"])
	}
	if fields["members"].Kind != domain.FieldList || !containsString(fields["members"].Options, "alice") {
		t.Fatalf("member choices = %#v", fields["members"])
	}
	if fields["metadata"].Format != domain.FormatJSONObject {
		t.Fatalf("metadata format = %q", fields["metadata"].Format)
	}
	setEditorField(&editor, "policies", "new-policy")
	setEditorField(&editor, "members", "alice")

	if _, err := client.SaveEditor(context.Background(), editor); err != nil {
		t.Fatalf("SaveEditor() error = %v", err)
	}

	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.writeCount != 1 {
		t.Fatalf("write count = %d, want 1", fixture.writeCount)
	}
	if !fixture.tokenSeen {
		t.Fatal("Vault token header was not sent")
	}
	if fixture.payload["name"] != "developers" || fixture.payload["type"] != "internal" {
		t.Fatalf("group identity fields were not preserved: %#v", fixture.payload)
	}
	if got := stringSlice(fixture.payload["policies"]); !reflect.DeepEqual(got, []string{"new-policy"}) {
		t.Fatalf("policies = %#v", got)
	}
	if got := stringSlice(fixture.payload["member_entity_ids"]); !reflect.DeepEqual(got, []string{"entity-1"}) {
		t.Fatalf("member IDs = %#v", got)
	}
}

func TestGroupEditorRejectsConcurrentChangeWithoutWrite(t *testing.T) {
	fixture := newGroupFixture()
	server := httptest.NewServer(fixture)
	defer server.Close()

	client := newTestClient(t, server.URL)
	item := domain.Item{ID: "group-1", Name: "developers"}
	editor, err := client.LoadEditor(context.Background(), domain.SectionGroups, item, "edit")
	if err != nil {
		t.Fatalf("LoadEditor() error = %v", err)
	}
	setEditorField(&editor, "policies", "new-policy")
	fixture.mu.Lock()
	fixture.modify = 2
	fixture.mu.Unlock()

	_, err = client.SaveEditor(context.Background(), editor)
	if !errorsIs(err, errConflict) {
		t.Fatalf("SaveEditor() error = %v, want conflict", err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.writeCount != 0 {
		t.Fatalf("conflicting save wrote %d requests", fixture.writeCount)
	}
}

func TestValidationErrorDoesNotExposeSecret(t *testing.T) {
	fixture := newGroupFixture()
	server := httptest.NewServer(fixture)
	defer server.Close()

	client := newTestClient(t, server.URL)
	item := domain.Item{ID: "group-1", Name: "developers"}
	editor, err := client.LoadEditor(context.Background(), domain.SectionGroups, item, "edit")
	if err != nil {
		t.Fatalf("LoadEditor() error = %v", err)
	}
	setEditorField(&editor, "policies", "secret-token-value")

	_, err = client.SaveEditor(context.Background(), editor)
	if err == nil {
		t.Fatal("SaveEditor() accepted an unknown policy")
	}
	if strings.Contains(err.Error(), "test-token") {
		t.Fatalf("error exposed Vault token: %v", err)
	}
}

func newTestClient(t *testing.T, address string) *Client {
	t.Helper()
	client, err := New(config.Config{
		Vault: config.Vault{Address: address, Token: "test-token", RequestTimeout: "2s"},
		UI:    config.UI{Color: "never"},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return client
}

func setEditorField(editor *domain.Editor, id, value string) {
	for index := range editor.Fields {
		if editor.Fields[index].ID == id {
			editor.Fields[index].Value = value
			return
		}
	}
}

func writeJSON(w http.ResponseWriter, value interface{}) {
	_ = json.NewEncoder(w).Encode(value)
}

func errorsIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		next, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = next.Unwrap()
	}
	return false
}
