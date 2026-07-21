package badge_test

import (
	"bytes"
	"image/color"
	"image/png"
	"testing"

	"github.com/suapapa/mon64/internal/badge"
	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/domain"
)

func TestRect64BadgePNGSize(t *testing.T) {
	cpu, gpu := 10.0, 50.0
	two := domain.NodeState{
		Name:      "spark",
		Reachable: true,
		Collects:  []string{"cpu", "gpu"},
		CPU:       &cpu,
		GPU:       &gpu,
	}
	data, err := badge.BadgePNG(config.BadgeTypeRect64, []domain.NodeState{two})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	w, h, err := badge.BadgeSize(config.BadgeTypeRect64, []domain.NodeState{two})
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() != w || b.Dy() != h {
		t.Fatalf("size = %dx%d, want %dx%d", b.Dx(), b.Dy(), w, h)
	}
	if w != 64 {
		t.Fatalf("width = %d, want 64", w)
	}

	four := two
	four.Collects = []string{"cpu", "gpu", "mem", "swap"}
	_, h4, err := badge.BadgeSize(config.BadgeTypeRect64, []domain.NodeState{four})
	if err != nil {
		t.Fatal(err)
	}
	if h4 <= h {
		t.Fatalf("more meters should grow height: 2=%d 4=%d", h, h4)
	}

	down := domain.NodeState{Name: "down", Reachable: false, LastError: "timeout"}
	_, hd, err := badge.BadgeSize(config.BadgeTypeRect64, []domain.NodeState{down})
	if err != nil {
		t.Fatal(err)
	}
	if hd <= 0 {
		t.Fatal("unreachable height must be positive")
	}
}

func TestRect64OmitsUncollected(t *testing.T) {
	cpu := 10.0
	gpu := 50.0
	node := domain.NodeState{
		Name:      "vraptor",
		Reachable: true,
		Collects:  []string{"cpu", "mem", "swap"},
		CPU:       &cpu,
		GPU:       &gpu, // present but not in collects — must not be rendered
	}
	data, err := badge.BadgePNG(config.BadgeTypeRect64, []domain.NodeState{node})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty png")
	}
}

func TestRect64Unreachable(t *testing.T) {
	node := domain.NodeState{
		Name:      "down",
		Reachable: false,
		LastError: "connection refused",
	}
	data, err := badge.BadgePNG(config.BadgeTypeRect64, []domain.NodeState{node})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty png")
	}
}

func TestRect64StackPNG(t *testing.T) {
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
	data, err := badge.BadgePNG(config.BadgeTypeRect64, nodes)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	w, h, err := badge.BadgeSize(config.BadgeTypeRect64, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != w || img.Bounds().Dy() != h {
		t.Fatalf("size = %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), w, h)
	}

	_, firstHeight, err := badge.BadgeSize(config.BadgeTypeRect64, nodes[:1])
	if err != nil {
		t.Fatal(err)
	}
	for x := 0; x < w; x++ {
		if got := color.RGBAModel.Convert(img.At(x, firstHeight)).(color.RGBA); got != (color.RGBA{A: 0xff}) {
			t.Fatalf("separator pixel (%d,%d) = %v, want black", x, firstHeight, got)
		}
	}
}

// 3 nodes with 3+2+2 meters + 2 separators → exactly 64×64.
func TestRect64Fits64(t *testing.T) {
	v := 1.0
	nodes := []domain.NodeState{
		{
			Name:      "spark",
			Reachable: true,
			Collects:  []string{"cpu", "gpu", "mem"},
			CPU:       &v,
			GPU:       &v,
			MemUsed:   &v,
		},
		{
			Name:      "vraptor",
			Reachable: true,
			Collects:  []string{"cpu", "mem"},
			CPU:       &v,
			MemUsed:   &v,
		},
		{
			Name:      "OMV",
			Reachable: true,
			Collects:  []string{"cpu", "mem"},
			CPU:       &v,
			MemUsed:   &v,
		},
	}
	w, h, err := badge.BadgeSize(config.BadgeTypeRect64, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if w != 64 || h != 64 {
		t.Fatalf("stack size = %dx%d, want 64x64", w, h)
	}
	data, err := badge.BadgePNG(config.BadgeTypeRect64, nodes)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got.X != 64 || got.Y != 64 {
		t.Fatalf("png size = %dx%d, want 64x64", got.X, got.Y)
	}
}

func TestRect64Empty(t *testing.T) {
	data, err := badge.BadgePNG(config.BadgeTypeRect64, []domain.NodeState{})
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

func TestSelectBadgeNodes(t *testing.T) {
	snap := domain.Snapshot{Nodes: []domain.NodeState{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
	}}
	badgeCfg := config.BadgeConfig{Nodes: []string{"c", "a"}}
	got := badge.SelectBadgeNodes(badgeCfg, snap)
	if len(got) != 2 || got[0].Name != "c" || got[1].Name != "a" {
		t.Fatalf("got %#v", got)
	}
}

func TestCircle128NotImplemented(t *testing.T) {
	_, err := badge.BadgePNG(config.BadgeTypeCircle128, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCircle240BadgePNG(t *testing.T) {
	cpu, mem := 42.0, 67.0
	node := domain.NodeState{
		Name:      "omv",
		Reachable: true,
		Collects:  []string{"cpu", "mem"},
		CPU:       &cpu,
		MemUsed:   &mem,
	}
	data, err := badge.BadgePNG(config.BadgeTypeCircle240, []domain.NodeState{node})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	w, h, err := badge.BadgeSize(config.BadgeTypeCircle240, []domain.NodeState{node})
	if err != nil {
		t.Fatal(err)
	}
	if w != 240 || h != 240 {
		t.Fatalf("size = %dx%d, want 240x240", w, h)
	}
	b := img.Bounds()
	if b.Dx() != w || b.Dy() != h {
		t.Fatalf("png size = %dx%d, want %dx%d", b.Dx(), b.Dy(), w, h)
	}
}

func TestCircle240Unreachable(t *testing.T) {
	node := domain.NodeState{
		Name:      "omv",
		Reachable: false,
		LastError: "timeout",
	}
	data, err := badge.BadgePNG(config.BadgeTypeCircle240, []domain.NodeState{node})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty png")
	}
}

func TestCircle240Empty(t *testing.T) {
	data, err := badge.BadgePNG(config.BadgeTypeCircle240, []domain.NodeState{})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got.X != 240 || got.Y != 240 {
		t.Fatalf("empty size = %dx%d, want 240x240", got.X, got.Y)
	}
}
