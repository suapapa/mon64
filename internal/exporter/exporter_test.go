package exporter_test

import (
	"bytes"
	"image/png"
	"testing"
	"time"

	"github.com/suapapa/mon64/internal/domain"
	"github.com/suapapa/mon64/internal/exporter"
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
	j, err := exporter.JSON(snap)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(j, []byte(`"cpu"`)) {
		t.Fatalf("json missing cpu: %s", j)
	}
	y, err := exporter.YAML(snap)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(y, []byte("cpu:")) {
		t.Fatalf("yaml missing cpu: %s", y)
	}
}

func TestBadgePNG64(t *testing.T) {
	cpu, gpu := 10.0, 50.0
	node := domain.NodeState{
		Name:      "spark",
		Reachable: true,
		Collects:  []string{"cpu", "gpu"},
		CPU:       &cpu,
		GPU:       &gpu,
	}
	data, err := exporter.BadgePNG(node)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	w, h := exporter.BadgeDimensions()
	b := img.Bounds()
	if b.Dx() != w || b.Dy() != h {
		t.Fatalf("size = %dx%d, want %dx%d", b.Dx(), b.Dy(), w, h)
	}
}

func TestBadgeOmitsUncollected(t *testing.T) {
	cpu := 10.0
	gpu := 50.0
	node := domain.NodeState{
		Name:      "vraptor",
		Reachable: true,
		Collects:  []string{"cpu", "mem", "swap"},
		CPU:       &cpu,
		GPU:       &gpu, // present but not in collects — must not be rendered
	}
	data, err := exporter.BadgePNG(node)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty png")
	}
}

func TestBadgeUnreachable(t *testing.T) {
	node := domain.NodeState{
		Name:      "down",
		Reachable: false,
		LastError: "connection refused",
	}
	data, err := exporter.BadgePNG(node)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty png")
	}
}
