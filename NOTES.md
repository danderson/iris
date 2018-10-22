# Random bag of notes

## The eye

From "center" outwards in a photo, we're looking at the pupil (black),
the iris (colored and textured), and the sclera (white stuff with
blood vessels).

Typical eyeball diameter: ~24mm
Typical iris diameter: 10-13mm
Typical pupil diameter: 2-8mm (varies depending on people + light conditions)

Worst case, we can expect the iris-sclera border to be 6.5x further
away from the pupil center than the pupil-iris boundary.

## Algorithms and terms of art

### Iris location:

- [Accurate Iris Localization Using Edge Map Generation and Adaptive Circular Hough Transform for Less Constrained Iris Images](http://www.iaescore.com/journals/index.php/IJECE/article/viewFile/732/489)

### Random other computer vision names I need to remember

- Edge detection
  - Canny edge detector (oldest, not hugely great)
  - Sobel transform (OpenCV says it's great?)
  - Binary thresholding + hole filling (precursor to Canny or Sobel to compute edges - works very well on clean, bright borders)

- Circle detection
  - Circle Hough Transform (beautiful algorithm, but super slow, O(N^3)-ish
  - CHT with Gaussian Pyramids: run CHT on a thumbnail of the original
    image to reduce the impact of N^3, then use that approximate
    location to do a more targeted search in the original image.
    - I implement this, using the thumbnail scale multiplier to pick
      out the boundaries of the search.
  - [Fast Circle Detect using Gradient Pair
    Vectors](http://staff.itee.uq.edu.au/lovell/aprs/dicta2003/pdf/0879.pdf). Possibly
    _very_ fast, but only works on clean bright borders.
