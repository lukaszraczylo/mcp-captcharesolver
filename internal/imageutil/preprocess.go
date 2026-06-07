package imageutil

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg" // register JPEG decoder
	"image/png"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Upscale decodes a PNG/JPEG image, scales it by factor (clamped to 1..6)
// using high-quality resampling, and returns a PNG re-encode plus "image/png".
func Upscale(data []byte, factor int) ([]byte, string, error) {
	if factor < 1 {
		factor = 1
	}
	if factor > 6 {
		factor = 6
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("upscale: decode: %w", err)
	}

	bounds := src.Bounds()
	newW := bounds.Dx() * factor
	newH := bounds.Dy() * factor
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))

	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, "", fmt.Errorf("upscale: encode: %w", err)
	}
	return buf.Bytes(), "image/png", nil
}

// AnnotateGrid decodes the image and overlays a rows x cols grid: thin lines
// on cell boundaries and the 0-based row-major cell index drawn in the
// top-left of each cell, returning a PNG. Helps a vision model reference cells
// unambiguously. If rows<=0 or cols<=0, returns the original re-encoded PNG.
func AnnotateGrid(data []byte, rows, cols int) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("annotategrid: decode: %w", err)
	}

	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)

	if rows <= 0 || cols <= 0 {
		var buf bytes.Buffer
		if err := png.Encode(&buf, dst); err != nil {
			return nil, fmt.Errorf("annotategrid: encode: %w", err)
		}
		return buf.Bytes(), nil
	}

	w := bounds.Dx()
	h := bounds.Dy()
	lineColor := color.RGBA{R: 220, G: 30, B: 30, A: 160}

	// Draw vertical grid lines.
	for c := 1; c < cols; c++ {
		x := c * w / cols
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			dst.SetRGBA(x, y, lineColor)
			if x+1 < bounds.Max.X {
				dst.SetRGBA(x+1, y, lineColor)
			}
		}
	}

	// Draw horizontal grid lines.
	for r := 1; r < rows; r++ {
		y := r * h / rows
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.SetRGBA(x, y, lineColor)
			if y+1 < bounds.Max.Y {
				dst.SetRGBA(x+1, y+1, lineColor)
			}
		}
	}

	// Draw cell labels.
	face := basicfont.Face7x13
	ascent := face.Ascent   // plain int pixels
	descent := face.Descent // plain int pixels
	advance := face.Advance // plain int pixels per glyph
	labelColor := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	bgColor := color.RGBA{R: 0, G: 0, B: 0, A: 180}

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			idx := r*cols + c
			label := fmt.Sprintf("%d", idx)

			cellX := c * w / cols
			cellY := r * h / rows

			// Label position: 2px padding from cell top-left.
			px := cellX + 2
			py := cellY + ascent + 2

			labelW := len(label) * advance

			// Draw dark background rect behind label.
			bgRect := image.Rect(px-1, py-ascent-1, px+labelW+1, py+descent+1)
			draw.Draw(dst, bgRect, &image.Uniform{bgColor}, image.Point{}, draw.Over)

			// Draw label text.
			d := &font.Drawer{
				Dst:  dst,
				Src:  image.NewUniform(labelColor),
				Face: face,
				Dot:  fixed.P(px, py),
			}
			d.DrawString(label)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, fmt.Errorf("annotategrid: encode: %w", err)
	}
	return buf.Bytes(), nil
}

