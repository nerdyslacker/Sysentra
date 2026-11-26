package trayicon

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strconv"
)

var digits = [10][5]string{
	{"111", "101", "101", "101", "111"},
	{"010", "110", "010", "010", "111"},
	{"111", "001", "111", "100", "111"},
	{"111", "001", "111", "001", "111"},
	{"101", "101", "111", "001", "001"},
	{"111", "100", "111", "001", "111"},
	{"111", "100", "111", "101", "111"},
	{"111", "001", "010", "010", "010"},
	{"111", "101", "111", "101", "111"},
	{"111", "101", "111", "001", "111"},
}

func Updates(base []byte, count int) ([]byte, error) {
	decoded, err := png.Decode(bytes.NewReader(base))
	if err != nil {
		return nil, fmt.Errorf("decode tray icon: %w", err)
	}
	bounds := decoded.Bounds()
	result := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := decoded.At(x, y).RGBA()
			light := uint8((299*r + 587*g + 114*b) / 1000 >> 8)
			result.SetNRGBA(x, y, color.NRGBA{R: light, G: uint8(uint16(light) * 150 / 255), B: uint8(uint16(light) * 35 / 255), A: uint8(a >> 8)})
		}
	}
	if count < 1 {
		count = 1
	}
	if count > 99 {
		count = 99
	}
	size := min(bounds.Dx(), bounds.Dy())
	radius := size / 5
	cx, cy := bounds.Max.X-radius, bounds.Max.Y-radius
	drawCircle(result, cx, cy, radius+size/80, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	drawCircle(result, cx, cy, radius, color.NRGBA{R: 238, G: 124, B: 25, A: 255})
	drawCount(result, cx, cy, radius, strconv.Itoa(count))
	var output bytes.Buffer
	if err := png.Encode(&output, result); err != nil {
		return nil, fmt.Errorf("encode tray icon: %w", err)
	}
	return output.Bytes(), nil
}

func drawCircle(target *image.NRGBA, cx, cy, radius int, fill color.NRGBA) {
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= radius*radius {
				target.SetNRGBA(x, y, fill)
			}
		}
	}
}

func drawCount(target *image.NRGBA, cx, cy, radius int, text string) {
	scale := max(1, radius/5)
	width := (len(text)*3 + len(text) - 1) * scale
	startX, startY := cx-width/2, cy-(5*scale)/2
	for index, character := range text {
		pattern := digits[character-'0']
		for row, line := range pattern {
			for column, pixel := range line {
				if pixel != '1' {
					continue
				}
				for y := 0; y < scale; y++ {
					for x := 0; x < scale; x++ {
						target.SetNRGBA(startX+(index*4+column)*scale+x, startY+row*scale+y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
					}
				}
			}
		}
	}
}
