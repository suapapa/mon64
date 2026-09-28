package makit

import (
	"context"
	"image"
	"image/color"
	"io"
	"log/slog"
	"testing"

	"github.com/suapapa/mon64/internal/badge"
	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/domain"
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
	frames     [][]byte
	brightness []uint8
	frameErr   error
	brightErr  error
	closed     bool
}

func (f *fakeClient) PutFrame(_ context.Context, rgb []byte) error {
	if f.frameErr != nil {
		return f.frameErr
	}
	cp := append([]byte{}, rgb...)
	f.frames = append(f.frames, cp)
	return nil
}

func (f *fakeClient) PutBrightness(_ context.Context, level uint8) error {
	if f.brightErr != nil {
		return f.brightErr
	}
	f.brightness = append(f.brightness, level)
	return nil
}

func (f *fakeClient) Close() error {
	f.closed = true
	return nil
}

func TestFitDisplayAndRGB888(t *testing.T) {
	nodes := []domain.NodeState{{
		Name:      "spark",
		Reachable: true,
		Collects:  []string{"cpu"},
		CPU:       domain.Ptr(42),
	}}
	img, err := badge.BadgeImage(config.BadgeTypeRect64, nodes)
	if err != nil {
		t.Fatal(err)
	}
	frame := fitDisplay(img)
	if got := frame.Bounds().Size(); got != (image.Pt(64, 64)) {
		t.Fatalf("frame size = %v, want 64x64", got)
	}
	rgb := toRGB888(frame)
	if len(rgb) != frameBytes {
		t.Fatalf("rgb len = %d, want %d", len(rgb), frameBytes)
	}
	// Top-left should be opaque badge pixels or black padding — not transparent.
	c := color.RGBAModel.Convert(frame.At(0, 0)).(color.RGBA)
	if c.A != 0xff {
		t.Fatalf("alpha = %d, want 255", c.A)
	}
}

func TestExporterSendSetsBrightnessAndFrame(t *testing.T) {
	level := 64
	source := &fakeSource{
		snapshot: domain.Snapshot{Nodes: []domain.NodeState{{
			Name:      "spark",
			Reachable: true,
			Collects:  []string{"cpu"},
			CPU:       domain.Ptr(42),
		}}},
		badges: map[string]config.BadgeConfig{
			"homin-lan": {
				Name:  "homin-lan",
				Type:  config.BadgeTypeRect64,
				Nodes: []string{"spark"},
			},
		},
	}
	client := &fakeClient{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	exp, err := newExporter(
		source,
		[]config.Makit64Export{{Badge: "homin-lan", Brightness: &level}},
		log,
		func(string) (frameClient, error) { return client, nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(client.brightness) != 1 || client.brightness[0] != 64 {
		t.Fatalf("initial brightness = %v, want [64]", client.brightness)
	}

	exp.send(context.Background(), 0)

	if len(client.frames) != 1 || len(client.frames[0]) != frameBytes {
		t.Fatalf("frames = %d (len=%v)", len(client.frames), frameLens(client.frames))
	}
	if len(client.brightness) != 2 || client.brightness[1] != 64 {
		t.Fatalf("brightness after send = %v, want [64 64]", client.brightness)
	}
}

func TestExporterSendWithoutBrightness(t *testing.T) {
	source := &fakeSource{
		snapshot: domain.Snapshot{Nodes: []domain.NodeState{{
			Name:      "spark",
			Reachable: true,
			Collects:  []string{"cpu"},
			CPU:       domain.Ptr(42),
		}}},
		badges: map[string]config.BadgeConfig{
			"homin-lan": {
				Name:  "homin-lan",
				Type:  config.BadgeTypeRect64,
				Nodes: []string{"spark"},
			},
		},
	}
	client := &fakeClient{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	exp, err := newExporter(
		source,
		[]config.Makit64Export{{Badge: "homin-lan"}},
		log,
		func(string) (frameClient, error) { return client, nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(client.brightness) != 0 {
		t.Fatalf("brightness = %v, want none", client.brightness)
	}
	exp.send(context.Background(), 0)
	if len(client.frames) != 1 {
		t.Fatalf("frames = %d", len(client.frames))
	}
	if len(client.brightness) != 0 {
		t.Fatalf("brightness after send = %v", client.brightness)
	}
}

func TestExporterDialsConfiguredAddr(t *testing.T) {
	source := &fakeSource{
		snapshot: domain.Snapshot{},
		badges: map[string]config.BadgeConfig{
			"homin-lan": {
				Name:  "homin-lan",
				Type:  config.BadgeTypeRect64,
				Nodes: []string{"spark"},
			},
		},
	}
	client := &fakeClient{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	var gotAddr string
	exp, err := newExporter(
		source,
		[]config.Makit64Export{{Badge: "homin-lan", Addr: "192.168.0.42"}},
		log,
		func(addr string) (frameClient, error) {
			gotAddr = addr
			return client, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if gotAddr != "192.168.0.42:5683" {
		t.Fatalf("dialed addr = %q, want 192.168.0.42:5683", gotAddr)
	}
	if exp.addr != "192.168.0.42:5683" {
		t.Fatalf("exporter addr = %q", exp.addr)
	}
}

func frameLens(frames [][]byte) []int {
	out := make([]int, len(frames))
	for i, f := range frames {
		out[i] = len(f)
	}
	return out
}
