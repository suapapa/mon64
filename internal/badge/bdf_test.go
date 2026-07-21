package badge

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseTomThumbBDF(t *testing.T) {
	f, err := parseBDF(tomThumbBDF)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.glyphs) < 95 {
		t.Fatalf("glyphs = %d, want >= 95 (ASCII)", len(f.glyphs))
	}
	g, ok := f.glyphs['A']
	if !ok {
		t.Fatal("missing glyph A")
	}
	if g.advance != 4 {
		t.Fatalf("A advance = %d, want 4", g.advance)
	}
	if f.ascent != 5 || f.descent != 1 {
		t.Fatalf("ascent/descent = %d/%d, want 5/1", f.ascent, f.descent)
	}
}

func TestRefTomThumbMatchesEmbed(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	ref := filepath.Join(filepath.Dir(file), "..", "..", "ref", "tom-thumb.bdf")
	data, err := os.ReadFile(ref)
	if err != nil {
		t.Fatalf("read ref font: %v", err)
	}
	if !bytes.Equal(data, tomThumbBDF) {
		t.Fatal("ref/tom-thumb.bdf differs from embedded internal/badge/tom-thumb.bdf")
	}
}
