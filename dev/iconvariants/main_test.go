package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
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

// TestUpToDate checks that each variant in assets is the output of
// iconvariants for assets/app.ico. It compares the pixels, not the bytes:
// other Go versions can compress the PNG data in a different way.
func TestUpToDate(t *testing.T) {
	dir := filepath.Join("..", "..", "assets")
	blueICO, err := os.ReadFile(filepath.Join(dir, "app.ico"))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range variants {
		t.Run(v.file, func(t *testing.T) {
			committed, err := os.ReadFile(filepath.Join(dir, v.file))
			if err != nil {
				t.Fatal(err)
			}
			want, err := convertICO(blueICO, v.pixel)
			if err != nil {
				t.Fatal(err)
			}
			compareICO(t, committed, want)
		})
	}
}

func compareICO(t *testing.T, got, want []byte) {
	t.Helper()
	we, wi := decodeFrames(t, want)
	ge, gi := decodeFrames(t, got)
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
	img.SetNRGBA(0, 0, white)
	img.SetNRGBA(1, 0, blue)
	img.SetNRGBA(2, 0, color.NRGBA{R: blue.R, G: blue.G, B: blue.B, A: 40})
	g := convert(img, gray)
	if c := g.NRGBAAt(0, 0); c != white {
		t.Errorf("white: %v", c)
	}
	for x := 1; x <= 2; x++ {
		if c := g.NRGBAAt(x, 0); c.R != c.G || c.G != c.B || c.A != img.NRGBAAt(x, 0).A {
			t.Errorf("blue %d: %v", x, c)
		}
	}
}

// TestTint checks that blue becomes the square color, white becomes the
// lock color, a mix stays a mix and the alpha does not change.
func TestTint(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 1))
	img.SetNRGBA(0, 0, blue)
	img.SetNRGBA(1, 0, white)
	img.SetNRGBA(2, 0, color.NRGBA{R: 146, G: 177, B: 245, A: 255})
	img.SetNRGBA(3, 0, color.NRGBA{R: blue.R, G: blue.G, B: blue.B, A: 40})
	g := convert(img, tint(amber, dark))
	if c := g.NRGBAAt(0, 0); c != amber {
		t.Errorf("blue: %v, want %v", c, amber)
	}
	if c := g.NRGBAAt(1, 0); c != dark {
		t.Errorf("white: %v, want %v", c, dark)
	}
	if c := g.NRGBAAt(2, 0); c.R >= amber.R || c.R <= dark.R || c.G >= amber.G || c.G <= dark.G {
		t.Errorf("mix: %v", c)
	}
	if c := g.NRGBAAt(3, 0); c.A != 40 || c.R != amber.R {
		t.Errorf("alpha: %v", c)
	}
}

// luminance gives the relative luminance of WCAG 2.
func luminance(c color.NRGBA) float64 {
	ch := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.R) + 0.7152*ch(c.G) + 0.0722*ch(c.B)
}

// contrast gives the WCAG 2 contrast ratio of two colors.
func contrast(a, b color.NRGBA) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// TestContrast checks that the lock of each colored icon has a contrast of
// 4.5:1 or more, as the blue icon has.
func TestContrast(t *testing.T) {
	for _, c := range []struct {
		name     string
		sq, lock color.NRGBA
	}{{"blue", blue, white}, {"warn", amber, dark}, {"error", red, white}} {
		if r := contrast(c.sq, c.lock); r < 4.5 {
			t.Errorf("%s: contrast %.2f", c.name, r)
		}
	}
}
