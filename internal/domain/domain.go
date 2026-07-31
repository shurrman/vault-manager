package domain

import "context"

type Section string

const (
	SectionStatus      Section = "status"
	SectionGroups      Section = "groups"
	SectionUsers       Section = "users"
	SectionPolicies    Section = "policies"
	SectionEngines     Section = "engines"
	SectionSSH         Section = "ssh"
	SectionAuthMethods Section = "auth-methods"
	SectionAudit       Section = "audit"
	SectionTokens      Section = "tokens"
)

var Sections = []Section{
	SectionStatus,
	SectionGroups,
	SectionUsers,
	SectionPolicies,
	SectionEngines,
	SectionSSH,
	SectionAuthMethods,
	SectionAudit,
	SectionTokens,
}

func (s Section) Label() string {
	switch s {
	case SectionStatus:
		return "Status"
	case SectionGroups:
		return "Groups"
	case SectionUsers:
		return "Users"
	case SectionPolicies:
		return "Policies"
	case SectionEngines:
		return "Engines"
	case SectionSSH:
		return "SSH"
	case SectionAuthMethods:
		return "Auth Methods"
	case SectionAudit:
		return "Audit"
	case SectionTokens:
		return "Tokens"
	default:
		return string(s)
	}
}

type Item struct {
	ID       string
	Name     string
	Subtitle string
}

type Row struct {
	ID       string
	Label    string
	Value    string
	Editable bool
}

type Action struct {
	ID      string
	Label   string
	Enabled bool
	Reason  string
}

type Preview struct {
	Title    string
	Subtitle string
	Rows     []Row
	Actions  []Action
}

type Backend interface {
	List(context.Context, Section) ([]Item, error)
	Preview(context.Context, Section, Item) (Preview, error)
}
