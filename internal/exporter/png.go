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

const badgeWidth = 64

const (
	badgeSidePad    = 1
	badgeNameGap    = 0
	badgeMeterGap   = 1
	badgeUnreachGap = 0
	badgeBottomPad  = 0
	badgeStackGap   = 2
)

var (
	bgColor    = color.RGBA{R: 0x1a, G: 0x1a, B: 0x2e, A: 0xff}
	textColor  = color.RGBA{R: 0xe0, G: 0xe0, B: 0xe0, A: 0xff}
	mutedColor = color.RGBA{R: 0x88, G: 0x88, B: 0x99, A: 0xff}
	errorColor = color.RGBA{R: 0xff, G: 0x44, B: 0x44, A: 0xff}
	barBG      = color.RGBA{R: 0x33, G: 0x33, B: 0x44, A: 0xff}

	// Load gradient stops: blue (0%) → green → orange → red (100%).
	loadBlue   = color.RGBA{R: 0x33, G: 0x88, B: 0xff, A: 0xff}
	loadGreen  = color.RGBA{R: 0x44, G: 0xcc, B: 0x66, A: 0xff}
	loadOrange = color.RGBA{R: 0xff, G: 0xaa, B: 0x33, A: 0xff}
	loadRed    = color.RGBA{R: 0xff, G: 0x44, B: 0x44, A: 0xff}
)

// rect64Image stacks per-node tiles vertically with black separators.
// Meter (cpu/mem/…) drawing lives under this badge type only.
func rect64Image(nodes []domain.NodeState) image.Image {
	height := rect64Height(nodes)
	if height == 0 {
		height = 1
	}
	stack := image.NewRGBA(image.Rect(0, 0, badgeWidth, height))
	draw.Draw(stack, stack.Bounds(), image.Black, image.Point{}, draw.Src)

	y := 0
	for _, node := range nodes {
		tile := nodeTileImage(node)
		target := image.Rect(0, y, badgeWidth, y+tile.Bounds().Dy())
		draw.Draw(stack, target, tile, image.Point{}, draw.Src)
		y = target.Max.Y + badgeStackGap
	}

	return stack
}

func rect64Height(nodes []domain.NodeState) int {
	if len(nodes) == 0 {
		return 0
	}
	height := (len(nodes) - 1) * badgeStackGap
	for _, node := range nodes {
		height += nodeTileHeight(node)
	}
	return height
}

func nodeTileImage(node domain.NodeState) *image.RGBA {
	h := nodeTileHeight(node)
	img := image.NewRGBA(image.Rect(0, 0, badgeWidth, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	lh := badgeFont.lineHeight(badgeFontScale)
	contentW := badgeWidth - 2*badgeSidePad
	name := truncateToWidth(node.Name, contentW)
	nameX := (badgeWidth - badgeFont.measure(name, badgeFontScale)) / 2
	drawText(img, nameX, lh-badgeFont.descent*badgeFontScale, name, textColor)

	y := lh + badgeNameGap
	if !node.Reachable {
		drawText(img, badgeSidePad, y+badgeFont.ascent*badgeFontScale, "UNREACHABLE", errorColor)
		y += lh + badgeUnreachGap
		drawText(img, badgeSidePad, y+badgeFont.ascent*badgeFontScale, truncateToWidth(node.LastError, contentW), mutedColor)
	} else {
		if node.Wants("cpu") {
			y = drawMeter(img, badgeSidePad, y, "CPU", node.CPU)
		}
		if node.Wants("gpu") {
			y = drawMeter(img, badgeSidePad, y, "GPU", node.GPU)
		}
		if node.Wants("mem") {
			y = drawMeter(img, badgeSidePad, y, "MEM", node.MemUsed)
		}
		if node.Wants("swap") {
			y = drawMeter(img, badgeSidePad, y, "SWP", node.SwapUsed)
		}
	}

	return img
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("png encode: %w", err)
	}
	return buf.Bytes(), nil
}

func nodeTileHeight(node domain.NodeState) int {
	lh := badgeFont.lineHeight(badgeFontScale)
	h := lh + badgeNameGap
	if !node.Reachable {
		return h + lh + badgeUnreachGap + lh + badgeBottomPad
	}
	n := meterCount(node)
	if n == 0 {
		return h + badgeBottomPad
	}
	// Match drawMeter stride (lh-1 content + badgeMeterGap).
	return h + n*meterStride(lh)
}

func meterCount(node domain.NodeState) int {
	n := 0
	for _, kind := range []string{"cpu", "gpu", "mem", "swap"} {
		if node.Wants(kind) {
			n++
		}
	}
	return n
}

// meterStride is the vertical advance per meter row (bar is lh-1 tall).
func meterStride(lh int) int {
	return lh - 1 + badgeMeterGap
}

func drawMeter(img *image.RGBA, x, y int, label string, val *float64) int {
	lh := badgeFont.lineHeight(badgeFontScale)
	baseline := y + badgeFont.ascent*badgeFontScale
	drawText(img, x, baseline, label, mutedColor)

	const gap = 1
	right := badgeWidth - badgeSidePad
	barX := x + badgeFont.measure(label, badgeFontScale) + gap
	barW := right - barX
	if barW > 0 {
		drawRect(img, barX, y, barW, lh-1, barBG)
		if val != nil {
			fill := int(math.Round(float64(barW) * (*val / 100)))
			if fill > barW {
				fill = barW
			}
			if fill > 0 {
				drawRect(img, barX, y, fill, lh-1, levelColor(*val))
			}
		}
	}

	valueStr := "n/a"
	valueColor := mutedColor
	if val != nil {
		valueStr = fmt.Sprintf("%.0f%%", *val)
		valueColor = textColor
	}
	valueW := badgeFont.measure(valueStr, badgeFontScale)
	drawText(img, right-valueW, baseline, valueStr, valueColor)
	return y + meterStride(lh)
}

func levelColor(pct float64) color.RGBA {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	type stop struct {
		t float64
		c color.RGBA
	}
	stops := []stop{
		{0, loadBlue},
		{33, loadGreen},
		{66, loadOrange},
		{100, loadRed},
	}
	for i := 1; i < len(stops); i++ {
		if pct <= stops[i].t {
			span := stops[i].t - stops[i-1].t
			f := (pct - stops[i-1].t) / span
			return lerpRGBA(stops[i-1].c, stops[i].c, f)
		}
	}
	return loadRed
}

func lerpRGBA(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(math.Round(float64(a.R) + float64(b.R-a.R)*t)),
		G: uint8(math.Round(float64(a.G) + float64(b.G-a.G)*t)),
		B: uint8(math.Round(float64(a.B) + float64(b.B-a.B)*t)),
		A: 0xff,
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
