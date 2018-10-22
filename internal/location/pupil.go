package location

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"gocv.io/x/gocv"
)

type Circle struct {
	image.Point
	R int
}

func (p Circle) String() string {
	return fmt.Sprintf("(%d,%d,%d)", p.X, p.Y, p.R)
}

// FindPupil locates a single pupil in the provided image, and returns it.
func FindPupil(im gocv.Mat) (Circle, Circle) {
	// This is the algorithm from "Accurate Iris Localization Using
	// Edge Map Generation and Adaptive Circular Hough Transform for
	// Less Constrained Iris Images", by Kumar, Asati and Gupta.

	// For our edgeMap1, we're assuming that the pupil will be one of
	// the darkest things in the image. Poor quality images can have a
	// "brightness floor" that's too high. To compensate for that, we
	// renormalize the image so that the pupil *should* be found in
	// the 10% darkest pixels of the image.
	//
	// Stretching pixel values also helps edgeMap2's edges be a bit
	// more crisp, which is why we do it as a common step before both.
	norm := gocv.NewMat()
	gocv.Normalize(im, &norm, 255.0, 0.0, gocv.NormMinMax)

	// Edge detection just works better if you filter out
	// high-frequency noise. A 5x5 Gaussian blur is traditional.
	blur := gocv.NewMat()
	gocv.GaussianBlur(norm, &blur, image.Point{5, 5}, 0, 0, gocv.BorderDefault)

	// Compute our two edge maps. See the functions for details of
	// what they do, but the short version is that they should both
	// include the pupil edge, and *different* kinds of unrelated
	// noise.
	//
	// em1 will return false edges around non-pupil dark patches in
	// the image, whereas em2 will return false edges around
	// reflections, eyelids and eyelashes.
	em1 := edgeMap1(blur)
	em2 := edgeMap2(blur)

	// We now have two edge maps, which mostly only have the pupil
	// edge in common. ANDing them together removes everything else,
	// and leaves us with (hopefully) just a nice clean circle to
	// apply circle detection on!
	edge := gocv.NewMat()
	gocv.BitwiseAnd(em1, em2, &edge)
	return findBestCircle(edge)
}

// circlePoints lists (x,y) coordinates for pixels on a circle of a
// given radius.
var circlePoints map[int][]image.Point

func init() {
	circlePoints = map[int][]image.Point{}
	for r := 5; r < 15; r++ {
		circlePoints[r] = calcCirclePoints(r)
	}
}

// circlePoints computes the (x,y) coordinates for pixels on a circle
// of a given radius.
func calcCirclePoints(r int) []image.Point {
	var (
		ret  []image.Point
		last image.Point
	)
	for i := 0; i < 360; i++ {
		x := int(float64(r) * math.Cos(float64(i)*math.Pi/180.0))
		y := int(float64(r) * math.Sin(float64(i)*math.Pi/180.0))
		p := image.Point{x, y}
		if p != last {
			ret = append(ret, p)
			last = p
		}
	}
	return ret
}

// findBestCircle finds the single best defined circle in im.
//
// Input pixels should be zero for non-candidate points, any other
// value is assumed to be a point on the circle we're looking for.
func findBestCircle(im gocv.Mat) (Circle, Circle) {
	st := time.Now()
	// This algorithm is very expensive in the number of pixels
	// processed. To work around this, we first run it on a small
	// version of the image to get an approximate center and
	// radius. Then we rerun on the larger image with a much smaller
	// search space, to refine things.
	small, mult := shrink(im, 60)

	// We don't know the radius of the circle we're looking for, so
	// we're going to iterate through a set of plausible sizes,
	// looking for the radius that gives us the strongest match.
	//
	// Keep track of the best circle we've found so far.
	var (
		winner      Circle
		winnerVotes int16
	)

	for r, circlePoints := range circlePoints {
		// The circle Hough transform uses a "voting matrix". We make
		// a variety of guesses as to where the circle center might
		// be, and this matrix tracks the number of "votes" that each
		// pixel gets for being the center.
		votes := gocv.NewMatWithSize(small.Size()[0], small.Size()[1], gocv.MatTypeCV16S)

		for row := 0; row < small.Size()[0]; row++ {
			for col := 0; col < small.Size()[1]; col++ {
				// Skip black pixels.
				if small.GetUCharAt(row, col) == 0 {
					continue
				}

				// We think this pixel might be on our circle. If
				// true, its center would be somewhere on a circle of
				// radius r and centered here. Add a vote to each of
				// those locations in the voting matrix.
				for _, cp := range circlePoints {
					// (a, b) is our candidate centerpoint.
					// Annoyingly, image.Point's coordinates are
					// backwards from OpenCVs: point.X is the column,
					// point.Y is the row. That's why we seem to be
					// summing backwards here.
					a, b := row+cp.Y, col+cp.X

					// Check that (a, b) is in-bounds.
					if a >= small.Size()[0] ||
						a < 0 ||
						b >= small.Size()[1] ||
						b < 0 {
						continue
					}

					// One vote for (a,b) as the center.
					votes.SetShortAt(a, b, votes.GetShortAt(a, b)+1)
				}
			}
		}

		// The voting matrix is now complete. Time to count, and see
		// who won.
		for row := 0; row < small.Size()[0]; row++ {
			for col := 0; col < small.Size()[1]; col++ {
				if votes.GetShortAt(row, col) > winnerVotes {
					// We have a (provisional) winner! Record its
					// properties. Again, image.Point and gocv
					// coordinates are reversed from each other,
					// grumble.
					winner.X = col
					winner.Y = row
					winner.R = r
					winnerVotes = votes.GetShortAt(row, col)
				}
			}
		}
	}

	fmt.Println(time.Since(st))
	st = time.Now()

	// Having crunched through all possible radii, `winner` is now the
	// circle that had the most number of supportive pixels,
	// regardless of radius. We're done!
	//
	// Well, not quite. We've just done all this on a small version of
	// our image. We would just multiply out the coordinates and
	// radius and it'd be reasonably good, but we could be up to
	// `mult` pixels out, on both center and radius.
	//
	// so, let's do another round of hough circle detection, this time
	// on the full image... But now, we'll only look at radii and
	// centers that are "near" our approximation, to cut down
	// drastically on memory and CPU cost.

	approximate := Circle{
		Point: image.Point{
			X: int(float64(winner.X) * mult),
			Y: int(float64(winner.Y) * mult),
		},
		R: int(float64(winner.R) * mult),
	}

	// `mult` tells us how much bigger the original image was. Divide
	// by two, round up, that gives us the plus/minus count on center
	// and radius offsets.
	//
	// If `mult` was one, we didn't resize the image at all, so our
	// approximate guess is actually just the correct guess, and we
	// can return that.
	if mult == 1 {
		return approximate, approximate
	}
	uncertainty := int(math.Ceil(mult / 2))

	// We now know a cube of (uncertainty, uncertainty, uncertainty)
	// for where the circle (x, y, r) is. That's a pretty small grid
	// even on a very large image, so we can just search it
	// exhaustively, and pick the position that results in the most
	// non-zero pixels on the resulting circle.
	winner = Circle{}
	winnerVotes = 0

	for r := approximate.R - uncertainty; r < approximate.R+uncertainty; r++ {
		circlePoints := calcCirclePoints(r)
		for row := approximate.Y - uncertainty; row <= approximate.Y+uncertainty; row++ {
			for col := approximate.X - uncertainty; col <= approximate.X+uncertainty; col++ {
				var votes int16
				for _, cp := range circlePoints {
					a, b := row+cp.Y, col+cp.X
					if im.GetUCharAt(a, b) != 0 {
						votes++
					}
				}
				if votes > winnerVotes {
					winner.X = col
					winner.Y = row
					winner.R = r
					winnerVotes = votes
				}
			}
		}
	}

	fmt.Println(time.Since(st))

	return approximate, winner
}

// edgeMap1 computes an edge map using thresholding and hole filling.
func edgeMap1(src gocv.Mat) gocv.Mat {
	// Make the darkest 10% of pixels perfectly black, and the rest
	// perfectly white.
	thresh := gocv.NewMat()
	gocv.Threshold(src, &thresh, 25, 255, gocv.ThresholdBinary)

	// Color in white blotches, so that we have fewer false
	// edges. This is particularly important for pupil decection,
	// because it's very common to have the camera's light array
	// reflected in the center of the pupil, which creates a false
	// circle. Hole filling completely fixes that.
	filled := fillHoles(thresh)

	// "Open" the image. Opening is a morphological operation where
	// you "thin" objects, and then "fatten" them back up. For most of
	// the image, this is a no-op, but if there's little dots of
	// noise, those dots get wiped out during thinning. So effectively
	// it's a noise-reduction step.
	opened := gocv.NewMat()
	gocv.MorphologyEx(filled, &opened, gocv.MorphOpen, gocv.GetStructuringElement(gocv.MorphEllipse, image.Point{7, 7}))

	// Finally, detect edges and get (hopefully) a crisp circle where
	// the pupil boundary lies.
	edge := sobelEdge(opened)

	return edge
}

// edgeMap2 computes a naive edge map for the image.
func edgeMap2(src gocv.Mat) gocv.Mat {
	// Just run an edge detector over the entire image. There will be
	// a plethora of false edges here (meaning edges that aren't our
	// pupil).
	return sobelEdge(src)
}

// sobelEdge detects edges in src using Sobel filters.
func sobelEdge(src gocv.Mat) gocv.Mat {
	// Calculate pixel gradient in the horizontal, using a 3x3 Sobel kernel.
	dx := gocv.NewMat()
	gocv.Sobel(src, &dx, gocv.MatTypeCV16S, 1, 0, 3, 1, 0, gocv.BorderDefault)
	// Our output is signed 16-bit pixels. This takes absolute values
	// and smashes them back into 8-bit pixels. The absolute value is
	// important here because it makes dark-to-bright and
	// bright-to-dark edges equally "important" in our output.
	gocv.ConvertScaleAbs(dx, &dx, 1, 0)

	// Same again, in the vertical direction.
	dy := gocv.NewMat()
	gocv.Sobel(src, &dy, gocv.MatTypeCV16S, 0, 1, 3, 1, 0, gocv.BorderDefault)
	gocv.ConvertScaleAbs(dy, &dy, 1, 0)

	// We now have X and Y absolute values for gradients. To combine
	// them, in theory we want the magnitude, but that involves icky
	// expensive square roots. Instead, we approximate the magnitude
	// by just averaging the X and Y magnitudes together.
	ret := gocv.NewMat()
	gocv.AddWeighted(dx, 0.5, dy, 0.5, 0, &ret)

	// Finally, normalize to make the edges shine more.
	// TODO: do I really need this?
	gocv.Normalize(ret, &ret, 255, 0, gocv.NormMinMax)
	return ret
}

// fillHoles fills in white blobs that aren't connected to an
// edge. The input is assumed to be a binary black-and-white image.
func fillHoles(src gocv.Mat) gocv.Mat {
	ret := src.Clone()

	// Create a white border around the edge, so that a flood on (0,0)
	// reaches all white areas reachable from any edge pixel.
	for row := 0; row < ret.Size()[0]; row++ {
		ret.SetUCharAt(row, 0, 255)
		ret.SetUCharAt(row, ret.Size()[1]-1, 255)
	}
	for col := 1; col < ret.Size()[1]-1; col++ {
		ret.SetUCharAt(0, col, 255)
		ret.SetUCharAt(ret.Size()[0]-1, col, 255)
	}

	// Flood white-to-black from (0,0). This will make everything
	// black, *except* the bits we're trying to fill in.
	gocv.FloodFill(&ret, image.Point{0, 0}, color.RGBA{0, 0, 0, 255})

	// Invert, so now the only black is the things we're filling in.
	gocv.BitwiseNot(ret, &ret)

	// Finally, AND the original input and this mask together, which
	// zeroes out the unconnected blobs!
	gocv.BitwiseAnd(src, ret, &ret)
	return ret
}

// shrink resizes im down so that its height is at most
// maxHeight. Returns the shrunken image, as well as the factor you'd
// need to multiply by to get back to the original image.
func shrink(im gocv.Mat, maxHeight int) (gocv.Mat, float64) {
	ret := im.Clone()
	mult := float64(1)

	sz := float64(im.Size()[0])
	tgtSz := float64(maxHeight)
	if sz > tgtSz {
		gocv.Resize(ret, &ret, image.Point{}, tgtSz/sz, tgtSz/sz, gocv.InterpolationDefault)
		mult = sz / tgtSz
	}
	return ret, mult
}
