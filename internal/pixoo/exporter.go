package pixoo

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log/slog"
	"strings"
	"time"

	divoom "github.com/suapapa/go_divoom"
	"github.com/suapapa/mon64/internal/domain"
	"github.com/suapapa/mon64/internal/exporter"
)

const (
	displaySize  = 64
	animationID  = 1
	frameSpeedMS = 10_000
	updateDelay  = 200 * time.Millisecond
)

type animationClient interface {
	ResetSendingAnimationPicID() error
	SendAnimationImgs(id int, speedMSecs []int, imgs []image.Image) error
}

type snapshotSource interface {
	Snapshot() domain.Snapshot
	Subscribe() (<-chan struct{}, func())
}

// Exporter sends the latest stacked badge to a Pixoo64.
type Exporter struct {
	source snapshotSource
	client animationClient
	log    *slog.Logger
}

// New discovers a Pixoo64 on the local network.
func New(source snapshotSource, log *slog.Logger) (*Exporter, error) {
	devices, err := divoom.FindDevice()
	if err != nil {
		return nil, fmt.Errorf("discover Pixoo64: %w", err)
	}

	for _, device := range devices {
		if !isPixoo64(device.DeviceName) {
			continue
		}
		client := divoom.NewClient(device)
		if err := client.ResetSendingAnimationPicID(); err != nil {
			return nil, fmt.Errorf("reset Pixoo64 animation: %w", err)
		}
		log.Info(
			"Pixoo64 exporter enabled",
			"device", device.DeviceName,
			"address", device.DevicePrivateIP,
		)
		return &Exporter{source: source, client: client, log: log}, nil
	}

	return nil, fmt.Errorf("discover Pixoo64: no Pixoo64 found on LAN")
}

// Run sends a debounced update whenever the in-memory snapshot changes.
func (e *Exporter) Run(ctx context.Context) {
	updates, unsubscribe := e.source.Subscribe()
	defer unsubscribe()

	if len(e.source.Snapshot().Nodes) > 0 {
		e.send()
	}

	var timer *time.Timer
	var send <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-updates:
			if timer == nil {
				timer = time.NewTimer(updateDelay)
				send = timer.C
				continue
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(updateDelay)
		case <-send:
			e.send()
			timer = nil
			send = nil
		}
	}
}

func (e *Exporter) send() {
	snapshot := e.source.Snapshot()
	frame := badgeFrame(snapshot.Nodes)
	// Pixoo64 keeps showing the previous frame when PicID is reused
	// without Draw/ResetHttpGifId first.
	if err := e.client.ResetSendingAnimationPicID(); err != nil {
		e.log.Error("Pixoo64 export failed", "err", err)
		return
	}
	err := e.client.SendAnimationImgs(
		animationID,
		[]int{frameSpeedMS},
		[]image.Image{frame},
	)
	if err != nil {
		e.log.Error("Pixoo64 export failed", "err", err)
		return
	}
	e.log.Debug("Pixoo64 updated", "nodes", len(snapshot.Nodes))
}

func badgeFrame(nodes []domain.NodeState) image.Image {
	src := exporter.BadgeStackImage(nodes)
	dst := image.NewRGBA(image.Rect(0, 0, displaySize, displaySize))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)

	srcBounds := src.Bounds()
	if srcBounds.Dy() <= displaySize {
		draw.Draw(dst, srcBounds, src, srcBounds.Min, draw.Src)
		return dst
	}

	targetHeight := displaySize
	targetWidth := srcBounds.Dx() * targetHeight / srcBounds.Dy()
	if targetWidth < 1 {
		targetWidth = 1
	}
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

func isPixoo64(name string) bool {
	normalized := strings.ToLower(name)
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	return normalized == "pixoo64"
}
