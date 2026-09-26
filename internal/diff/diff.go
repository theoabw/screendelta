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
	"bytes"
	"crypto/sha256"
	"encoding/hex"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/fielderr"
	"github.com/theoabw/screendelta/internal/frame"
	"github.com/theoabw/screendelta/internal/identity"
)

// wordBytes is how much of a row the comparison skips at a time once it knows the row changed.
const wordBytes = 8

// growthMargin is how far a reported region is expanded beyond the pixels that changed, so
// the bounds cover the element that produced the change rather than its inside. It is the one
// place where the engine reports more than it measured, and the accuracy measurement scores
// against the changed pixels themselves, so the margin has to stay small enough that a padded
// region still overlaps its own change by more than half. Measured: a sixteen pixel element
// keeps an intersection over union of 0.64, a sixty pixel one 0.86.
const growthMargin = 2

// luma weights are the integer BT.601 coefficients, used instead of floating point so the same
// input produces the same numbers everywhere, and pre-multiplied into tables so converting a
// pixel costs three lookups rather than three multiplications. Converting every pixel of every
// frame is the largest single cost in the engine, which is why it is done once per frame and
// reused by both the comparison and the fingerprint.
const (
	lumaRed   = 299
	lumaGreen = 587
	lumaBlue  = 114
	lumaScale = 1000
)

var (
	lumaRedTable  [256]uint32
	lumaGreenBlue [65536]uint32
)

func init() {
	for value := 0; value < 256; value++ {
		lumaRedTable[value] = uint32(lumaRed * value)
	}
	// Indexed by green in the high byte and blue in the low byte: one lookup covers both.
	for green := 0; green < 256; green++ {
		for blue := 0; blue < 256; blue++ {
			lumaGreenBlue[green<<8|blue] = uint32(lumaGreen*green + lumaBlue*blue)
		}
	}
}

// lumaOfPixel converts one rgba8 pixel to the single channel the engine compares.

// Differ compares consecutive frames of one stream.
// Differ holds the two most recent luma planes. The engine asks for a frame's fingerprint and
// then for the comparison against the frame before it, and both need the same conversion:
// converting twice per frame was most of the engine's cost.
//
// A frame is identified by its sequence, which the engine requires to increase, so a cached
// plane cannot be confused with a different frame.
type Differ struct {
	// liveElements is the snapshot of the tracked elements the classifier works from, refreshed at the
	// start of every comparison. It is scratch rather than state: the identity map holds the truth.
	liveElements []identity.LiveElement
	mask         []byte
	stack        []int
	grid         []uint8

	newest    []byte
	newestSeq uint64
	older     []byte
	olderSeq  uint64

	changedRows  []int
	gridSequence uint64

	identities       *identity.Map
	identityOccluded int
	identityMotion   int

	hasPrevious    bool
	previousWidth  int
	previousHeight int
}

// regionState is what the differ needs to remember about the previous frame: where the
// elements were and how large they were.
type regionState struct {
	bounds delta.Bounds
	width  int
	height int
}

// New creates a differ for one stream.
//
// The identity map exists from construction rather than being created on the first comparison, so
// there is no state in which a differ has none. The matching rules come from the configuration and
// are applied before the first comparison; a differ used without a configuration matches only
// rectangles in the same place, which is the strictest useful default.
func New() *Differ {
	return &Differ{identities: identity.New(0, 0)}
}

// Reset ends every comparison the differ remembers: the previous geometry and every tracked
// identity. It is what the engine calls when it reports a viewport change, because bounds are
// normalised and a rectangle tracked at one frame size says nothing at another.
func (d *Differ) Reset() {
	d.hasPrevious = false
	d.liveElements = d.liveElements[:0]
	d.identities.RetireAll()
}

// plane returns the luma plane for a frame, converting it only if it is not already held.
//
// The two slots hold the two most recent frames. A miss on a frame at least as recent as the
// newest rotates the pair and reuses the buffer that just became the oldest, so a stream of
// constant geometry allocates its planes once.
func (d *Differ) plane(f frame.Frame, gridSize int) []byte {
	if d.newest != nil && d.newestSeq == f.Sequence {
		return d.newest
	}
	if d.older != nil && d.olderSeq == f.Sequence {
		return d.older
	}

	needed := f.Width * f.Height
	if needed <= 0 {
		return nil
	}

	if f.Sequence >= d.newestSeq {
		buffer := d.older
		if cap(buffer) < needed {
			buffer = make([]byte, needed)
		}
		buffer = buffer[:needed]
		d.older, d.olderSeq = d.newest, d.newestSeq
		d.newest, d.newestSeq = buffer, f.Sequence
		d.fillPlane(buffer, f, gridSize)
		return buffer
	}

	// A frame older than both slots: compute it into the older slot, the least recently used.
	buffer := d.older
	if cap(buffer) < needed {
		buffer = make([]byte, needed)
	}
	buffer = buffer[:needed]
	d.older, d.olderSeq = buffer, f.Sequence
	d.fillPlane(buffer, f, gridSize)
	return buffer
}

func (d *Differ) prepareCells(size int) {
	needed := size * size
	if len(d.grid) != needed {
		d.grid = make([]uint8, needed)
	}
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
		d.hasPrevious = true
		// Nothing is reported, and nothing changed as far as the engine is concerned, so the tracked
		// elements are refreshed rather than aged. Leaving the map untouched instead would freeze it:
		// after a suppressed stretch every element would look as old as it was when suppression
		// started.
		d.identities.EndFrame(current.Sequence, nil, current.Width, current.Height)
		return []delta.Region{}, nil, nil
	}

	// A geometry change invalidates every remembered rectangle: the bounds are normalised, so an
	// element tracked at one frame size means nothing at another. The engine reports a viewport
	// change and starts the stream again; the differ clears its state so it cannot match across it.
	if d.hasPrevious && (d.previousWidth != current.Width || d.previousHeight != current.Height) {
		// Every tracked element ends at a viewport change, and the identifier counter keeps rising:
		// rebuilding the table would restart the sequence and hand a consumer a number that used to
		// mean something else.
		d.identities.RetireAll()
	}
	if d.identityOccluded != cfg.OcclusionFrames || d.identityMotion != cfg.MotionTolerancePixels {
		// A rule change says nothing about where anything is, so the tracked elements are kept and
		// only the rules move.
		d.identities.SetRules(cfg.OcclusionFrames, cfg.MotionTolerancePixels)
		d.identityOccluded, d.identityMotion = cfg.OcclusionFrames, cfg.MotionTolerancePixels
	}

	components := d.components(previous, current, cfg)
	candidates := d.filter(components, current, cfg)
	regions := d.classify(candidates, previous, current, cfg)

	// This frame becomes the baseline for the next one. It used to be recorded by the second memory of
	// geometry that the identity map replaced, which is why the differ has to say it here.
	d.hasPrevious = true
	d.previousWidth, d.previousHeight = current.Width, current.Height

	return regions, nil, nil
}

// component is a connected group of changed pixels.
type component struct {
	left, top, right, bottom int
	magnitude                float64
}

func (d *Differ) components(previous, current frame.Frame, cfg config.Config) []component {
	width, height := current.Width, current.Height
	needed := width * height
	if needed <= 0 {
		return nil
	}

	previousPlane := d.plane(previous, cfg.Fingerprint.GridSize)
	currentPlane := d.plane(current, cfg.Fingerprint.GridSize)
	if previousPlane == nil || currentPlane == nil {
		return nil
	}

	if cap(d.mask) < needed {
		d.mask = make([]byte, needed)
	}
	mask := d.mask[:needed]

	threshold := int(cfg.NoiseFloor*255 + 0.5)
	d.changedRows = d.changedRows[:0]

	for y := 0; y < height; y++ {
		row := y * width
		previousRow := previousPlane[row : row+width]
		currentRow := currentPlane[row : row+width]
		maskRow := mask[row : row+width]

		// Identical rows are the common case in a desktop frame, and comparing a row whole costs a
		// fraction of looking at every pixel in it.
		if bytes.Equal(previousRow, currentRow) {
			clear(maskRow)
			continue
		}

		clear(maskRow)
		d.changedRows = append(d.changedRows, y)
		for x := 0; x < width; {
			word := min(wordBytes, width-x)
			if word == wordBytes && bytes.Equal(previousRow[x:x+word], currentRow[x:x+word]) {
				x += word
				continue
			}
			for index := x; index < x+word; index++ {
				difference := int(currentRow[index]) - int(previousRow[index])
				if difference < 0 {
					difference = -difference
				}
				if difference <= threshold {
					continue
				}
				maskRow[index] = 1
			}
			x += word
		}
	}
	if len(d.changedRows) == 0 {
		return nil
	}

	// Only the rows that changed are scanned for component starts. Walking the whole mask cost
	// half a millisecond on a 1080p frame for the sake of a few thousand pixels.
	var components []component
	for _, y := range d.changedRows {
		rowStart := y * width
		rowEnd := rowStart + width
		for startIndex := rowStart; startIndex < rowEnd; startIndex++ {
			if mask[startIndex] != 1 {
				continue
			}
			d.stack = append(d.stack[:0], startIndex)
			mask[startIndex] = 2

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
				difference := int(currentPlane[index]) - int(previousPlane[index])
				if difference < 0 {
					difference = -difference
				}
				sum += difference
				count++

				for dy := -1; dy <= 1; dy++ {
					neighbourY := y + dy
					if neighbourY < 0 || neighbourY >= height {
						continue
					}
					neighbourRow := neighbourY * width
					for dx := -1; dx <= 1; dx++ {
						neighbourX := x + dx
						if neighbourX < 0 || neighbourX >= width || (dx == 0 && dy == 0) {
							continue
						}
						neighbour := neighbourRow + neighbourX
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
	}

	// Deterministic order before any matching: top edge, then left edge.
	sortComponents(components)
	return components
}

// Fingerprint summarises a frame as a grid of quantised luma cells.
//
// The grid is produced by the same pass that converts the frame for the comparison, so a frame is
// walked once no matter how many things ask about it. The cells are only recomputed here when the
// plane came from the cache, which happens when a caller asks twice about one frame.
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

	plane := d.plane(current, size)
	if plane == nil {
		return delta.Fingerprint{}, &fielderr.Error{
			Op:      "diff.Fingerprint",
			Subject: "frame",
			Field:   "geometry",
			Problem: "must have a positive width and height",
		}
	}

	d.prepareCells(size)
	// The grid is always derived from the plane, whether the plane was just converted or already
	// held from the comparison.
	d.gridFrom(plane, current, size)

	needed := size * size
	if len(d.grid) != needed {
		return delta.Fingerprint{}, &fielderr.Error{
			Op:      "diff.Fingerprint",
			Subject: "frame",
			Field:   "geometry",
			Problem: "changed while the fingerprint was being taken",
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

func (d *Differ) gridFrom(plane []byte, f frame.Frame, size int) {
	// A fingerprint is compared with tolerance, so a sample of each cell measures the same thing
	// as its exact mean at a small fraction of the cost. Summing every pixel of every cell cost
	// more than the comparison it helps.
	stepX := max(1, f.Width/size/4)
	stepY := max(1, f.Height/size/4)

	for cellY := 0; cellY < size; cellY++ {
		top := cellY * f.Height / size
		bottom := max((cellY+1)*f.Height/size, top+1)

		for cellX := 0; cellX < size; cellX++ {
			left := cellX * f.Width / size
			right := max((cellX+1)*f.Width/size, left+1)

			sum, count := 0, 0
			for y := top; y < min(bottom, f.Height); y += stepY {
				row := y * f.Width
				for x := left; x < min(right, f.Width); x += stepX {
					sum += int(plane[row+x])
					count++
				}
			}
			cell := cellY*size + cellX
			if count == 0 {
				d.grid[cell] = 0
				continue
			}
			d.grid[cell] = uint8(sum / count)
		}
	}
	d.gridSequence = f.Sequence
}

func cellsToBytes(cells []uint8) []byte {
	if len(cells) == 0 {
		return nil
	}
	buffer := make([]byte, len(cells))
	copy(buffer, cells)
	return buffer
}

// lumaOfPixel converts one rgba8 pixel to the single channel the engine compares.
func lumaOfPixel(pixel []byte) int {
	return int(lumaRedTable[pixel[0]]+lumaGreenBlue[uint16(pixel[1])<<8|uint16(pixel[2])]) / lumaScale
}

func (d *Differ) fillPlane(destination []byte, f frame.Frame, gridSize int) {
	width, height := f.Width, f.Height
	count := width * height
	if count <= 0 || len(f.Pixels) < count*4 {
		return
	}

	// Sliced to their exact lengths so the compiler can see that every index below is in range,
	// which removes a bounds check from the hottest loop in the engine.
	pixels := f.Pixels[: count*4 : count*4]
	destination = destination[:count]

	for index := 0; index < count; index++ {
		offset := index * 4
		destination[index] = byte((lumaRedTable[pixels[offset]] +
			lumaGreenBlue[uint16(pixels[offset+1])<<8|uint16(pixels[offset+2])]) / lumaScale)
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
