package vaultapi

import (
	"sort"
	"strings"

	"vault-manager/internal/domain"
)

func listActionField(id, label, value, help string, options []string, examples ...string) domain.EditField {
	field := actionField(id, label, value, domain.FieldList, help, examples...)
	field.Options = sortedUnique(options)
	return field
}

func formattedActionField(id, label, value string, kind domain.FieldKind, format domain.FieldFormat, help string, examples ...string) domain.EditField {
	field := actionField(id, label, value, kind, help, examples...)
	field.Format = format
	return field
}

func listEditField(id, label, value, help string, options []string) domain.EditField {
	field := editField(id, label, value, domain.FieldList, help)
	field.Options = sortedUnique(options)
	return field
}

func formattedEditField(id, label, value string, kind domain.FieldKind, format domain.FieldFormat, help string) domain.EditField {
	field := editField(id, label, value, kind, help)
	field.Format = format
	return field
}

func itemOptionNames(items []domain.Item) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return sortedUnique(names)
}

func identityOptionNames(info map[string]map[string]interface{}, excludedID string, internalOnly bool) []string {
	names := make([]string, 0, len(info))
	for id, details := range info {
		if id == excludedID {
			continue
		}
		groupType := stringValue(details["type"])
		if internalOnly && groupType != "" && groupType != "internal" {
			continue
		}
		if name := stringValue(details["name"]); name != "" {
			names = append(names, name)
		}
	}
	return sortedUnique(names)
}

func sortedUnique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i]) < strings.ToLower(result[j])
	})
	return result
}
