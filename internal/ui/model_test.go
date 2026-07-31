package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"vault-manager/internal/domain"
)

type fakeBackend struct {
	items       []domain.Item
	preview     domain.Preview
	editor      domain.Editor
	loadEditors int
	saves       int
	err         error
}

func (f *fakeBackend) List(context.Context, domain.Section) ([]domain.Item, error) {
	return f.items, f.err
}

func (f *fakeBackend) Preview(context.Context, domain.Section, domain.Item) (domain.Preview, error) {
	return f.preview, f.err
}

func (f *fakeBackend) LoadEditor(context.Context, domain.Section, domain.Item, string) (domain.Editor, error) {
	f.loadEditors++
	return f.editor, f.err
}

func (f *fakeBackend) SaveEditor(context.Context, domain.Editor) (domain.Preview, error) {
	f.saves++
	return f.preview, f.err
}

func TestDisabledActionNeverProducesEffect(t *testing.T) {
	backend := &fakeBackend{}
	model := New(backend, "never")
	model.pane = PanePreview
	model.previewState = stateReady
	model.items = []domain.Item{{ID: "group-1", Name: "developers"}}
	model.preview = domain.Preview{Actions: []domain.Action{{
		ID: "delete", Label: "Delete group", Reason: "Not available in MVP",
	}}}

	_, cmd := model.activate()
	if cmd != nil {
		t.Fatal("disabled action returned a command")
	}
	if backend.loadEditors != 0 || backend.saves != 0 {
		t.Fatalf("disabled action touched backend: loads=%d saves=%d", backend.loadEditors, backend.saves)
	}
	if !strings.Contains(model.status, "Not available in MVP") {
		t.Fatalf("status does not explain disabled action: %q", model.status)
	}
}

func TestEnabledActionLoadsEditor(t *testing.T) {
	backend := &fakeBackend{editor: domain.Editor{Title: "Edit group"}}
	model := New(backend, "never")
	model.pane = PanePreview
	model.previewState = stateReady
	model.items = []domain.Item{{ID: "group-1", Name: "developers"}}
	model.preview = domain.Preview{Actions: []domain.Action{{ID: "edit", Label: "Edit group", Enabled: true}}}

	_, cmd := model.activate()
	if cmd == nil || model.mode != modeEditorLoading {
		t.Fatalf("editor was not scheduled: mode=%v cmd=%v", model.mode, cmd)
	}
	msg, ok := cmd().(editorLoadedMsg)
	if !ok {
		t.Fatalf("unexpected editor command message %T", cmd())
	}
	model.handleEditorLoaded(msg)
	if model.mode != modeEditing || backend.loadEditors != 1 {
		t.Fatalf("editor did not open: mode=%v loads=%d", model.mode, backend.loadEditors)
	}
}

func TestTabMovesRightAndTargetsFirstAction(t *testing.T) {
	model := New(&fakeBackend{}, "never")
	model.listState = stateReady
	model.previewState = stateReady
	model.items = []domain.Item{{ID: "group-1", Name: "developers"}}
	model.preview = domain.Preview{
		Rows: []domain.Row{
			{Label: "Name", Value: "developers"},
			{Label: "Type", Value: "internal"},
		},
		Actions: []domain.Action{
			{ID: "edit", Label: "Edit group", Enabled: true},
			{ID: "delete", Label: "Delete group"},
		},
	}

	model.handleKey("tab")
	if model.pane != PaneItems {
		t.Fatalf("first Tab pane = %v, want items", model.pane)
	}
	model.handleKey("tab")
	if model.pane != PanePreview {
		t.Fatalf("second Tab pane = %v, want preview", model.pane)
	}
	if model.detailIndex != len(model.preview.Rows) {
		t.Fatalf("detailIndex = %d, want first action at %d", model.detailIndex, len(model.preview.Rows))
	}
}

func TestTabFallsBackToFirstPropertyWithoutActions(t *testing.T) {
	model := New(&fakeBackend{}, "never")
	model.pane = PaneItems
	model.previewState = stateReady
	model.detailIndex = 3
	model.preview = domain.Preview{Rows: []domain.Row{{Label: "Name", Value: "developers"}}}

	model.handleKey("tab")
	if model.pane != PanePreview || model.detailIndex != 0 {
		t.Fatalf("Tab without actions = pane %v detail %d, want preview detail 0", model.pane, model.detailIndex)
	}
}

func TestShiftTabMovesLeftAndPreservesSelection(t *testing.T) {
	model := New(&fakeBackend{}, "never")
	model.pane = PanePreview
	model.sectionIndex = 2
	model.itemIndex = 4
	model.detailIndex = 7

	model.handleKey("shift+tab")
	if model.pane != PaneItems || model.sectionIndex != 2 || model.itemIndex != 4 || model.detailIndex != 7 {
		t.Fatalf("first Shift+Tab changed selection: pane=%v section=%d item=%d detail=%d",
			model.pane, model.sectionIndex, model.itemIndex, model.detailIndex)
	}
	model.handleKey("shift+tab")
	if model.pane != PaneSections || model.sectionIndex != 2 || model.itemIndex != 4 {
		t.Fatalf("second Shift+Tab changed selection: pane=%v section=%d item=%d",
			model.pane, model.sectionIndex, model.itemIndex)
	}
	model.handleKey("shift+tab")
	if model.pane != PaneSections {
		t.Fatalf("Shift+Tab moved left of sections: pane=%v", model.pane)
	}
}

func TestRightArrowKeepsCurrentPreviewSelection(t *testing.T) {
	model := New(&fakeBackend{}, "never")
	model.pane = PaneItems
	model.previewState = stateReady
	model.detailIndex = 1
	model.preview = domain.Preview{
		Rows:    []domain.Row{{Label: "Name"}, {Label: "Type"}},
		Actions: []domain.Action{{Label: "Edit group"}},
	}

	model.handleKey("right")
	if model.pane != PanePreview || model.detailIndex != 1 {
		t.Fatalf("right arrow = pane %v detail %d, want preview detail 1", model.pane, model.detailIndex)
	}
}

func TestDirtyEditorDefaultsToCancel(t *testing.T) {
	model := New(&fakeBackend{}, "never")
	model.mode = modeEditing
	model.editor.spec = domain.Editor{Fields: []domain.EditField{{
		ID: "policies", Value: "new", Original: "old", Kind: domain.FieldText,
	}}}

	model.leaveEditor(false)
	if model.mode != modeConfirm {
		t.Fatalf("mode=%v, want confirm", model.mode)
	}
	if model.editor.confirmIndex != 2 {
		t.Fatalf("confirmIndex=%d, want Cancel", model.editor.confirmIndex)
	}
}

func TestSaveProducesExactlyOneBackendEffect(t *testing.T) {
	backend := &fakeBackend{preview: domain.Preview{Title: "saved"}}
	model := New(backend, "never")
	model.mode = modeConfirm
	model.items = []domain.Item{{ID: "group-1", Name: "developers"}}
	model.editor.spec = domain.Editor{
		Section: domain.SectionGroups,
		Item:    model.items[0],
		Fields: []domain.EditField{{
			ID: "policies", Value: "new", Original: "old", Kind: domain.FieldText,
		}},
	}
	model.editor.confirmIndex = 0

	_, cmd := model.confirmChoice()
	if cmd == nil || model.mode != modeSaving {
		t.Fatalf("save was not scheduled: mode=%v cmd=%v", model.mode, cmd)
	}
	msg := cmd().(editorSavedMsg)
	model.handleEditorSaved(msg)
	if backend.saves != 1 {
		t.Fatalf("backend saves=%d, want 1", backend.saves)
	}
	if model.mode != modeBrowse || model.preview.Title != "saved" {
		t.Fatalf("successful save did not return to preview: mode=%v preview=%q", model.mode, model.preview.Title)
	}
}

func TestSaveErrorKeepsDraft(t *testing.T) {
	backend := &fakeBackend{err: errors.New("conflict")}
	model := New(backend, "never")
	model.mode = modeConfirm
	model.items = []domain.Item{{ID: "group-1", Name: "developers"}}
	model.editor.spec = domain.Editor{
		Section: domain.SectionGroups,
		Item:    model.items[0],
		Fields: []domain.EditField{{
			ID: "policies", Value: "new", Original: "old", Kind: domain.FieldText,
		}},
	}
	model.editor.confirmIndex = 0

	_, cmd := model.confirmChoice()
	msg := cmd().(editorSavedMsg)
	model.handleEditorSaved(msg)
	if model.mode != modeEditing {
		t.Fatalf("mode=%v, want editing", model.mode)
	}
	if got := model.editor.spec.Fields[0].Value; got != "new" {
		t.Fatalf("draft lost after save error: %q", got)
	}
}

func TestPartialSaveReturnsToFreshPreview(t *testing.T) {
	partial := &domain.PartialSaveError{
		Applied: []string{"remove alice from alpha"},
		Failed:  "add alice to beta",
	}
	backend := &fakeBackend{
		preview: domain.Preview{Title: "alice"},
		err:     partial,
	}
	model := New(backend, "never")
	model.mode = modeConfirm
	model.items = []domain.Item{{ID: "entity:entity-1", Name: "alice"}}
	model.editor.spec = domain.Editor{
		Section: domain.SectionUsers,
		Item:    model.items[0],
		Fields: []domain.EditField{{
			ID: "groups", Value: "beta", Original: "alpha", Kind: domain.FieldText,
		}},
	}
	model.editor.confirmIndex = 0

	_, cmd := model.confirmChoice()
	msg := cmd().(editorSavedMsg)
	model.handleEditorSaved(msg)
	if model.mode != modeBrowse || model.preview.Title != "alice" {
		t.Fatalf("partial save did not return to fresh preview: mode=%v preview=%q", model.mode, model.preview.Title)
	}
	if !strings.Contains(model.status, "reload before continuing") {
		t.Fatalf("partial save status is not actionable: %q", model.status)
	}
}

func TestStalePreviewResponseIsIgnored(t *testing.T) {
	model := New(&fakeBackend{}, "never")
	model.items = []domain.Item{{ID: "new", Name: "new"}}
	model.previewRequestID = 2
	model.preview = domain.Preview{Title: "current"}

	model.handlePreviewLoaded(previewLoadedMsg{
		requestID: 1,
		section:   domain.SectionStatus,
		itemID:    "old",
		preview:   domain.Preview{Title: "stale"},
	})
	if model.preview.Title != "current" {
		t.Fatalf("stale response replaced preview: %q", model.preview.Title)
	}
}

func TestNoColorUsesTextMarkersAndShowsDisabledControls(t *testing.T) {
	model := New(&fakeBackend{}, "never")
	model.height = 24
	model.pane = PaneSections
	lines := model.sectionLines()
	if !strings.HasPrefix(lines[0], "> ") {
		t.Fatalf("active no-color marker missing: %q", lines[0])
	}

	model.pane = PanePreview
	model.previewState = stateReady
	model.preview = domain.Preview{
		Rows:    []domain.Row{{Label: "Name", Value: "developers"}},
		Actions: []domain.Action{{Label: "Delete group", Reason: "Not available in MVP"}},
	}
	rendered := strings.Join(model.previewLines(), "\n")
	if !strings.Contains(rendered, "[disabled] Delete group") {
		t.Fatalf("disabled control not rendered: %q", rendered)
	}
}

func TestFooterMentionsTabNavigation(t *testing.T) {
	model := New(&fakeBackend{}, "never")
	footer := model.footerText()
	if !strings.Contains(footer, "Tab/Shift+Tab") {
		t.Fatalf("footer does not mention Tab navigation: %q", footer)
	}
}

func TestEditorDebugRedactsSecrets(t *testing.T) {
	value := editorDebugValue(domain.Editor{Fields: []domain.EditField{
		{ID: "password", Value: "do-not-leak", Kind: domain.FieldSecret},
		{ID: "policies", Value: "read-only", Kind: domain.FieldText},
	}})
	if strings.Contains(value, "do-not-leak") || !strings.Contains(value, "[redacted]") {
		t.Fatalf("secret was not redacted: %q", value)
	}
}
