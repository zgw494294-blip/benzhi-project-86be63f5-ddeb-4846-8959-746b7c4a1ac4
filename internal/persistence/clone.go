package persistence

import (
	"encoding/json"

	"oral-history-release-studio/internal/domain"
)

func cloneCase(in *domain.ReleaseCase) (*domain.ReleaseCase, error) {
	out := *in
	return &out, nil
}
func cloneSnapshot(in snapshot) (snapshot, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return snapshot{}, err
	}
	var out snapshot
	if err := json.Unmarshal(b, &out); err != nil {
		return snapshot{}, err
	}
	return out, nil
}
