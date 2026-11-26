// Command grayicon makes the gray icon of the program from the blue icon.
//
// It reads each PNG frame of the ICO file, sets each pixel to its gray
// value (the luma of Rec. 601) and keeps the alpha. White stays white, thus
// the lock stays white on a gray square. The ICO directory keeps the sizes
// and the order of the frames. The output is the same for the same input.
//
// go generate ./assets runs it:
//
//	go run ./dev/grayicon [-in assets/app.ico] [-out assets/app-idle.ico]
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
)

func main() {
	in := flag.String("in", "app.ico", "the blue icon")
	out := flag.String("out", "app-idle.ico", "the gray icon")
	flag.Parse()
	if err := run(*in, *out); err != nil {
		fmt.Fprintln(os.Stderr, "grayicon:", err)
		os.Exit(1)
	}
}

func run(in, out string) error {
	data, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	g, err := grayICO(data)
	if err != nil {
		return fmt.Errorf("%s: %w", in, err)
	}
	return os.WriteFile(out, g, 0o644)
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

// grayICO gives the gray ICO file. All frames must be PNG.
func grayICO(ico []byte) ([]byte, error) {
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
		if err := enc.Encode(&b, gray(img)); err != nil {
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

// gray gives the image with the gray value of each pixel and the same
// alpha.
func gray(img image.Image) *image.NRGBA {
	r := img.Bounds()
	out := image.NewNRGBA(r)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			// The weights add up to 1000, thus l is 255 or less.
			l := min((299*uint32(c.R)+587*uint32(c.G)+114*uint32(c.B)+500)/1000, 255)
			v := uint8(l)
			out.SetNRGBA(x, y, color.NRGBA{R: v, G: v, B: v, A: c.A})
		}
	}
	return out
}
