package vaultapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/vault/api"

	"vault-manager/internal/domain"
)

func (c *Client) loadActionEditor(ctx context.Context, section domain.Section, item domain.Item, actionID string) (domain.Editor, error) {
	switch section {
	case domain.SectionStatus:
		return c.loadStatusAction(item, actionID)
	case domain.SectionGroups:
		return c.loadGroupAction(ctx, item, actionID)
	case domain.SectionUsers:
		return c.loadUserAction(ctx, item, actionID)
	case domain.SectionPolicies:
		return c.loadPolicyAction(ctx, item, actionID)
	case domain.SectionEngines:
		return c.loadEngineAction(ctx, item, actionID)
	case domain.SectionSSH:
		return c.loadSSHAction(ctx, item, actionID)
	case domain.SectionAuthMethods:
		return c.loadAuthAction(ctx, item, actionID)
	case domain.SectionAudit:
		return c.loadAuditAction(ctx, item, actionID)
	case domain.SectionTokens:
		return c.loadTokenAction(ctx, item, actionID)
	default:
		return domain.Editor{}, errors.New("this editor is not available")
	}
}

func (c *Client) saveActionEditor(ctx context.Context, editor domain.Editor) (domain.Preview, error) {
	switch editor.Section {
	case domain.SectionStatus:
		return c.saveStatusAction(ctx, editor)
	case domain.SectionGroups:
		return c.saveGroupAction(ctx, editor)
	case domain.SectionUsers:
		return c.saveUserAction(ctx, editor)
	case domain.SectionPolicies:
		return c.savePolicyAction(ctx, editor)
	case domain.SectionEngines:
		return c.saveEngineAction(ctx, editor)
	case domain.SectionSSH:
		return c.saveSSHAction(ctx, editor)
	case domain.SectionAuthMethods:
		return c.saveAuthAction(ctx, editor)
	case domain.SectionAudit:
		return c.saveAuditAction(ctx, editor)
	case domain.SectionTokens:
		return c.saveTokenAction(ctx, editor)
	default:
		return domain.Preview{}, errors.New("this editor cannot be saved")
	}
}

func newActionEditor(section domain.Section, item domain.Item, actionID, title string, fields []domain.EditField) domain.Editor {
	return domain.Editor{
		Section:  section,
		Item:     item,
		ActionID: actionID,
		Title:    title,
		Fields:   fields,
		Context:  make(map[string]string),
	}
}

func confirmationEditor(section domain.Section, item domain.Item, actionID, title, submit, warning string) domain.Editor {
	editor := newActionEditor(section, item, actionID, title, nil)
	editor.SubmitLabel = submit
	editor.SubmitAlways = true
	editor.RefreshList = true
	editor.Warnings = []string{warning}
	return editor
}

func actionField(id, label, value string, kind domain.FieldKind, help string, examples ...string) domain.EditField {
	return domain.EditField{
		ID:       id,
		Label:    label,
		Value:    value,
		Original: value,
		Kind:     kind,
		Help:     help,
		Examples: examples,
	}
}

func requiredField(editor domain.Editor, id string) (string, error) {
	value := strings.TrimSpace(fieldValue(editor, id))
	if value == "" {
		return "", fmt.Errorf("%w: %s must not be empty", errValidation, id)
	}
	return value, nil
}

func requiredRawField(editor domain.Editor, id string) (string, error) {
	value := fieldValue(editor, id)
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%w: %s must not be empty", errValidation, id)
	}
	return value, nil
}

func boolField(editor domain.Editor, id string) (bool, error) {
	value, err := strconv.ParseBool(strings.TrimSpace(fieldValue(editor, id)))
	if err != nil {
		return false, fmt.Errorf("%w: %s must be true or false", errValidation, id)
	}
	return value, nil
}

func parseJSONObjectField(editor domain.Editor, id string) (map[string]interface{}, error) {
	value := strings.TrimSpace(fieldValue(editor, id))
	if value == "" {
		return map[string]interface{}{}, nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(value), &result); err != nil || result == nil {
		return nil, fmt.Errorf("%w: %s must be a JSON object", errValidation, id)
	}
	return result, nil
}

func parseStringMapField(editor domain.Editor, id string) (map[string]string, error) {
	value := strings.TrimSpace(fieldValue(editor, id))
	if value == "" {
		return map[string]string{}, nil
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(value), &result); err != nil || result == nil {
		return nil, fmt.Errorf("%w: %s must be a JSON object with string values", errValidation, id)
	}
	return result, nil
}

func normalizeMountPath(value string) (string, error) {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" {
		return "", fmt.Errorf("%w: mount path must not be empty", errValidation)
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("%w: mount path contains an invalid segment", errValidation)
		}
	}
	return value + "/", nil
}

func secretPath(mount, relative string) (string, error) {
	relative = strings.Trim(strings.TrimSpace(relative), "/")
	if relative == "" {
		return mount, nil
	}
	for _, part := range strings.Split(relative, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("%w: secret path contains an invalid segment", errValidation)
		}
	}
	return mount + relative, nil
}

func editorResult(title string, values ...string) domain.Preview {
	return domain.Preview{Title: title, Rows: rows(values...)}
}

func (c *Client) loadStatusAction(item domain.Item, actionID string) (domain.Editor, error) {
	switch actionID {
	case "seal":
		editor := confirmationEditor(domain.SectionStatus, item, actionID, "Seal the Vault cluster", "Seal", "Sealing immediately blocks normal Vault operations until the unseal threshold is met again.")
		editor.SuccessMessage = "Cluster sealed"
		return editor, nil
	case "unseal":
		editor := newActionEditor(domain.SectionStatus, item, actionID, "Submit an unseal key", []domain.EditField{
			actionField("key", "Unseal key", "", domain.FieldSecret, "One unseal key share. Submit additional shares by running this action again."),
		})
		editor.SubmitLabel = "Submit key"
		editor.RefreshList = true
		editor.SuccessMessage = "Unseal key accepted"
		editor.Tips = []string{"The key is write-only in this UI and is never included in logs or error text."}
		return editor, nil
	case "restore":
		editor := newActionEditor(domain.SectionStatus, item, actionID, "Restore a Raft snapshot", []domain.EditField{
			actionField("file", "Snapshot file", "", domain.FieldText, "Local path to a Vault Raft snapshot readable by vault-manager.", "/secure/backups/vault.snap"),
			actionField("force", "Force restore", "false", domain.FieldBoolean, "Use only when the snapshot was created with a different seal key."),
		})
		editor.SubmitLabel = "Restore"
		editor.RefreshList = true
		editor.SuccessMessage = "Snapshot restore request completed"
		editor.Warnings = []string{"Restoring replaces Raft storage state and can make the cluster temporarily unavailable."}
		return editor, nil
	default:
		return domain.Editor{}, errors.New("unknown status action")
	}
}

func (c *Client) saveStatusAction(ctx context.Context, editor domain.Editor) (domain.Preview, error) {
	switch editor.ActionID {
	case "seal":
		if err := c.api.Sys().SealWithContext(ctx); err != nil {
			return domain.Preview{}, err
		}
		status, err := c.api.Sys().SealStatusWithContext(ctx)
		if err != nil {
			return domain.Preview{}, err
		}
		if !status.Sealed {
			return domain.Preview{}, errors.New("seal verification failed")
		}
		return sealStatusPreview(status), nil
	case "unseal":
		key, err := requiredField(editor, "key")
		if err != nil {
			return domain.Preview{}, err
		}
		status, err := c.api.Sys().UnsealWithContext(ctx, key)
		if err != nil {
			return domain.Preview{}, err
		}
		return sealStatusPreview(status), nil
	case "restore":
		path, err := requiredField(editor, "file")
		if err != nil {
			return domain.Preview{}, err
		}
		force, err := boolField(editor, "force")
		if err != nil {
			return domain.Preview{}, err
		}
		file, err := os.Open(path)
		if err != nil {
			return domain.Preview{}, fmt.Errorf("open snapshot: %w", err)
		}
		defer file.Close()
		if err := c.api.Sys().RaftSnapshotRestoreWithContext(ctx, file, force); err != nil {
			return domain.Preview{}, err
		}
		return editorResult("Raft snapshot restored", "File", path, "Forced", boolString(force)), nil
	default:
		return domain.Preview{}, errors.New("unknown status action")
	}
}

func sealStatusPreview(status *api.SealStatusResponse) domain.Preview {
	return editorResult("Seal status",
		"Initialized", boolString(status.Initialized),
		"Sealed", boolString(status.Sealed),
		"Progress", strconv.Itoa(status.Progress),
		"Threshold", strconv.Itoa(status.T),
		"Seal type", status.Type,
	)
}

func (c *Client) loadGroupAction(ctx context.Context, item domain.Item, actionID string) (domain.Editor, error) {
	switch actionID {
	case "create":
		if _, entityInfo, err := c.listIdentity(ctx, "identity/entity/id"); err == nil {
			c.mu.Lock()
			cacheNames(c.entityNames, entityInfo)
			c.mu.Unlock()
		}
		if _, groupInfo, err := c.listIdentity(ctx, "identity/group/id"); err == nil {
			c.mu.Lock()
			cacheNames(c.groupNames, groupInfo)
			c.mu.Unlock()
		}
		editor := newActionEditor(domain.SectionGroups, item, actionID, "Create an internal identity group", []domain.EditField{
			actionField("name", "Name", "", domain.FieldText, "Unique Vault identity group name.", "jira-developers"),
			actionField("policies", "Policies", "", domain.FieldText, "Comma-separated existing ACL policy names.", "default, jira-developers-policy"),
			actionField("members", "Members", "", domain.FieldText, "Comma-separated existing identity entity names.", "alice, bob"),
			actionField("nested_groups", "Nested groups", "", domain.FieldText, "Comma-separated existing internal group names.", "engineering"),
			actionField("metadata", "Metadata", "{}", domain.FieldText, "JSON object stored with the group.", `{"owner":"platform"}`),
		})
		editor.SubmitLabel = "Create"
		editor.RefreshList = true
		editor.SuccessMessage = "Group created and verified"
		editor.Tips = []string{"Names are resolved to Vault IDs immediately before the write."}
		return editor, nil
	case "delete":
		secret, err := c.api.Logical().ReadWithContext(ctx, "identity/group/id/"+url.PathEscape(item.ID))
		if err != nil {
			return domain.Editor{}, err
		}
		editor := confirmationEditor(domain.SectionGroups, item, actionID, "Delete group "+item.Name, "Delete", "Deleting a group removes its policy and membership assignments.")
		editor.Fingerprint = fingerprint(groupSnapshot(secretData(secret)))
		editor.Context["group_id"] = item.ID
		editor.SuccessMessage = "Group deleted and verified"
		return editor, nil
	default:
		return domain.Editor{}, errors.New("unknown group action")
	}
}

func (c *Client) saveGroupAction(ctx context.Context, editor domain.Editor) (domain.Preview, error) {
	switch editor.ActionID {
	case "create":
		name, err := requiredField(editor, "name")
		if err != nil {
			return domain.Preview{}, err
		}
		policies, err := c.validatePolicies(ctx, parseCSV(fieldValue(editor, "policies")))
		if err != nil {
			return domain.Preview{}, err
		}
		members, err := c.resolveEntityNames(ctx, parseCSV(fieldValue(editor, "members")))
		if err != nil {
			return domain.Preview{}, err
		}
		nested, err := c.resolveGroupNames(ctx, parseCSV(fieldValue(editor, "nested_groups")), "")
		if err != nil {
			return domain.Preview{}, err
		}
		metadata, err := parseMetadata(fieldValue(editor, "metadata"))
		if err != nil {
			return domain.Preview{}, err
		}
		payload := map[string]interface{}{
			"name": name, "type": "internal", "policies": policies,
			"member_entity_ids": members, "member_group_ids": nested, "metadata": metadata,
		}
		created, err := c.api.Logical().WriteWithContext(ctx, "identity/group", payload)
		if err != nil {
			return domain.Preview{}, err
		}
		groupID := stringValue(secretData(created)["id"])
		if groupID == "" {
			_, info, listErr := c.listIdentity(ctx, "identity/group/id")
			if listErr != nil {
				return domain.Preview{}, listErr
			}
			for id, details := range info {
				if stringValue(details["name"]) == name {
					if groupID != "" {
						return domain.Preview{}, errors.New("group verification found duplicate names")
					}
					groupID = id
				}
			}
		}
		if groupID == "" {
			return domain.Preview{}, errors.New("group create response did not include an id")
		}
		verified, err := c.api.Logical().ReadWithContext(ctx, "identity/group/id/"+url.PathEscape(groupID))
		if err != nil {
			return domain.Preview{}, err
		}
		if fingerprint(groupEditableSnapshot(secretData(verified))) != fingerprint(groupEditableSnapshot(payload)) {
			return domain.Preview{}, errors.New("create verification failed for group")
		}
		return c.previewGroup(ctx, domain.Item{ID: groupID, Name: name})
	case "delete":
		groupID := editor.Context["group_id"]
		path := "identity/group/id/" + url.PathEscape(groupID)
		current, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if current == nil || fingerprint(groupSnapshot(secretData(current))) != editor.Fingerprint {
			return domain.Preview{}, errConflict
		}
		if _, err := c.api.Logical().DeleteWithContext(ctx, path); err != nil {
			return domain.Preview{}, err
		}
		remaining, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if remaining != nil {
			return domain.Preview{}, errors.New("delete verification failed for group")
		}
		return editorResult("Group deleted", "Name", editor.Item.Name), nil
	default:
		return domain.Preview{}, errors.New("unknown group action")
	}
}

type userpassChoice struct {
	Label       string `json:"label"`
	Mount       string `json:"mount"`
	Username    string `json:"username"`
	Fingerprint string `json:"fingerprint"`
}

func (c *Client) userpassChoices(ctx context.Context, item domain.Item) ([]userpassChoice, error) {
	ref, err := c.userReference(item.ID)
	if err != nil {
		return nil, err
	}
	if _, err := c.authMounts(ctx); err != nil {
		return nil, err
	}
	refs := []userRef{ref}
	if ref.EntityID != "" {
		refs = nil
		secret, readErr := c.api.Logical().ReadWithContext(ctx, "identity/entity/id/"+url.PathEscape(ref.EntityID))
		if readErr != nil {
			return nil, readErr
		}
		for _, alias := range mapSlice(secretData(secret)["aliases"]) {
			mount := c.authMount(stringValue(alias["mount_accessor"]))
			name := stringValue(alias["name"])
			if mount.Type == "userpass" && name != "" {
				refs = append(refs, userRef{EntityID: ref.EntityID, Mount: mount.APIPath, Username: name})
			}
		}
	}
	choices := make([]userpassChoice, 0, len(refs))
	for _, account := range refs {
		if account.Mount == "" || account.Username == "" {
			continue
		}
		secret, readErr := c.api.Logical().ReadWithContext(ctx, account.Mount+"users/"+url.PathEscape(account.Username))
		if readErr != nil {
			return nil, readErr
		}
		choices = append(choices, userpassChoice{
			Label:       strings.TrimSuffix(account.Mount, "/") + "/" + account.Username,
			Mount:       account.Mount,
			Username:    account.Username,
			Fingerprint: fingerprint(userpassSnapshot(secretData(secret))),
		})
	}
	sort.Slice(choices, func(i, j int) bool { return choices[i].Label < choices[j].Label })
	return choices, nil
}

func (c *Client) loadUserAction(ctx context.Context, item domain.Item, actionID string) (domain.Editor, error) {
	switch actionID {
	case "create":
		mounts, err := c.authMounts(ctx)
		if err != nil {
			return domain.Editor{}, err
		}
		paths := make([]string, 0)
		for _, mount := range mounts {
			if mount.Type == "userpass" {
				paths = append(paths, strings.TrimSuffix(mount.Path, "/"))
			}
		}
		sort.Strings(paths)
		if len(paths) == 0 {
			return domain.Editor{}, fmt.Errorf("%w: no userpass auth method is enabled", errValidation)
		}
		editor := newActionEditor(domain.SectionUsers, item, actionID, "Create a userpass account", []domain.EditField{
			actionField("mount", "Auth mount", paths[0], domain.FieldText, "Existing userpass auth mount path.", paths...),
			actionField("username", "Username", "", domain.FieldText, "New username within the selected auth mount.", "alice"),
			actionField("password", "Password", "", domain.FieldSecret, "Write-only initial password."),
			actionField("confirm", "Confirm password", "", domain.FieldSecret, "Must exactly match Password."),
			actionField("policies", "Token policies", "", domain.FieldText, "Comma-separated existing ACL policies assigned at login.", "default, developers"),
		})
		editor.SubmitLabel = "Create"
		editor.RefreshList = true
		editor.SuccessMessage = "Userpass account created and verified"
		editor.Tips = []string{"The account appears as auth-only until a successful login creates or links an identity alias."}
		return editor, nil
	case "revoke":
		choices, err := c.userpassChoices(ctx, item)
		if err != nil {
			return domain.Editor{}, err
		}
		if len(choices) == 0 {
			return domain.Editor{}, fmt.Errorf("%w: user has no userpass account", errValidation)
		}
		kind := domain.FieldReadOnly
		if len(choices) > 1 {
			kind = domain.FieldText
		}
		labels := make([]string, 0, len(choices))
		for _, choice := range choices {
			labels = append(labels, choice.Label)
		}
		encoded, err := json.Marshal(choices)
		if err != nil {
			return domain.Editor{}, err
		}
		editor := newActionEditor(domain.SectionUsers, item, actionID, "Revoke userpass access for "+item.Name, []domain.EditField{
			actionField("account", "Account", choices[0].Label, kind, "Userpass account to delete.", labels...),
		})
		editor.Context["accounts"] = string(encoded)
		editor.SubmitLabel = "Revoke"
		editor.SubmitAlways = true
		editor.RefreshList = true
		editor.SuccessMessage = "Userpass access revoked and verified"
		editor.Warnings = []string{"This deletes the selected login account. Existing tokens remain valid until they expire or are revoked separately."}
		return editor, nil
	case "delete":
		ref, err := c.userReference(item.ID)
		if err != nil {
			return domain.Editor{}, err
		}
		if ref.EntityID == "" {
			return domain.Editor{}, fmt.Errorf("%w: auth-only user has no identity entity", errValidation)
		}
		secret, err := c.api.Logical().ReadWithContext(ctx, "identity/entity/id/"+url.PathEscape(ref.EntityID))
		if err != nil {
			return domain.Editor{}, err
		}
		editor := confirmationEditor(domain.SectionUsers, item, actionID, "Delete identity entity "+item.Name, "Delete", "Deleting the entity removes its aliases and identity/group policy association. Auth backend accounts may still exist.")
		editor.Context["entity_id"] = ref.EntityID
		editor.Fingerprint = fingerprint(entitySnapshot(secretData(secret)))
		editor.SuccessMessage = "Identity entity deleted and verified"
		return editor, nil
	default:
		return domain.Editor{}, errors.New("unknown user action")
	}
}

func (c *Client) saveUserAction(ctx context.Context, editor domain.Editor) (domain.Preview, error) {
	switch editor.ActionID {
	case "create":
		mount, err := normalizeMountPath(fieldValue(editor, "mount"))
		if err != nil {
			return domain.Preview{}, err
		}
		username, err := requiredField(editor, "username")
		if err != nil {
			return domain.Preview{}, err
		}
		password, err := requiredRawField(editor, "password")
		if err != nil {
			return domain.Preview{}, err
		}
		if password != fieldValue(editor, "confirm") {
			return domain.Preview{}, fmt.Errorf("%w: password confirmation does not match", errValidation)
		}
		mounts, err := c.authMounts(ctx)
		if err != nil {
			return domain.Preview{}, err
		}
		apiMount := ""
		for _, available := range mounts {
			if available.Type == "userpass" && available.Path == mount {
				apiMount = available.APIPath
			}
		}
		if apiMount == "" {
			return domain.Preview{}, fmt.Errorf("%w: unknown userpass auth mount", errValidation)
		}
		policies, err := c.validatePolicies(ctx, parseCSV(fieldValue(editor, "policies")))
		if err != nil {
			return domain.Preview{}, err
		}
		path := apiMount + "users/" + url.PathEscape(username)
		existing, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if existing != nil {
			return domain.Preview{}, fmt.Errorf("%w: userpass account already exists", errValidation)
		}
		if _, err := c.api.Logical().WriteWithContext(ctx, path, map[string]interface{}{"password": password, "token_policies": policies}); err != nil {
			return domain.Preview{}, err
		}
		verified, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if verified == nil || !equalStringSets(stringSlice(secretData(verified)["token_policies"]), policies) {
			return domain.Preview{}, errors.New("create verification failed for userpass account")
		}
		return c.previewUserpass(ctx, username, apiMount, username, "auth-only")
	case "revoke":
		var choices []userpassChoice
		if err := json.Unmarshal([]byte(editor.Context["accounts"]), &choices); err != nil {
			return domain.Preview{}, errors.New("account selection context is invalid; reload")
		}
		selected := fieldValue(editor, "account")
		var choice *userpassChoice
		for index := range choices {
			if choices[index].Label == selected {
				choice = &choices[index]
			}
		}
		if choice == nil {
			return domain.Preview{}, fmt.Errorf("%w: select one of the listed accounts", errValidation)
		}
		path := choice.Mount + "users/" + url.PathEscape(choice.Username)
		current, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if current == nil || fingerprint(userpassSnapshot(secretData(current))) != choice.Fingerprint {
			return domain.Preview{}, errConflict
		}
		if _, err := c.api.Logical().DeleteWithContext(ctx, path); err != nil {
			return domain.Preview{}, err
		}
		remaining, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if remaining != nil {
			return domain.Preview{}, errors.New("revoke verification failed for userpass account")
		}
		return editorResult("Userpass access revoked", "Account", choice.Label), nil
	case "delete":
		entityID := editor.Context["entity_id"]
		path := "identity/entity/id/" + url.PathEscape(entityID)
		current, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if current == nil || fingerprint(entitySnapshot(secretData(current))) != editor.Fingerprint {
			return domain.Preview{}, errConflict
		}
		if _, err := c.api.Logical().DeleteWithContext(ctx, path); err != nil {
			return domain.Preview{}, err
		}
		remaining, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if remaining != nil {
			return domain.Preview{}, errors.New("delete verification failed for identity entity")
		}
		return editorResult("Identity entity deleted", "Name", editor.Item.Name), nil
	default:
		return domain.Preview{}, errors.New("unknown user action")
	}
}

func equalStringSets(left, right []string) bool {
	left = sortedStrings(left)
	right = sortedStrings(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (c *Client) loadPolicyAction(ctx context.Context, item domain.Item, actionID string) (domain.Editor, error) {
	switch actionID {
	case "edit":
		secret, err := c.api.Logical().ReadWithContext(ctx, "sys/policies/acl/"+url.PathEscape(item.ID))
		if err != nil {
			return domain.Editor{}, err
		}
		body := policyBody(secretData(secret))
		editor := newActionEditor(domain.SectionPolicies, item, actionID, "Edit ACL policy "+item.Name, []domain.EditField{
			actionField("body", "Policy", body, domain.FieldMultiline, "Vault ACL policy in HCL or JSON.", "path \"secret/data/app/*\" {\n  capabilities = [\"read\"]\n}"),
		})
		editor.Context["name"] = item.ID
		editor.Fingerprint = fingerprint(body)
		editor.SubmitLabel = "Update"
		editor.SuccessMessage = "Policy updated and verified"
		editor.Tips = []string{"Policy changes affect new authorization checks immediately."}
		return editor, nil
	case "create":
		editor := newActionEditor(domain.SectionPolicies, item, actionID, "Create an ACL policy", []domain.EditField{
			actionField("name", "Name", "", domain.FieldText, "Unique ACL policy name.", "app-readonly"),
			actionField("body", "Policy", "", domain.FieldMultiline, "Vault ACL policy in HCL or JSON.", "path \"secret/data/app/*\" {\n  capabilities = [\"read\"]\n}"),
		})
		editor.SubmitLabel = "Create"
		editor.RefreshList = true
		editor.SuccessMessage = "Policy created and verified"
		editor.Tips = []string{"An empty policy grants no permissions; use explicit paths and the minimum capabilities required."}
		return editor, nil
	case "delete":
		secret, err := c.api.Logical().ReadWithContext(ctx, "sys/policies/acl/"+url.PathEscape(item.ID))
		if err != nil {
			return domain.Editor{}, err
		}
		editor := confirmationEditor(domain.SectionPolicies, item, actionID, "Delete ACL policy "+item.Name, "Delete", "Tokens and identities referencing this policy will lose the permissions it grants.")
		editor.Context["name"] = item.ID
		editor.Fingerprint = fingerprint(policyBody(secretData(secret)))
		editor.SuccessMessage = "Policy deleted and verified"
		return editor, nil
	default:
		return domain.Editor{}, errors.New("unknown policy action")
	}
}

func (c *Client) savePolicyAction(ctx context.Context, editor domain.Editor) (domain.Preview, error) {
	name := editor.Context["name"]
	switch editor.ActionID {
	case "edit":
		path := "sys/policies/acl/" + url.PathEscape(name)
		current, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if current == nil || fingerprint(policyBody(secretData(current))) != editor.Fingerprint {
			return domain.Preview{}, errConflict
		}
		body, err := requiredRawField(editor, "body")
		if err != nil {
			return domain.Preview{}, err
		}
		if _, err := c.api.Logical().WriteWithContext(ctx, path, map[string]interface{}{"policy": body}); err != nil {
			return domain.Preview{}, err
		}
		verified, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if verified == nil || policyBody(secretData(verified)) != body {
			return domain.Preview{}, errors.New("update verification failed for policy")
		}
		return c.previewPolicy(ctx, editor.Item)
	case "create":
		var err error
		name, err = requiredField(editor, "name")
		if err != nil {
			return domain.Preview{}, err
		}
		body, err := requiredRawField(editor, "body")
		if err != nil {
			return domain.Preview{}, err
		}
		path := "sys/policies/acl/" + url.PathEscape(name)
		existing, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if existing != nil {
			return domain.Preview{}, fmt.Errorf("%w: policy already exists", errValidation)
		}
		if _, err := c.api.Logical().WriteWithContext(ctx, path, map[string]interface{}{"policy": body}); err != nil {
			return domain.Preview{}, err
		}
		verified, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if verified == nil || policyBody(secretData(verified)) != body {
			return domain.Preview{}, errors.New("create verification failed for policy")
		}
		return c.previewPolicy(ctx, domain.Item{ID: name, Name: name})
	case "delete":
		path := "sys/policies/acl/" + url.PathEscape(name)
		current, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if current == nil || fingerprint(policyBody(secretData(current))) != editor.Fingerprint {
			return domain.Preview{}, errConflict
		}
		if _, err := c.api.Logical().DeleteWithContext(ctx, path); err != nil {
			return domain.Preview{}, err
		}
		remaining, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if remaining != nil {
			return domain.Preview{}, errors.New("delete verification failed for policy")
		}
		return editorResult("Policy deleted", "Name", name), nil
	default:
		return domain.Preview{}, errors.New("unknown policy action")
	}
}

func policyBody(data map[string]interface{}) string {
	return fallback(stringValue(data["policy"]), stringValue(data["rules"]))
}

func (c *Client) loadEngineAction(ctx context.Context, item domain.Item, actionID string) (domain.Editor, error) {
	switch actionID {
	case "browse":
		editor := newActionEditor(domain.SectionEngines, item, actionID, "Browse "+item.Name, []domain.EditField{
			actionField("path", "Relative path", "", domain.FieldText, "Path inside the selected secrets engine; leave empty to address the mount root.", "app/config", "metadata/app"),
			actionField("operation", "Operation", "list", domain.FieldText, "Use list to enumerate keys or read to show data.", "list", "read"),
		})
		editor.Context["mount"] = item.ID
		editor.SubmitLabel = "Open"
		editor.SubmitAlways = true
		editor.SuccessMessage = "Secret path loaded"
		editor.Tips = []string{"KV v2 data reads normally use data/<name>; key listings normally use metadata/<name>."}
		return editor, nil
	case "enable":
		editor := newActionEditor(domain.SectionEngines, item, actionID, "Enable a secrets engine", []domain.EditField{
			actionField("path", "Mount path", "", domain.FieldText, "Unique mount path.", "secret"),
			actionField("type", "Engine type", "kv", domain.FieldText, "Vault secrets engine type or plugin name.", "kv", "ssh", "database"),
			actionField("description", "Description", "", domain.FieldText, "Human-readable purpose of the mount."),
			actionField("options", "Options", `{"version":"2"}`, domain.FieldText, "JSON object with string values; use {} when the engine has no options.", `{"version":"2"}`),
			actionField("local", "Local", "false", domain.FieldBoolean, "Local mounts are not replicated across clusters."),
		})
		editor.SubmitLabel = "Enable"
		editor.RefreshList = true
		editor.SuccessMessage = "Secrets engine enabled and verified"
		return editor, nil
	case "disable":
		mount, err := c.api.Sys().GetMountWithContext(ctx, strings.TrimSuffix(item.ID, "/"))
		if err != nil {
			return domain.Editor{}, err
		}
		editor := confirmationEditor(domain.SectionEngines, item, actionID, "Disable secrets engine "+item.Name, "Disable", "Disabling a secrets engine revokes its leases and makes all data at the mount inaccessible.")
		editor.Context["mount"] = item.ID
		editor.Fingerprint = fingerprint(mountSnapshot(mount))
		editor.SuccessMessage = "Secrets engine disabled and verified"
		return editor, nil
	default:
		return domain.Editor{}, errors.New("unknown engine action")
	}
}

func (c *Client) saveEngineAction(ctx context.Context, editor domain.Editor) (domain.Preview, error) {
	switch editor.ActionID {
	case "browse":
		mount, err := normalizeMountPath(editor.Context["mount"])
		if err != nil {
			return domain.Preview{}, err
		}
		path, err := secretPath(mount, fieldValue(editor, "path"))
		if err != nil {
			return domain.Preview{}, err
		}
		operation := strings.ToLower(strings.TrimSpace(fieldValue(editor, "operation")))
		var secret *api.Secret
		switch operation {
		case "list":
			secret, err = c.api.Logical().ListWithContext(ctx, path)
		case "read":
			secret, err = c.api.Logical().ReadWithContext(ctx, path)
		default:
			return domain.Preview{}, fmt.Errorf("%w: operation must be list or read", errValidation)
		}
		if err != nil {
			return domain.Preview{}, err
		}
		preview := domain.Preview{Title: path, Subtitle: operation}
		if secret == nil || len(secret.Data) == 0 {
			preview.Rows = rows("Result", "No data")
		} else {
			preview.Rows = mapRows(secret.Data)
		}
		return preview, nil
	case "enable":
		mount, err := normalizeMountPath(fieldValue(editor, "path"))
		if err != nil {
			return domain.Preview{}, err
		}
		engineType, err := requiredField(editor, "type")
		if err != nil {
			return domain.Preview{}, err
		}
		options, err := parseStringMapField(editor, "options")
		if err != nil {
			return domain.Preview{}, err
		}
		local, err := boolField(editor, "local")
		if err != nil {
			return domain.Preview{}, err
		}
		mounts, err := c.api.Sys().ListMountsWithContext(ctx)
		if err != nil {
			return domain.Preview{}, err
		}
		if _, exists := mounts[mount]; exists {
			return domain.Preview{}, fmt.Errorf("%w: secrets engine mount already exists", errValidation)
		}
		input := &api.MountInput{Type: engineType, Description: fieldValue(editor, "description"), Options: options, Local: local}
		if err := c.api.Sys().MountWithContext(ctx, strings.TrimSuffix(mount, "/"), input); err != nil {
			return domain.Preview{}, err
		}
		verified, err := c.api.Sys().GetMountWithContext(ctx, strings.TrimSuffix(mount, "/"))
		if err != nil {
			return domain.Preview{}, err
		}
		if verified.Type != engineType || verified.Local != local || !equalStringMaps(verified.Options, options) {
			return domain.Preview{}, errors.New("enable verification failed for secrets engine")
		}
		return editorResult("Secrets engine enabled", "Path", mount, "Type", engineType), nil
	case "disable":
		mount := editor.Context["mount"]
		current, err := c.api.Sys().GetMountWithContext(ctx, strings.TrimSuffix(mount, "/"))
		if err != nil {
			return domain.Preview{}, err
		}
		if fingerprint(mountSnapshot(current)) != editor.Fingerprint {
			return domain.Preview{}, errConflict
		}
		if err := c.api.Sys().UnmountWithContext(ctx, strings.TrimSuffix(mount, "/")); err != nil {
			return domain.Preview{}, err
		}
		mounts, err := c.api.Sys().ListMountsWithContext(ctx)
		if err != nil {
			return domain.Preview{}, err
		}
		if _, exists := mounts[mount]; exists {
			return domain.Preview{}, errors.New("disable verification failed for secrets engine")
		}
		return editorResult("Secrets engine disabled", "Path", mount), nil
	default:
		return domain.Preview{}, errors.New("unknown engine action")
	}
}

func mountSnapshot(mount *api.MountOutput) map[string]interface{} {
	return map[string]interface{}{"type": mount.Type, "description": mount.Description, "options": mount.Options, "local": mount.Local, "config": mount.Config}
}

func equalStringMaps(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func sshRoleRef(item domain.Item) (string, string, error) {
	parts := strings.SplitN(item.ID, "\x00", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("invalid SSH role reference")
	}
	return parts[0], parts[1], nil
}

func (c *Client) loadSSHAction(ctx context.Context, item domain.Item, actionID string) (domain.Editor, error) {
	mount, role, refErr := sshRoleRef(item)
	switch actionID {
	case "edit":
		if refErr != nil {
			return domain.Editor{}, refErr
		}
		secret, err := c.api.Logical().ReadWithContext(ctx, mount+"roles/"+url.PathEscape(role))
		if err != nil {
			return domain.Editor{}, err
		}
		data := secretData(secret)
		editor := newActionEditor(domain.SectionSSH, item, actionID, "Edit SSH role "+role, []domain.EditField{
			actionField("config", "Role JSON", pretty(data), domain.FieldMultiline, "Complete SSH role configuration as a JSON object.", `{"key_type":"ca","allow_user_certificates":true,"allowed_users":"*"}`),
		})
		editor.Context["mount"] = mount
		editor.Context["role"] = role
		editor.Fingerprint = fingerprint(data)
		editor.SubmitLabel = "Update"
		editor.SuccessMessage = "SSH role updated and verified"
		editor.Tips = []string{"Fields differ between CA, OTP, and dynamic-key roles; keep type-specific fields together."}
		return editor, nil
	case "create":
		if refErr != nil {
			mount = strings.TrimSuffix(item.Subtitle, "/") + "/"
		}
		editor := newActionEditor(domain.SectionSSH, item, actionID, "Create an SSH role", []domain.EditField{
			actionField("mount", "SSH mount", strings.TrimSuffix(mount, "/"), domain.FieldText, "Existing SSH secrets engine mount.", "ssh"),
			actionField("name", "Role name", "", domain.FieldText, "New role name.", "developer"),
			actionField("config", "Role JSON", "{}", domain.FieldMultiline, "SSH role configuration as a JSON object.", `{"key_type":"ca","allow_user_certificates":true,"allowed_users":"*"}`),
		})
		editor.SubmitLabel = "Create"
		editor.RefreshList = true
		editor.SuccessMessage = "SSH role created and verified"
		return editor, nil
	case "delete":
		if refErr != nil {
			return domain.Editor{}, refErr
		}
		secret, err := c.api.Logical().ReadWithContext(ctx, mount+"roles/"+url.PathEscape(role))
		if err != nil {
			return domain.Editor{}, err
		}
		editor := confirmationEditor(domain.SectionSSH, item, actionID, "Delete SSH role "+role, "Delete", "New SSH credentials can no longer be issued through this role.")
		editor.Context["mount"] = mount
		editor.Context["role"] = role
		editor.Fingerprint = fingerprint(secretData(secret))
		editor.SuccessMessage = "SSH role deleted and verified"
		return editor, nil
	default:
		return domain.Editor{}, errors.New("unknown SSH action")
	}
}

func (c *Client) saveSSHAction(ctx context.Context, editor domain.Editor) (domain.Preview, error) {
	switch editor.ActionID {
	case "edit":
		path := editor.Context["mount"] + "roles/" + url.PathEscape(editor.Context["role"])
		current, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if current == nil || fingerprint(secretData(current)) != editor.Fingerprint {
			return domain.Preview{}, errConflict
		}
		payload, err := parseJSONObjectField(editor, "config")
		if err != nil {
			return domain.Preview{}, err
		}
		if _, err := c.api.Logical().WriteWithContext(ctx, path, payload); err != nil {
			return domain.Preview{}, err
		}
		verified, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if verified == nil || !payloadMatches(secretData(verified), payload) {
			return domain.Preview{}, errors.New("update verification failed for SSH role")
		}
		return c.previewSSHRole(ctx, editor.Item)
	case "create":
		mount, err := normalizeMountPath(fieldValue(editor, "mount"))
		if err != nil {
			return domain.Preview{}, err
		}
		role, err := requiredField(editor, "name")
		if err != nil {
			return domain.Preview{}, err
		}
		mounted, err := c.api.Sys().GetMountWithContext(ctx, strings.TrimSuffix(mount, "/"))
		if err != nil {
			return domain.Preview{}, err
		}
		if mounted.Type != "ssh" {
			return domain.Preview{}, fmt.Errorf("%w: selected mount is not an SSH secrets engine", errValidation)
		}
		payload, err := parseJSONObjectField(editor, "config")
		if err != nil {
			return domain.Preview{}, err
		}
		path := mount + "roles/" + url.PathEscape(role)
		existing, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if existing != nil {
			return domain.Preview{}, fmt.Errorf("%w: SSH role already exists", errValidation)
		}
		if _, err := c.api.Logical().WriteWithContext(ctx, path, payload); err != nil {
			return domain.Preview{}, err
		}
		verified, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if verified == nil || !payloadMatches(secretData(verified), payload) {
			return domain.Preview{}, errors.New("create verification failed for SSH role")
		}
		return c.previewSSHRole(ctx, domain.Item{ID: mount + "\x00" + role, Name: role, Subtitle: strings.TrimSuffix(mount, "/")})
	case "delete":
		path := editor.Context["mount"] + "roles/" + url.PathEscape(editor.Context["role"])
		current, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if current == nil || fingerprint(secretData(current)) != editor.Fingerprint {
			return domain.Preview{}, errConflict
		}
		if _, err := c.api.Logical().DeleteWithContext(ctx, path); err != nil {
			return domain.Preview{}, err
		}
		remaining, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if remaining != nil {
			return domain.Preview{}, errors.New("delete verification failed for SSH role")
		}
		return editorResult("SSH role deleted", "Role", editor.Context["role"]), nil
	default:
		return domain.Preview{}, errors.New("unknown SSH action")
	}
}

func payloadMatches(actual, expected map[string]interface{}) bool {
	for key, value := range expected {
		if fingerprint(actual[key]) != fingerprint(value) {
			return false
		}
	}
	return true
}

func (c *Client) loadAuthAction(ctx context.Context, item domain.Item, actionID string) (domain.Editor, error) {
	switch actionID {
	case "tune":
		path := "sys/auth/" + strings.Trim(item.ID, "/") + "/tune"
		secret, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Editor{}, err
		}
		data := secretData(secret)
		editor := newActionEditor(domain.SectionAuthMethods, item, actionID, "Tune auth method "+item.Name, []domain.EditField{
			actionField("description", "Description", stringValue(data["description"]), domain.FieldText, "Human-readable purpose of the auth mount."),
			actionField("default_ttl", "Default TTL", numberString(data["default_lease_ttl"]), domain.FieldText, "Default token lifetime in seconds or as a duration string.", "3600", "1h"),
			actionField("max_ttl", "Max TTL", numberString(data["max_lease_ttl"]), domain.FieldText, "Maximum token lifetime in seconds or as a duration string.", "86400", "24h"),
			actionField("visibility", "Listing visibility", stringValue(data["listing_visibility"]), domain.FieldText, "Set to unauth to show this mount before login, or leave empty.", "unauth"),
			actionField("token_type", "Token type", stringValue(data["token_type"]), domain.FieldText, "Default token type for this auth mount.", "default-service", "default-batch"),
		})
		editor.Context["mount"] = item.ID
		editor.Fingerprint = fingerprint(data)
		editor.SubmitLabel = "Tune"
		editor.SuccessMessage = "Auth method tuned and verified"
		return editor, nil
	case "enable":
		editor := newActionEditor(domain.SectionAuthMethods, item, actionID, "Enable an auth method", []domain.EditField{
			actionField("path", "Mount path", "", domain.FieldText, "Unique auth mount path.", "userpass"),
			actionField("type", "Auth type", "userpass", domain.FieldText, "Vault auth method type or plugin name.", "userpass", "ldap", "oidc"),
			actionField("description", "Description", "", domain.FieldText, "Human-readable purpose of the auth mount."),
			actionField("options", "Options", "{}", domain.FieldText, "JSON object with string values.", "{}"),
			actionField("local", "Local", "false", domain.FieldBoolean, "Local auth mounts are not replicated across clusters."),
		})
		editor.SubmitLabel = "Enable"
		editor.RefreshList = true
		editor.SuccessMessage = "Auth method enabled and verified"
		return editor, nil
	case "disable":
		mount, err := c.api.Sys().GetAuthWithContext(ctx, strings.Trim(item.ID, "/"))
		if err != nil {
			return domain.Editor{}, err
		}
		editor := confirmationEditor(domain.SectionAuthMethods, item, actionID, "Disable auth method "+item.Name, "Disable", "Disabling an auth method prevents new logins and removes its configuration.")
		editor.Context["mount"] = item.ID
		editor.Fingerprint = fingerprint(authSnapshot(mount))
		editor.SuccessMessage = "Auth method disabled and verified"
		return editor, nil
	default:
		return domain.Editor{}, errors.New("unknown auth action")
	}
}

func (c *Client) saveAuthAction(ctx context.Context, editor domain.Editor) (domain.Preview, error) {
	switch editor.ActionID {
	case "tune":
		path := "sys/auth/" + strings.Trim(editor.Context["mount"], "/") + "/tune"
		current, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if current == nil || fingerprint(secretData(current)) != editor.Fingerprint {
			return domain.Preview{}, errConflict
		}
		payload := map[string]interface{}{
			"description":        fieldValue(editor, "description"),
			"default_lease_ttl":  fieldValue(editor, "default_ttl"),
			"max_lease_ttl":      fieldValue(editor, "max_ttl"),
			"listing_visibility": fieldValue(editor, "visibility"),
			"token_type":         fieldValue(editor, "token_type"),
		}
		if _, err := c.api.Logical().WriteWithContext(ctx, path, payload); err != nil {
			return domain.Preview{}, err
		}
		verified, err := c.api.Logical().ReadWithContext(ctx, path)
		if err != nil {
			return domain.Preview{}, err
		}
		if verified == nil || !tuneValuesMatch(secretData(verified), editor) {
			return domain.Preview{}, errors.New("tune verification failed for auth method")
		}
		return c.previewAuthMethod(ctx, editor.Item)
	case "enable":
		mount, err := normalizeMountPath(fieldValue(editor, "path"))
		if err != nil {
			return domain.Preview{}, err
		}
		authType, err := requiredField(editor, "type")
		if err != nil {
			return domain.Preview{}, err
		}
		options, err := parseStringMapField(editor, "options")
		if err != nil {
			return domain.Preview{}, err
		}
		local, err := boolField(editor, "local")
		if err != nil {
			return domain.Preview{}, err
		}
		mounts, err := c.api.Sys().ListAuthWithContext(ctx)
		if err != nil {
			return domain.Preview{}, err
		}
		if _, exists := mounts[mount]; exists {
			return domain.Preview{}, fmt.Errorf("%w: auth mount already exists", errValidation)
		}
		input := &api.EnableAuthOptions{Type: authType, Description: fieldValue(editor, "description"), Options: options, Local: local}
		if err := c.api.Sys().EnableAuthWithOptionsWithContext(ctx, strings.TrimSuffix(mount, "/"), input); err != nil {
			return domain.Preview{}, err
		}
		verified, err := c.api.Sys().GetAuthWithContext(ctx, strings.TrimSuffix(mount, "/"))
		if err != nil {
			return domain.Preview{}, err
		}
		if verified.Type != authType || verified.Local != local || !equalStringMaps(verified.Options, options) {
			return domain.Preview{}, errors.New("enable verification failed for auth method")
		}
		return editorResult("Auth method enabled", "Path", mount, "Type", authType), nil
	case "disable":
		mount := editor.Context["mount"]
		current, err := c.api.Sys().GetAuthWithContext(ctx, strings.Trim(mount, "/"))
		if err != nil {
			return domain.Preview{}, err
		}
		if fingerprint(authSnapshot(current)) != editor.Fingerprint {
			return domain.Preview{}, errConflict
		}
		if err := c.api.Sys().DisableAuthWithContext(ctx, strings.Trim(mount, "/")); err != nil {
			return domain.Preview{}, err
		}
		mounts, err := c.api.Sys().ListAuthWithContext(ctx)
		if err != nil {
			return domain.Preview{}, err
		}
		if _, exists := mounts[mount]; exists {
			return domain.Preview{}, errors.New("disable verification failed for auth method")
		}
		return editorResult("Auth method disabled", "Path", mount), nil
	default:
		return domain.Preview{}, errors.New("unknown auth action")
	}
}

func authSnapshot(mount *api.AuthMount) map[string]interface{} {
	return map[string]interface{}{"type": mount.Type, "description": mount.Description, "options": mount.Options, "local": mount.Local, "config": mount.Config}
}

func tuneValuesMatch(data map[string]interface{}, editor domain.Editor) bool {
	return stringValue(data["description"]) == fieldValue(editor, "description") &&
		numberString(data["default_lease_ttl"]) == fieldValue(editor, "default_ttl") &&
		numberString(data["max_lease_ttl"]) == fieldValue(editor, "max_ttl") &&
		stringValue(data["listing_visibility"]) == fieldValue(editor, "visibility") &&
		stringValue(data["token_type"]) == fieldValue(editor, "token_type")
}

func (c *Client) loadAuditAction(ctx context.Context, item domain.Item, actionID string) (domain.Editor, error) {
	switch actionID {
	case "enable":
		editor := newActionEditor(domain.SectionAudit, item, actionID, "Enable an audit device", []domain.EditField{
			actionField("path", "Device path", "", domain.FieldText, "Unique audit device path.", "file"),
			actionField("type", "Device type", "file", domain.FieldText, "Vault audit device type.", "file", "socket", "syslog"),
			actionField("description", "Description", "", domain.FieldText, "Human-readable purpose of the audit device."),
			actionField("options", "Options", `{"file_path":"/var/log/vault_audit.log"}`, domain.FieldText, "JSON object with string values; required keys depend on the device type.", `{"file_path":"/var/log/vault_audit.log"}`),
			actionField("local", "Local", "false", domain.FieldBoolean, "Local audit devices are configured only on the current cluster."),
		})
		editor.SubmitLabel = "Enable"
		editor.RefreshList = true
		editor.SuccessMessage = "Audit device enabled and verified"
		editor.Tips = []string{"The file path is on the Vault server, not on the machine running vault-manager."}
		return editor, nil
	case "disable":
		devices, err := c.api.Sys().ListAuditWithContext(ctx)
		if err != nil {
			return domain.Editor{}, err
		}
		device := devices[item.ID]
		if device == nil {
			return domain.Editor{}, errConflict
		}
		editor := confirmationEditor(domain.SectionAudit, item, actionID, "Disable audit device "+item.Name, "Disable", "Vault cannot write new events to this device after it is disabled.")
		editor.Context["path"] = item.ID
		editor.Fingerprint = fingerprint(auditSnapshot(device))
		editor.SuccessMessage = "Audit device disabled and verified"
		return editor, nil
	default:
		return domain.Editor{}, errors.New("unknown audit action")
	}
}

func (c *Client) saveAuditAction(ctx context.Context, editor domain.Editor) (domain.Preview, error) {
	switch editor.ActionID {
	case "enable":
		path, err := normalizeMountPath(fieldValue(editor, "path"))
		if err != nil {
			return domain.Preview{}, err
		}
		deviceType, err := requiredField(editor, "type")
		if err != nil {
			return domain.Preview{}, err
		}
		options, err := parseStringMapField(editor, "options")
		if err != nil {
			return domain.Preview{}, err
		}
		local, err := boolField(editor, "local")
		if err != nil {
			return domain.Preview{}, err
		}
		devices, err := c.api.Sys().ListAuditWithContext(ctx)
		if err != nil {
			return domain.Preview{}, err
		}
		if _, exists := devices[path]; exists {
			return domain.Preview{}, fmt.Errorf("%w: audit device already exists", errValidation)
		}
		input := &api.EnableAuditOptions{Type: deviceType, Description: fieldValue(editor, "description"), Options: options, Local: local}
		if err := c.api.Sys().EnableAuditWithOptionsWithContext(ctx, strings.TrimSuffix(path, "/"), input); err != nil {
			return domain.Preview{}, err
		}
		devices, err = c.api.Sys().ListAuditWithContext(ctx)
		if err != nil {
			return domain.Preview{}, err
		}
		verified := devices[path]
		if verified == nil || verified.Type != deviceType || verified.Local != local || !equalStringMaps(verified.Options, options) {
			return domain.Preview{}, errors.New("enable verification failed for audit device")
		}
		return editorResult("Audit device enabled", "Path", path, "Type", deviceType), nil
	case "disable":
		path := editor.Context["path"]
		devices, err := c.api.Sys().ListAuditWithContext(ctx)
		if err != nil {
			return domain.Preview{}, err
		}
		current := devices[path]
		if current == nil || fingerprint(auditSnapshot(current)) != editor.Fingerprint {
			return domain.Preview{}, errConflict
		}
		if err := c.api.Sys().DisableAuditWithContext(ctx, strings.Trim(path, "/")); err != nil {
			return domain.Preview{}, err
		}
		devices, err = c.api.Sys().ListAuditWithContext(ctx)
		if err != nil {
			return domain.Preview{}, err
		}
		if _, exists := devices[path]; exists {
			return domain.Preview{}, errors.New("disable verification failed for audit device")
		}
		return editorResult("Audit device disabled", "Path", path), nil
	default:
		return domain.Preview{}, errors.New("unknown audit action")
	}
}

func auditSnapshot(device *api.Audit) map[string]interface{} {
	return map[string]interface{}{"type": device.Type, "description": device.Description, "options": device.Options, "local": device.Local}
}

func (c *Client) loadTokenAction(ctx context.Context, item domain.Item, actionID string) (domain.Editor, error) {
	if actionID != "revoke" {
		return domain.Editor{}, errors.New("unknown token action")
	}
	secret, err := c.api.Logical().WriteWithContext(ctx, "auth/token/lookup-accessor", map[string]interface{}{"accessor": item.ID})
	if err != nil {
		return domain.Editor{}, err
	}
	if secret == nil {
		return domain.Editor{}, errConflict
	}
	editor := confirmationEditor(domain.SectionTokens, item, actionID, "Revoke token "+item.Name, "Revoke", "Revoking by accessor also revokes child tokens and dynamic secrets leased by this token tree.")
	editor.Context["accessor"] = item.ID
	editor.SuccessMessage = "Token revoked and verified"
	return editor, nil
}

func (c *Client) saveTokenAction(ctx context.Context, editor domain.Editor) (domain.Preview, error) {
	if editor.ActionID != "revoke" {
		return domain.Preview{}, errors.New("unknown token action")
	}
	accessor := editor.Context["accessor"]
	if accessor == "" {
		return domain.Preview{}, errors.New("token accessor context is invalid; reload")
	}
	if _, err := c.api.Logical().WriteWithContext(ctx, "auth/token/revoke-accessor", map[string]interface{}{"accessor": accessor}); err != nil {
		return domain.Preview{}, err
	}
	listed, err := c.api.Logical().ListWithContext(ctx, "auth/token/accessors")
	if err != nil {
		return domain.Preview{}, err
	}
	for _, remaining := range keys(listed) {
		if remaining == accessor {
			return domain.Preview{}, errors.New("revoke verification failed for token")
		}
	}
	return editorResult("Token revoked", "Accessor", accessor), nil
}
