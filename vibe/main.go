// The vibe-coded pass. Written in one go from the prompt below, without reading specs/ and without a
// specification to check it against. It is deliberately kept outside the module until it compiles and has been
// compared with the specification-driven engine, because unverified code in the repository would turn its gate red.
//
// The prompt, verbatim: "Write a Go program that takes two screenshots and prints the areas that changed as JSON."
//
// What a specification would have caught, to be measured rather than asserted: no noise floor, no configuration,
// no total order on the output, no element identity, no conditions, and no classifier.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
)

type bounds struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type region struct {
	Bounds bounds `json:"bounds"`
	Class  string `json:"class"`
}

type document struct {
	Regions []region `json:"regions"`
}

func load(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return png.Decode(file)
}

func luma(img image.Image, x, y int) int {
	r, g, b, _ := img.At(x, y).RGBA()
	// The weights a first pass reaches for.
	return int((299*int(r>>8) + 587*int(g>>8) + 114*int(b>>8)) / 1000)
}

func main() {
	threshold := flag.Int("threshold", 25, "how much a pixel has to differ")
	minArea := flag.Int("min-area", 40, "how many pixels an area needs")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: vibe <before.png> <after.png>")
		os.Exit(2)
	}

	before, err := load(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "before:", err)
		os.Exit(1)
	}
	after, err := load(flag.Arg(1))
	if err != nil {
		fmt.Fprintln(os.Stderr, "after:", err)
		os.Exit(1)
	}
	if before.Bounds() != after.Bounds() {
		fmt.Fprintln(os.Stderr, "the frames are not the same size")
		os.Exit(1)
	}

	width, height := before.Bounds().Dx(), before.Bounds().Dy()
	changed := make([]bool, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			delta := luma(after, x, y) - luma(before, x, y)
			if delta < 0 {
				delta = -delta
			}
			changed[y*width+x] = delta > *threshold
		}
	}

	// Flood fill the changed pixels into areas. The visited set is a map, which is the kind of choice a first
	// pass makes and which a specification would have refused: it is what makes the output order depend on the
	// run.
	seen := map[int]bool{}
	var regions []region
	for index := range changed {
		if !changed[index] || seen[index] {
			continue
		}
		stack := []int{index}
		seen[index] = true
		minX, minY := width, height
		maxX, maxY := 0, 0
		count := 0
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := current%width, current/width
			count++
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
			for _, next := range [4]int{current - 1, current + 1, current - width, current + width} {
				if next < 0 || next >= len(changed) || seen[next] || !changed[next] {
					continue
				}
				if next == current-1 && x == 0 {
					continue
				}
				if next == current+1 && x == width-1 {
					continue
				}
				seen[next] = true
				stack = append(stack, next)
			}
		}
		if count < *minArea {
			continue
		}
		regions = append(regions, region{
			Bounds: bounds{X: minX, Y: minY, W: maxX - minX + 1, H: maxY - minY + 1},
			Class:  "changed",
		})
	}

	encoded, err := json.Marshal(document{Regions: regions})
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode:", err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}
