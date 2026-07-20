package pixoo

import (
	"image"
	"image/color"
	"io"
	"log/slog"
	"testing"

	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/domain"
	"github.com/suapapa/mon64/internal/exporter"
)

type fakeSource struct {
	snapshot domain.Snapshot
	badges   map[string]config.BadgeConfig
}

func (f *fakeSource) Snapshot() domain.Snapshot {
	return f.snapshot
}

func (f *fakeSource) Subscribe() (<-chan struct{}, func()) {
	return make(chan struct{}), func() {}
}

func (f *fakeSource) BadgeByName(name string) (config.BadgeConfig, bool) {
	b, ok := f.badges[name]
	return b, ok
}

type fakeClient struct {
	resets int
	id     int
	speeds []int
	images []image.Image
}

func (f *fakeClient) ResetSendingAnimationPicID() error {
	f.resets++
	return nil
}

func (f *fakeClient) SendAnimationImgs(id int, speeds []int, images []image.Image) error {
	f.id = id
	f.speeds = speeds
	f.images = images
	return nil
}

func TestFitDisplayPadsShortBadge(t *testing.T) {
	nodes := []domain.NodeState{{
		Name:      "spark",
		Reachable: true,
		Collects:  []string{"cpu"},
		CPU:       domain.Ptr(42),
	}}
	img, err := exporter.BadgeImage(config.BadgeTypeRect64, nodes)
	if err != nil {
		t.Fatal(err)
	}

	frame := fitDisplay(img)
	if got := frame.Bounds().Size(); got != (image.Pt(64, 64)) {
		t.Fatalf("frame size = %v, want 64x64", got)
	}
	if got := color.RGBAModel.Convert(frame.At(0, 63)).(color.RGBA); got != (color.RGBA{A: 0xff}) {
		t.Fatalf("bottom padding = %v, want black", got)
	}
}

func TestFitDisplayFitsTallBadge(t *testing.T) {
	nodes := make([]domain.NodeState, 4)
	for i := range nodes {
		nodes[i] = domain.NodeState{
			Name:      "node",
			Reachable: true,
			Collects:  []string{"cpu", "gpu", "mem", "swap"},
			CPU:       domain.Ptr(42),
			GPU:       domain.Ptr(42),
			MemUsed:   domain.Ptr(42),
			SwapUsed:  domain.Ptr(42),
		}
	}
	img, err := exporter.BadgeImage(config.BadgeTypeRect64, nodes)
	if err != nil {
		t.Fatal(err)
	}

	frame := fitDisplay(img)
	if got := frame.Bounds().Size(); got != (image.Pt(64, 64)) {
		t.Fatalf("frame size = %v, want 64x64", got)
	}
	if got := color.RGBAModel.Convert(frame.At(0, 0)).(color.RGBA); got != (color.RGBA{A: 0xff}) {
		t.Fatalf("left padding = %v, want black", got)
	}
}

func TestExporterSend(t *testing.T) {
	source := &fakeSource{
		snapshot: domain.Snapshot{Nodes: []domain.NodeState{{
			Name:      "spark",
			Reachable: true,
			Collects:  []string{"cpu"},
			CPU:       domain.Ptr(42),
		}}},
		badges: map[string]config.BadgeConfig{
			"homelab": {
				Name:  "homelab",
				Type:  config.BadgeTypeRect64,
				Nodes: []string{"spark"},
			},
		},
	}
	client := &fakeClient{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	pixooExporter := &Exporter{
		source:     source,
		client:     client,
		log:        log,
		badgeNames: []string{"homelab"},
	}

	pixooExporter.send()
	pixooExporter.send()

	if client.resets != 2 {
		t.Fatalf("resets = %d, want 2", client.resets)
	}
	if client.id != animationID {
		t.Fatalf("animation ID = %d, want %d", client.id, animationID)
	}
	if len(client.speeds) != 1 || client.speeds[0] != frameSpeedMS {
		t.Fatalf("speeds = %v, want [%d]", client.speeds, frameSpeedMS)
	}
	if len(client.images) != 1 || client.images[0].Bounds().Size() != image.Pt(64, 64) {
		t.Fatalf("images = %d with invalid size", len(client.images))
	}
}

func TestIsPixoo64(t *testing.T) {
	for _, name := range []string{"Pixoo64", "pixoo-64", "PIXOO 64"} {
		if !isPixoo64(name) {
			t.Errorf("isPixoo64(%q) = false", name)
		}
	}
	if isPixoo64("TimeBox") {
		t.Error("isPixoo64(TimeBox) = true")
	}
}
