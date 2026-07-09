//go:build ignore

// Generates assets/icon.png: 256×256, dark rounded square with an accent
// play triangle. Run: go run scripts/gen-icon.go
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
)

func main() {
	const size = 256
	bg := color.NRGBA{0x12, 0x12, 0x14, 0xff}     // near-black
	accent := color.NRGBA{0xfa, 0xfa, 0xfa, 0xff} // opencode-mono white
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	corner := 40.0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Rounded-rect mask
			dx, dy := 0.0, 0.0
			if float64(x) < corner {
				dx = corner - float64(x)
			} else if float64(x) > size-corner {
				dx = float64(x) - (size - corner)
			}
			if float64(y) < corner {
				dy = corner - float64(y)
			} else if float64(y) > size-corner {
				dy = float64(y) - (size - corner)
			}
			if dx*dx+dy*dy > corner*corner {
				continue // transparent corner
			}
			img.Set(x, y, bg)
		}
	}

	// Play triangle: vertices (96,72), (96,184), (184,128)
	for y := 72; y <= 184; y++ {
		half := 1.0 - abs(float64(y)-128)/56.0 // 1 at center, 0 at tips
		right := 96 + int(half*88)
		for x := 96; x <= right; x++ {
			img.Set(x, y, accent)
		}
	}

	f, err := os.Create("assets/icon.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
