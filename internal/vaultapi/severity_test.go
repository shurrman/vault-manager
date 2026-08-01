package vaultapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"vault-manager/internal/domain"
)

func TestDangerSeverityOnHealthAndSealStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/sys/health":
			writeJSON(w, map[string]interface{}{
				"initialized": true, "sealed": true, "standby": true,
				"version": "1.20.0", "cluster_name": "test", "cluster_id": "cluster-1",
			})
		case "/v1/sys/seal-status":
			writeJSON(w, map[string]interface{}{
				"initialized": true, "sealed": true, "t": 3, "n": 5,
				"progress": 0, "type": "shamir", "storage_type": "raft",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	health, err := client.Preview(context.Background(), domain.SectionStatus, domain.Item{ID: "health", Name: "Health"})
	if err != nil {
		t.Fatalf("Preview(health) error = %v", err)
	}
	for _, label := range []string{"Sealed", "Standby"} {
		if rowSeverity(health.Rows, label) != domain.SeverityDanger {
			t.Errorf("health %s severity = %q", label, rowSeverity(health.Rows, label))
		}
	}
	seal, err := client.Preview(context.Background(), domain.SectionStatus, domain.Item{ID: "seal", Name: "Seal status"})
	if err != nil {
		t.Fatalf("Preview(seal) error = %v", err)
	}
	if rowSeverity(seal.Rows, "Sealed") != domain.SeverityDanger {
		t.Fatalf("seal severity = %q", rowSeverity(seal.Rows, "Sealed"))
	}
}

func rowSeverity(rows []domain.Row, label string) domain.Severity {
	for _, row := range rows {
		if row.Label == label {
			return row.Severity
		}
	}
	return domain.SeverityNormal
}
