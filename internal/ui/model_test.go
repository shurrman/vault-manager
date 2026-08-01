package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vault-manager/internal/domain"
)

type fakeBackend struct {
	items        []domain.Item
	preview      domain.Preview
	editor       domain.Editor
	loadEditors  int
	saves        int
	err          error
	browserPages map[string]domain.BrowserPage
	browsePaths  []string
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

func (f *fakeBackend) Browse(_ context.Context, _ domain.Section, _ domain.Item, path string, read bool) (domain.BrowserPage, error) {
	f.browsePaths = append(f.browsePaths, path)
	key := path
	if read {
		key += "|read"
	}
	return f.browserPages[key], f.err
}

func TestDisabledActionNeverProducesEffect(t *testing.T) {
	backend := &fakeBackend{}
	model := New(backend, "never")
	model.pane = PanePreview
	model.previewState = stateReady
	model.items = []domain.Item{{ID: "group-1", Name: "developers"}}
	model.preview = domain.Preview{Actions: []domain.Action{{
		ID: "delete", Label: "Delete group", Reason: "Unavailable in this context",
	}}}

	_, cmd := model.activate()
	if cmd != nil {
		t.Fatal("disabled action returned a command")
	}
	if backend.loadEditors != 0 || backend.saves != 0 {
		t.Fatalf("disabled action touched backend: loads=%d saves=%d", backend.loadEditors, backend.saves)
	}
	if !strings.Contains(model.status, "Unavailable in this context") {
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
		Actions: []domain.Action{{Label: "Delete group", Reason: "Unavailable in this context"}},
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

func TestPreviewPreservesMultilinePolicyBody(t *testing.T) {
	model := New(&fakeBackend{}, "never")
	model.height = 24
	model.pane = PanePreview
	model.previewState = stateReady
	model.preview = domain.Preview{Rows: []domain.Row{{
		Label: "Policy",
		Value: "# readable comment\npath \"secret/data/app/*\" {\n  capabilities = [\"read\"]\n}",
	}}}

	rendered := strings.Join(model.previewLines(), "\n")
	if !strings.Contains(rendered, "Policy:\n    # readable comment\n    path") {
		t.Fatalf("multiline policy formatting was lost: %q", rendered)
	}
	if strings.Contains(rendered, " · ") {
		t.Fatalf("multiline policy was flattened: %q", rendered)
	}
}

func TestEditorShowsPersistentHelpZone(t *testing.T) {
	model := New(&fakeBackend{}, "never")
	model.height = 28
	model.mode = modeEditing
	model.editor.spec = domain.Editor{
		Title: "Create policy",
		Fields: []domain.EditField{{
			ID: "body", Label: "Policy", Kind: domain.FieldMultiline,
			Help:     "Vault ACL policy in HCL or JSON.",
			Examples: []string{"path \"secret/data/app/*\" { ... }"},
		}},
		Tips: []string{"Use the minimum capabilities required."},
	}

	rendered := strings.Join(model.editorLines(), "\n")
	for _, wanted := range []string{"Help", "Vault ACL policy", "Example:", "Tip:"} {
		if !strings.Contains(rendered, wanted) {
			t.Fatalf("help zone does not contain %q: %q", wanted, rendered)
		}
	}
}

func TestConfirmationOnlyActionDefaultsToCancel(t *testing.T) {
	backend := &fakeBackend{editor: domain.Editor{
		Section: domain.SectionTokens, ActionID: "revoke", Title: "Revoke token",
		SubmitLabel: "Revoke", SubmitAlways: true,
	}}
	model := New(backend, "never")
	model.items = []domain.Item{{ID: "accessor-1", Name: "token"}}
	model.editorRequestID = 1

	model.handleEditorLoaded(editorLoadedMsg{
		requestID: 1, section: domain.SectionStatus, itemID: "accessor-1", editor: backend.editor,
	})
	if model.mode != modeConfirm || model.editor.confirmIndex != 1 {
		t.Fatalf("confirmation-only action mode=%v index=%d, want confirm/cancel", model.mode, model.editor.confirmIndex)
	}
	model.confirmChoice()
	if model.mode != modeBrowse || backend.saves != 0 {
		t.Fatalf("default cancel changed backend: mode=%v saves=%d", model.mode, backend.saves)
	}
}

func TestDangerRowUsesWholeRowMarkerWithoutColor(t *testing.T) {
	model := New(&fakeBackend{}, "never")
	model.height = 24
	model.pane = PanePreview
	model.previewState = stateReady
	model.preview = domain.Preview{Rows: []domain.Row{{Label: "Sealed", Value: "yes", Severity: domain.SeverityDanger}}}

	rendered := strings.Join(model.previewLines(), "\n")
	if !strings.Contains(rendered, "! Sealed: yes") {
		t.Fatalf("danger row is not visibly marked: %q", rendered)
	}
}

func TestListSuggestionsFilterExcludeAndAcceptWithKeyboard(t *testing.T) {
	model := New(&fakeBackend{}, "never")
	model.width = 120
	model.mode = modeEditing
	model.editor.spec = domain.Editor{Fields: []domain.EditField{{
		ID: "members", Label: "Members", Kind: domain.FieldList,
		Value: "alpha", Original: "alpha", Options: []string{"alpha", "albert", "amber", "beta"},
	}}}
	model.startFieldEdit()
	model.editor.input.SetValue("alpha, a")
	model.editor.input.CursorEnd()
	model.syncInput()

	if got := model.listSuggestions(); len(got) != 2 || got[0] != "albert" || got[1] != "amber" {
		t.Fatalf("filtered suggestions = %#v", got)
	}
	model.handleEditingKey(tea.KeyPressMsg{Code: tea.KeyDown})
	model.handleEditingKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := model.editor.spec.Fields[0].Value; got != "alpha, amber" {
		t.Fatalf("accepted list value = %q", got)
	}
}

func TestInvalidStructuredFieldCannotReachConfirmation(t *testing.T) {
	model := New(&fakeBackend{}, "never")
	model.mode = modeEditing
	model.editor.spec = domain.Editor{Fields: []domain.EditField{{
		ID: "body", Label: "Policy", Kind: domain.FieldMultiline, Format: domain.FormatPolicy,
		Original: "path \"secret/*\" {}", Value: "path \"secret/*\" {\n capabilities = [\"read\"\n}",
	}}}

	model.reviewEditor()
	if model.mode != modeEditing || !strings.Contains(model.editor.validationError, "line") {
		t.Fatalf("invalid format was not blocked with a location: mode=%v error=%q", model.mode, model.editor.validationError)
	}
}

func TestEngineBrowserNavigatesAndEscapesToParent(t *testing.T) {
	backend := &fakeBackend{browserPages: map[string]domain.BrowserPage{
		"":                {Path: "", DisplayPath: "secret/", Entries: []domain.BrowserEntry{{Name: "app", Path: "app", Directory: true}}},
		"app":             {Path: "app", DisplayPath: "secret/app", Entries: []domain.BrowserEntry{{Name: "config", Path: "app/config"}}},
		"app/config|read": {Path: "app/config", DisplayPath: "secret/app/config", Leaf: true, Rows: []domain.Row{{Label: "name", Value: "demo"}}},
	}}
	model := New(backend, "never")
	model.sectionIndex = 4
	model.pane = PanePreview
	model.previewState = stateReady
	model.items = []domain.Item{{ID: "secret/", Name: "secret/"}}
	model.preview = domain.Preview{Actions: []domain.Action{{ID: "browse", Label: "Browse values", Enabled: true}}}

	_, cmd := model.activate()
	model.handleBrowserLoaded(cmd().(browserLoadedMsg))
	_, cmd = model.handleBrowserKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	model.handleBrowserLoaded(cmd().(browserLoadedMsg))
	_, cmd = model.handleBrowserKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	model.handleBrowserLoaded(cmd().(browserLoadedMsg))
	if model.browserTitle() != "secret/app/config" || !model.browser.page.Leaf {
		t.Fatalf("leaf browser state = %#v", model.browser)
	}
	_, cmd = model.handleBrowserKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	model.handleBrowserLoaded(cmd().(browserLoadedMsg))
	if model.browser.page.Path != "app" {
		t.Fatalf("Esc parent path = %q", model.browser.page.Path)
	}
}

func TestRussianChromeUsesConfiguredLanguage(t *testing.T) {
	model := New(&fakeBackend{}, "never", "ru")
	if got := model.sectionLines()[0]; !strings.Contains(got, "Статус") {
		t.Fatalf("Russian section label = %q", got)
	}
	if got := model.editorTitle(); got != "Просмотр" {
		t.Fatalf("Russian preview title = %q", got)
	}
}

func TestEnginePathRowOpensBrowserAsDefaultAction(t *testing.T) {
	backend := &fakeBackend{browserPages: map[string]domain.BrowserPage{
		"": {Path: "", DisplayPath: "legacy/", Entries: []domain.BrowserEntry{{Name: "app", Path: "app", Directory: true}}},
	}}
	model := New(backend, "never")
	model.sectionIndex = 4
	model.pane = PanePreview
	model.previewState = stateReady
	model.items = []domain.Item{{ID: "legacy/", Name: "legacy/"}}
	model.preview = domain.Preview{
		Rows:    []domain.Row{{ID: "path", Label: "Path", Value: "legacy/"}},
		Actions: []domain.Action{{ID: "browse", Label: "Browse values", Enabled: true}},
	}

	_, cmd := model.activate()
	if cmd == nil || model.mode != modeBrowserLoading || backend.loadEditors != 0 {
		t.Fatalf("Path activation did not open browser directly: mode=%v cmd=%v editors=%d", model.mode, cmd, backend.loadEditors)
	}
	model.handleBrowserLoaded(cmd().(browserLoadedMsg))
	if model.mode != modeBrowser || model.browserTitle() != "legacy/" {
		t.Fatalf("browser root = mode %v title %q", model.mode, model.browserTitle())
	}
	model.handleBrowserKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if model.mode != modeBrowse || model.pane != PanePreview || model.detailIndex != 0 {
		t.Fatalf("Esc from root did not return to Path preview: mode=%v pane=%v detail=%d", model.mode, model.pane, model.detailIndex)
	}
}
