package imageutil

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/png" // register PNG decoder for test helpers
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// makePNG creates an in-memory PNG of the given size filled with a solid color.
func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	c := color.RGBA{R: 100, G: 149, B: 237, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("makePNG encode: %v", err)
	}
	return buf.Bytes()
}

func TestDecode(t *testing.T) {
	raw := []byte{0x89, 0x50, 0x4e, 0x47} // PNG magic prefix
	b64 := base64.StdEncoding.EncodeToString(raw)

	t.Run("data uri", func(t *testing.T) {
		data, mt, err := Decode("data:image/png;base64," + b64)
		if err != nil || mt != "image/png" || string(data) != string(raw) {
			t.Fatalf("data=%v mt=%q err=%v", data, mt, err)
		}
	})
	t.Run("bare base64 sniffs png", func(t *testing.T) {
		data, mt, err := Decode(b64)
		if err != nil || mt != "image/png" || string(data) != string(raw) {
			t.Fatalf("data=%v mt=%q err=%v", data, mt, err)
		}
	})
	t.Run("file path", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "x.png")
		if err := os.WriteFile(p, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		data, mt, err := Decode(p)
		if err != nil || mt != "image/png" || string(data) != string(raw) {
			t.Fatalf("data=%v mt=%q err=%v", data, mt, err)
		}
	})
}

func TestUpscale(t *testing.T) {
	const srcW, srcH = 30, 30

	t.Run("factor 3 doubles dims", func(t *testing.T) {
		src := makePNG(t, srcW, srcH)
		out, mt, err := Upscale(src, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mt != "image/png" {
			t.Fatalf("want image/png, got %q", mt)
		}
		img, _, err := image.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("result not decodable: %v", err)
		}
		b := img.Bounds()
		if b.Dx() != srcW*3 || b.Dy() != srcH*3 {
			t.Fatalf("want %dx%d, got %dx%d", srcW*3, srcH*3, b.Dx(), b.Dy())
		}
	})

	t.Run("factor 10 clamps to 6", func(t *testing.T) {
		src := makePNG(t, srcW, srcH)
		out, _, err := Upscale(src, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		img, _, err := image.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("result not decodable: %v", err)
		}
		b := img.Bounds()
		if b.Dx() != srcW*6 || b.Dy() != srcH*6 {
			t.Fatalf("want %dx%d (clamped to 6), got %dx%d", srcW*6, srcH*6, b.Dx(), b.Dy())
		}
	})

	t.Run("factor 1 unchanged dims", func(t *testing.T) {
		src := makePNG(t, srcW, srcH)
		out, mt, err := Upscale(src, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mt != "image/png" {
			t.Fatalf("want image/png, got %q", mt)
		}
		img, _, err := image.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("result not decodable: %v", err)
		}
		b := img.Bounds()
		if b.Dx() != srcW || b.Dy() != srcH {
			t.Fatalf("want %dx%d, got %dx%d", srcW, srcH, b.Dx(), b.Dy())
		}
	})
}

func TestAnnotateGrid(t *testing.T) {
	const size = 90

	t.Run("3x3 grid same dims", func(t *testing.T) {
		src := makePNG(t, size, size)
		out, err := AnnotateGrid(src, 3, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		img, _, err := image.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("result not decodable: %v", err)
		}
		b := img.Bounds()
		if b.Dx() != size || b.Dy() != size {
			t.Fatalf("want %dx%d, got %dx%d", size, size, b.Dx(), b.Dy())
		}
	})

	t.Run("rows=0 returns valid image", func(t *testing.T) {
		src := makePNG(t, size, size)
		out, err := AnnotateGrid(src, 0, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		img, _, err := image.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("result not decodable: %v", err)
		}
		b := img.Bounds()
		if b.Dx() != size || b.Dy() != size {
			t.Fatalf("want %dx%d, got %dx%d", size, size, b.Dx(), b.Dy())
		}
	})
}

func TestGridCentroids(t *testing.T) {
	pts := GridCentroids(300, 300, 3, 3)
	if len(pts) != 9 {
		t.Fatalf("want 9 centroids, got %d", len(pts))
	}
	// index 0 = top-left tile centre at (50,50)
	if pts[0].X != 50 || pts[0].Y != 50 {
		t.Fatalf("centroid[0]=%+v", pts[0])
	}
	// index 4 = centre tile at (150,150)
	if pts[4].X != 150 || pts[4].Y != 150 {
		t.Fatalf("centroid[4]=%+v", pts[4])
	}
}
