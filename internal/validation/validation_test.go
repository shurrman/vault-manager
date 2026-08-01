package validation

import (
	"strings"
	"testing"

	"vault-manager/internal/domain"
)

func TestFieldReportsJSONPosition(t *testing.T) {
	err := Field(domain.FormatJSONObject, "{\n  \"name\": true,\n  broken\n}")
	if err == nil {
		t.Fatal("Field() accepted invalid JSON")
	}
	if !strings.Contains(err.Error(), "line 3") || !strings.Contains(err.Error(), "column 3") {
		t.Fatalf("Field() error = %q, want line and column", err)
	}
}

func TestFieldRejectsNonStringMapValue(t *testing.T) {
	err := Field(domain.FormatStringMap, `{"version":2}`)
	if err == nil {
		t.Fatal("Field() accepted a non-string map value")
	}
	if !strings.Contains(err.Error(), "line 1") || !strings.Contains(err.Error(), "column") {
		t.Fatalf("Field() error = %q, want line and column", err)
	}
}

func TestFieldReportsPolicyPosition(t *testing.T) {
	err := Field(domain.FormatPolicy, "path \"secret/data/app\" {\n  capabilities = [\"read\"\n}")
	if err == nil {
		t.Fatal("Field() accepted invalid policy")
	}
	if !strings.Contains(err.Error(), "line ") || !strings.Contains(err.Error(), "column ") {
		t.Fatalf("Field() error = %q, want line and column", err)
	}
}

func TestFieldAcceptsSupportedFormats(t *testing.T) {
	values := map[domain.FieldFormat]string{
		domain.FormatJSONObject: `{"owner":"platform"}`,
		domain.FormatStringMap:  `{"version":"2"}`,
		domain.FormatPolicy:     "path \"secret/data/app\" { capabilities = [\"read\"] }",
	}
	for format, value := range values {
		if err := Field(format, value); err != nil {
			t.Fatalf("Field(%q) error = %v", format, err)
		}
	}
}
