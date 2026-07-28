package badge

import (
	"image/color"
	"testing"
)

func TestLevelColorStops(t *testing.T) {
	cases := []struct {
		pct  float64
		want color.RGBA
	}{
		{0, loadBlue},
		{33, loadGreen},
		{66, loadOrange},
		{100, loadRed},
	}
	for _, tc := range cases {
		got := levelColor(tc.pct)
		if got != tc.want {
			t.Errorf("levelColor(%v) = %v, want %v", tc.pct, got, tc.want)
		}
	}
}

func TestLevelColorMidtonesNotPurple(t *testing.T) {
	// Green→orange midpoint must stay warm (yellowish), not purple.
	// Regression: uint8 underflow in lerpRGBA made this magenta.
	c := levelColor(50)
	if c.B > c.G {
		t.Fatalf("levelColor(50) = %v: blue channel dominates green (purple)", c)
	}
	if c.R < 100 || c.G < 100 {
		t.Fatalf("levelColor(50) = %v: expected warm yellow-green midtone", c)
	}
}

func TestLerpRGBADecreasingChannel(t *testing.T) {
	a := color.RGBA{R: 0x44, G: 0xcc, B: 0x66, A: 0xff} // green
	b := color.RGBA{R: 0xff, G: 0xaa, B: 0x33, A: 0xff} // orange
	mid := lerpRGBA(a, b, 0.5)
	want := color.RGBA{R: 0xa2, G: 0xbb, B: 0x4d, A: 0xff}
	if mid != want {
		t.Fatalf("lerpRGBA mid = %v, want %v", mid, want)
	}
}
