// Command iconvariants makes the gray, the yellow and the red icons of the
// program from the blue icon.
//
// It reads each PNG frame of the ICO file and changes the color of each
// pixel. The alpha does not change. The ICO directory keeps the sizes and
// the order of the frames. The output is the same for the same input.
//
//   - app-idle.ico: each pixel gets its gray value (the luma of Rec. 601).
//     White stays white, thus the lock stays white on a gray square.
//   - app-warn.ico: an amber square with a dark lock. A white lock on amber
//     has a contrast of only 2:1, a dark lock has 8:1.
//   - app-error.ico: a red square with a white lock (contrast 4.8:1, as the
//     blue icon has 5.2:1).
//
// The blue icon has two colors: the blue of the square and the white of the
// lock. The other pixels are a mix of the two at the edge of the lock. For
// the yellow and the red icon, the tool finds the part of white in each
// pixel and makes the same mix of the new two colors.
//
// go generate ./assets runs it:
//
//	go run ./dev/iconvariants [-in assets/app.ico] [-dir assets]
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// The colors of the blue icon (app.svg) and of the variants.
var (
	blue  = color.NRGBA{R: 37, G: 99, B: 235, A: 255}
	white = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	// amber is amber-500 of Tailwind; dark is stone-900.
	amber = color.NRGBA{R: 245, G: 158, B: 11, A: 255}
	dark  = color.NRGBA{R: 28, G: 25, B: 23, A: 255}
	// red is red-600 of Tailwind.
	red = color.NRGBA{R: 220, G: 38, B: 38, A: 255}
)

// variant is an output file and the change of its pixels.
type variant struct {
	file  string
	pixel func(color.NRGBA) color.NRGBA
}

// variants are the icons that the tool makes.
var variants = []variant{
	{"app-idle.ico", gray},
	{"app-warn.ico", tint(amber, dark)},
	{"app-error.ico", tint(red, white)},
}

func main() {
	in := flag.String("in", "app.ico", "the blue icon")
	dir := flag.String("dir", ".", "the folder of the output icons")
	flag.Parse()
	if err := run(*in, *dir); err != nil {
		fmt.Fprintln(os.Stderr, "iconvariants:", err)
		os.Exit(1)
	}
}

func run(in, dir string) error {
	data, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	for _, v := range variants {
		out, err := convertICO(data, v.pixel)
		if err != nil {
			return fmt.Errorf("%s: %w", in, err)
		}
		if err := os.WriteFile(filepath.Join(dir, v.file), out, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// The sizes of the ICO header and of a directory entry.
const (
	headerSize = 6
	entrySize  = 16
)

// frames gives the directory entries and the image data of an ICO file.
func frames(ico []byte) (entries [][]byte, images [][]byte, err error) {
	if len(ico) < headerSize || binary.LittleEndian.Uint16(ico[0:2]) != 0 || binary.LittleEndian.Uint16(ico[2:4]) != 1 {
		return nil, nil, errors.New("not an ICO file")
	}
	n := int(binary.LittleEndian.Uint16(ico[4:6]))
	if n == 0 || len(ico) < headerSize+n*entrySize {
		return nil, nil, errors.New("bad ICO directory")
	}
	for i := range n {
		e := ico[headerSize+i*entrySize : headerSize+(i+1)*entrySize]
		size := uint64(binary.LittleEndian.Uint32(e[8:12]))
		off := uint64(binary.LittleEndian.Uint32(e[12:16]))
		if off+size > uint64(len(ico)) {
			return nil, nil, fmt.Errorf("frame %d is outside the file", i)
		}
		entries = append(entries, e)
		images = append(images, ico[off:off+size])
	}
	return entries, images, nil
}

// convertICO gives the ICO file with the pixel change. All frames must be
// PNG.
func convertICO(ico []byte, pixel func(color.NRGBA) color.NRGBA) ([]byte, error) {
	entries, images, err := frames(ico)
	if err != nil {
		return nil, err
	}
	var pngs [][]byte
	for i, data := range images {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("frame %d: %w (only PNG frames)", i, err)
		}
		var b bytes.Buffer
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(&b, convert(img, pixel)); err != nil {
			return nil, err
		}
		pngs = append(pngs, b.Bytes())
	}
	var out bytes.Buffer
	out.Write(ico[:headerSize])
	off := uint64(headerSize + len(entries)*entrySize)
	for i, e := range entries {
		size := uint64(len(pngs[i]))
		if size > math.MaxUint32 || off > math.MaxUint32 || off > math.MaxUint32-size {
			return nil, errors.New("the ICO file is too large")
		}
		ne := bytes.Clone(e)
		binary.LittleEndian.PutUint32(ne[8:12], uint32(size))
		binary.LittleEndian.PutUint32(ne[12:16], uint32(off))
		out.Write(ne)
		off += size
	}
	for _, p := range pngs {
		out.Write(p)
	}
	return out.Bytes(), nil
}

// convert gives the image with the pixel change. The alpha does not change.
func convert(img image.Image, pixel func(color.NRGBA) color.NRGBA) *image.NRGBA {
	r := img.Bounds()
	out := image.NewNRGBA(r)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			n := pixel(c)
			n.A = c.A
			out.SetNRGBA(x, y, n)
		}
	}
	return out
}

// gray gives the gray value of the pixel.
func gray(c color.NRGBA) color.NRGBA {
	// The weights add up to 1000, thus l is 255 or less.
	l := min((299*uint32(c.R)+587*uint32(c.G)+114*uint32(c.B)+500)/1000, 255)
	v := uint8(l)
	return color.NRGBA{R: v, G: v, B: v, A: c.A}
}

// tint gives the change from the blue square and the white lock to the
// square sq and the lock lock.
func tint(sq, lock color.NRGBA) func(color.NRGBA) color.NRGBA {
	return func(c color.NRGBA) color.NRGBA {
		w := whiteness(c)
		mix := func(a, b uint8) uint8 {
			// w is 0 to 1000, thus the value is 0 to 255.
			return uint8(min((uint32(a)*(1000-w)+uint32(b)*w+500)/1000, 255))
		}
		return color.NRGBA{R: mix(sq.R, lock.R), G: mix(sq.G, lock.G), B: mix(sq.B, lock.B), A: c.A}
	}
}

// whiteness gives the part of white in a mix of blue and white, from 0
// (blue) to 1000 (white). It uses the sum of the three channels: the sum
// of blue is 371 and the sum of white is 765.
func whiteness(c color.NRGBA) uint32 {
	lo := uint32(blue.R) + uint32(blue.G) + uint32(blue.B)
	hi := uint32(white.R) + uint32(white.G) + uint32(white.B)
	s := uint32(c.R) + uint32(c.G) + uint32(c.B)
	switch {
	case s <= lo:
		return 0
	case s >= hi:
		return 1000
	}
	return ((s-lo)*1000 + (hi-lo)/2) / (hi - lo)
}
