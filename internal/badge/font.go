package badge

import (
	_ "embed"
)

// Tom Thumb — tiny 4×6 monospace by Robey Pointer
// https://robey.lag.net/2010/01/23/tiny-monospace-font.html
//
//go:embed tom-thumb.bdf
var tomThumbBDF []byte

// SUIT Heavy — used by circle240 OpenType text.
// https://github.com/sunflower-lab/SUIT
//
//go:embed SUIT-Heavy.ttf
var suitHeavyTTF []byte

// badgeFontScale draws 4×6 Tom Thumb glyphs 1:1 on the 64×H badge
// (web UI enlarges the PNG 2× for display).
const badgeFontScale = 1

var badgeFont = mustParseBDF(tomThumbBDF)

func mustParseBDF(data []byte) *bdfFont {
	f, err := parseBDF(data)
	if err != nil {
		panic("badge: parse tom-thumb.bdf: " + err.Error())
	}
	return f
}
