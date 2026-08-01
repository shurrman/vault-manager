package vaultapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"vault-manager/internal/domain"
)

func TestBrowseKVV2HierarchyAndReadLeaf(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/sys/mounts/secret":
			writeJSON(w, map[string]interface{}{"data": map[string]interface{}{
				"type": "kv", "options": map[string]string{"version": "2"},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/secret/metadata" && r.URL.Query().Get("list") == "true":
			writeJSON(w, map[string]interface{}{"data": map[string]interface{}{"keys": []string{"standalone", "app/"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/secret/metadata/app" && r.URL.Query().Get("list") == "true":
			writeJSON(w, map[string]interface{}{"data": map[string]interface{}{"keys": []string{"nested/", "config"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/secret/data/app/config":
			writeJSON(w, map[string]interface{}{"data": map[string]interface{}{
				"data":     map[string]interface{}{"password": "secret", "username": "alice"},
				"metadata": map[string]interface{}{"version": 3},
			}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	item := domain.Item{ID: "secret/", Name: "secret/", Subtitle: "kv"}
	root, err := client.Browse(context.Background(), domain.SectionEngines, item, "", false)
	if err != nil {
		t.Fatalf("Browse(root) error = %v", err)
	}
	if root.DisplayPath != "secret/" || len(root.Entries) != 2 {
		t.Fatalf("root page = %#v", root)
	}
	if !root.Entries[0].Directory || root.Entries[0].Name != "app" || root.Entries[1].Name != "standalone" {
		t.Fatalf("root entries = %#v", root.Entries)
	}

	app, err := client.Browse(context.Background(), domain.SectionEngines, item, "app", false)
	if err != nil {
		t.Fatalf("Browse(app) error = %v", err)
	}
	if app.DisplayPath != "secret/app" || len(app.Entries) != 2 || !app.Entries[0].Directory {
		t.Fatalf("app page = %#v", app)
	}

	leaf, err := client.Browse(context.Background(), domain.SectionEngines, item, "app/config", true)
	if err != nil {
		t.Fatalf("Browse(leaf) error = %v", err)
	}
	if !leaf.Leaf || rowValue(leaf.Rows, "username") != "alice" || rowValue(leaf.Rows, "password") != "secret" {
		t.Fatalf("leaf page = %#v", leaf)
	}
}

func TestNormalizeBrowserPathRejectsTraversal(t *testing.T) {
	for _, value := range []string{"../secret", "app/../secret", "app//secret"} {
		if _, err := normalizeBrowserPath(value); err == nil {
			t.Fatalf("normalizeBrowserPath(%q) accepted traversal", value)
		}
	}
	if parent := parentBrowserPath("app/nested"); parent != "app" {
		t.Fatalf("parentBrowserPath() = %q", parent)
	}
}
