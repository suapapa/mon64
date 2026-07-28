package badge

import (
	"image/color"
	"math"
)

var (
	// Load gradient stops: blue (0%) → green → orange → red (100%).
	loadBlue   = color.RGBA{R: 0x33, G: 0x88, B: 0xff, A: 0xff}
	loadGreen  = color.RGBA{R: 0x44, G: 0xcc, B: 0x66, A: 0xff}
	loadOrange = color.RGBA{R: 0xff, G: 0xaa, B: 0x33, A: 0xff}
	loadRed    = color.RGBA{R: 0xff, G: 0x44, B: 0x44, A: 0xff}
)

// levelColor maps a load percent onto the shared badge gauge gradient.
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
	// Subtract in float64 — uint8 (b-a) underflows when a channel decreases
	// (e.g. green→orange), which produced purple midtones.
	return color.RGBA{
		R: uint8(math.Round(float64(a.R) + (float64(b.R)-float64(a.R))*t)),
		G: uint8(math.Round(float64(a.G) + (float64(b.G)-float64(a.G))*t)),
		B: uint8(math.Round(float64(a.B) + (float64(b.B)-float64(a.B))*t)),
		A: 0xff,
	}
}
