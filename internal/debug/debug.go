package debug

import (
	"strconv"

	"gocv.io/x/gocv"
)

func ShowMats(ms ...gocv.Mat) {
	var window *gocv.Window
	for i, m := range ms {
		window = gocv.NewWindow(strconv.Itoa(i))
		defer window.Close()
		window.IMShow(m)
	}
	window.WaitKey(0)
}
