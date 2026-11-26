package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// decodeFrames gives the decoded PNG frames of an ICO file.
func decodeFrames(t *testing.T, ico []byte) ([][]byte, []image.Image) {
	t.Helper()
	entries, images, err := frames(ico)
	if err != nil {
		t.Fatal(err)
	}
	var out []image.Image
	for i, data := range images {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		out = append(out, img)
	}
	return entries, out
}

// TestUpToDate checks that assets/app-idle.ico is the output of grayicon
// for assets/app.ico. It compares the pixels, not the bytes: other Go
// versions can compress the PNG data in a different way.
func TestUpToDate(t *testing.T) {
	dir := filepath.Join("..", "..", "assets")
	blue, err := os.ReadFile(filepath.Join(dir, "app.ico"))
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(filepath.Join(dir, "app-idle.ico"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := grayICO(blue)
	if err != nil {
		t.Fatal(err)
	}
	we, wi := decodeFrames(t, want)
	ge, gi := decodeFrames(t, committed)
	if len(ge) != len(we) {
		t.Fatalf("%d frames, want %d: run go generate ./assets", len(ge), len(we))
	}
	for i := range we {
		// Width, height, colors, planes and bit count: the first 8 bytes.
		if !bytes.Equal(ge[i][:8], we[i][:8]) {
			t.Errorf("frame %d: entry %x, want %x: run go generate ./assets", i, ge[i][:8], we[i][:8])
			continue
		}
		g, w := gi[i], wi[i]
		if g.Bounds() != w.Bounds() {
			t.Errorf("frame %d: bounds %v, want %v: run go generate ./assets", i, g.Bounds(), w.Bounds())
			continue
		}
		r := w.Bounds()
	pixels:
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				if color.NRGBAModel.Convert(g.At(x, y)) != color.NRGBAModel.Convert(w.At(x, y)) {
					t.Errorf("frame %d: pixel (%d, %d) is not the same: run go generate ./assets", i, x, y)
					break pixels
				}
			}
		}
	}
}

// TestGray checks that white stays white, that blue becomes gray and that
// the alpha does not change.
func TestGray(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{R: 37, G: 99, B: 235, A: 255})
	img.SetNRGBA(2, 0, color.NRGBA{R: 37, G: 99, B: 235, A: 40})
	g := gray(img)
	if c := g.NRGBAAt(0, 0); c != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Errorf("white: %v", c)
	}
	for x := 1; x <= 2; x++ {
		if c := g.NRGBAAt(x, 0); c.R != c.G || c.G != c.B || c.A != img.NRGBAAt(x, 0).A {
			t.Errorf("blue %d: %v", x, c)
		}
	}
}
