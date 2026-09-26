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

// TileSize is the edge of one comparison tile in pixels. Sixteen keeps a glyph from
// spanning many tiles while staying cache friendly.
const TileSize = 16

// growthMargin is how far a reported region is expanded beyond the tiles that changed,
// so the bounds cover the element that produced the change rather than the inside of it.
const growthMargin = 2

// luma weights are the integer BT.601 coefficients, used instead of floating point so
// the same input produces the same numbers everywhere.
const (
	lumaRed   = 299
	lumaGreen = 587
	lumaBlue  = 114
	lumaScale = 1000
)

// Differ compares consecutive frames of one stream.
type Differ struct {
	tiles    []tile
	parent   []int
	previous []regionState
	grid     []uint8

	previousWidth  int
	previousHeight int
	hasPrevious    bool
	nextIdentity   uint64
}

// tile is one cell of the comparison grid.
type tile struct {
	tilesX int
	tilesY int
	mean   float64
	change bool
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

	// A configuration that asks for nothing at all is honoured before any work is done,
	// and the remembered geometry is cleared so a later change of configuration cannot
	// produce removals for regions the caller asked never to hear about.
	if cfg.SuppressesAllRegions() {
		d.previous = d.previous[:0]
		d.hasPrevious = true
		d.previousWidth, d.previousHeight = current.Width, current.Height
		return []delta.Region{}, nil, nil
	}

	d.measure(previous, current, cfg)

	components := d.components(previous, current, cfg)
	candidates := d.filter(components, current, cfg)
	regions := d.classify(candidates, previous, current, cfg)

	return regions, nil, nil
}

// measure fills the tile grid with the mean absolute luma difference per tile.
func (d *Differ) measure(previous, current frame.Frame, cfg config.Config) {
	tilesX := (current.Width + TileSize - 1) / TileSize
	tilesY := (current.Height + TileSize - 1) / TileSize
	needed := tilesX * tilesY

	if len(d.tiles) != needed {
		d.tiles = make([]tile, needed)
		d.parent = make([]int, needed)
	} else {
		for index := range d.tiles {
			d.tiles[index] = tile{}
		}
	}

	threshold := cfg.NoiseFloor * 255

	for tileY := 0; tileY < tilesY; tileY++ {
		for tileX := 0; tileX < tilesX; tileX++ {
			index := tileY*tilesX + tileX
			sum, count := 0, 0

			for y := tileY * TileSize; y < min((tileY+1)*TileSize, current.Height); y++ {
				rowOffset := y * current.Width * 4
				for x := tileX * TileSize; x < min((tileX+1)*TileSize, current.Width); x++ {
					offset := rowOffset + x*4
					difference := absInt(luma(current.Pixels[offset:]) - luma(previous.Pixels[offset:]))
					sum += difference
					count++
				}
			}

			mean := 0.0
			if count > 0 {
				mean = float64(sum) / float64(count)
			}
			d.tiles[index] = tile{
				tilesX: tilesX,
				tilesY: tilesY,
				mean:   mean,
				change: mean > threshold,
			}
			d.parent[index] = index
		}
	}
}

// component is a connected group of changed tiles, refined to the pixels that actually
// differ and expressed in pixels.
//
// The refinement matters: a tile is sixteen pixels wide, so a tile bounding box inflates
// a small element by up to one tile on each side, which would both overstate the region
// and blur the motion detection that compares footprints between frames.
type component struct {
	left, top, right, bottom int
	magnitude                float64
}

// components merges neighbouring changed tiles into rectangles.
//
// Eight-way connectivity is used because a one pixel diagonal join between two areas
// that a person would call one region is common in rendered text and icons.
func (d *Differ) components(previous, current frame.Frame, cfg config.Config) []component {
	tilesX := (current.Width + TileSize - 1) / TileSize
	tilesY := (current.Height + TileSize - 1) / TileSize

	anyChanged := false
	for tileY := 0; tileY < tilesY; tileY++ {
		for tileX := 0; tileX < tilesX; tileX++ {
			if !d.tiles[tileY*tilesX+tileX].change {
				continue
			}
			anyChanged = true
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					neighbourX, neighbourY := tileX+dx, tileY+dy
					if neighbourX < 0 || neighbourY < 0 || neighbourX >= tilesX || neighbourY >= tilesY {
						continue
					}
					neighbour := neighbourY*tilesX + neighbourX
					if !d.tiles[neighbour].change {
						continue
					}
					d.union(tileY*tilesX+tileX, neighbour)
				}
			}
		}
	}
	if !anyChanged {
		return nil
	}

	// Groups are collected in scan order and then keyed by root, which keeps the result
	// independent of map iteration order.
	groups := make(map[int]*component, 8)
	order := make([]int, 0, 8)
	for tileY := 0; tileY < tilesY; tileY++ {
		for tileX := 0; tileX < tilesX; tileX++ {
			index := tileY*tilesX + tileX
			if !d.tiles[index].change {
				continue
			}
			root := d.find(index)
			group, seen := groups[root]
			if !seen {
				group = &component{
					left:   tileX * TileSize,
					top:    tileY * TileSize,
					right:  min((tileX+1)*TileSize, current.Width),
					bottom: min((tileY+1)*TileSize, current.Height),
				}
				groups[root] = group
				order = append(order, root)
				continue
			}
			group.left = min(group.left, tileX*TileSize)
			group.top = min(group.top, tileY*TileSize)
			group.right = max(group.right, min((tileX+1)*TileSize, current.Width))
			group.bottom = max(group.bottom, min((tileY+1)*TileSize, current.Height))
		}
	}

	// Magnitude is the mean difference over the tiles in the group, before any growth.
	components := make([]component, 0, len(order))
	for _, root := range order {
		group := groups[root]
		sum, count := 0.0, 0
		for tileY := 0; tileY < tilesY; tileY++ {
			for tileX := 0; tileX < tilesX; tileX++ {
				index := tileY*tilesX + tileX
				if !d.tiles[index].change || d.find(index) != root {
					continue
				}
				sum += d.tiles[index].mean
				count++
			}
		}
		if count > 0 {
			group.magnitude = sum / float64(count) / 255
		}
		refined, magnitude := refine(previous, current, *group, cfg.NoiseFloor*255)
		components = append(components, component{
			left:      refined.Min.X,
			top:       refined.Min.Y,
			right:     refined.Max.X,
			bottom:    refined.Max.Y,
			magnitude: magnitude,
		})
	}

	// Deterministic order before any matching: top edge, then left edge.
	sortComponents(components)
	return components
}

func (d *Differ) find(index int) int {
	for d.parent[index] != index {
		d.parent[index] = d.parent[d.parent[index]]
		index = d.parent[index]
	}
	return index
}

func (d *Differ) union(a, b int) {
	rootA, rootB := d.find(a), d.find(b)
	if rootA == rootB {
		return
	}
	if rootA < rootB {
		d.parent[rootB] = rootA
		return
	}
	d.parent[rootA] = rootB
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
