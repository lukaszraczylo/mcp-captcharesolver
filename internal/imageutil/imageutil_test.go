package imageutil

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

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
