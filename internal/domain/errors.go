package domain

import (
	"fmt"
	"strings"
)

type PartialSaveError struct {
	Applied []string
	Failed  string
	Pending []string
}

func (e *PartialSaveError) Error() string {
	parts := []string{"save partially applied"}
	if len(e.Applied) > 0 {
		parts = append(parts, "applied: "+strings.Join(e.Applied, ", "))
	}
	if e.Failed != "" {
		parts = append(parts, "failed: "+e.Failed)
	}
	if len(e.Pending) > 0 {
		parts = append(parts, "not attempted: "+strings.Join(e.Pending, ", "))
	}
	return fmt.Sprintf("%s; reload before continuing", strings.Join(parts, "; "))
}
