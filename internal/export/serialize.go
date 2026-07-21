package export

import (
	"encoding/json"
	"fmt"

	"github.com/suapapa/mon64/internal/domain"
	"gopkg.in/yaml.v3"
)

// JSON encodes snapshot as JSON bytes.
func JSON(snap domain.Snapshot) ([]byte, error) {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("json encode: %w", err)
	}
	return data, nil
}

// YAML encodes snapshot as YAML bytes.
func YAML(snap domain.Snapshot) ([]byte, error) {
	data, err := yaml.Marshal(snap)
	if err != nil {
		return nil, fmt.Errorf("yaml encode: %w", err)
	}
	return data, nil
}
