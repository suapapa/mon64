package exporter

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	"github.com/suapapa/mon64/internal/domain"
)

const (
	badgeWidth  = 128
	badgeHeight = 128
)

var (
	bgColor     = color.RGBA{R: 0x1a, G: 0x1a, B: 0x2e, A: 0xff}
	textColor   = color.RGBA{R: 0xe0, G: 0xe0, B: 0xe0, A: 0xff}
	mutedColor  = color.RGBA{R: 0x88, G: 0x88, B: 0x99, A: 0xff}
	errorColor  = color.RGBA{R: 0xff, G: 0x44, B: 0x44, A: 0xff}
	okColor     = color.RGBA{R: 0x44, G: 0xcc, B: 0x88, A: 0xff}
	warnColor   = color.RGBA{R: 0xff, G: 0xcc, B: 0x44, A: 0xff}
	dangerColor = color.RGBA{R: 0xff, G: 0x55, B: 0x55, A: 0xff}
	barBG       = color.RGBA{R: 0x33, G: 0x33, B: 0x44, A: 0xff}
)

// BadgePNG renders a 128×128 status badge for one node.
// Font: Tom Thumb (ref/tom-thumb.bdf), drawn at 2× for legibility.
func BadgePNG(node domain.NodeState) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, badgeWidth, badgeHeight))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	lh := badgeFont.lineHeight(badgeFontScale)
	name := truncateToWidth(node.Name, badgeWidth-8)
	drawText(img, 4, lh-badgeFont.descent*badgeFontScale, name, textColor)

	y := lh + 4
	if !node.Reachable {
		drawText(img, 4, y+badgeFont.ascent*badgeFontScale, "UNREACHABLE", errorColor)
		y += lh + 2
		drawText(img, 4, y+badgeFont.ascent*badgeFontScale, truncateToWidth(node.LastError, badgeWidth-8), mutedColor)
	} else {
		y = drawMeter(img, 4, y, "CPU", node.CPU)
		y = drawMeter(img, 4, y, "GPU", node.GPU)
		y = drawMeter(img, 4, y, "MEM", node.MemUsed)
		y = drawMeter(img, 4, y, "SWP", node.SwapUsed)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("png encode: %w", err)
	}
	return buf.Bytes(), nil
}

func drawMeter(img *image.RGBA, x, y int, label string, val *float64) int {
	baseline := y + badgeFont.ascent*badgeFontScale
	drawText(img, x, baseline, label, mutedColor)
	barY := baseline + badgeFont.descent*badgeFontScale + 2
	barW := badgeWidth - 8
	barH := 6
	drawRect(img, x, barY, barW, barH, barBG)

	if val == nil {
		na := "n/a"
		drawText(img, x+barW-badgeFont.measure(na, badgeFontScale), baseline, na, mutedColor)
		return barY + barH + 4
	}
	fill := int(math.Round(float64(barW) * (*val / 100)))
	if fill > barW {
		fill = barW
	}
	if fill > 0 {
		drawRect(img, x, barY, fill, barH, levelColor(*val))
	}
	pct := fmt.Sprintf("%.0f%%", *val)
	drawText(img, x+barW-badgeFont.measure(pct, badgeFontScale), baseline, pct, textColor)
	return barY + barH + 4
}

func levelColor(pct float64) color.RGBA {
	switch {
	case pct >= 90:
		return dangerColor
	case pct >= 75:
		return warnColor
	default:
		return okColor
	}
}

func drawRect(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	r := image.Rect(x, y, x+w, y+h).Intersect(img.Bounds())
	if r.Empty() {
		return
	}
	draw.Draw(img, r, &image.Uniform{C: c}, image.Point{}, draw.Src)
}

func drawText(img *image.RGBA, x, y int, s string, c color.RGBA) {
	badgeFont.drawString(img, x, y, s, c, badgeFontScale)
}

func truncateToWidth(s string, maxPx int) string {
	if badgeFont.measure(s, badgeFontScale) <= maxPx {
		return s
	}
	ellipsis := "..."
	ew := badgeFont.measure(ellipsis, badgeFontScale)
	if ew >= maxPx {
		return ellipsis
	}
	runes := []rune(s)
	for n := len(runes); n > 0; n-- {
		cand := string(runes[:n]) + ellipsis
		if badgeFont.measure(cand, badgeFontScale) <= maxPx {
			return cand
		}
	}
	return ellipsis
}

// BadgeDimensions returns fixed badge size for tests.
func BadgeDimensions() (int, int) {
	return badgeWidth, badgeHeight
}
