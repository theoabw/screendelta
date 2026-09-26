package diff

import (
	"image"

	"github.com/theoabw/screendelta/internal/identity"
)

// The signature sampler reads at most this many pixels for any rectangle, so its cost is bounded and
// does not grow with the size of an element. Sixteen cells with sixteen samples each is enough for a
// coarse appearance and small enough to be free next to the frame's own scan.
const maxSignatureReads = 256

// signatureOf summarises how a rectangle looks, from the frame's luma plane.
//
// The rectangle is divided into a four by four grid and each cell is sampled on a stride, so a large
// element costs the same as a small one. Cells that fall outside the plane are clamped, because a
// region at the frame edge is ordinary and must not produce a signature of nothing.
//
// It returns false when there is nothing to read, which the caller handles by falling back to
// geometry alone rather than recording a signature that means "empty".
func signatureOf(plane []byte, r image.Rectangle, width, height int) (identity.Signature, bool) {
	var signature identity.Signature
	if width <= 0 || height <= 0 || len(plane) < width*height {
		return signature, false
	}

	left := clampInt(r.Min.X, 0, width-1)
	top := clampInt(r.Min.Y, 0, height-1)
	right := clampInt(r.Max.X, left+1, width)
	bottom := clampInt(r.Max.Y, top+1, height)

	rectWidth := right - left
	rectHeight := bottom - top
	// At most sixteen samples per cell in each axis, which bounds the reads at sixteen cells times
	// sixteen samples whatever the rectangle's size.
	stepX := max(1, rectWidth/4)
	stepY := max(1, rectHeight/4)

	for cellY := 0; cellY < identity.SignatureGrid; cellY++ {
		cellTop := top + cellY*rectHeight/identity.SignatureGrid
		cellBottom := top + (cellY+1)*rectHeight/identity.SignatureGrid
		if cellBottom <= cellTop {
			cellBottom = cellTop + 1
		}
		if cellBottom > bottom {
			cellBottom = bottom
		}

		for cellX := 0; cellX < identity.SignatureGrid; cellX++ {
			cellLeft := left + cellX*rectWidth/identity.SignatureGrid
			cellRight := left + (cellX+1)*rectWidth/identity.SignatureGrid
			if cellRight <= cellLeft {
				cellRight = cellLeft + 1
			}
			if cellRight > right {
				cellRight = right
			}

			sum, count := 0, 0
			for y := cellTop; y < cellBottom; y += stepY {
				row := y * width
				for x := cellLeft; x < cellRight; x += stepX {
					sum += int(plane[row+x])
					count++
				}
			}
			if count == 0 {
				// A cell thinner than its stride still contributes the pixel it contains.
				sum = int(plane[cellTop*width+cellLeft])
				count = 1
			}
			signature[cellY*identity.SignatureGrid+cellX] = uint8(sum / count)
		}
	}
	return signature, true
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
