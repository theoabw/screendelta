// Package diff compares two frames and reports what changed between them.
//
// The comparison is deliberately plain: luma differences over a tile grid, connected
// tiles merged back into rectangles. Nothing here is learned and nothing is tuned to a
// particular application, because the specification asks for a change to be located,
// not interpreted.
//
// One Differ belongs to one stream. It keeps the previous frame's region geometry so it
// can say whether a region is new, changed, moved or gone, and it reuses its scratch
// storage so a long stream does not allocate.
package diff

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/fielderr"
	"github.com/theoabw/screendelta/internal/frame"
)

// growthMargin is how far a reported region is expanded beyond the pixels that changed, so
// the bounds cover the element that produced the change rather than its inside. It is the one
// place where the engine reports more than it measured, and the accuracy measurement scores
// against the changed pixels themselves, so the margin has to stay small enough that a padded
// region still overlaps its own change by more than half. Measured: a sixteen pixel element
// keeps an intersection over union of 0.64, a sixty pixel one 0.86.
const growthMargin = 2

// luma weights are the integer BT.601 coefficients, used instead of floating point so the
// same input produces the same numbers everywhere.
const (
	lumaRed   = 299
	lumaGreen = 587
	lumaBlue  = 114
	lumaScale = 1000
)

// Differ compares consecutive frames of one stream.
type Differ struct {
	previous []regionState
	mask     []byte
	stack    []int
	grid     []uint8

	hasPrevious  bool
	nextIdentity uint64
}

// regionState is what the differ needs to remember about the previous frame: where the
// elements were and how large they were.
type regionState struct {
	bounds delta.Bounds
	width  int
	height int
}

// New creates a differ for one stream.
func New() *Differ {
	return &Differ{}
}

// Compare returns the regions that differ between two frames.
func (d *Differ) Compare(previous, current frame.Frame, cfg config.Config) ([]delta.Region, []delta.Condition, error) {
	if previous.Width != current.Width || previous.Height != current.Height {
		return nil, nil, &fielderr.Error{
			Op:      "diff.Compare",
			Subject: "frame",
			Field:   "dimensions",
			Problem: "must match: the engine reports a viewport change instead of calling for a comparison",
		}
	}

	// A configuration that asks for nothing at all is honoured before any work is done, and
	// the remembered geometry is cleared so a later change of configuration cannot produce
	// removals for regions the caller asked never to hear about.
	if cfg.SuppressesAllRegions() {
		d.previous = d.previous[:0]
		d.hasPrevious = true
		return []delta.Region{}, nil, nil
	}

	components := d.components(previous, current, cfg)
	candidates := d.filter(components, current, cfg)
	regions := d.classify(candidates, previous, current, cfg)

	return regions, nil, nil
}

// component is a connected group of changed pixels.
type component struct {
	left, top, right, bottom int
	magnitude                float64
}

// components finds the changed pixels and groups them into connected areas.
//
// A pixel counts as changed when its luma differs by more than the configured noise floor, and
// areas are eight-connected, because a one pixel diagonal join between two parts of the same
// glyph or icon is common. The whole frame is scanned once; an earlier version gated the scan
// on a sixteen pixel tile grid, which was cheaper to merge but cut a genuine change at a tile
// boundary whenever a tile's mean fell under the floor, reporting one change as two overlapping
// regions.
func (d *Differ) components(previous, current frame.Frame, cfg config.Config) []component {
	width, height := current.Width, current.Height
	needed := width * height
	if needed <= 0 {
		return nil
	}
	if cap(d.mask) < needed {
		d.mask = make([]byte, needed)
	}
	mask := d.mask[:needed]

	threshold := cfg.NoiseFloor * 255
	any := false
	for y := 0; y < height; y++ {
		rowOffset := y * width * 4
		for x := 0; x < width; x++ {
			offset := rowOffset + x*4
			difference := absInt(luma(current.Pixels[offset:]) - luma(previous.Pixels[offset:]))
			if float64(difference) <= threshold {
				mask[y*width+x] = 0
				continue
			}
			mask[y*width+x] = 1
			any = true
		}
	}
	if !any {
		return nil
	}

	var components []component
	for start := 0; start < needed; start++ {
		if mask[start] != 1 {
			continue
		}
		d.stack = append(d.stack[:0], start)
		mask[start] = 2

		boxLeft, boxTop := width, height
		boxRight, boxBottom := 0, 0
		sum, count := 0, 0

		for len(d.stack) > 0 {
			index := d.stack[len(d.stack)-1]
			d.stack = d.stack[:len(d.stack)-1]
			x, y := index%width, index/width

			if x < boxLeft {
				boxLeft = x
			}
			if y < boxTop {
				boxTop = y
			}
			if x+1 > boxRight {
				boxRight = x + 1
			}
			if y+1 > boxBottom {
				boxBottom = y + 1
			}
			offset := y*width*4 + x*4
			sum += absInt(luma(current.Pixels[offset:]) - luma(previous.Pixels[offset:]))
			count++

			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					neighbourX, neighbourY := x+dx, y+dy
					if neighbourX < 0 || neighbourY < 0 || neighbourX >= width || neighbourY >= height {
						continue
					}
					neighbour := neighbourY*width + neighbourX
					if mask[neighbour] != 1 {
						continue
					}
					mask[neighbour] = 2
					d.stack = append(d.stack, neighbour)
				}
			}
		}

		if count == 0 {
			continue
		}
		components = append(components, component{
			left:      boxLeft,
			top:       boxTop,
			right:     boxRight,
			bottom:    boxBottom,
			magnitude: float64(sum) / float64(count) / 255,
		})
	}

	// Deterministic order before any matching: top edge, then left edge.
	sortComponents(components)
	return components
}

// Fingerprint summarises a frame as a grid of quantised luma cells.
func (d *Differ) Fingerprint(current frame.Frame, cfg config.Config) (delta.Fingerprint, error) {
	size := cfg.Fingerprint.GridSize
	if size < 8 || size > 256 {
		return delta.Fingerprint{}, &fielderr.Error{
			Op:      "diff.Fingerprint",
			Subject: "config",
			Field:   "fingerprint.gridSize",
			Problem: "must be between 8 and 256",
		}
	}

	needed := size * size
	if len(d.grid) != needed {
		d.grid = make([]uint8, needed)
	}

	for cellY := 0; cellY < size; cellY++ {
		for cellX := 0; cellX < size; cellX++ {
			left := cellX * current.Width / size
			right := max((cellX+1)*current.Width/size, left+1)
			top := cellY * current.Height / size
			bottom := max((cellY+1)*current.Height/size, top+1)

			sum, count := 0, 0
			for y := top; y < min(bottom, current.Height); y++ {
				rowOffset := y * current.Width * 4
				for x := left; x < min(right, current.Width); x++ {
					sum += luma(current.Pixels[rowOffset+x*4:])
					count++
				}
			}
			if count == 0 {
				d.grid[cellY*size+cellX] = 0
				continue
			}
			d.grid[cellY*size+cellX] = uint8(sum / count)
		}
	}

	cells := make([]int, needed)
	for index, value := range d.grid {
		cells[index] = int(value)
	}

	sum := sha256.Sum256(cellsToBytes(d.grid))
	return delta.Fingerprint{
		Algorithm:  "grid-luma-1",
		GridSize:   size,
		Cells:      cells,
		StrictHash: hex.EncodeToString(sum[:]),
	}, nil
}

func cellsToBytes(cells []uint8) []byte {
	if len(cells) == 0 {
		return nil
	}
	buffer := make([]byte, len(cells))
	copy(buffer, cells)
	return buffer
}

// luma converts one RGBA pixel to a single channel value.
func luma(pixel []byte) int {
	return (lumaRed*int(pixel[0]) + lumaGreen*int(pixel[1]) + lumaBlue*int(pixel[2])) / lumaScale
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
