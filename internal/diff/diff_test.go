package diff

import (
	"bytes"
	"image"
	"image/color"
	"math/rand"
	"testing"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/frame"
)

const (
	background = 30
	panelValue = 200
	panelAlt   = 90
)

type panel struct {
	x, y, w, h int
	value      uint8
}

func buildFrame(sequence uint64, width, height int, panels []panel) frame.Frame {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	fillRect(img, 0, 0, width, height, background)
	for _, p := range panels {
		fillRect(img, p.x, p.y, p.w, p.h, p.value)
	}
	return frame.Frame{
		Sequence:    sequence,
		Width:       width,
		Height:      height,
		Format:      frame.FormatRGBA8,
		ScaleFactor: 1,
		Pixels:      img.Pix,
	}
}

func fillRect(img *image.RGBA, x, y, w, h int, value uint8) {
	grey := color.RGBA{R: value, G: value, B: value, A: 255}
	for py := y; py < y+h; py++ {
		for px := x; px < x+w; px++ {
			if px < 0 || py < 0 || px >= img.Rect.Dx() || py >= img.Rect.Dy() {
				continue
			}
			img.SetRGBA(px, py, grey)
		}
	}
}

func pixelRect(bounds delta.Bounds, f frame.Frame) image.Rectangle {
	left := int(bounds.X * float64(f.Width))
	top := int(bounds.Y * float64(f.Height))
	return image.Rect(left, top, left+int(bounds.W*float64(f.Width)+0.5), top+int(bounds.H*float64(f.Height)+0.5))
}

func defaults() config.Config { return config.Defaults() }

func TestIdenticalFramesReportNothing(t *testing.T) {
	panels := []panel{{x: 40, y: 60, w: 80, h: 30, value: panelValue}}
	first := buildFrame(1, 320, 240, panels)
	second := buildFrame(2, 320, 240, panels)

	regions, conditions, err := New().Compare(first, second, defaults())
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if len(regions) != 0 {
		t.Fatalf("identical frames produced %d regions: %+v", len(regions), regions)
	}
	if len(conditions) != 0 {
		t.Fatalf("identical frames produced conditions: %v", conditions)
	}
}

func TestChangedPanelIsReportedOnceWithCoveringBounds(t *testing.T) {
	target := panel{x: 40, y: 60, w: 80, h: 30, value: panelValue}
	first := buildFrame(1, 320, 240, []panel{target})
	second := buildFrame(2, 320, 240, []panel{{x: target.x, y: target.y, w: target.w, h: target.h, value: panelAlt}})

	regions, _, err := New().Compare(first, second, defaults())
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if len(regions) != 1 {
		t.Fatalf("expected exactly one region, got %d: %+v", len(regions), regions)
	}

	region := regions[0]
	if region.Class != delta.ClassChanged {
		t.Fatalf("class = %q, want changed: the first comparison has no baseline to call it added", region.Class)
	}
	reported := pixelRect(region.Bounds, second)
	inside := image.Rect(target.x, target.y, target.x+target.w, target.y+target.h)
	if overlap := intersectionOverUnion(reported, inside); overlap < 0.8 {
		t.Fatalf("reported bounds %v barely overlap the changed panel %v (IoU %.2f)", reported, inside, overlap)
	}
	if region.Magnitude <= 0 {
		t.Fatalf("magnitude = %v, want a positive value", region.Magnitude)
	}
	if region.AreaPixels != reported.Dx()*reported.Dy() {
		t.Fatalf("area %d does not match the reported bounds %v", region.AreaPixels, reported)
	}
}

func TestNoiseBelowTheFloorReportsNothing(t *testing.T) {
	width, height := 320, 240
	panels := []panel{{x: 40, y: 60, w: 80, h: 30, value: panelValue}}
	rng := rand.New(rand.NewSource(1))

	base := buildFrame(1, width, height, panels)
	for attempt := 0; attempt < 20; attempt++ {
		noisy := buildFrame(uint64(attempt+2), width, height, panels)
		for index := 0; index < len(noisy.Pixels); index += 4 {
			delta := rng.Intn(7) - 3
			for channel := 0; channel < 3; channel++ {
				value := int(noisy.Pixels[index+channel]) + delta
				if value < 0 {
					value = 0
				}
				if value > 255 {
					value = 255
				}
				noisy.Pixels[index+channel] = uint8(value)
			}
		}
		regions, _, err := New().Compare(base, noisy, defaults())
		if err != nil {
			t.Fatalf("Compare failed: %v", err)
		}
		if len(regions) != 0 {
			t.Fatalf("capture noise produced %d regions: %+v", len(regions), regions)
		}
	}
}

func TestMovedElementIsReportedAsMoved(t *testing.T) {
	width, height := 320, 240
	button := panel{x: 80, y: 120, w: 80, h: 30, value: panelValue}
	step := 8

	frames := make([]frame.Frame, 0, 3)
	for index := 0; index < 3; index++ {
		shifted := button
		shifted.x = button.x + index*step
		frames = append(frames, buildFrame(uint64(index+1), width, height, []panel{shifted}))
	}

	differ := New()
	if _, _, err := differ.Compare(frames[0], frames[1], defaults()); err != nil {
		t.Fatalf("first comparison failed: %v", err)
	}
	regions, _, err := differ.Compare(frames[1], frames[2], defaults())
	if err != nil {
		t.Fatalf("second comparison failed: %v", err)
	}
	if len(regions) == 0 {
		t.Fatal("a translated panel produced no regions")
	}

	moved := 0
	for _, region := range regions {
		if region.Class == delta.ClassMoved {
			moved++
			if region.PreviousBounds == nil {
				t.Fatalf("a moved region must carry where it was: %+v", region)
			}
		}
	}
	if moved == 0 {
		t.Fatalf("no region was classified as moved: %+v", regions)
	}
}

func TestIgnoredAreaSuppressesChanges(t *testing.T) {
	clock := panel{x: 300, y: 10, w: 16, h: 16, value: panelValue}
	first := buildFrame(1, 320, 240, []panel{{x: 40, y: 60, w: 80, h: 30, value: panelValue}, clock})
	second := buildFrame(2, 320, 240, []panel{
		{x: 40, y: 60, w: 80, h: 30, value: panelValue},
		{x: clock.x, y: clock.y, w: clock.w, h: clock.h, value: panelAlt},
	})

	cfg := defaults()
	cfg.IgnoredAreas = []delta.Bounds{{
		X: float64(clock.x) / 320, Y: float64(clock.y) / 240,
		W: float64(clock.w) / 320, H: float64(clock.h) / 240,
	}}

	regions, _, err := New().Compare(first, second, cfg)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if len(regions) != 0 {
		t.Fatalf("a change inside an ignored area was reported: %+v", regions)
	}
}

func TestRegionsOfInterestRestrictAndSuppress(t *testing.T) {
	left := panel{x: 20, y: 60, w: 40, h: 30, value: panelValue}
	right := panel{x: 240, y: 60, w: 40, h: 30, value: panelValue}

	first := buildFrame(1, 320, 240, []panel{left, right})
	second := buildFrame(2, 320, 240, []panel{
		{x: left.x, y: left.y, w: left.w, h: left.h, value: panelAlt},
		{x: right.x, y: right.y, w: right.w, h: right.h, value: panelAlt},
	})

	cfg := defaults()
	roi := []config.RegionOfInterest{{
		Label:  "left half",
		Bounds: delta.Bounds{X: 0, Y: 0, W: 0.5, H: 1},
	}}
	cfg.RegionsOfInterest = &roi

	regions, _, err := New().Compare(first, second, cfg)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if len(regions) != 1 {
		t.Fatalf("expected only the region of interest to be reported, got %d: %+v", len(regions), regions)
	}
	if pixelRect(regions[0].Bounds, second).Max.X > 160 {
		t.Fatalf("reported region extends outside the region of interest: %+v", regions[0])
	}

	// An empty list is not "unrestricted": it asks for nothing at all.
	empty := []config.RegionOfInterest{}
	cfg.RegionsOfInterest = &empty
	regions, _, err = New().Compare(first, second, cfg)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if len(regions) != 0 {
		t.Fatalf("an empty regions-of-interest list reported %d regions", len(regions))
	}
}

func TestMinimumAreaFiltersSmallChanges(t *testing.T) {
	small := panel{x: 100, y: 100, w: 3, h: 3, value: panelValue}
	first := buildFrame(1, 320, 240, []panel{small})
	second := buildFrame(2, 320, 240, []panel{{x: small.x, y: small.y, w: small.w, h: small.h, value: panelAlt}})

	cfg := defaults()
	cfg.MinRegionAreaPixels = 4000
	regions, _, err := New().Compare(first, second, cfg)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if len(regions) != 0 {
		t.Fatalf("a change below the minimum area was reported: %+v", regions)
	}
}

func TestReplacedElementIsReportedAsRemovedAndAdded(t *testing.T) {
	// The classification rules are exercised directly here, because a pixel difference
	// cannot distinguish a changed element from a replaced one on its own.
	width, height := 320, 240
	current := buildFrame(2, width, height, nil)
	previous := buildFrame(1, width, height, nil)

	differ := New()
	previousRect := image.Rect(40, 60, 70, 80)
	differ.hasPrevious = true
	differ.previous = []regionState{{bounds: boundsOf(previousRect, current), width: previousRect.Dx(), height: previousRect.Dy()}}

	// A larger area covers the old element's footprint, so the old element is gone and
	// something else occupies the place.
	replacement := candidate{left: 30, top: 50, right: 140, bottom: 120, magnitude: 0.4}
	regions := differ.classify([]candidate{replacement}, previous, current, defaults())

	classes := map[delta.RegionClass]int{}
	for _, region := range regions {
		classes[region.Class]++
	}
	if classes[delta.ClassRemoved] != 1 || classes[delta.ClassAdded] != 1 {
		t.Fatalf("expected one removed and one added region, got %+v", regions)
	}
	for _, region := range regions {
		if region.Class == delta.ClassRemoved && region.PreviousBounds == nil {
			t.Fatalf("a removed region must say where the element was: %+v", region)
		}
	}
}

func TestIdentitiesAreUniqueAcrossAStream(t *testing.T) {
	width, height := 320, 240
	seen := map[uint64]bool{}
	differ := New()

	for index := 0; index < 6; index++ {
		value := uint8(panelValue)
		if index%2 == 1 {
			value = panelAlt
		}
		next := buildFrame(uint64(index+2), width, height, []panel{{x: 40 + index*4, y: 60, w: 80, h: 30, value: value}})
		current := buildFrame(uint64(index+1), width, height, []panel{{x: 40 + (index-1)*4, y: 60, w: 80, h: 30, value: panelValue}})
		if index == 0 {
			current = buildFrame(1, width, height, []panel{{x: 40, y: 60, w: 80, h: 30, value: panelValue}})
		}
		regions, _, err := differ.Compare(current, next, defaults())
		if err != nil {
			t.Fatalf("Compare failed: %v", err)
		}
		for _, region := range regions {
			if seen[region.Identity] {
				t.Fatalf("identity %d was reused", region.Identity)
			}
			seen[region.Identity] = true
		}
	}
	if len(seen) == 0 {
		t.Fatal("the stream produced no identities at all")
	}
}

func TestCompareIsDeterministic(t *testing.T) {
	width, height := 320, 240
	build := func() []byte {
		differ := New()
		var encoded bytes.Buffer
		for index := 0; index < 5; index++ {
			value := uint8(panelValue)
			if index%2 == 1 {
				value = panelAlt
			}
			current := buildFrame(uint64(index+1), width, height, []panel{{x: 40 + index*8, y: 60, w: 80, h: 30, value: uint8(panelValue)}})
			next := buildFrame(uint64(index+2), width, height, []panel{{x: 40 + (index+1)*8, y: 60, w: 80, h: 30, value: value}})
			regions, _, err := differ.Compare(current, next, defaults())
			if err != nil {
				t.Fatalf("Compare failed: %v", err)
			}
			document := delta.Document{
				SchemaVersion: delta.SchemaVersion,
				Frame:         delta.FrameRef{Sequence: uint64(index + 2), Width: width, Height: height, ScaleFactor: 1},
				Fingerprint:   delta.Fingerprint{Algorithm: "grid-luma-1", GridSize: 8, Cells: make([]int, 64), StrictHash: "0123456789abcdef"},
				Regions:       regions,
				Conditions:    []delta.Condition{},
			}
			if err := document.Encode(&encoded, false); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
		}
		return encoded.Bytes()
	}
	if !bytes.Equal(build(), build()) {
		t.Fatal("two identical runs produced different documents")
	}
}

func TestFingerprintTracksContentNotNoise(t *testing.T) {
	width, height := 320, 240
	panels := []panel{{x: 40, y: 60, w: 80, h: 30, value: panelValue}}
	base := buildFrame(1, width, height, panels)
	same := buildFrame(2, width, height, panels)
	changed := buildFrame(3, width, height, []panel{{x: 40, y: 60, w: 80, h: 30, value: panelAlt}})

	differ := New()
	first, err := differ.Fingerprint(base, defaults())
	if err != nil {
		t.Fatalf("Fingerprint failed: %v", err)
	}
	second, err := differ.Fingerprint(same, defaults())
	if err != nil {
		t.Fatalf("Fingerprint failed: %v", err)
	}
	third, err := differ.Fingerprint(changed, defaults())
	if err != nil {
		t.Fatalf("Fingerprint failed: %v", err)
	}

	if first.StrictHash != second.StrictHash {
		t.Fatalf("identical frames produced different hashes: %s vs %s", first.StrictHash, second.StrictHash)
	}
	if first.StrictHash == third.StrictHash {
		t.Fatal("a materially changed frame produced the same hash")
	}
	if first.Algorithm != "grid-luma-1" || first.GridSize != 32 || len(first.Cells) != 32*32 {
		t.Fatalf("fingerprint shape is wrong: %+v", first)
	}
}

func TestCompareRejectsMismatchedDimensions(t *testing.T) {
	first := buildFrame(1, 320, 240, nil)
	second := buildFrame(2, 160, 120, nil)
	if _, _, err := New().Compare(first, second, defaults()); err == nil {
		t.Fatal("Compare accepted frames of different dimensions")
	}
}
