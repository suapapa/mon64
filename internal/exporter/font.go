package exporter

import (
	_ "embed"
)

// Tom Thumb — tiny 4×6 monospace by Robey Pointer
// https://robey.lag.net/2010/01/23/tiny-monospace-font.html
// Source copy also kept at ref/tom-thumb.bdf.
//
//go:embed tom-thumb.bdf
var tomThumbBDF []byte

// badgeFontScale enlarges the 4×6 glyphs for a 128×128 badge.
const badgeFontScale = 2

var badgeFont = mustParseBDF(tomThumbBDF)

func mustParseBDF(data []byte) *bdfFont {
	f, err := parseBDF(data)
	if err != nil {
		panic("exporter: parse tom-thumb.bdf: " + err.Error())
	}
	return f
}
