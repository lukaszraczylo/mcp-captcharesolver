// Package imageutil decodes image/audio inputs (base64, data-URI, file path,
// http(s) URL) and computes grid geometry for tile-based captcha solving.
package imageutil

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var dataURIRe = regexp.MustCompile(`^data:([^;,]+)(;base64)?,(.*)$`)

// Point is a pixel coordinate.
type Point struct {
	X int
	Y int
}

// Decode resolves input (data-URI | bare base64 | file path | http(s) URL) to
// raw bytes plus a best-effort media type sniffed from the content.
func Decode(input string) ([]byte, string, error) {
	input = strings.TrimSpace(input)
	switch {
	case strings.HasPrefix(input, "data:"):
		m := dataURIRe.FindStringSubmatch(input)
		if m == nil {
			return nil, "", fmt.Errorf("malformed data URI")
		}
		var data []byte
		var err error
		if m[2] == ";base64" {
			data, err = base64.StdEncoding.DecodeString(m[3])
		} else {
			data = []byte(m[3])
		}
		if err != nil {
			return nil, "", err
		}
		return data, m[1], nil
	case strings.HasPrefix(input, "http://"), strings.HasPrefix(input, "https://"):
		return fetchURL(input)
	default:
		// File path if it exists; otherwise treat as bare base64.
		if data, err := os.ReadFile(input); err == nil { //nolint:gosec // path is caller-provided automation input
			return data, sniff(data), nil
		}
		data, err := base64.StdEncoding.DecodeString(input)
		if err != nil {
			return nil, "", fmt.Errorf("input is not a data-URI, URL, readable file, or base64: %w", err)
		}
		return data, sniff(data), nil
	}
}

func fetchURL(url string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, "", err
	}
	mt := resp.Header.Get("Content-Type")
	if mt == "" {
		mt = sniff(data)
	}
	return data, mt, nil
}

// sniff returns a media type from magic bytes, defaulting to image/png.
func sniff(b []byte) string {
	switch {
	case len(b) >= 4 && b[0] == 0x89 && b[1] == 0x50 && b[2] == 0x4e && b[3] == 0x47:
		return "image/png"
	case len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff:
		return "image/jpeg"
	case len(b) >= 4 && b[0] == 'R' && b[1] == 'I' && b[2] == 'F' && b[3] == 'F':
		return "audio/wav"
	case len(b) >= 6 && string(b[:3]) == "ID3":
		return "audio/mpeg"
	default:
		return "image/png"
	}
}

// GridCentroids returns the pixel centre of each tile for a rows x cols grid
// laid over a width x height image, in row-major order (index 0 = top-left).
func GridCentroids(width, height, rows, cols int) []Point {
	pts := make([]Point, 0, rows*cols)
	cw := float64(width) / float64(cols)
	ch := float64(height) / float64(rows)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			pts = append(pts, Point{
				X: int(cw*float64(c) + cw/2),
				Y: int(ch*float64(r) + ch/2),
			})
		}
	}
	return pts
}
