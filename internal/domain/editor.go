package domain

import "context"

type FieldKind string

const (
	FieldText     FieldKind = "text"
	FieldBoolean  FieldKind = "boolean"
	FieldSecret   FieldKind = "secret"
	FieldReadOnly FieldKind = "read-only"
)

type EditField struct {
	ID       string
	Label    string
	Value    string
	Original string
	Kind     FieldKind
	Help     string
}

type Editor struct {
	Section     Section
	Item        Item
	ActionID    string
	Title       string
	Fingerprint string
	Fields      []EditField
	Context     map[string]string
	Warnings    []string
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
