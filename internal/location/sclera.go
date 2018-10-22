package location

import (
	"fmt"
	"image"
	"image/color"

	"gocv.io/x/gocv"

	"go.universe.tf/iris/internal/debug"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

func FindSclera(im gocv.Mat, pupil Circle) {
	// We want to zoom in the image to reduce the search space
	// some. To do this, we rely on some eye facts. On average, the
	// pupil (which we know about) is about 4mm, and the whole iris is
	// at most 13mm. Rouding up and fuzzing a bit, let's say the whole
	// iris is up to 3.5x bigger than the pupil. We'll use that
	// dimension vertically, and double it horizontally (7x). All this
	// is centered on the pupil center, even though the iris center is
	// likely not going to be in the same spot.

	halfWidth := float64(pupil.R) * 3.5
	halfHeight := float64(pupil.R) * 3.5 / 2

	bounding := image.Rectangle{
		Min: image.Point{
			X: max(pupil.X-int(halfWidth), 0),
			Y: max(pupil.Y-int(halfHeight), 0),
		},
		Max: image.Point{
			X: min(pupil.X+int(halfWidth), im.Size()[1]),
			Y: min(pupil.Y+int(halfHeight), im.Size()[0]),
		},
	}

	im = im.Region(bounding)

	norm := gocv.NewMat()
	gocv.Normalize(im, &norm, 255.0, 0.0, gocv.NormMinMax)

	// Apply a median blur, which destroys fine detail but preserves
	// edge structure. AKA removes eyelashes.
	median := gocv.NewMat()
	gocv.MedianBlur(norm, &median, 9)

	// Sobel gradient in the X direction, which ends up highlighting
	// vertical-ish edges.
	dx := gocv.NewMat()
	gocv.Sobel(median, &dx, gocv.MatTypeCV16S, 1, 0, 3, 1, 0, gocv.BorderDefault)
	gocv.ConvertScaleAbs(dx, &dx, 1, 0)

	widerPupil := int(float64(pupil.R) * 1.1)
	wipeout := image.Rectangle{
		Min: image.Point{
			X: max(pupil.X-widerPupil, 0),
			Y: 0,
		},
		Max: image.Point{
			X: min(pupil.X+widerPupil, dx.Size()[1]),
			Y: dx.Size()[0],
		},
	}

	wipeoutDx := dx.Clone()
	gocv.Rectangle(&wipeoutDx, wipeout, color.RGBA{0, 0, 0, 255}, -1)

	gocv.Normalize(wipeoutDx, &wipeoutDx, 255.0, 0.0, gocv.NormMinMax)

	small, mult := shrink(wipeoutDx, 120)

	fmt.Println(mult)
	debug.ShowMats(im, median, dx, wipeoutDx, small)
}
