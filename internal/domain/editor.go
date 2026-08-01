package domain

import "context"

type FieldKind string

const (
	FieldText      FieldKind = "text"
	FieldMultiline FieldKind = "multiline"
	FieldList      FieldKind = "list"
	FieldBoolean   FieldKind = "boolean"
	FieldSecret    FieldKind = "secret"
	FieldReadOnly  FieldKind = "read-only"
)

type FieldFormat string

const (
	FormatNone       FieldFormat = ""
	FormatJSONObject FieldFormat = "json-object"
	FormatStringMap  FieldFormat = "string-map"
	FormatPolicy     FieldFormat = "policy"
)

type EditField struct {
	ID       string
	Label    string
	Value    string
	Original string
	Kind     FieldKind
	Format   FieldFormat
	Options  []string
	Help     string
	Examples []string
}

type Editor struct {
	Section        Section
	Item           Item
	ActionID       string
	Title          string
	Fingerprint    string
	Fields         []EditField
	Context        map[string]string
	Warnings       []string
	Tips           []string
	SubmitLabel    string
	SubmitAlways   bool
	RefreshList    bool
	SuccessMessage string
}

func (e Editor) Dirty() bool {
	for _, field := range e.Fields {
		if field.Value != field.Original {
			return true
		}
	}
	return false
}

type EditorBackend interface {
	Backend
	LoadEditor(context.Context, Section, Item, string) (Editor, error)
	SaveEditor(context.Context, Editor) (Preview, error)
}
