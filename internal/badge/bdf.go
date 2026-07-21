package badge

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strconv"
	"strings"
)

// bdfFont is a parsed BDF bitmap font.
type bdfFont struct {
	ascent  int
	descent int
	glyphs  map[rune]bdfGlyph
}

type bdfGlyph struct {
	advance       int
	width, height int
	xOff, yOff    int
	rows          []byte // packed MSB-first rows; stride = (width+7)/8
}

func parseBDF(data []byte) (*bdfFont, error) {
	f := &bdfFont{glyphs: make(map[rune]bdfGlyph)}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		enc                int
		dwidth             int
		bbw, bbh, bbx, bby int
		inBitmap           bool
		bitmap             []byte
		haveEnc            bool
	)

	flush := func() error {
		if !haveEnc {
			return nil
		}
		stride := (bbw + 7) / 8
		if bbw > 0 && bbh > 0 {
			if len(bitmap) != bbh*stride {
				return fmt.Errorf("bdf: encoding %d bitmap size %d, want %d", enc, len(bitmap), bbh*stride)
			}
		}
		f.glyphs[rune(enc)] = bdfGlyph{
			advance: dwidth,
			width:   bbw,
			height:  bbh,
			xOff:    bbx,
			yOff:    bby,
			rows:    append([]byte{}, bitmap...),
		}
		haveEnc = false
		inBitmap = false
		bitmap = nil
		return nil
	}

	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "FONT_ASCENT":
			if len(fields) < 2 {
				return nil, fmt.Errorf("bdf: bad FONT_ASCENT: %q", line)
			}
			v, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, fmt.Errorf("bdf: FONT_ASCENT: %w", err)
			}
			f.ascent = v
		case "FONT_DESCENT":
			if len(fields) < 2 {
				return nil, fmt.Errorf("bdf: bad FONT_DESCENT: %q", line)
			}
			v, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, fmt.Errorf("bdf: FONT_DESCENT: %w", err)
			}
			f.descent = v
		case "ENCODING":
			if err := flush(); err != nil {
				return nil, err
			}
			if len(fields) < 2 {
				return nil, fmt.Errorf("bdf: bad ENCODING: %q", line)
			}
			v, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, fmt.Errorf("bdf: ENCODING: %w", err)
			}
			enc = v
			haveEnc = true
			dwidth = 0
			bbw, bbh, bbx, bby = 0, 0, 0, 0
		case "DWIDTH":
			if len(fields) < 2 {
				return nil, fmt.Errorf("bdf: bad DWIDTH: %q", line)
			}
			v, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, fmt.Errorf("bdf: DWIDTH: %w", err)
			}
			dwidth = v
		case "BBX":
			if len(fields) < 5 {
				return nil, fmt.Errorf("bdf: bad BBX: %q", line)
			}
			var err error
			bbw, err = strconv.Atoi(fields[1])
			if err != nil {
				return nil, fmt.Errorf("bdf: BBX w: %w", err)
			}
			bbh, err = strconv.Atoi(fields[2])
			if err != nil {
				return nil, fmt.Errorf("bdf: BBX h: %w", err)
			}
			bbx, err = strconv.Atoi(fields[3])
			if err != nil {
				return nil, fmt.Errorf("bdf: BBX x: %w", err)
			}
			bby, err = strconv.Atoi(fields[4])
			if err != nil {
				return nil, fmt.Errorf("bdf: BBX y: %w", err)
			}
		case "BITMAP":
			inBitmap = true
			bitmap = bitmap[:0]
		case "ENDCHAR":
			inBitmap = false
			if err := flush(); err != nil {
				return nil, err
			}
		default:
			if !inBitmap {
				continue
			}
			hex := strings.TrimSpace(line)
			if hex == "" {
				continue
			}
			if len(hex)%2 != 0 {
				hex = "0" + hex
			}
			for i := 0; i < len(hex); i += 2 {
				v, err := strconv.ParseUint(hex[i:i+2], 16, 8)
				if err != nil {
					return nil, fmt.Errorf("bdf: bitmap hex %q: %w", hex, err)
				}
				bitmap = append(bitmap, byte(v))
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if len(f.glyphs) == 0 {
		return nil, fmt.Errorf("bdf: no glyphs")
	}
	if f.ascent == 0 {
		f.ascent = 5
	}
	return f, nil
}

// drawString paints s with face at pixel scale, baseline at (x, y).
func (f *bdfFont) drawString(dst *image.RGBA, x, y int, s string, c color.RGBA, scale int) {
	if scale < 1 {
		scale = 1
	}
	penX := x
	for _, r := range s {
		g, ok := f.glyphs[r]
		if !ok {
			g, ok = f.glyphs['?']
			if !ok {
				penX += 4 * scale
				continue
			}
		}
		stride := (g.width + 7) / 8
		for row := 0; row < g.height; row++ {
			// BDF y grows up; image y grows down. Bottom of bitmap is at baseline+yOff.
			py := y - (g.yOff+g.height)*scale + row*scale
			for col := 0; col < g.width; col++ {
				byteI := row*stride + col/8
				if byteI < 0 || byteI >= len(g.rows) {
					continue
				}
				bit := g.rows[byteI] & (0x80 >> uint(col%8))
				if bit == 0 {
					continue
				}
				px := penX + (g.xOff+col)*scale
				fillScale(dst, px, py, scale, c)
			}
		}
		adv := g.advance
		if adv == 0 {
			adv = max(g.width+g.xOff, 1)
		}
		penX += adv * scale
	}
}

func fillScale(dst *image.RGBA, x, y, scale int, c color.RGBA) {
	r := image.Rect(x, y, x+scale, y+scale).Intersect(dst.Bounds())
	if r.Empty() {
		return
	}
	draw.Draw(
		dst,
		r,
		&image.Uniform{C: c},
		image.Point{},
		draw.Src,
	)
}

func (f *bdfFont) lineHeight(scale int) int {
	return (f.ascent + f.descent) * scale
}

func (f *bdfFont) measure(s string, scale int) int {
	if scale < 1 {
		scale = 1
	}
	var w int
	for _, r := range s {
		g, ok := f.glyphs[r]
		if !ok {
			w += 4 * scale
			continue
		}
		adv := g.advance
		if adv == 0 {
			adv = 4
		}
		w += adv * scale
	}
	return w
}
