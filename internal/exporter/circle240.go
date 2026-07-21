package exporter

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/suapapa/mon64/internal/domain"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	circle240Size   = 240
	circle240Center = circle240Size / 2
)

var (
	c240BgColor         = color.RGBA{R: 11, G: 14, B: 18, A: 255}
	c240GridColor       = color.RGBA{R: 22, G: 26, B: 32, A: 255}
	c240CenterGridColor = color.RGBA{R: 38, G: 43, B: 51, A: 255}
	c240TrackColor      = color.RGBA{R: 24, G: 28, B: 35, A: 255}
	c240LabelColor      = color.RGBA{R: 210, G: 215, B: 220, A: 255}
	c240MutedColor      = color.RGBA{R: 136, G: 140, B: 148, A: 255}
	c240ErrorColor      = color.RGBA{R: 255, G: 59, B: 48, A: 255}
)

func circle240Image(nodes []domain.NodeState) (image.Image, error) {
	if len(nodes) == 0 {
		img := image.NewRGBA(image.Rect(0, 0, circle240Size, circle240Size))
		draw.Draw(img, img.Bounds(), &image.Uniform{C: c240BgColor}, image.Point{}, draw.Src)
		return img, nil
	}
	return renderCircle240(nodes[0], suitHeavyTTF)
}

func renderCircle240(node domain.NodeState, fontData []byte) (image.Image, error) {
	f, err := opentype.Parse(fontData)
	if err != nil {
		return nil, fmt.Errorf("parse opentype font: %w", err)
	}

	faceSmall, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    15,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("create small face: %w", err)
	}
	defer faceSmall.Close()

	faceBig, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    32,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("create big face: %w", err)
	}
	defer faceBig.Close()

	canvas := image.NewRGBA(image.Rect(0, 0, circle240Size, circle240Size))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: c240BgColor}, image.Point{}, draw.Src)
	c240DrawGrid(canvas)

	cpuVal, cpuOK := metricPercent(node.CPU)
	memVal, memOK := metricPercent(node.MemUsed)
	if !node.Reachable {
		cpuVal, cpuOK = 0, false
		memVal, memOK = 0, false
	}

	cpuColor := c240GaugeColor(cpuVal)
	memColor := c240GaugeColor(memVal)
	if !cpuOK {
		cpuColor = c240MutedColor
	}
	if !memOK {
		memColor = c240MutedColor
	}

	gauges := []c240GaugeSpec{
		{
			value:      cpuVal,
			accent:     cpuColor,
			centerY:    circle240Center,
			startAngle: 175 * math.Pi / 180,
			endAngle:   5 * math.Pi / 180,
		},
		{
			value:      memVal,
			accent:     memColor,
			centerY:    circle240Center,
			startAngle: 185 * math.Pi / 180,
			endAngle:   355 * math.Pi / 180,
		},
	}
	for _, g := range gauges {
		c240DrawGauge(canvas, g)
	}

	c240DrawOTText(canvas, c240TextConfig{
		face: faceSmall,
		text: "CPU",
		col:  cpuColor,
		cx:   circle240Center,
		cy:   46,
	})
	c240DrawGlowText(canvas, c240TextConfig{
		face: faceBig,
		text: formatPercent(cpuVal, cpuOK),
		col:  cpuColor,
		cx:   circle240Center,
		cy:   78,
	})

	hostCol := c240LabelColor
	hostText := node.Name
	if !node.Reachable {
		hostCol = c240ErrorColor
		if node.LastError != "" {
			hostText = "DOWN"
		}
	}
	c240DrawOTText(canvas, c240TextConfig{
		face: faceSmall,
		text: hostText,
		col:  hostCol,
		cx:   circle240Center,
		cy:   120,
	})

	c240DrawGlowText(canvas, c240TextConfig{
		face: faceBig,
		text: formatPercent(memVal, memOK),
		col:  memColor,
		cx:   circle240Center,
		cy:   162,
	})
	c240DrawOTText(canvas, c240TextConfig{
		face: faceSmall,
		text: "MEM",
		col:  memColor,
		cx:   circle240Center,
		cy:   194,
	})

	return canvas, nil
}

func metricPercent(p *float64) (float64, bool) {
	if p == nil {
		return 0, false
	}
	return domain.ClampPercent(*p), true
}

func formatPercent(v float64, ok bool) string {
	if !ok {
		return "n/a"
	}
	return fmt.Sprintf("%.0f%%", v)
}

func c240GaugeColor(percent float64) color.RGBA {
	// 0%   -> Green (52, 199, 89)
	// 50%  -> Orange (255, 149, 0)
	// 100% -> Red (255, 59, 48)
	var r, g, b float64
	if percent <= 50 {
		t := percent / 50.0
		r = 52 + (255-52)*t
		g = 199 + (149-199)*t
		b = 89 + (0-89)*t
	} else {
		t := (percent - 50.0) / 50.0
		r = 255
		g = 149 + (59-149)*t
		b = 0 + (48-0)*t
	}
	return color.RGBA{
		R: uint8(math.Round(r)),
		G: uint8(math.Round(g)),
		B: uint8(math.Round(b)),
		A: 255,
	}
}

type c240GaugeSpec struct {
	value      float64
	accent     color.RGBA
	centerY    float64
	startAngle float64
	endAngle   float64
}

type c240ArcConfig struct {
	cx     float64
	cy     float64
	radius float64
	width  float64
	start  float64
	end    float64
	col    color.RGBA
}

type c240TextConfig struct {
	face font.Face
	text string
	col  color.RGBA
	cx   int
	cy   int
}

func c240Blend(dst, src color.RGBA) color.RGBA {
	if src.A == 255 {
		return src
	}
	if src.A == 0 {
		return dst
	}
	sa := float64(src.A) / 255
	da := float64(dst.A) / 255
	outA := sa + da*(1-sa)
	if outA == 0 {
		return color.RGBA{}
	}
	outR := (float64(src.R)*sa + float64(dst.R)*da*(1-sa)) / outA
	outG := (float64(src.G)*sa + float64(dst.G)*da*(1-sa)) / outA
	outB := (float64(src.B)*sa + float64(dst.B)*da*(1-sa)) / outA
	return color.RGBA{
		R: uint8(outR),
		G: uint8(outG),
		B: uint8(outB),
		A: uint8(outA * 255),
	}
}

func c240DrawGrid(img *image.RGBA) {
	step := 20
	for x := 0; x < circle240Size; x += step {
		c240DrawVLine(img, x, 0, circle240Size-1, c240GridColor)
	}
	for y := 0; y < circle240Size; y += step {
		c240DrawHLine(img, 0, circle240Size-1, y, c240GridColor)
	}

	c240DrawVLine(img, circle240Center, 0, circle240Size-1, c240CenterGridColor)
	c240DrawHLine(img, 0, circle240Size-1, circle240Center, c240CenterGridColor)

	for x := circle240Center - 110; x <= circle240Center+110; x += 5 {
		tickLen := 1
		if (x-circle240Center)%20 == 0 {
			tickLen = 2
		}
		c240DrawVLine(img, x, circle240Center-tickLen, circle240Center+tickLen, c240CenterGridColor)
	}
}

func c240DrawGauge(img *image.RGBA, g c240GaugeSpec) {
	const (
		radius  = 102
		trackW  = 4.0
		activeW = 7.0
	)

	c240DrawArc(img, c240ArcConfig{
		cx:     circle240Center,
		cy:     g.centerY,
		radius: radius,
		width:  trackW,
		start:  g.startAngle,
		end:    g.endAngle,
		col:    c240TrackColor,
	})

	valueAngle := g.startAngle + (g.endAngle-g.startAngle)*(g.value/100)
	c240DrawArc(img, c240ArcConfig{
		cx:     circle240Center,
		cy:     g.centerY,
		radius: radius,
		width:  activeW,
		start:  g.startAngle,
		end:    valueAngle,
		col:    g.accent,
	})
}

func c240DrawOTText(img *image.RGBA, cfg c240TextConfig) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(cfg.col),
		Face: cfg.face,
	}
	width := d.MeasureString(cfg.text)
	metrics := cfg.face.Metrics()
	ascent := metrics.Ascent
	descent := metrics.Descent

	dotX := fixed.I(cfg.cx) - width/2
	dotY := fixed.I(cfg.cy) + ascent/2 - descent/2

	d.Dot = fixed.Point26_6{X: dotX, Y: dotY}
	d.DrawString(cfg.text)
}

func c240DrawGlowText(img *image.RGBA, cfg c240TextConfig) {
	d := &font.Drawer{Face: cfg.face}
	width := d.MeasureString(cfg.text).Ceil()
	metrics := cfg.face.Metrics()
	ascent := metrics.Ascent.Ceil()
	descent := metrics.Descent.Ceil()
	height := ascent + descent

	margin := 12
	w := width + margin*2
	h := height + margin*2

	mask := image.NewRGBA(image.Rect(0, 0, w, h))
	td := &font.Drawer{
		Dst:  mask,
		Src:  image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 255}),
		Face: cfg.face,
		Dot:  fixed.P(margin, margin+ascent),
	}
	td.DrawString(cfg.text)

	tempMask := image.NewRGBA(mask.Bounds())
	blur := func(src, dst *image.RGBA) {
		for y := 1; y < h-1; y++ {
			for x := 1; x < w-1; x++ {
				var sumA uint32
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						sumA += uint32(src.RGBAAt(x+dx, y+dy).A)
					}
				}
				avgA := uint8(sumA / 9)
				dst.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: avgA})
			}
		}
	}
	blur(mask, tempMask)
	blur(tempMask, mask)

	startX := cfg.cx - width/2 - margin
	startY := cfg.cy - height/2 - margin
	glowColor := cfg.col
	glowColor.A = 80

	for y := range h {
		dstY := startY + y
		if dstY < 0 || dstY >= circle240Size {
			continue
		}
		for x := range w {
			dstX := startX + x
			if dstX < 0 || dstX >= circle240Size {
				continue
			}
			maskAlpha := float64(mask.RGBAAt(x, y).A) / 255.0
			if maskAlpha <= 0.01 {
				continue
			}
			pixelGlow := glowColor
			pixelGlow.A = uint8(float64(glowColor.A) * maskAlpha)
			c240BlendPixel(img, dstX, dstY, pixelGlow)
		}
	}

	c240DrawOTText(img, cfg)
}

func c240DrawArc(img *image.RGBA, cfg c240ArcConfig) {
	steps := max(int(cfg.radius*math.Abs(cfg.end-cfg.start)), 16)
	r := max(int(cfg.width/2), 1)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		angle := cfg.start + t*(cfg.end-cfg.start)
		x := int(cfg.cx + cfg.radius*math.Cos(angle))
		y := int(cfg.cy - cfg.radius*math.Sin(angle))
		c240FillCircle(img, x, y, r, cfg.col)
	}
}

func c240FillCircle(img *image.RGBA, cx, cy, radius int, col color.RGBA) {
	for y := cy - radius - 1; y <= cy+radius+1; y++ {
		if y < 0 || y >= circle240Size {
			continue
		}
		for x := cx - radius - 1; x <= cx+radius+1; x++ {
			if x < 0 || x >= circle240Size {
				continue
			}
			dx := float64(x - cx)
			dy := float64(y - cy)
			dist := math.Sqrt(dx*dx + dy*dy)

			rF := float64(radius)
			if dist >= rF+0.5 {
				continue
			}
			if dist <= rF-0.5 {
				c240BlendPixel(img, x, y, col)
				continue
			}
			alpha := rF + 0.5 - dist
			blendCol := col
			blendCol.A = uint8(float64(col.A) * alpha)
			c240BlendPixel(img, x, y, blendCol)
		}
	}
}

func c240BlendPixel(img *image.RGBA, x, y int, col color.RGBA) {
	i := img.PixOffset(x, y)
	dst := color.RGBA{
		R: img.Pix[i+0],
		G: img.Pix[i+1],
		B: img.Pix[i+2],
		A: img.Pix[i+3],
	}
	out := c240Blend(dst, col)
	img.Pix[i+0] = out.R
	img.Pix[i+1] = out.G
	img.Pix[i+2] = out.B
	img.Pix[i+3] = out.A
}

func c240DrawVLine(img *image.RGBA, x, y0, y1 int, col color.RGBA) {
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	for y := y0; y <= y1; y++ {
		if c240InCircle(x, y) {
			c240BlendPixel(img, x, y, col)
		}
	}
}

func c240DrawHLine(img *image.RGBA, x0, x1, y int, col color.RGBA) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	for x := x0; x <= x1; x++ {
		if c240InCircle(x, y) {
			c240BlendPixel(img, x, y, col)
		}
	}
}

func c240InCircle(x, y int) bool {
	dx := float64(x-circle240Center) + 0.5
	dy := float64(y-circle240Center) + 0.5
	return dx*dx+dy*dy <= float64(120*120)
}
