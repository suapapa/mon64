package exporter_test

import (
	"bytes"
	"image/color"
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

func TestBadgePNGSize(t *testing.T) {
	cpu, gpu := 10.0, 50.0
	two := domain.NodeState{
		Name:      "spark",
		Reachable: true,
		Collects:  []string{"cpu", "gpu"},
		CPU:       &cpu,
		GPU:       &gpu,
	}
	data, err := exporter.BadgePNG(two)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	w, h := exporter.BadgeSize(two)
	b := img.Bounds()
	if b.Dx() != w || b.Dy() != h {
		t.Fatalf("size = %dx%d, want %dx%d", b.Dx(), b.Dy(), w, h)
	}
	if w != 64 {
		t.Fatalf("width = %d, want 64", w)
	}

	four := two
	four.Collects = []string{"cpu", "gpu", "mem", "swap"}
	_, h4 := exporter.BadgeSize(four)
	if h4 <= h {
		t.Fatalf("more meters should grow height: 2=%d 4=%d", h, h4)
	}

	down := domain.NodeState{Name: "down", Reachable: false, LastError: "timeout"}
	_, hd := exporter.BadgeSize(down)
	if hd <= 0 {
		t.Fatal("unreachable height must be positive")
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

func TestBadgeStackPNG(t *testing.T) {
	cpu := 10.0
	nodes := []domain.NodeState{
		{
			Name:      "spark",
			Reachable: true,
			Collects:  []string{"cpu"},
			CPU:       &cpu,
		},
		{
			Name:      "down",
			Reachable: false,
			LastError: "timeout",
		},
	}
	data, err := exporter.BadgeStackPNG(nodes)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	w, h := exporter.BadgeStackSize(nodes)
	if img.Bounds().Dx() != w || img.Bounds().Dy() != h {
		t.Fatalf("size = %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), w, h)
	}

	_, firstHeight := exporter.BadgeSize(nodes[0])
	for x := 0; x < w; x++ {
		if got := color.RGBAModel.Convert(img.At(x, firstHeight)).(color.RGBA); got != (color.RGBA{A: 0xff}) {
			t.Fatalf("separator pixel (%d,%d) = %v, want black", x, firstHeight, got)
		}
	}
}

func TestBadgeStackPNGEmpty(t *testing.T) {
	data, err := exporter.BadgeStackPNG([]domain.NodeState{})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got.X != 64 || got.Y != 1 {
		t.Fatalf("empty size = %dx%d, want 64x1", got.X, got.Y)
	}
}
