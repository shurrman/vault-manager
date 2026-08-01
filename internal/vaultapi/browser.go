package vaultapi

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"vault-manager/internal/domain"
)

func (c *Client) Browse(ctx context.Context, section domain.Section, item domain.Item, relative string, read bool) (domain.BrowserPage, error) {
	if section != domain.SectionEngines {
		return domain.BrowserPage{}, errors.New("hierarchical browsing is not available for this section")
	}
	mount, err := normalizeMountPath(item.ID)
	if err != nil {
		return domain.BrowserPage{}, err
	}
	relative, err = normalizeBrowserPath(relative)
	if err != nil {
		return domain.BrowserPage{}, err
	}
	mounted, err := c.api.Sys().GetMountWithContext(ctx, strings.TrimSuffix(mount, "/"))
	if err != nil {
		return domain.BrowserPage{}, publicError("inspect secrets engine", err)
	}
	kvV2 := mounted.Type == "kv" && mounted.Options["version"] == "2"
	displayPath := mount + relative
	if displayPath == "" {
		displayPath = "/"
	}
	page := domain.BrowserPage{
		Title:       "Browse values",
		Path:        relative,
		DisplayPath: displayPath,
		Leaf:        read,
	}
	if read {
		apiPath := mount + relative
		if kvV2 {
			apiPath = mount + "data/" + relative
		}
		secret, readErr := c.api.Logical().ReadWithContext(ctx, apiPath)
		if readErr != nil {
			return domain.BrowserPage{}, publicError("read "+displayPath, readErr)
		}
		data := secretData(secret)
		if kvV2 {
			data = interfaceMap(data["data"])
		}
		if len(data) == 0 {
			page.Rows = rows("Result", "No data")
		} else {
			page.Rows = mapRows(data)
		}
		return page, nil
	}

	apiPath := mount + relative
	if kvV2 {
		apiPath = mount + "metadata/" + relative
	}
	secret, listErr := c.api.Logical().ListWithContext(ctx, apiPath)
	if listErr != nil {
		return domain.BrowserPage{}, publicError("list "+displayPath, listErr)
	}
	for _, key := range keys(secret) {
		directory := strings.HasSuffix(key, "/")
		name := strings.TrimSuffix(key, "/")
		if name == "" || strings.Contains(name, "/") {
			continue
		}
		path := name
		if relative != "" {
			path = relative + "/" + name
		}
		page.Entries = append(page.Entries, domain.BrowserEntry{
			Name:      name,
			Path:      path,
			Directory: directory,
		})
	}
	sort.Slice(page.Entries, func(i, j int) bool {
		if page.Entries[i].Directory != page.Entries[j].Directory {
			return page.Entries[i].Directory
		}
		return strings.ToLower(page.Entries[i].Name) < strings.ToLower(page.Entries[j].Name)
	})
	return page, nil
}

func normalizeBrowserPath(value string) (string, error) {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" {
		return "", nil
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("%w: secret path contains an invalid segment", errValidation)
		}
	}
	return value, nil
}

func parentBrowserPath(value string) string {
	value = strings.Trim(value, "/")
	if index := strings.LastIndex(value, "/"); index >= 0 {
		return value[:index]
	}
	return ""
}
