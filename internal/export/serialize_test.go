package export_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/suapapa/mon64/internal/domain"
	"github.com/suapapa/mon64/internal/export"
)

func TestJSONAndYAML(t *testing.T) {
	cpu := 42.5
	snap := domain.Snapshot{
		CollectedAt: time.Now().UTC(),
		Nodes: []domain.NodeState{{
			Name:        "spark",
			CollectedAt: time.Now().UTC(),
			Reachable:   true,
			CPU:         &cpu,
		}},
	}
	j, err := export.JSON(snap)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(j, []byte(`"cpu"`)) {
		t.Fatalf("json missing cpu: %s", j)
	}
	y, err := export.YAML(snap)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(y, []byte("cpu:")) {
		t.Fatalf("yaml missing cpu: %s", y)
	}
}
