package vaultapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/vault/api"

	"vault-manager/internal/config"
	"vault-manager/internal/domain"
)

const (
	entityUserPrefix   = "entity:"
	userpassUserPrefix = "userpass:"
)

type Client struct {
	api *api.Client

	mu               sync.RWMutex
	entityNames      map[string]string
	groupNames       map[string]string
	authByAccessor   map[string]authMount
	userRefs         map[string]userRef
	membershipNotice string
}

type authMount struct {
	Path     string
	APIPath  string
	Type     string
	Accessor string
}

type userRef struct {
	EntityID string
	Mount    string
	Username string
}

func New(cfg config.Config) (*Client, error) {
	timeout, err := cfg.Timeout()
	if err != nil {
		return nil, err
	}
	apiConfig := api.DefaultConfig()
	apiConfig.Address = strings.TrimRight(cfg.Vault.Address, "/")
	apiConfig.Timeout = timeout
	client, err := api.NewClient(apiConfig)
	if err != nil {
		return nil, fmt.Errorf("create Vault client: %w", err)
	}
	client.SetToken(cfg.Vault.Token)
	return &Client{
		api:              client,
		entityNames:      make(map[string]string),
		groupNames:       make(map[string]string),
		authByAccessor:   make(map[string]authMount),
		userRefs:         make(map[string]userRef),
		membershipNotice: strings.TrimSpace(cfg.Deployment.MembershipNotice),
	}, nil
}

func (c *Client) List(ctx context.Context, section domain.Section) ([]domain.Item, error) {
	var (
		items []domain.Item
		err   error
	)
	switch section {
	case domain.SectionStatus:
		items = []domain.Item{
			{ID: "health", Name: "Health"},
			{ID: "leader", Name: "Leader"},
			{ID: "seal", Name: "Seal status"},
			{ID: "raft", Name: "Raft peers"},
		}
	case domain.SectionGroups:
		items, err = c.listGroups(ctx)
	case domain.SectionUsers:
		items, err = c.listUsers(ctx)
	case domain.SectionPolicies:
		items, err = c.listPolicies(ctx)
	case domain.SectionEngines:
		items, err = c.listMountMap(ctx, "sys/mounts")
	case domain.SectionSSH:
		items, err = c.listSSHRoles(ctx)
	case domain.SectionAuthMethods:
		items, err = c.listAuthMethods(ctx)
	case domain.SectionAudit:
		items, err = c.listMountMap(ctx, "sys/audit")
	case domain.SectionTokens:
		items, err = c.listTokenAccessors(ctx)
	default:
		return nil, fmt.Errorf("unsupported section %q", section)
	}
	if err != nil {
		return nil, publicError("list "+section.Label(), err)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items, nil
}

func (c *Client) Preview(ctx context.Context, section domain.Section, item domain.Item) (domain.Preview, error) {
	var (
		preview domain.Preview
		err     error
	)
	switch section {
	case domain.SectionStatus:
		preview, err = c.previewStatus(ctx, item)
	case domain.SectionGroups:
		preview, err = c.previewGroup(ctx, item)
	case domain.SectionUsers:
		preview, err = c.previewUser(ctx, item)
	case domain.SectionPolicies:
		preview, err = c.previewPolicy(ctx, item)
	case domain.SectionEngines:
		preview, err = c.previewEngine(ctx, item)
	case domain.SectionSSH:
		preview, err = c.previewSSHRole(ctx, item)
	case domain.SectionAuthMethods:
		preview, err = c.previewAuthMethod(ctx, item)
	case domain.SectionAudit:
		preview, err = c.previewAudit(ctx, item)
	case domain.SectionTokens:
		preview, err = c.previewToken(ctx, item)
	default:
		return domain.Preview{}, fmt.Errorf("unsupported section %q", section)
	}
	if err != nil {
		return domain.Preview{}, publicError("read "+section.Label(), err)
	}
	preview.Actions = actionsFor(section)
	return preview, nil
}

func (c *Client) listGroups(ctx context.Context) ([]domain.Item, error) {
	items, info, err := c.listIdentity(ctx, "identity/group/id")
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	for id, details := range info {
		name := stringValue(details["name"])
		if name != "" {
			c.groupNames[id] = name
		}
	}
	c.mu.Unlock()
	_, entityInfo, entityErr := c.listIdentity(ctx, "identity/entity/id")
	if entityErr == nil {
		c.mu.Lock()
		cacheNames(c.entityNames, entityInfo)
		c.mu.Unlock()
	}
	return items, nil
}

func (c *Client) listUsers(ctx context.Context) ([]domain.Item, error) {
	entities, info, err := c.listIdentity(ctx, "identity/entity/id")
	if err != nil {
		return nil, err
	}
	_, groupInfo, groupErr := c.listIdentity(ctx, "identity/group/id")
	if groupErr == nil {
		c.mu.Lock()
		cacheNames(c.groupNames, groupInfo)
		c.mu.Unlock()
	}
	authMounts, err := c.authMounts(ctx)
	if err != nil {
		return nil, err
	}

	linked := make(map[string]bool)
	refs := make(map[string]userRef)
	items := make([]domain.Item, 0, len(entities))
	for _, entity := range entities {
		id := entity.ID
		details := info[id]
		name := entity.Name
		if name == "" {
			name = "entity " + shortID(id)
		}
		subtitle := "entity"
		for _, alias := range mapSlice(details["aliases"]) {
			accessor := stringValue(alias["mount_accessor"])
			aliasName := stringValue(alias["name"])
			mount, ok := authMounts[accessor]
			if !ok || mount.Type != "userpass" || aliasName == "" {
				continue
			}
			key := accessor + "\x00" + aliasName
			linked[key] = true
			subtitle = "linked · " + strings.TrimSuffix(mount.Path, "/") + "/" + aliasName
		}
		itemID := entityUserPrefix + id
		refs[itemID] = userRef{EntityID: id}
		items = append(items, domain.Item{ID: itemID, Name: name, Subtitle: subtitle})
	}

	for _, mount := range authMounts {
		if mount.Type != "userpass" {
			continue
		}
		secret, listErr := c.api.Logical().ListWithContext(ctx, mount.APIPath+"users")
		if listErr != nil {
			return nil, listErr
		}
		for _, username := range keys(secret) {
			key := mount.Accessor + "\x00" + username
			if linked[key] {
				continue
			}
			itemID := userpassUserPrefix + url.PathEscape(mount.Path) + ":" + url.PathEscape(username)
			refs[itemID] = userRef{Mount: mount.APIPath, Username: username}
			items = append(items, domain.Item{
				ID:       itemID,
				Name:     username,
				Subtitle: "auth-only · " + strings.TrimSuffix(mount.Path, "/"),
			})
		}
	}

	c.mu.Lock()
	for id, details := range info {
		if name := stringValue(details["name"]); name != "" {
			c.entityNames[id] = name
		}
	}
	c.userRefs = refs
	c.mu.Unlock()
	return items, nil
}

func (c *Client) listIdentity(ctx context.Context, path string) ([]domain.Item, map[string]map[string]interface{}, error) {
	secret, err := c.api.Logical().ListWithContext(ctx, path)
	if err != nil {
		return nil, nil, err
	}
	if secret == nil {
		return []domain.Item{}, map[string]map[string]interface{}{}, nil
	}
	info := nestedMap(secret.Data["key_info"])
	items := make([]domain.Item, 0, len(keys(secret)))
	for _, id := range keys(secret) {
		name := stringValue(info[id]["name"])
		if name == "" {
			name = shortID(id)
		}
		items = append(items, domain.Item{ID: id, Name: name})
	}
	return items, info, nil
}

func (c *Client) listPolicies(ctx context.Context) ([]domain.Item, error) {
	secret, err := c.api.Logical().ListWithContext(ctx, "sys/policies/acl")
	if err != nil {
		return nil, err
	}
	result := make([]domain.Item, 0)
	for _, name := range keys(secret) {
		result = append(result, domain.Item{ID: name, Name: name})
	}
	return result, nil
}

func (c *Client) listMountMap(ctx context.Context, path string) ([]domain.Item, error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		return []domain.Item{}, nil
	}
	items := make([]domain.Item, 0, len(secret.Data))
	for mountPath, raw := range secret.Data {
		details := interfaceMap(raw)
		items = append(items, domain.Item{
			ID:       mountPath,
			Name:     mountPath,
			Subtitle: stringValue(details["type"]),
		})
	}
	return items, nil
}

func (c *Client) listSSHRoles(ctx context.Context) ([]domain.Item, error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, "sys/mounts")
	if err != nil {
		return nil, err
	}
	items := make([]domain.Item, 0)
	if secret == nil {
		return items, nil
	}
	for mountPath, raw := range secret.Data {
		if stringValue(interfaceMap(raw)["type"]) != "ssh" {
			continue
		}
		roles, listErr := c.api.Logical().ListWithContext(ctx, mountPath+"roles")
		if listErr != nil {
			return nil, listErr
		}
		for _, role := range keys(roles) {
			items = append(items, domain.Item{
				ID:       mountPath + "\x00" + role,
				Name:     role,
				Subtitle: strings.TrimSuffix(mountPath, "/"),
			})
		}
	}
	return items, nil
}

func (c *Client) listAuthMethods(ctx context.Context) ([]domain.Item, error) {
	mounts, err := c.authMounts(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.Item, 0, len(mounts))
	for _, mount := range mounts {
		items = append(items, domain.Item{ID: mount.Path, Name: mount.Path, Subtitle: mount.Type})
	}
	return items, nil
}

func (c *Client) listTokenAccessors(ctx context.Context) ([]domain.Item, error) {
	secret, err := c.api.Logical().ListWithContext(ctx, "auth/token/accessors")
	if err != nil {
		return nil, err
	}
	_, entityInfo, entityErr := c.listIdentity(ctx, "identity/entity/id")
	if entityErr == nil {
		c.mu.Lock()
		cacheNames(c.entityNames, entityInfo)
		c.mu.Unlock()
	}
	items := make([]domain.Item, 0)
	for _, accessor := range keys(secret) {
		lookup, lookupErr := c.api.Logical().WriteWithContext(ctx, "auth/token/lookup-accessor", map[string]interface{}{"accessor": accessor})
		if lookupErr != nil {
			return nil, lookupErr
		}
		name := "token"
		subtitle := shortID(accessor)
		if lookup != nil {
			if displayName := stringValue(lookup.Data["display_name"]); displayName != "" {
				name = displayName
			}
			if entityName := c.entityName(stringValue(lookup.Data["entity_id"])); entityName != "" {
				name = entityName + " · " + name
			}
		}
		items = append(items, domain.Item{ID: accessor, Name: name, Subtitle: subtitle})
	}
	return items, nil
}

func (c *Client) previewStatus(ctx context.Context, item domain.Item) (domain.Preview, error) {
	preview := domain.Preview{Title: item.Name}
	switch item.ID {
	case "health":
		health, err := c.api.Sys().HealthWithContext(ctx)
		if err != nil {
			return preview, err
		}
		preview.Rows = rows(
			"Initialized", boolString(health.Initialized),
			"Sealed", boolString(health.Sealed),
			"Standby", boolString(health.Standby),
			"Version", health.Version,
			"Cluster", health.ClusterName,
			"Cluster ID", health.ClusterID,
			"Enterprise", boolString(health.Enterprise),
			"Server time", time.Unix(health.ServerTimeUTC, 0).UTC().Format(time.RFC3339),
		)
	case "leader":
		leader, err := c.api.Sys().LeaderWithContext(ctx)
		if err != nil {
			return preview, err
		}
		preview.Rows = rows(
			"HA enabled", boolString(leader.HAEnabled),
			"This node active", boolString(leader.IsSelf),
			"Leader address", leader.LeaderAddress,
			"Cluster address", leader.LeaderClusterAddress,
			"Active since", timeString(leader.ActiveTime),
			"Raft committed", strconv.FormatUint(leader.RaftCommittedIndex, 10),
			"Raft applied", strconv.FormatUint(leader.RaftAppliedIndex, 10),
		)
	case "seal":
		seal, err := c.api.Sys().SealStatusWithContext(ctx)
		if err != nil {
			return preview, err
		}
		preview.Rows = rows(
			"Initialized", boolString(seal.Initialized),
			"Sealed", boolString(seal.Sealed),
			"Seal type", seal.Type,
			"Shares", strconv.Itoa(seal.N),
			"Threshold", strconv.Itoa(seal.T),
			"Storage", seal.StorageType,
			"Version", seal.Version,
		)
	case "raft":
		secret, err := c.api.Logical().ReadWithContext(ctx, "sys/storage/raft/configuration")
		if err != nil {
			return preview, err
		}
		preview.Rows = []domain.Row{{ID: "configuration", Label: "Configuration", Value: pretty(secretData(secret))}}
	default:
		return preview, errors.New("unknown status item")
	}
	return preview, nil
}

func (c *Client) previewGroup(ctx context.Context, item domain.Item) (domain.Preview, error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, "identity/group/id/"+url.PathEscape(item.ID))
	if err != nil {
		return domain.Preview{}, err
	}
	data := secretData(secret)
	return domain.Preview{
		Title: item.Name,
		Rows: rows(
			"Name", stringValue(data["name"]),
			"Type", stringValue(data["type"]),
			"Policies", joinStrings(stringSlice(data["policies"])),
			"Members", c.entityNamesFor(stringSlice(data["member_entity_ids"])),
			"Nested groups", c.groupNamesFor(stringSlice(data["member_group_ids"])),
			"Parent groups", c.groupNamesFor(stringSlice(data["parent_group_ids"])),
			"Metadata", pretty(data["metadata"]),
			"ID", stringValue(data["id"]),
			"Created", stringValue(data["creation_time"]),
			"Updated", stringValue(data["last_update_time"]),
			"Modify index", numberString(data["modify_index"]),
		),
	}, nil
}

func (c *Client) previewUser(ctx context.Context, item domain.Item) (domain.Preview, error) {
	c.mu.RLock()
	ref, ok := c.userRefs[item.ID]
	c.mu.RUnlock()
	if !ok {
		return domain.Preview{}, errors.New("user selection is stale")
	}
	if ref.EntityID == "" {
		return c.previewUserpass(ctx, item.Name, ref.Mount, ref.Username, "auth-only")
	}

	secret, err := c.api.Logical().ReadWithContext(ctx, "identity/entity/id/"+url.PathEscape(ref.EntityID))
	if err != nil {
		return domain.Preview{}, err
	}
	data := secretData(secret)
	aliases := mapSlice(data["aliases"])
	aliasNames := make([]string, 0, len(aliases))
	accountRows := make([]domain.Row, 0)
	for _, alias := range aliases {
		name := stringValue(alias["name"])
		accessor := stringValue(alias["mount_accessor"])
		mount := c.authMount(accessor)
		label := name
		if mount.Path != "" {
			label += " @ " + strings.TrimSuffix(mount.Path, "/")
		}
		aliasNames = append(aliasNames, label)
		if mount.Type == "userpass" && name != "" {
			account, readErr := c.api.Logical().ReadWithContext(ctx, mount.APIPath+"users/"+url.PathEscape(name))
			if readErr != nil {
				return domain.Preview{}, readErr
			}
			accountData := secretData(account)
			accountRows = append(accountRows,
				domain.Row{ID: "userpass-policies-" + name, Label: "Userpass policies", Value: joinStrings(stringSlice(accountData["token_policies"]))},
				domain.Row{ID: "userpass-ttl-" + name, Label: "Userpass TTL", Value: numberString(accountData["token_ttl"])},
			)
		}
	}
	preview := domain.Preview{
		Title: item.Name,
		Rows: rows(
			"Name", stringValue(data["name"]),
			"State", "linked",
			"Disabled", boolString(boolValue(data["disabled"])),
			"Direct policies", joinStrings(stringSlice(data["policies"])),
			"Direct groups", c.groupNamesFor(stringSlice(data["direct_group_ids"])),
			"Inherited groups", c.groupNamesFor(stringSlice(data["inherited_group_ids"])),
			"Aliases", joinStrings(aliasNames),
			"Metadata", pretty(data["metadata"]),
			"Entity ID", ref.EntityID,
			"Created", stringValue(data["creation_time"]),
			"Updated", stringValue(data["last_update_time"]),
		),
	}
	preview.Rows = append(preview.Rows, accountRows...)
	return preview, nil
}

func (c *Client) previewUserpass(ctx context.Context, title, mount, username, state string) (domain.Preview, error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, mount+"users/"+url.PathEscape(username))
	if err != nil {
		return domain.Preview{}, err
	}
	data := secretData(secret)
	return domain.Preview{
		Title: title,
		Rows: rows(
			"Username", username,
			"State", state,
			"Auth mount", strings.TrimPrefix(strings.TrimSuffix(mount, "/"), "auth/"),
			"Token policies", joinStrings(stringSlice(data["token_policies"])),
			"Token TTL", numberString(data["token_ttl"]),
			"Token max TTL", numberString(data["token_max_ttl"]),
			"Token type", stringValue(data["token_type"]),
		),
	}, nil
}

func (c *Client) previewPolicy(ctx context.Context, item domain.Item) (domain.Preview, error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, "sys/policies/acl/"+url.PathEscape(item.ID))
	if err != nil {
		return domain.Preview{}, err
	}
	data := secretData(secret)
	body := stringValue(data["policy"])
	if body == "" {
		body = stringValue(data["rules"])
	}
	return domain.Preview{Title: item.Name, Rows: []domain.Row{
		{ID: "name", Label: "Name", Value: item.Name},
		{ID: "body", Label: "Policy", Value: body},
	}}, nil
}

func (c *Client) previewEngine(ctx context.Context, item domain.Item) (domain.Preview, error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, "sys/mounts/"+strings.TrimSuffix(item.ID, "/")+"/tune")
	if err != nil {
		return domain.Preview{}, err
	}
	data := secretData(secret)
	return domain.Preview{Title: item.Name, Rows: rows(
		"Path", item.ID,
		"Type", item.Subtitle,
		"Description", stringValue(data["description"]),
		"Default TTL", numberString(data["default_lease_ttl"]),
		"Max TTL", numberString(data["max_lease_ttl"]),
		"Listing visibility", stringValue(data["listing_visibility"]),
		"Options", pretty(data["options"]),
	)}, nil
}

func (c *Client) previewSSHRole(ctx context.Context, item domain.Item) (domain.Preview, error) {
	parts := strings.SplitN(item.ID, "\x00", 2)
	if len(parts) != 2 {
		return domain.Preview{}, errors.New("invalid SSH role reference")
	}
	secret, err := c.api.Logical().ReadWithContext(ctx, parts[0]+"roles/"+url.PathEscape(parts[1]))
	if err != nil {
		return domain.Preview{}, err
	}
	return domain.Preview{Title: item.Name, Subtitle: item.Subtitle, Rows: mapRows(secretData(secret))}, nil
}

func (c *Client) previewAuthMethod(ctx context.Context, item domain.Item) (domain.Preview, error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, "sys/auth")
	if err != nil {
		return domain.Preview{}, err
	}
	data := interfaceMap(secretData(secret)[item.ID])
	return domain.Preview{Title: item.Name, Rows: rows(
		"Path", item.ID,
		"Type", stringValue(data["type"]),
		"Accessor", stringValue(data["accessor"]),
		"Description", stringValue(data["description"]),
		"Local", boolString(boolValue(data["local"])),
		"Seal wrap", boolString(boolValue(data["seal_wrap"])),
		"Options", pretty(data["options"]),
	)}, nil
}

func (c *Client) previewAudit(ctx context.Context, item domain.Item) (domain.Preview, error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, "sys/audit")
	if err != nil {
		return domain.Preview{}, err
	}
	data := interfaceMap(secretData(secret)[item.ID])
	return domain.Preview{Title: item.Name, Rows: rows(
		"Path", item.ID,
		"Type", stringValue(data["type"]),
		"Description", stringValue(data["description"]),
		"Local", boolString(boolValue(data["local"])),
		"Options", pretty(data["options"]),
	)}, nil
}

func (c *Client) previewToken(ctx context.Context, item domain.Item) (domain.Preview, error) {
	secret, err := c.api.Logical().WriteWithContext(ctx, "auth/token/lookup-accessor", map[string]interface{}{"accessor": item.ID})
	if err != nil {
		return domain.Preview{}, err
	}
	data := secretData(secret)
	entityID := stringValue(data["entity_id"])
	return domain.Preview{Title: item.Name, Rows: rows(
		"Display name", stringValue(data["display_name"]),
		"Entity", fallback(c.entityName(entityID), shortID(entityID)),
		"Policies", joinStrings(stringSlice(data["policies"])),
		"Path", stringValue(data["path"]),
		"TTL", numberString(data["ttl"]),
		"Expires", stringValue(data["expire_time"]),
		"Orphan", boolString(boolValue(data["orphan"])),
		"Uses remaining", numberString(data["num_uses"]),
		"Accessor", item.ID,
	)}, nil
}

func (c *Client) authMounts(ctx context.Context) (map[string]authMount, error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, "sys/auth")
	if err != nil {
		return nil, err
	}
	result := make(map[string]authMount)
	if secret == nil {
		return result, nil
	}
	for path, raw := range secret.Data {
		data := interfaceMap(raw)
		apiPath := path
		if !strings.HasPrefix(apiPath, "auth/") {
			apiPath = "auth/" + apiPath
		}
		mount := authMount{Path: path, APIPath: apiPath, Type: stringValue(data["type"]), Accessor: stringValue(data["accessor"])}
		result[mount.Accessor] = mount
	}
	c.mu.Lock()
	c.authByAccessor = result
	c.mu.Unlock()
	return result, nil
}

func (c *Client) authMount(accessor string) authMount {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.authByAccessor[accessor]
}

func (c *Client) entityName(id string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.entityNames[id]
}

func (c *Client) entityNamesFor(ids []string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, fallback(c.entityNames[id], shortID(id)))
	}
	return joinStrings(names)
}

func (c *Client) groupNamesFor(ids []string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, fallback(c.groupNames[id], shortID(id)))
	}
	return joinStrings(names)
}

func actionsFor(section domain.Section) []domain.Action {
	enabled := func(id, label string) domain.Action {
		return domain.Action{ID: id, Label: label, Enabled: true}
	}
	switch section {
	case domain.SectionStatus:
		return []domain.Action{enabled("seal", "Seal cluster"), enabled("unseal", "Unseal cluster"), enabled("restore", "Restore snapshot")}
	case domain.SectionGroups:
		return []domain.Action{enabled("edit", "Edit group"), enabled("create", "Create group"), enabled("delete", "Delete group")}
	case domain.SectionUsers:
		return []domain.Action{enabled("edit", "Edit access"), enabled("password", "Change password"), enabled("create", "Create user"), enabled("revoke", "Revoke access"), enabled("delete", "Delete entity")}
	case domain.SectionPolicies:
		return []domain.Action{enabled("edit", "Edit policy body"), enabled("create", "Create policy"), enabled("delete", "Delete policy")}
	case domain.SectionEngines:
		return []domain.Action{enabled("browse", "Browse values"), enabled("enable", "Enable engine"), enabled("disable", "Disable engine")}
	case domain.SectionSSH:
		return []domain.Action{enabled("edit", "Edit role"), enabled("create", "Create role"), enabled("delete", "Delete role")}
	case domain.SectionAuthMethods:
		return []domain.Action{enabled("tune", "Tune auth method"), enabled("enable", "Enable auth method"), enabled("disable", "Disable auth method")}
	case domain.SectionAudit:
		return []domain.Action{enabled("enable", "Enable audit device"), enabled("disable", "Disable audit device")}
	case domain.SectionTokens:
		return []domain.Action{enabled("revoke", "Revoke token")}
	default:
		return nil
	}
}

func publicError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: request timed out", operation)
	}
	var responseErr *api.ResponseError
	if errors.As(err, &responseErr) {
		return fmt.Errorf("%s: Vault returned HTTP %d", operation, responseErr.StatusCode)
	}
	return fmt.Errorf("%s: request failed", operation)
}

func keys(secret *api.Secret) []string {
	if secret == nil {
		return nil
	}
	return stringSlice(secret.Data["keys"])
}

func secretData(secret *api.Secret) map[string]interface{} {
	if secret == nil || secret.Data == nil {
		return map[string]interface{}{}
	}
	return secret.Data
}

func nestedMap(value interface{}) map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})
	for key, raw := range interfaceMap(value) {
		result[key] = interfaceMap(raw)
	}
	return result
}

func cacheNames(destination map[string]string, info map[string]map[string]interface{}) {
	for id, details := range info {
		name := stringValue(details["name"])
		if name != "" {
			destination[id] = name
		}
	}
}

func interfaceMap(value interface{}) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}
	if typed, ok := value.(map[string]interface{}); ok {
		return typed
	}
	return map[string]interface{}{}
}

func mapSlice(value interface{}) []map[string]interface{} {
	values, ok := value.([]interface{})
	if !ok {
		return nil
	}
	result := make([]map[string]interface{}, 0, len(values))
	for _, raw := range values {
		result = append(result, interfaceMap(raw))
	}
	return result
}

func stringSlice(value interface{}) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		result := make([]string, 0, len(typed))
		for _, raw := range typed {
			if text := stringValue(raw); text != "" {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func stringValue(value interface{}) string {
	if value == nil {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprint(value)
}

func boolValue(value interface{}) bool {
	typed, _ := value.(bool)
	return typed
}

func boolString(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func numberString(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func joinStrings(values []string) string {
	if len(values) == 0 {
		return "—"
	}
	sort.Strings(values)
	return strings.Join(values, ", ")
}

func shortID(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:8] + "…"
}

func fallback(value, fallbackValue string) string {
	if value != "" {
		return value
	}
	return fallbackValue
}

func pretty(value interface{}) string {
	if value == nil {
		return "—"
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "[unavailable]"
	}
	if string(encoded) == "{}" || string(encoded) == "null" {
		return "—"
	}
	return string(encoded)
}

func rows(values ...string) []domain.Row {
	result := make([]domain.Row, 0, len(values)/2)
	for index := 0; index+1 < len(values); index += 2 {
		value := values[index+1]
		if value == "" {
			value = "—"
		}
		result = append(result, domain.Row{ID: strings.ToLower(strings.ReplaceAll(values[index], " ", "-")), Label: values[index], Value: value})
	}
	return result
}

func mapRows(data map[string]interface{}) []domain.Row {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]domain.Row, 0, len(keys))
	for _, key := range keys {
		result = append(result, domain.Row{ID: key, Label: key, Value: pretty(data[key])})
	}
	return result
}

func timeString(value time.Time) string {
	if value.IsZero() {
		return "—"
	}
	return value.UTC().Format(time.RFC3339)
}
