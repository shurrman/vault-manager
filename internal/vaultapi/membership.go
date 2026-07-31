package vaultapi

import (
	"context"
	"sort"

	"vault-manager/internal/domain"
)

type membershipUpdate struct {
	groupID   string
	groupName string
	operation string
	payload   map[string]interface{}
}

func symmetricDifference(left, right []string) []string {
	leftSet := make(map[string]bool, len(left))
	rightSet := make(map[string]bool, len(right))
	for _, value := range left {
		leftSet[value] = true
	}
	for _, value := range right {
		rightSet[value] = true
	}
	result := make([]string, 0)
	for value := range leftSet {
		if !rightSet[value] {
			result = append(result, value)
		}
	}
	for value := range rightSet {
		if !leftSet[value] {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func removeString(values []string, removed string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != removed {
			result = append(result, value)
		}
	}
	return result
}

func updateOperations(updates []membershipUpdate) []string {
	result := make([]string, 0, len(updates))
	for _, update := range updates {
		result = append(result, update.operation)
	}
	return result
}

func (c *Client) partialEntityResult(
	ctx context.Context,
	editor domain.Editor,
	applied []string,
	failed string,
	pending []string,
) (domain.Preview, error) {
	preview, _ := c.previewUser(ctx, editor.Item)
	preview.Actions = actionsFor(domain.SectionUsers)
	return preview, &domain.PartialSaveError{
		Applied: append([]string(nil), applied...),
		Failed:  failed,
		Pending: append([]string(nil), pending...),
	}
}
