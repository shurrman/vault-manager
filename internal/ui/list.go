package ui

import (
	"sort"
	"strings"
	"unicode"

	"vault-manager/internal/domain"
)

func (m *Model) listSuggestions() []string {
	if !m.editor.inputActive || len(m.editor.spec.Fields) == 0 {
		return nil
	}
	field := m.editor.spec.Fields[m.editor.fieldIndex]
	if field.Kind != domain.FieldList || len(field.Options) == 0 {
		return nil
	}
	value := []rune(m.editor.input.Value())
	position := clamp(m.editor.input.Position(), 0, len(value))
	start, _ := listSegmentBounds(value, position)
	prefix := strings.ToLower(strings.TrimSpace(string(value[start:position])))
	currentIndex := strings.Count(string(value[:position]), ",")
	selected := make(map[string]struct{})
	for index, part := range strings.Split(string(value), ",") {
		if index == currentIndex {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(part))
		if name != "" {
			selected[name] = struct{}{}
		}
	}
	candidates := make([]string, 0, len(field.Options))
	for _, option := range field.Options {
		name := strings.TrimSpace(option)
		lower := strings.ToLower(name)
		if name == "" || lower == prefix || !strings.HasPrefix(lower, prefix) {
			continue
		}
		if _, exists := selected[lower]; exists {
			continue
		}
		candidates = append(candidates, name)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return strings.ToLower(candidates[i]) < strings.ToLower(candidates[j])
	})
	return candidates
}

func (m *Model) acceptListSuggestion(candidate string) {
	value := []rune(m.editor.input.Value())
	position := clamp(m.editor.input.Position(), 0, len(value))
	start, end := listSegmentBounds(value, position)
	prefix := append([]rune(nil), value[:start]...)
	if start > 0 {
		prefix = []rune(strings.TrimRightFunc(string(prefix), unicode.IsSpace))
		prefix = append(prefix, ' ')
	}
	replacement := []rune(candidate)
	updated := append(prefix, replacement...)
	cursor := len(updated)
	updated = append(updated, value[end:]...)
	m.editor.input.SetValue(string(updated))
	m.editor.input.SetCursor(cursor)
	m.editor.suggestionIndex = 0
	m.editor.validationError = ""
	m.syncInput()
	m.status = "List value selected"
}

func listSegmentBounds(value []rune, position int) (int, int) {
	start := 0
	for index := position - 1; index >= 0; index-- {
		if value[index] == ',' {
			start = index + 1
			break
		}
	}
	for start < position && unicode.IsSpace(value[start]) {
		start++
	}
	end := len(value)
	for index := position; index < len(value); index++ {
		if value[index] == ',' {
			end = index
			break
		}
	}
	return start, end
}

func (m *Model) listSuggestionLines() []string {
	candidates := m.listSuggestions()
	if len(candidates) == 0 {
		return nil
	}
	m.editor.suggestionIndex = clamp(m.editor.suggestionIndex, 0, len(candidates)-1)
	limit := min(6, len(candidates))
	lines := []string{"    " + m.text.Text("Suggestions")}
	for index := 0; index < limit; index++ {
		lines = append(lines, m.renderSelection(candidates[index], index == m.editor.suggestionIndex, index == m.editor.suggestionIndex))
	}
	return lines
}
