package trayicon

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"testing"
)

func TestUpdatesPreservesSizeAndAddsOrangeBadge(t *testing.T) {
	file, err := os.Open("../../assets/sysentra.png")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	base, _, err := image.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	var source bytes.Buffer
	if err := png.Encode(&source, base); err != nil {
		t.Fatal(err)
	}
	data, err := Updates(source.Bytes(), 12)
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != base.Bounds() {
		t.Fatalf("bounds = %v, want %v", got.Bounds(), base.Bounds())
	}
	size := min(got.Bounds().Dx(), got.Bounds().Dy())
	radius := size / 5
	cx, cy := got.Bounds().Max.X-radius, got.Bounds().Max.Y-radius
	r, g, b, _ := got.At(cx-radius/2, cy+radius*3/4).RGBA()
	if r <= g || g <= b {
		t.Fatalf("badge pixel is not orange: r=%d g=%d b=%d", r, g, b)
	}
}
