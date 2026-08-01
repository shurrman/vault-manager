package vaultapi

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"vault-manager/internal/domain"
)

func TestAllPublishedActionsAreEnabled(t *testing.T) {
	total := 0
	for _, section := range domain.Sections {
		actions := actionsFor(section)
		if len(actions) == 0 {
			t.Fatalf("section %s has no actions", section)
		}
		for _, action := range actions {
			total++
			if !action.Enabled || action.Reason != "" {
				t.Fatalf("section %s action %s is still disabled: %#v", section, action.ID, action)
			}
		}
	}
	if total != 27 {
		t.Fatalf("published action count = %d, want 27", total)
	}
}

type policyActionFixture struct {
	mu       sync.Mutex
	policies map[string]string
}

func (f *policyActionFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	const prefix = "/v1/sys/policies/acl/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, prefix)
	switch r.Method {
	case http.MethodGet:
		body, ok := f.policies[name]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]interface{}{"data": map[string]interface{}{"policy": body}})
	case http.MethodPut, http.MethodPost:
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		f.policies[name] = stringValue(payload["policy"])
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		delete(f.policies, name)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestPolicyActionsRoundTripMultilineBodies(t *testing.T) {
	fixture := &policyActionFixture{policies: map[string]string{
		"existing": "path \"secret/data/old\" {\n  capabilities = [\"read\"]\n}\n",
	}}
	server := httptest.NewServer(fixture)
	defer server.Close()
	client := newTestClient(t, server.URL)
	ctx := context.Background()

	item := domain.Item{ID: "existing", Name: "existing"}
	editor, err := client.LoadEditor(ctx, domain.SectionPolicies, item, "edit")
	if err != nil {
		t.Fatalf("LoadEditor(edit policy) error = %v", err)
	}
	if editor.Fields[0].Kind != domain.FieldMultiline || editor.Fields[0].Format != domain.FormatPolicy || len(editor.Fields[0].Examples) == 0 {
		t.Fatalf("policy editor is not multiline/helpful: %#v", editor.Fields[0])
	}
	setEditorField(&editor, "body", "path \"secret/broken\" {\n  capabilities = [\"read\"\n}")
	if _, err := client.SaveEditor(ctx, editor); err == nil || !strings.Contains(err.Error(), "line") {
		t.Fatalf("invalid policy error = %v, want source location", err)
	}

	updated := "# comment\npath \"secret/data/new/*\" {\n  capabilities = [\"read\", \"list\"]\n}\n"
	setEditorField(&editor, "body", updated)
	preview, err := client.SaveEditor(ctx, editor)
	if err != nil {
		t.Fatalf("SaveEditor(edit policy) error = %v", err)
	}
	if rowValue(preview.Rows, "Policy") != updated {
		t.Fatalf("updated policy preview = %q", rowValue(preview.Rows, "Policy"))
	}

	creator, err := client.LoadEditor(ctx, domain.SectionPolicies, item, "create")
	if err != nil {
		t.Fatalf("LoadEditor(create policy) error = %v", err)
	}
	setEditorField(&creator, "name", "created")
	setEditorField(&creator, "body", "path \"sys/health\" { capabilities = [\"read\"] }")
	if _, err := client.SaveEditor(ctx, creator); err != nil {
		t.Fatalf("SaveEditor(create policy) error = %v", err)
	}

	createdItem := domain.Item{ID: "created", Name: "created"}
	deleter, err := client.LoadEditor(ctx, domain.SectionPolicies, createdItem, "delete")
	if err != nil {
		t.Fatalf("LoadEditor(delete policy) error = %v", err)
	}
	if _, err := client.SaveEditor(ctx, deleter); err != nil {
		t.Fatalf("SaveEditor(delete policy) error = %v", err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if _, exists := fixture.policies["created"]; exists {
		t.Fatal("deleted policy still exists")
	}
}

func validSnapshot(t *testing.T) string {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(compressed)
	content := []byte("checksum")
	if err := archive.WriteHeader(&tar.Header{Name: "SHA256SUMS.sealed", Mode: 0o600, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.String()
}

type statusActionFixture struct {
	mu       sync.Mutex
	sealed   bool
	progress int
	restored string
	snapshot string
}

func (f *statusActionFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPut && r.URL.Path == "/v1/sys/seal":
		f.sealed = true
		f.progress = 0
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/sys/seal-status":
		writeJSON(w, map[string]interface{}{
			"initialized": true, "sealed": f.sealed, "t": 1, "n": 1,
			"progress": f.progress, "type": "shamir", "storage_type": "raft",
		})
	case r.Method == http.MethodPut && r.URL.Path == "/v1/sys/unseal":
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if stringValue(payload["key"]) == "key-share" {
			f.sealed = false
			f.progress = 1
		}
		writeJSON(w, map[string]interface{}{
			"initialized": true, "sealed": f.sealed, "t": 1, "n": 1,
			"progress": f.progress, "type": "shamir", "storage_type": "raft",
		})
	case r.Method == http.MethodPost && (r.URL.Path == "/v1/sys/storage/raft/snapshot" || r.URL.Path == "/v1/sys/storage/raft/snapshot-force"):
		body, _ := io.ReadAll(r.Body)
		f.restored = string(body)
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/sys/storage/raft/snapshot":
		_, _ = io.WriteString(w, f.snapshot)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestStatusActionsUseExplicitConfirmationAndSecretInput(t *testing.T) {
	fixture := &statusActionFixture{snapshot: validSnapshot(t)}
	server := httptest.NewServer(fixture)
	defer server.Close()
	client := newTestClient(t, server.URL)
	ctx := context.Background()
	item := domain.Item{ID: "seal", Name: "Seal status"}

	sealer, err := client.LoadEditor(ctx, domain.SectionStatus, item, "seal")
	if err != nil {
		t.Fatalf("LoadEditor(seal) error = %v", err)
	}
	if !sealer.SubmitAlways || sealer.SubmitLabel != "Seal" || len(sealer.Fields) != 0 {
		t.Fatalf("seal editor is not confirmation-only: %#v", sealer)
	}
	if _, err := client.SaveEditor(ctx, sealer); err != nil {
		t.Fatalf("SaveEditor(seal) error = %v", err)
	}

	unsealer, err := client.LoadEditor(ctx, domain.SectionStatus, item, "unseal")
	if err != nil {
		t.Fatalf("LoadEditor(unseal) error = %v", err)
	}
	if unsealer.Fields[0].Kind != domain.FieldSecret {
		t.Fatalf("unseal key field kind = %s", unsealer.Fields[0].Kind)
	}
	setEditorField(&unsealer, "key", "key-share")
	if _, err := client.SaveEditor(ctx, unsealer); err != nil {
		t.Fatalf("SaveEditor(unseal) error = %v", err)
	}

	savedSnapshot := filepath.Join(t.TempDir(), "saved.snap")
	saver, err := client.LoadEditor(ctx, domain.SectionStatus, item, "snapshot")
	if err != nil {
		t.Fatalf("LoadEditor(snapshot) error = %v", err)
	}
	setEditorField(&saver, "file", savedSnapshot)
	if _, err := client.SaveEditor(ctx, saver); err != nil {
		t.Fatalf("SaveEditor(snapshot) error = %v", err)
	}
	saved, err := os.ReadFile(savedSnapshot)
	if err != nil {
		t.Fatalf("read saved snapshot: %v", err)
	}
	if string(saved) != fixture.snapshot {
		t.Fatalf("saved snapshot = %q", saved)
	}
	info, err := os.Stat(savedSnapshot)
	if err != nil {
		t.Fatalf("stat saved snapshot: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("saved snapshot mode = %o, want 600", info.Mode().Perm())
	}
	if _, err := client.SaveEditor(ctx, saver); err == nil {
		t.Fatal("SaveEditor(snapshot) overwrote an existing file")
	}

	snapshot := filepath.Join(t.TempDir(), "snapshot.snap")
	if err := os.WriteFile(snapshot, []byte("fake-snapshot"), 0o600); err != nil {
		t.Fatalf("write snapshot fixture: %v", err)
	}
	restorer, err := client.LoadEditor(ctx, domain.SectionStatus, item, "restore")
	if err != nil {
		t.Fatalf("LoadEditor(restore) error = %v", err)
	}
	setEditorField(&restorer, "file", snapshot)
	if _, err := client.SaveEditor(ctx, restorer); err != nil {
		t.Fatalf("SaveEditor(restore) error = %v", err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if fixture.sealed || fixture.restored != "fake-snapshot" {
		t.Fatalf("status effects = sealed:%v snapshot:%q", fixture.sealed, fixture.restored)
	}
}

type tokenActionFixture struct {
	revoked bool
}

func (f *tokenActionFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPut && r.URL.Path == "/v1/auth/token/lookup-accessor":
		if f.revoked {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]interface{}{"data": map[string]interface{}{"display_name": "userpass-alice", "orphan": true}})
	case r.Method == http.MethodPut && r.URL.Path == "/v1/auth/token/revoke-accessor":
		f.revoked = true
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/auth/token/accessors" && r.URL.Query().Get("list") == "true":
		keys := []string{"accessor-1"}
		if f.revoked {
			keys = nil
		}
		writeJSON(w, map[string]interface{}{"data": map[string]interface{}{"keys": keys}})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestTokenRevokeUsesAccessorAndVerifiesRemoval(t *testing.T) {
	fixture := &tokenActionFixture{}
	server := httptest.NewServer(fixture)
	defer server.Close()
	client := newTestClient(t, server.URL)
	item := domain.Item{ID: "accessor-1", Name: "alice"}

	editor, err := client.LoadEditor(context.Background(), domain.SectionTokens, item, "revoke")
	if err != nil {
		t.Fatalf("LoadEditor(revoke token) error = %v", err)
	}
	if _, err := client.SaveEditor(context.Background(), editor); err != nil {
		t.Fatalf("SaveEditor(revoke token) error = %v", err)
	}
	if !fixture.revoked {
		t.Fatal("token accessor was not revoked")
	}
}

type systemActionFixture struct {
	mounts map[string]map[string]interface{}
	auths  map[string]map[string]interface{}
	tune   map[string]interface{}
	audits map[string]map[string]interface{}
	roles  map[string]map[string]interface{}
}

func newSystemActionFixture() *systemActionFixture {
	return &systemActionFixture{
		mounts: map[string]map[string]interface{}{
			"secret/": {"type": "kv", "description": "KV", "options": map[string]string{"version": "2"}, "local": false, "config": map[string]interface{}{}},
			"ssh/":    {"type": "ssh", "description": "SSH", "options": map[string]string{}, "local": false, "config": map[string]interface{}{}},
		},
		auths: map[string]map[string]interface{}{
			"userpass/": {"type": "userpass", "description": "Users", "options": map[string]string{}, "local": false, "config": map[string]interface{}{}, "accessor": "auth_userpass_1"},
		},
		tune: map[string]interface{}{
			"description": "Users", "default_lease_ttl": 3600, "max_lease_ttl": 86400,
			"listing_visibility": "", "token_type": "default-service",
		},
		audits: map[string]map[string]interface{}{
			"file/": {"type": "file", "description": "File audit", "options": map[string]string{"file_path": "/tmp/audit.log"}, "local": false},
		},
		roles: map[string]map[string]interface{}{
			"developer": {"key_type": "ca", "allow_user_certificates": true, "allowed_users": "*"},
		},
	}
}

func (f *systemActionFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/sys/mounts":
		writeJSON(w, map[string]interface{}{"data": f.mounts})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/sys/mounts/auth/"):
		path := strings.TrimPrefix(r.URL.Path, "/v1/sys/mounts/auth/") + "/"
		value, ok := f.auths[path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]interface{}{"data": value})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/sys/mounts/"):
		path := strings.TrimPrefix(r.URL.Path, "/v1/sys/mounts/") + "/"
		value, ok := f.mounts[path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]interface{}{"data": value})
	case (r.Method == http.MethodPut || r.Method == http.MethodPost) && strings.HasPrefix(r.URL.Path, "/v1/sys/mounts/"):
		path := strings.TrimPrefix(r.URL.Path, "/v1/sys/mounts/") + "/"
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		f.mounts[path] = map[string]interface{}{
			"type": payload["type"], "description": payload["description"], "options": payload["options"],
			"local": payload["local"], "config": map[string]interface{}{},
		}
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/sys/mounts/"):
		delete(f.mounts, strings.TrimPrefix(r.URL.Path, "/v1/sys/mounts/")+"/")
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/sys/auth":
		writeJSON(w, map[string]interface{}{"data": f.auths})
	case r.URL.Path == "/v1/sys/auth/userpass/tune" && r.Method == http.MethodGet:
		writeJSON(w, map[string]interface{}{"data": f.tune})
	case r.URL.Path == "/v1/sys/auth/userpass/tune" && (r.Method == http.MethodPut || r.Method == http.MethodPost):
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		for key, value := range payload {
			f.tune[key] = value
		}
		f.auths["userpass/"]["description"] = payload["description"]
		w.WriteHeader(http.StatusNoContent)
	case (r.Method == http.MethodPut || r.Method == http.MethodPost) && strings.HasPrefix(r.URL.Path, "/v1/sys/auth/"):
		path := strings.TrimPrefix(r.URL.Path, "/v1/sys/auth/") + "/"
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		f.auths[path] = map[string]interface{}{
			"type": payload["type"], "description": payload["description"], "options": payload["options"],
			"local": payload["local"], "config": map[string]interface{}{}, "accessor": "auth_new_1",
		}
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/sys/auth/"):
		delete(f.auths, strings.TrimPrefix(r.URL.Path, "/v1/sys/auth/")+"/")
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/sys/audit":
		writeJSON(w, map[string]interface{}{"data": f.audits})
	case (r.Method == http.MethodPut || r.Method == http.MethodPost) && strings.HasPrefix(r.URL.Path, "/v1/sys/audit/"):
		path := strings.TrimPrefix(r.URL.Path, "/v1/sys/audit/") + "/"
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		f.audits[path] = map[string]interface{}{
			"type": payload["type"], "description": payload["description"], "options": payload["options"], "local": payload["local"],
		}
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/sys/audit/"):
		delete(f.audits, strings.TrimPrefix(r.URL.Path, "/v1/sys/audit/")+"/")
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/ssh/roles/"):
		role := strings.TrimPrefix(r.URL.Path, "/v1/ssh/roles/")
		value, ok := f.roles[role]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]interface{}{"data": value})
	case (r.Method == http.MethodPut || r.Method == http.MethodPost) && strings.HasPrefix(r.URL.Path, "/v1/ssh/roles/"):
		role := strings.TrimPrefix(r.URL.Path, "/v1/ssh/roles/")
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		f.roles[role] = payload
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/ssh/roles/"):
		delete(f.roles, strings.TrimPrefix(r.URL.Path, "/v1/ssh/roles/"))
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestEngineSSHAuthAndAuditActionsRoundTrip(t *testing.T) {
	fixture := newSystemActionFixture()
	server := httptest.NewServer(fixture)
	defer server.Close()
	client := newTestClient(t, server.URL)
	ctx := context.Background()
	engineItem := domain.Item{ID: "secret/", Name: "secret/", Subtitle: "kv"}

	enabler, _ := client.LoadEditor(ctx, domain.SectionEngines, engineItem, "enable")
	setEditorField(&enabler, "path", "newkv")
	if _, err := client.SaveEditor(ctx, enabler); err != nil {
		t.Fatalf("SaveEditor(enable engine) error = %v", err)
	}
	newEngine := domain.Item{ID: "newkv/", Name: "newkv/", Subtitle: "kv"}
	disabler, err := client.LoadEditor(ctx, domain.SectionEngines, newEngine, "disable")
	if err != nil {
		t.Fatalf("LoadEditor(disable engine) error = %v", err)
	}
	if _, err := client.SaveEditor(ctx, disabler); err != nil {
		t.Fatalf("SaveEditor(disable engine) error = %v", err)
	}

	roleItem := domain.Item{ID: "ssh/\x00developer", Name: "developer", Subtitle: "ssh"}
	roleEditor, _ := client.LoadEditor(ctx, domain.SectionSSH, roleItem, "edit")
	setEditorField(&roleEditor, "config", `{"key_type":"ca","allow_user_certificates":true,"allowed_users":"alice"}`)
	if _, err := client.SaveEditor(ctx, roleEditor); err != nil {
		t.Fatalf("SaveEditor(edit SSH role) error = %v", err)
	}
	roleCreator, _ := client.LoadEditor(ctx, domain.SectionSSH, roleItem, "create")
	setEditorField(&roleCreator, "name", "tester")
	setEditorField(&roleCreator, "config", `{"key_type":"ca","allow_user_certificates":true}`)
	if _, err := client.SaveEditor(ctx, roleCreator); err != nil {
		t.Fatalf("SaveEditor(create SSH role) error = %v", err)
	}
	roleDeleter, _ := client.LoadEditor(ctx, domain.SectionSSH, domain.Item{ID: "ssh/\x00tester", Name: "tester", Subtitle: "ssh"}, "delete")
	if _, err := client.SaveEditor(ctx, roleDeleter); err != nil {
		t.Fatalf("SaveEditor(delete SSH role) error = %v", err)
	}

	authItem := domain.Item{ID: "userpass/", Name: "userpass/", Subtitle: "userpass"}
	tuner, _ := client.LoadEditor(ctx, domain.SectionAuthMethods, authItem, "tune")
	setEditorField(&tuner, "description", "Local users")
	if _, err := client.SaveEditor(ctx, tuner); err != nil {
		t.Fatalf("SaveEditor(tune auth) error = %v", err)
	}
	authEnabler, _ := client.LoadEditor(ctx, domain.SectionAuthMethods, authItem, "enable")
	setEditorField(&authEnabler, "path", "oidc")
	setEditorField(&authEnabler, "type", "oidc")
	if _, err := client.SaveEditor(ctx, authEnabler); err != nil {
		t.Fatalf("SaveEditor(enable auth) error = %v", err)
	}
	authDisabler, _ := client.LoadEditor(ctx, domain.SectionAuthMethods, domain.Item{ID: "oidc/", Name: "oidc/", Subtitle: "oidc"}, "disable")
	if _, err := client.SaveEditor(ctx, authDisabler); err != nil {
		t.Fatalf("SaveEditor(disable auth) error = %v", err)
	}

	auditItem := domain.Item{ID: "file/", Name: "file/", Subtitle: "file"}
	auditEnabler, _ := client.LoadEditor(ctx, domain.SectionAudit, auditItem, "enable")
	setEditorField(&auditEnabler, "path", "socket")
	setEditorField(&auditEnabler, "type", "socket")
	setEditorField(&auditEnabler, "options", `{"address":"127.0.0.1:9090","socket_type":"tcp"}`)
	if _, err := client.SaveEditor(ctx, auditEnabler); err != nil {
		t.Fatalf("SaveEditor(enable audit) error = %v", err)
	}
	auditDisabler, _ := client.LoadEditor(ctx, domain.SectionAudit, domain.Item{ID: "socket/", Name: "socket/", Subtitle: "socket"}, "disable")
	if _, err := client.SaveEditor(ctx, auditDisabler); err != nil {
		t.Fatalf("SaveEditor(disable audit) error = %v", err)
	}
}

type identityActionFixture struct {
	groups   map[string]map[string]interface{}
	entities map[string]map[string]interface{}
	accounts map[string]map[string]interface{}
}

func newIdentityActionFixture() *identityActionFixture {
	return &identityActionFixture{
		groups: map[string]map[string]interface{}{},
		entities: map[string]map[string]interface{}{
			"entity-1": {
				"id": "entity-1", "name": "alice", "disabled": false,
				"policies": []string{}, "direct_group_ids": []string{}, "inherited_group_ids": []string{},
				"aliases": []interface{}{}, "metadata": map[string]interface{}{},
				"modify_index": 1, "creation_time": "2026-01-01T00:00:00Z", "last_update_time": "2026-01-01T00:00:00Z",
			},
		},
		accounts: map[string]map[string]interface{}{},
	}
}

func identityList(values map[string]map[string]interface{}) map[string]interface{} {
	keys := make([]string, 0, len(values))
	info := make(map[string]interface{}, len(values))
	for id, value := range values {
		keys = append(keys, id)
		info[id] = map[string]interface{}{"name": value["name"], "aliases": value["aliases"]}
	}
	sort.Strings(keys)
	return map[string]interface{}{"keys": keys, "key_info": info}
}

func (f *identityActionFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/identity/group/id" && r.URL.Query().Get("list") == "true":
		writeJSON(w, map[string]interface{}{"data": identityList(f.groups)})
	case r.Method == http.MethodGet && r.URL.Path == "/v1/identity/entity/id" && r.URL.Query().Get("list") == "true":
		writeJSON(w, map[string]interface{}{"data": identityList(f.entities)})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/identity/group/id/"):
		id := strings.TrimPrefix(r.URL.Path, "/v1/identity/group/id/")
		value, ok := f.groups[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]interface{}{"data": value})
	case (r.Method == http.MethodPut || r.Method == http.MethodPost) && r.URL.Path == "/v1/identity/group":
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		payload["id"] = "group-new"
		payload["modify_index"] = 1
		payload["parent_group_ids"] = []string{}
		payload["creation_time"] = "2026-01-01T00:00:00Z"
		payload["last_update_time"] = "2026-01-01T00:00:00Z"
		f.groups["group-new"] = payload
		writeJSON(w, map[string]interface{}{"data": map[string]interface{}{"id": "group-new", "name": payload["name"]}})
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/identity/group/id/"):
		delete(f.groups, strings.TrimPrefix(r.URL.Path, "/v1/identity/group/id/"))
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/identity/entity/id/"):
		id := strings.TrimPrefix(r.URL.Path, "/v1/identity/entity/id/")
		value, ok := f.entities[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]interface{}{"data": value})
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/identity/entity/id/"):
		delete(f.entities, strings.TrimPrefix(r.URL.Path, "/v1/identity/entity/id/"))
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/sys/policies/acl" && r.URL.Query().Get("list") == "true":
		writeJSON(w, map[string]interface{}{"data": map[string]interface{}{"keys": []string{"default"}}})
	case r.Method == http.MethodGet && r.URL.Path == "/v1/sys/auth":
		writeJSON(w, map[string]interface{}{"data": map[string]interface{}{
			"userpass/": map[string]interface{}{"type": "userpass", "accessor": "auth_userpass_1", "description": "Users", "config": map[string]interface{}{}, "options": map[string]string{}},
		}})
	case r.Method == http.MethodGet && r.URL.Path == "/v1/auth/userpass/users" && r.URL.Query().Get("list") == "true":
		keys := make([]string, 0, len(f.accounts))
		for name := range f.accounts {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		writeJSON(w, map[string]interface{}{"data": map[string]interface{}{"keys": keys}})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/auth/userpass/users/"):
		name := strings.TrimPrefix(r.URL.Path, "/v1/auth/userpass/users/")
		value, ok := f.accounts[name]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]interface{}{"data": value})
	case (r.Method == http.MethodPut || r.Method == http.MethodPost) && strings.HasPrefix(r.URL.Path, "/v1/auth/userpass/users/"):
		name := strings.TrimPrefix(r.URL.Path, "/v1/auth/userpass/users/")
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		f.accounts[name] = map[string]interface{}{
			"token_policies": payload["token_policies"], "token_ttl": 0, "token_max_ttl": 0,
			"token_period": 0, "token_type": "default-service", "token_num_uses": 0, "token_bound_cidrs": []string{},
		}
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/auth/userpass/users/"):
		delete(f.accounts, strings.TrimPrefix(r.URL.Path, "/v1/auth/userpass/users/"))
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestGroupAndUserCreateDeleteActionsRoundTrip(t *testing.T) {
	fixture := newIdentityActionFixture()
	server := httptest.NewServer(fixture)
	defer server.Close()
	client := newTestClient(t, server.URL)
	ctx := context.Background()

	anchorGroup := domain.Item{ID: "anchor", Name: "anchor"}
	groupCreator, err := client.LoadEditor(ctx, domain.SectionGroups, anchorGroup, "create")
	if err != nil {
		t.Fatalf("LoadEditor(create group) error = %v", err)
	}
	setEditorField(&groupCreator, "name", "operations")
	setEditorField(&groupCreator, "policies", "default")
	if _, err := client.SaveEditor(ctx, groupCreator); err != nil {
		t.Fatalf("SaveEditor(create group) error = %v", err)
	}
	groupItem := domain.Item{ID: "group-new", Name: "operations"}
	groupDeleter, err := client.LoadEditor(ctx, domain.SectionGroups, groupItem, "delete")
	if err != nil {
		t.Fatalf("LoadEditor(delete group) error = %v", err)
	}
	if _, err := client.SaveEditor(ctx, groupDeleter); err != nil {
		t.Fatalf("SaveEditor(delete group) error = %v", err)
	}

	anchorUser := domain.Item{ID: "entity:entity-1", Name: "alice"}
	client.mu.Lock()
	client.userRefs[anchorUser.ID] = userRef{EntityID: "entity-1"}
	client.mu.Unlock()
	userCreator, err := client.LoadEditor(ctx, domain.SectionUsers, anchorUser, "create")
	if err != nil {
		t.Fatalf("LoadEditor(create user) error = %v", err)
	}
	setEditorField(&userCreator, "username", "charlie")
	setEditorField(&userCreator, "password", "correct horse battery staple")
	setEditorField(&userCreator, "confirm", "correct horse battery staple")
	setEditorField(&userCreator, "policies", "default")
	if _, err := client.SaveEditor(ctx, userCreator); err != nil {
		t.Fatalf("SaveEditor(create user) error = %v", err)
	}
	users, err := client.List(ctx, domain.SectionUsers)
	if err != nil {
		t.Fatalf("List(users) error = %v", err)
	}
	var charlie domain.Item
	var alice domain.Item
	for _, user := range users {
		switch user.Name {
		case "charlie":
			charlie = user
		case "alice":
			alice = user
		}
	}
	if charlie.ID == "" || alice.ID == "" {
		t.Fatalf("created/auth entity users not listed: %#v", users)
	}
	revoker, err := client.LoadEditor(ctx, domain.SectionUsers, charlie, "revoke")
	if err != nil {
		t.Fatalf("LoadEditor(revoke access) error = %v", err)
	}
	if _, err := client.SaveEditor(ctx, revoker); err != nil {
		t.Fatalf("SaveEditor(revoke access) error = %v", err)
	}
	entityDeleter, err := client.LoadEditor(ctx, domain.SectionUsers, alice, "delete")
	if err != nil {
		t.Fatalf("LoadEditor(delete entity) error = %v", err)
	}
	if _, err := client.SaveEditor(ctx, entityDeleter); err != nil {
		t.Fatalf("SaveEditor(delete entity) error = %v", err)
	}
	if len(fixture.groups) != 0 || len(fixture.accounts) != 0 || len(fixture.entities) != 0 {
		t.Fatalf("resources remain after verified deletes: groups=%#v accounts=%#v entities=%#v", fixture.groups, fixture.accounts, fixture.entities)
	}
}
