package main

import (
	"fmt"
	"image/color"
	"os"
	"time"

	"gocv.io/x/gocv"

	"go.universe.tf/iris/internal/debug"
	"go.universe.tf/iris/internal/location"
)

func main() {
	im := gocv.IMRead(os.Args[1], gocv.IMReadGrayScale)
	defer im.Close()

	st := time.Now()
	appx, p := location.FindPupil(im)
	fmt.Println("total:", time.Since(st))

	gocv.CvtColor(im, &im, gocv.ColorGrayToBGR)
	im2 := im.Clone()
	gocv.Circle(&im, appx.Point, appx.R, color.RGBA{255, 0, 0, 255}, 2)
	gocv.Circle(&im2, p.Point, p.R, color.RGBA{0, 255, 0, 255}, 2)

	debug.ShowMats(im, im2)
}
