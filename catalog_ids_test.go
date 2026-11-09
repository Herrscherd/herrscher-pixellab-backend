package pixellab

import (
	"encoding/json"
	"slices"
	"testing"
)

func catalogIDs(t *testing.T, raw []byte) []string {
	t.Helper()
	var entries []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.ID)
	}
	slices.Sort(keys)
	return keys
}
