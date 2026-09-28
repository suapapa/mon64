package makit

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log/slog"
	"time"

	"github.com/suapapa/mon64/internal/badge"
	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/domain"
)

const (
	displaySize = 64
	frameBytes  = displaySize * displaySize * 3
	updateDelay = 200 * time.Millisecond
	framePeriod = 10 * time.Second
)

type badgeSource interface {
	Snapshot() domain.Snapshot
	Subscribe() (<-chan struct{}, func())
	BadgeByName(name string) (config.BadgeConfig, bool)
}

// Exporter sends configured named badges to a makit64 panel over CoAP.
type Exporter struct {
	source  badgeSource
	client  frameClient
	log     *slog.Logger
	addr    string
	exports []config.Makit64Export
}

// New dials the configured makit64 addr (default makit.local:5683) and
// optionally sets brightness from the first entry that specifies it.
func New(
	source badgeSource,
	exports []config.Makit64Export,
	log *slog.Logger,
) (*Exporter, error) {
	return newExporter(source, exports, log, dialDevice)
}

func newExporter(
	source badgeSource,
	exports []config.Makit64Export,
	log *slog.Logger,
	dial func(addr string) (frameClient, error),
) (*Exporter, error) {
	if len(exports) == 0 {
		return nil, fmt.Errorf("makit64: at least one export is required")
	}
	if log == nil {
		log = slog.Default()
	}

	addr := exports[0].Addr
	normalized, err := config.NormalizeMakit64Addr(addr)
	if err != nil {
		return nil, fmt.Errorf("makit64 addr: %w", err)
	}

	client, err := dial(normalized)
	if err != nil {
		return nil, err
	}

	copied := make([]config.Makit64Export, len(exports))
	for i, exp := range exports {
		copied[i] = exp
		copied[i].Addr = normalized
		if exp.Brightness != nil {
			b := *exp.Brightness
			copied[i].Brightness = &b
		}
	}

	e := &Exporter{
		source:  source,
		client:  client,
		log:     log,
		addr:    normalized,
		exports: copied,
	}

	ctx, cancel := context.WithTimeout(context.Background(), putTimeout)
	defer cancel()
	if err := e.applyInitialBrightness(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}

	log.Info(
		"makit64 exporter enabled",
		"addr", normalized,
		"badges", badgeNames(copied),
	)
	return e, nil
}

func (e *Exporter) applyInitialBrightness(ctx context.Context) error {
	for _, exp := range e.exports {
		if exp.Brightness == nil {
			continue
		}
		level := uint8(*exp.Brightness)
		if err := e.client.PutBrightness(ctx, level); err != nil {
			return fmt.Errorf("set brightness %d: %w", level, err)
		}
		e.log.Info("makit64 brightness set", "brightness", level, "badge", exp.Badge)
		return nil
	}
	return nil
}

// Run sends a debounced update whenever the in-memory snapshot changes.
// When multiple badges are configured, they cycle every framePeriod.
func (e *Exporter) Run(ctx context.Context) {
	defer func() {
		if err := e.client.Close(); err != nil {
			e.log.Error("makit64 close failed", "err", err)
		}
	}()

	updates, unsubscribe := e.source.Subscribe()
	defer unsubscribe()

	idx := 0
	e.send(ctx, idx)

	var debounce *time.Timer
	var debounceC <-chan time.Time
	var cycle <-chan time.Time
	if len(e.exports) > 1 {
		t := time.NewTicker(framePeriod)
		defer t.Stop()
		cycle = t.C
	}

	for {
		select {
		case <-ctx.Done():
			if debounce != nil {
				debounce.Stop()
			}
			return
		case <-updates:
			if debounce == nil {
				debounce = time.NewTimer(updateDelay)
				debounceC = debounce.C
				continue
			}
			if !debounce.Stop() {
				select {
				case <-debounce.C:
				default:
				}
			}
			debounce.Reset(updateDelay)
		case <-debounceC:
			e.send(ctx, idx)
			debounce = nil
			debounceC = nil
		case <-cycle:
			idx = (idx + 1) % len(e.exports)
			e.send(ctx, idx)
		}
	}
}

func (e *Exporter) send(ctx context.Context, idx int) {
	if idx < 0 || idx >= len(e.exports) {
		return
	}
	exp := e.exports[idx]
	badgeCfg, ok := e.source.BadgeByName(exp.Badge)
	if !ok {
		e.log.Error("makit64 export skipped missing badge", "badge", exp.Badge)
		return
	}
	nodes := badge.SelectBadgeNodes(badgeCfg, e.source.Snapshot())
	img, err := badge.BadgeImage(badgeCfg.Type, nodes)
	if err != nil {
		e.log.Error("makit64 export render failed", "badge", exp.Badge, "err", err)
		return
	}
	rgb := toRGB888(fitDisplay(img))

	if exp.Brightness != nil {
		level := uint8(*exp.Brightness)
		if err := e.client.PutBrightness(ctx, level); err != nil {
			e.log.Error("makit64 brightness failed", "badge", exp.Badge, "err", err)
			return
		}
	}

	if err := e.client.PutFrame(ctx, rgb); err != nil {
		e.log.Error("makit64 export failed", "badge", exp.Badge, "err", err)
		return
	}
	e.log.Debug("makit64 updated", "badge", exp.Badge)
}

func fitDisplay(src image.Image) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, displaySize, displaySize))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)

	srcBounds := src.Bounds()
	if srcBounds.Dy() <= displaySize {
		draw.Draw(dst, srcBounds, src, srcBounds.Min, draw.Src)
		return dst
	}

	targetHeight := displaySize
	targetWidth := max(srcBounds.Dx()*targetHeight/srcBounds.Dy(), 1)
	offsetX := (displaySize - targetWidth) / 2
	for y := range targetHeight {
		srcY := srcBounds.Min.Y + y*srcBounds.Dy()/targetHeight
		for x := range targetWidth {
			srcX := srcBounds.Min.X + x*srcBounds.Dx()/targetWidth
			dst.Set(offsetX+x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func toRGB888(img *image.RGBA) []byte {
	out := make([]byte, frameBytes)
	for y := range displaySize {
		for x := range displaySize {
			c := img.RGBAAt(x, y)
			i := (y*displaySize + x) * 3
			out[i] = c.R
			out[i+1] = c.G
			out[i+2] = c.B
		}
	}
	return out
}

func badgeNames(exports []config.Makit64Export) []string {
	names := make([]string, len(exports))
	for i, exp := range exports {
		names[i] = exp.Badge
	}
	return names
}
