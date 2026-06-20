package ui

import (
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// createRoundedImage loads an image, crops it to a square, scales it to `size` using nearest-neighbor,
// and applies a rounded corner mask.
func createRoundedImage(path string, size int, radius float64) fyne.CanvasObject {
	f, err := os.Open(path)
	if err != nil {
		return canvas.NewImageFromFile(path) // fallback
	}
	defer f.Close()

	src, _, err := image.Decode(f)
	if err != nil {
		return canvas.NewImageFromFile(path) // fallback
	}

	srcBounds := src.Bounds()
	srcW, srcH := srcBounds.Dx(), srcBounds.Dy()
	
	minDim := srcW
	if srcH < minDim {
		minDim = srcH
	}
	
	startX := srcBounds.Min.X + (srcW - minDim) / 2
	startY := srcBounds.Min.Y + (srcH - minDim) / 2

	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			srcX := startX + int(float64(x)*float64(minDim)/float64(size))
			srcY := startY + int(float64(y)*float64(minDim)/float64(size))
			
			dx, dy := 0.0, 0.0
			if float64(x) < radius {
				dx = radius - float64(x)
			} else if float64(x) > float64(size)-radius {
				dx = float64(x) - (float64(size) - radius)
			}
			
			if float64(y) < radius {
				dy = radius - float64(y)
			} else if float64(y) > float64(size)-radius {
				dy = float64(y) - (float64(size) - radius)
			}
			
			if dx > 0 && dy > 0 && math.Sqrt(dx*dx+dy*dy) > radius {
				dst.Set(x, y, color.Transparent)
			} else {
				dst.Set(x, y, src.At(srcX, srcY))
			}
		}
	}

	img := canvas.NewImageFromImage(dst)
	img.FillMode = canvas.ImageFillContain
	return img
}
