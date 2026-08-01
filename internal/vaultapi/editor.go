package vaultapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"vault-manager/internal/domain"
	"vault-manager/internal/validation"
)

func (c *Client) LoadEditor(ctx context.Context, section domain.Section, item domain.Item, actionID string) (domain.Editor, error) {
	var (
		editor domain.Editor
		err    error
	)
	switch {
	case section == domain.SectionGroups && actionID == "edit":
		editor, err = c.loadGroupEditor(ctx, item)
	case section == domain.SectionUsers && actionID == "edit":
		editor, err = c.loadEntityEditor(ctx, item)
	case section == domain.SectionUsers && actionID == "password":
		editor, err = c.loadPasswordEditor(ctx, item)
	default:
		editor, err = c.loadActionEditor(ctx, section, item, actionID)
	}
	if err != nil {
		if errors.Is(err, errConflict) || errors.Is(err, errValidation) {
			return domain.Editor{}, err
		}
		return domain.Editor{}, publicError("open editor", err)
	}
	return editor, nil
}

func (c *Client) SaveEditor(ctx context.Context, editor domain.Editor) (domain.Preview, error) {
	var (
		preview domain.Preview
		err     error
	)
	switch {
	case editor.Section == domain.SectionGroups && editor.ActionID == "edit":
		preview, err = c.saveGroupEditor(ctx, editor)
	case editor.Section == domain.SectionUsers && editor.ActionID == "edit":
		preview, err = c.saveEntityEditor(ctx, editor)
	case editor.Section == domain.SectionUsers && editor.ActionID == "password":
		preview, err = c.savePasswordEditor(ctx, editor)
	default:
		preview, err = c.saveActionEditor(ctx, editor)
	}
	if err != nil {
		var partial *domain.PartialSaveError
		if errors.As(err, &partial) {
			return preview, err
		}
		if errors.Is(err, errConflict) || errors.Is(err, errValidation) {
			return domain.Preview{}, err
		}
		return domain.Preview{}, publicError("save editor", err)
	}
	preview.Actions = actionsFor(editor.Section)
	return preview, nil
}

var (
	errConflict   = errors.New("resource changed in Vault; reload before saving")
	errValidation = errors.New("editor validation failed")
)

func (c *Client) loadGroupEditor(ctx context.Context, item domain.Item) (domain.Editor, error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, "identity/group/id/"+url.PathEscape(item.ID))
	if err != nil {
		return domain.Editor{}, err
	}
	data := secretData(secret)
	policyItems, err := c.listPolicies(ctx)
	if err != nil {
		return domain.Editor{}, err
	}
	_, entityInfo, err := c.listIdentity(ctx, "identity/entity/id")
	if err != nil {
		return domain.Editor{}, err
	}
	_, groupInfo, err := c.listIdentity(ctx, "identity/group/id")
	if err != nil {
		return domain.Editor{}, err
	}
	c.mu.Lock()
	cacheNames(c.entityNames, entityInfo)
	cacheNames(c.groupNames, groupInfo)
	c.mu.Unlock()
	metadata, err := compactJSON(data["metadata"])
	if err != nil {
		return domain.Editor{}, err
	}
	fields := []domain.EditField{
		listEditField("policies", "Policies", joinCSV(stringSlice(data["policies"])), "Comma-separated existing policy names", itemOptionNames(policyItems)),
		listEditField("members", "Members", c.entityNamesForCSV(stringSlice(data["member_entity_ids"])), "Comma-separated entity names", identityOptionNames(entityInfo, "", false)),
		listEditField("nested_groups", "Nested groups", c.groupNamesForCSV(stringSlice(data["member_group_ids"])), "Comma-separated internal group names", identityOptionNames(groupInfo, item.ID, true)),
		formattedEditField("metadata", "Metadata", metadata, domain.FieldText, domain.FormatJSONObject, "JSON object"),
	}
	return domain.Editor{
		Section:     domain.SectionGroups,
		Item:        item,
		ActionID:    "edit",
		Title:       "Edit group " + item.Name,
		Fingerprint: fingerprint(groupSnapshot(data)),
		Fields:      fields,
		Context:     map[string]string{"group_id": item.ID},
	}, nil
}

func (c *Client) saveGroupEditor(ctx context.Context, editor domain.Editor) (domain.Preview, error) {
	groupID := editor.Context["group_id"]
	current, err := c.api.Logical().ReadWithContext(ctx, "identity/group/id/"+url.PathEscape(groupID))
	if err != nil {
		return domain.Preview{}, err
	}
	data := secretData(current)
	if fingerprint(groupSnapshot(data)) != editor.Fingerprint {
		return domain.Preview{}, errConflict
	}

	policies, err := c.validatePolicies(ctx, parseCSV(fieldValue(editor, "policies")))
	if err != nil {
		return domain.Preview{}, err
	}
	memberIDs, err := c.resolveEntityNames(ctx, parseCSV(fieldValue(editor, "members")))
	if err != nil {
		return domain.Preview{}, err
	}
	memberGroupIDs, err := c.resolveGroupNames(ctx, parseCSV(fieldValue(editor, "nested_groups")), groupID)
	if err != nil {
		return domain.Preview{}, err
	}
	metadata, err := parseMetadata(fieldValue(editor, "metadata"))
	if err != nil {
		return domain.Preview{}, err
	}

	payload := map[string]interface{}{
		"name":              stringValue(data["name"]),
		"type":              stringValue(data["type"]),
		"policies":          policies,
		"member_entity_ids": memberIDs,
		"member_group_ids":  memberGroupIDs,
		"metadata":          metadata,
	}
	if _, err := c.api.Logical().WriteWithContext(ctx, "identity/group/id/"+url.PathEscape(groupID), payload); err != nil {
		return domain.Preview{}, err
	}
	verified, err := c.api.Logical().ReadWithContext(ctx, "identity/group/id/"+url.PathEscape(groupID))
	if err != nil {
		return domain.Preview{}, err
	}
	if fingerprint(groupEditableSnapshot(secretData(verified))) != fingerprint(groupEditableSnapshot(payload)) {
		return domain.Preview{}, errors.New("save verification failed for group")
	}
	return c.previewGroup(ctx, editor.Item)
}

func (c *Client) loadEntityEditor(ctx context.Context, item domain.Item) (domain.Editor, error) {
	ref, err := c.userReference(item.ID)
	if err != nil {
		return domain.Editor{}, err
	}
	if ref.EntityID == "" {
		return domain.Editor{}, errors.New("auth-only user has no identity entity to edit")
	}
	secret, err := c.api.Logical().ReadWithContext(ctx, "identity/entity/id/"+url.PathEscape(ref.EntityID))
	if err != nil {
		return domain.Editor{}, err
	}
	data := secretData(secret)

	policyItems, err := c.listPolicies(ctx)
	if err != nil {
		return domain.Editor{}, err
	}
	_, groupInfo, err := c.listIdentity(ctx, "identity/group/id")
	if err != nil {
		return domain.Editor{}, err
	}
	groupFingerprints := make(map[string]string, len(groupInfo))
	c.mu.Lock()
	cacheNames(c.groupNames, groupInfo)
	c.mu.Unlock()
	for groupID := range groupInfo {
		group, readErr := c.api.Logical().ReadWithContext(ctx, "identity/group/id/"+url.PathEscape(groupID))
		if readErr != nil {
			return domain.Editor{}, readErr
		}
		groupFingerprints[groupID] = fingerprint(groupSnapshot(secretData(group)))
	}

	encodedGroups, err := json.Marshal(sortedStrings(stringSlice(data["direct_group_ids"])))
	if err != nil {
		return domain.Editor{}, err
	}
	encodedFingerprints, err := json.Marshal(groupFingerprints)
	if err != nil {
		return domain.Editor{}, err
	}

	warnings := []string{
		"Group membership updates multiple Vault groups sequentially and is not atomic.",
	}
	if c.membershipNotice != "" {
		warnings = append(warnings, c.membershipNotice)
	}
	fields := []domain.EditField{
		listEditField("policies", "Direct policies", joinCSV(stringSlice(data["policies"])), "Comma-separated existing policy names", itemOptionNames(policyItems)),
		editField("disabled", "Disabled", strconv.FormatBool(boolValue(data["disabled"])), domain.FieldBoolean, "Disables all tokens associated with this entity"),
		listEditField("groups", "Groups", c.groupNamesForCSV(stringSlice(data["direct_group_ids"])), "Comma-separated internal group names; save is non-atomic", identityOptionNames(groupInfo, "", true)),
	}
	return domain.Editor{
		Section:     domain.SectionUsers,
		Item:        item,
		ActionID:    "edit",
		Title:       "Edit access for " + item.Name,
		Fingerprint: fingerprint(entitySnapshot(data)),
		Fields:      fields,
		Context: map[string]string{
			"entity_id":          ref.EntityID,
			"original_group_ids": string(encodedGroups),
			"group_fingerprints": string(encodedFingerprints),
		},
		Warnings: warnings,
	}, nil
}

func (c *Client) saveEntityEditor(ctx context.Context, editor domain.Editor) (domain.Preview, error) {
	entityID := editor.Context["entity_id"]
	current, err := c.api.Logical().ReadWithContext(ctx, "identity/entity/id/"+url.PathEscape(entityID))
	if err != nil {
		return domain.Preview{}, err
	}
	data := secretData(current)
	if fingerprint(entitySnapshot(data)) != editor.Fingerprint {
		return domain.Preview{}, errConflict
	}

	policies, err := c.validatePolicies(ctx, parseCSV(fieldValue(editor, "policies")))
	if err != nil {
		return domain.Preview{}, err
	}
	disabled, err := strconv.ParseBool(fieldValue(editor, "disabled"))
	if err != nil {
		return domain.Preview{}, fmt.Errorf("%w: disabled must be true or false", errValidation)
	}
	desiredGroupIDs, err := c.resolveGroupNames(ctx, parseCSV(fieldValue(editor, "groups")), "")
	if err != nil {
		return domain.Preview{}, err
	}

	var originalGroupIDs []string
	if err := json.Unmarshal([]byte(editor.Context["original_group_ids"]), &originalGroupIDs); err != nil {
		return domain.Preview{}, errors.New("editor group context is invalid; reload")
	}
	var originalFingerprints map[string]string
	if err := json.Unmarshal([]byte(editor.Context["group_fingerprints"]), &originalFingerprints); err != nil {
		return domain.Preview{}, errors.New("editor group fingerprint context is invalid; reload")
	}

	affected := symmetricDifference(originalGroupIDs, desiredGroupIDs)
	updates := make([]membershipUpdate, 0, len(affected))
	for _, groupID := range affected {
		expectedFingerprint, ok := originalFingerprints[groupID]
		if !ok {
			return domain.Preview{}, errConflict
		}
		group, readErr := c.api.Logical().ReadWithContext(ctx, "identity/group/id/"+url.PathEscape(groupID))
		if readErr != nil {
			return domain.Preview{}, readErr
		}
		groupData := secretData(group)
		if fingerprint(groupSnapshot(groupData)) != expectedFingerprint {
			return domain.Preview{}, errConflict
		}
		if stringValue(groupData["type"]) != "internal" {
			return domain.Preview{}, fmt.Errorf("%w: membership of external group %q cannot be edited", errValidation, stringValue(groupData["name"]))
		}

		members := sortedStrings(stringSlice(groupData["member_entity_ids"]))
		desiredMember := containsString(desiredGroupIDs, groupID)
		currentMember := containsString(members, entityID)
		if desiredMember == currentMember {
			continue
		}
		operation := "remove " + editor.Item.Name + " from " + stringValue(groupData["name"])
		if desiredMember {
			members = append(members, entityID)
			members = sortedStrings(members)
			operation = "add " + editor.Item.Name + " to " + stringValue(groupData["name"])
		} else {
			members = removeString(members, entityID)
		}
		updates = append(updates, membershipUpdate{
			groupID:   groupID,
			groupName: stringValue(groupData["name"]),
			operation: operation,
			payload: map[string]interface{}{
				"name":              stringValue(groupData["name"]),
				"type":              stringValue(groupData["type"]),
				"policies":          sortedStrings(stringSlice(groupData["policies"])),
				"member_entity_ids": members,
				"member_group_ids":  sortedStrings(stringSlice(groupData["member_group_ids"])),
				"metadata":          interfaceMap(groupData["metadata"]),
			},
		})
	}
	sort.Slice(updates, func(i, j int) bool { return updates[i].groupName < updates[j].groupName })

	applied := make([]string, 0, len(updates)+1)
	for index, update := range updates {
		_, writeErr := c.api.Logical().WriteWithContext(ctx, "identity/group/id/"+url.PathEscape(update.groupID), update.payload)
		if writeErr != nil {
			return c.partialEntityResult(ctx, editor, applied, publicError("update group "+update.groupName, writeErr).Error(), updateOperations(updates[index+1:]))
		}
		verified, readErr := c.api.Logical().ReadWithContext(ctx, "identity/group/id/"+url.PathEscape(update.groupID))
		if readErr != nil {
			return c.partialEntityResult(ctx, editor, applied, "verification failed for group "+update.groupName, updateOperations(updates[index+1:]))
		}
		if fingerprint(groupEditableSnapshot(secretData(verified))) != fingerprint(groupEditableSnapshot(update.payload)) {
			return c.partialEntityResult(ctx, editor, applied, "verification mismatch for group "+update.groupName, updateOperations(updates[index+1:]))
		}
		applied = append(applied, update.operation)
	}

	entityPayload := map[string]interface{}{
		"name":     stringValue(data["name"]),
		"disabled": disabled,
		"policies": policies,
		"metadata": interfaceMap(data["metadata"]),
	}
	if fingerprint(entityEditableSnapshot(data)) != fingerprint(entityEditableSnapshot(entityPayload)) {
		if _, err := c.api.Logical().WriteWithContext(ctx, "identity/entity/id/"+url.PathEscape(entityID), entityPayload); err != nil {
			return c.partialEntityResult(ctx, editor, applied, publicError("update user "+editor.Item.Name, err).Error(), nil)
		}
		verified, err := c.api.Logical().ReadWithContext(ctx, "identity/entity/id/"+url.PathEscape(entityID))
		if err != nil || fingerprint(entityEditableSnapshot(secretData(verified))) != fingerprint(entityEditableSnapshot(entityPayload)) {
			return c.partialEntityResult(ctx, editor, applied, "verification failed for user "+editor.Item.Name, nil)
		}
		applied = append(applied, "update direct access for "+editor.Item.Name)
	}
	return c.previewUser(ctx, editor.Item)
}

func (c *Client) loadPasswordEditor(ctx context.Context, item domain.Item) (domain.Editor, error) {
	ref, err := c.userReference(item.ID)
	if err != nil {
		return domain.Editor{}, err
	}
	if ref.EntityID != "" {
		secret, readErr := c.api.Logical().ReadWithContext(ctx, "identity/entity/id/"+url.PathEscape(ref.EntityID))
		if readErr != nil {
			return domain.Editor{}, readErr
		}
		var found []userRef
		for _, alias := range mapSlice(secretData(secret)["aliases"]) {
			mount := c.authMount(stringValue(alias["mount_accessor"]))
			name := stringValue(alias["name"])
			if mount.Type == "userpass" && name != "" {
				found = append(found, userRef{EntityID: ref.EntityID, Mount: mount.APIPath, Username: name})
			}
		}
		if len(found) == 0 {
			return domain.Editor{}, errors.New("user has no userpass account")
		}
		if len(found) > 1 {
			return domain.Editor{}, errors.New("user has multiple userpass accounts; account selection is not available yet")
		}
		ref = found[0]
	}
	account, err := c.api.Logical().ReadWithContext(ctx, ref.Mount+"users/"+url.PathEscape(ref.Username))
	if err != nil {
		return domain.Editor{}, err
	}
	return domain.Editor{
		Section:     domain.SectionUsers,
		Item:        item,
		ActionID:    "password",
		Title:       "Change password for " + ref.Username,
		Fingerprint: fingerprint(userpassSnapshot(secretData(account))),
		Fields: []domain.EditField{
			editField("password", "New password", "", domain.FieldSecret, "Write-only; never logged or displayed"),
			editField("confirm", "Confirm password", "", domain.FieldSecret, "Must exactly match the new password"),
		},
		Context: map[string]string{"mount": ref.Mount, "username": ref.Username},
	}, nil
}

func (c *Client) savePasswordEditor(ctx context.Context, editor domain.Editor) (domain.Preview, error) {
	password := fieldValue(editor, "password")
	confirmation := fieldValue(editor, "confirm")
	if password == "" {
		return domain.Preview{}, fmt.Errorf("%w: password must not be empty", errValidation)
	}
	if password != confirmation {
		return domain.Preview{}, fmt.Errorf("%w: password confirmation does not match", errValidation)
	}
	mount := editor.Context["mount"]
	username := editor.Context["username"]
	account, err := c.api.Logical().ReadWithContext(ctx, mount+"users/"+url.PathEscape(username))
	if err != nil {
		return domain.Preview{}, err
	}
	if fingerprint(userpassSnapshot(secretData(account))) != editor.Fingerprint {
		return domain.Preview{}, errConflict
	}
	if _, err := c.api.Logical().WriteWithContext(ctx, mount+"users/"+url.PathEscape(username)+"/password", map[string]interface{}{"password": password}); err != nil {
		return domain.Preview{}, err
	}
	password = ""
	confirmation = ""
	return c.previewUser(ctx, editor.Item)
}

func (c *Client) userReference(itemID string) (userRef, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ref, ok := c.userRefs[itemID]
	if !ok {
		return userRef{}, errors.New("user selection is stale")
	}
	return ref, nil
}

func (c *Client) validatePolicies(ctx context.Context, requested []string) ([]string, error) {
	items, err := c.listPolicies(ctx)
	if err != nil {
		return nil, err
	}
	available := make(map[string]bool, len(items))
	for _, item := range items {
		available[item.Name] = true
	}
	for _, name := range requested {
		if !available[name] {
			return nil, fmt.Errorf("%w: unknown policy %q", errValidation, name)
		}
	}
	return requested, nil
}

func (c *Client) resolveEntityNames(ctx context.Context, names []string) ([]string, error) {
	_, info, err := c.listIdentity(ctx, "identity/entity/id")
	if err != nil {
		return nil, err
	}
	return resolveNames(names, info, "")
}

func (c *Client) resolveGroupNames(ctx context.Context, names []string, excludedID string) ([]string, error) {
	_, info, err := c.listIdentity(ctx, "identity/group/id")
	if err != nil {
		return nil, err
	}
	return resolveNames(names, info, excludedID)
}

func resolveNames(names []string, info map[string]map[string]interface{}, excludedID string) ([]string, error) {
	byName := make(map[string][]string)
	for id, details := range info {
		if id == excludedID {
			continue
		}
		name := stringValue(details["name"])
		if name != "" {
			byName[name] = append(byName[name], id)
		}
	}
	ids := make([]string, 0, len(names))
	for _, name := range names {
		matches := byName[name]
		switch len(matches) {
		case 0:
			return nil, fmt.Errorf("%w: unknown name %q", errValidation, name)
		case 1:
			ids = append(ids, matches[0])
		default:
			return nil, fmt.Errorf("%w: ambiguous name %q", errValidation, name)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func groupEditableSnapshot(data map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"name":              stringValue(data["name"]),
		"type":              stringValue(data["type"]),
		"policies":          sortedStrings(stringSlice(data["policies"])),
		"member_entity_ids": sortedStrings(stringSlice(data["member_entity_ids"])),
		"member_group_ids":  sortedStrings(stringSlice(data["member_group_ids"])),
		"metadata":          interfaceMap(data["metadata"]),
	}
}

func entityEditableSnapshot(data map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"name":     stringValue(data["name"]),
		"disabled": boolValue(data["disabled"]),
		"policies": sortedStrings(stringSlice(data["policies"])),
		"metadata": interfaceMap(data["metadata"]),
	}
}

func sortedStrings(values []string) []string {
	values = append([]string(nil), values...)
	sort.Strings(values)
	return values
}

func groupSnapshot(data map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":                data["id"],
		"name":              data["name"],
		"type":              data["type"],
		"policies":          data["policies"],
		"member_entity_ids": data["member_entity_ids"],
		"member_group_ids":  data["member_group_ids"],
		"metadata":          data["metadata"],
		"modify_index":      data["modify_index"],
		"last_update_time":  data["last_update_time"],
	}
}

func entitySnapshot(data map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"id":               data["id"],
		"name":             data["name"],
		"disabled":         data["disabled"],
		"policies":         data["policies"],
		"direct_group_ids": data["direct_group_ids"],
		"metadata":         data["metadata"],
		"modify_index":     data["modify_index"],
		"last_update_time": data["last_update_time"],
	}
}

func userpassSnapshot(data map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"token_policies":    data["token_policies"],
		"token_ttl":         data["token_ttl"],
		"token_max_ttl":     data["token_max_ttl"],
		"token_period":      data["token_period"],
		"token_type":        data["token_type"],
		"token_num_uses":    data["token_num_uses"],
		"token_bound_cidrs": data["token_bound_cidrs"],
	}
}

func fingerprint(value interface{}) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func editField(id, label, value string, kind domain.FieldKind, help string) domain.EditField {
	field := domain.EditField{ID: id, Label: label, Value: value, Original: value, Kind: kind, Help: help}
	switch id {
	case "policies":
		field.Examples = []string{"default, app-readonly"}
	case "members":
		field.Examples = []string{"alice, bob"}
	case "nested_groups":
		field.Examples = []string{"engineering, platform"}
	case "metadata":
		field.Examples = []string{`{"owner":"platform"}`}
	case "groups":
		field.Examples = []string{"engineering, on-call"}
	case "disabled":
		field.Examples = []string{"true", "false"}
	}
	return field
}

func fieldValue(editor domain.Editor, id string) string {
	for _, field := range editor.Fields {
		if field.ID == id {
			return field.Value
		}
	}
	return ""
}

func parseCSV(value string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		result = append(result, part)
	}
	sort.Strings(result)
	return result
}

func joinCSV(values []string) string {
	values = append([]string(nil), values...)
	sort.Strings(values)
	return strings.Join(values, ", ")
}

func (c *Client) entityNamesForCSV(ids []string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		name := c.entityNames[id]
		if name == "" {
			name = "unresolved:" + shortID(id)
		}
		names = append(names, name)
	}
	return joinCSV(names)
}

func (c *Client) groupNamesForCSV(ids []string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		name := c.groupNames[id]
		if name == "" {
			name = "unresolved:" + shortID(id)
		}
		names = append(names, name)
	}
	return joinCSV(names)
}

func compactJSON(value interface{}) (string, error) {
	if value == nil {
		return "{}", nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	if string(encoded) == "null" {
		return "{}", nil
	}
	return string(encoded), nil
}

func parseMetadata(value string) (map[string]interface{}, error) {
	if err := validation.Field(domain.FormatJSONObject, value); err != nil {
		return nil, fmt.Errorf("%w: metadata: %v", errValidation, err)
	}
	if strings.TrimSpace(value) == "" {
		return map[string]interface{}{}, nil
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(value), &metadata); err != nil {
		return nil, fmt.Errorf("%w: metadata must be a JSON object", errValidation)
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	return metadata, nil
}
