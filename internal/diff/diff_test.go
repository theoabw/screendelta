package diff

import (
	"image"
	"image/color"
	"math/rand"
	"testing"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/frame"
	"github.com/theoabw/screendelta/internal/identity"
)

const (
	// tolerancePixels is the motion tolerance the continuity test uses.
	tolerancePixels = 8
	background      = 30
	panelValue      = 200
	panelAlt        = 90
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

	// The classifier now reads the identity map, so the previous element is placed there rather than in
	// a second memory of the same thing.
	differ := New()
	previousRect := image.Rect(40, 60, 70, 80)
	differ.hasPrevious = true
	differ.identities = identity.New(2, 8)
	differ.identities.Appear(boundsOf(previousRect, current), width, height, 1, nil)
	differ.identities.EndFrame(1, nil, width, height)

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

// TestATrackedElementKeepsOneIdentityAcrossFrames is acceptance scenario 1 of user story 2: a region
// that drifts a few pixels per frame keeps one identifier throughout.
func TestATrackedElementKeepsOneIdentityAcrossFrames(t *testing.T) {
	width, height := 320, 240
	differ := New()

	identity := uint64(0)
	first := buildFrame(1, width, height, []panel{{x: 80, y: 120, w: 80, h: 30, value: panelValue}})
	previous := first
	for step := 1; step <= 8; step++ {
		next := buildFrame(uint64(step+1), width, height,
			[]panel{{x: 80 + step*3, y: 120, w: 80, h: 30, value: panelAlt}})
		regions, _, err := differ.Compare(previous, next, defaults())
		if err != nil {
			t.Fatalf("step %d: Compare failed: %v", step, err)
		}
		if len(regions) == 0 {
			t.Fatalf("step %d: a drifting element produced no regions", step)
		}
		for _, region := range regions {
			if region.Identity == 0 {
				t.Fatalf("step %d: a region carries no identity", step)
			}
			if identity == 0 {
				identity = region.Identity
				continue
			}
			if region.Identity != identity {
				t.Fatalf("step %d: identity changed from %d to %d", step, identity, region.Identity)
			}
			if region.IdentityUncertain {
				t.Fatalf("step %d: a tracked element was marked uncertain", step)
			}
			if region.IdentityConfidence <= 0 || region.IdentityConfidence > 1 {
				t.Fatalf("step %d: confidence %v is outside 0 to 1", step, region.IdentityConfidence)
			}
		}
		previous = next
	}
}

// TestContinuityOfAnIdentity checks the property that makes an identity worth having: when the same
// identity appears in consecutive frames, it refers to something in the same place. An identity that
// jumped across the frame would be worse than no identity at all, because a consumer would follow it.
func TestContinuityOfAnIdentity(t *testing.T) {
	width, height := 320, 240
	differ := New()

	footprint := map[uint64]image.Rectangle{}
	previous := buildFrame(1, width, height, []panel{{x: 60, y: 60, w: 60, h: 40, value: panelValue}})

	for step := 1; step <= 12; step++ {
		next := buildFrame(uint64(step+1), width, height, []panel{
			{x: 60 + step*4, y: 60, w: 60, h: 40, value: panelAlt},
			{x: 220, y: 180, w: 40, h: 30, value: uint8(80 + step*7)},
		})
		regions, _, err := differ.Compare(previous, next, defaults())
		if err != nil {
			t.Fatalf("step %d: Compare failed: %v", step, err)
		}
		// One element can be reported as more than one region in a frame, so the element's
		// footprint is the union of the regions that name it rather than any single one.
		seenThisFrame := map[uint64]image.Rectangle{}
		for _, region := range regions {
			if region.Class == delta.ClassRemoved {
				continue
			}
			rect := pixelRect(region.Bounds, next)
			if current, ok := seenThisFrame[region.Identity]; ok {
				seenThisFrame[region.Identity] = current.Union(rect)
				continue
			}
			seenThisFrame[region.Identity] = rect
		}
		for id, rect := range seenThisFrame {
			if last, seen := footprint[id]; seen {
				distance := absInt(rect.Min.X-last.Min.X) + absInt(rect.Min.Y-last.Min.Y)
				if distance > 2*tolerancePixels {
					t.Fatalf("step %d: identity %d moved %d pixels between frames, which is not the same element",
						step, id, distance)
				}
			}
			footprint[id] = rect
		}
		previous = next
	}
}

// TestAReplacementIsReportedAsRemovedAndAdded records what the engine can actually say about an
// element that is covered by a larger one, and why it cannot say more.
//
// The replacement is reported as a removal naming the covered element and an addition naming the
// thing that covered it, which is a truthful account of the pixels. The return of the covered element
// is then reported as a change of the thing that covered it, because that is where the pixels changed
// and it is still tracked. The engine cannot tell that the element underneath came back, and the
// identity it would need to report that is retired.
//
// This is a limitation, not a claim: the uncertain marker exists for exactly this case and the
// classifier cannot reach it here. Falsifying it took a review that constructed the sequence and read
// the documents; the test now records the behaviour so a later change has to be deliberate.
func TestAReplacementIsReportedAsRemovedAndAdded(t *testing.T) {
	width, height := 320, 240
	small := panel{x: 120, y: 100, w: 40, h: 30, value: panelValue}
	big := panel{x: 80, y: 60, w: 120, h: 100, value: 60}

	cfg := defaults()
	cfg.OcclusionFrames = 1

	frames := []frame.Frame{
		buildFrame(1, width, height, []panel{small}),
		buildFrame(2, width, height, []panel{{x: small.x, y: small.y, w: small.w, h: small.h, value: panelAlt}}),
		buildFrame(3, width, height, []panel{big}),
	}

	differ := New()
	tracked := uint64(0)
	if regions, _, err := differ.Compare(frames[0], frames[1], cfg); err != nil {
		t.Fatalf("the first comparison failed: %v", err)
	} else if len(regions) == 0 {
		t.Fatal("the first comparison reported nothing")
	} else {
		tracked = regions[0].Identity
	}

	regions, _, err := differ.Compare(frames[1], frames[2], cfg)
	if err != nil {
		t.Fatalf("the replacement comparison failed: %v", err)
	}

	classes := map[delta.RegionClass][]uint64{}
	for _, region := range regions {
		classes[region.Class] = append(classes[region.Class], region.Identity)
	}
	if len(classes[delta.ClassRemoved]) != 1 {
		t.Fatalf("a replacement produced %d removals, want one: %+v", len(classes[delta.ClassRemoved]), regions)
	}
	if classes[delta.ClassRemoved][0] != tracked {
		t.Fatalf("the removal named identity %d instead of the covered %d", classes[delta.ClassRemoved][0], tracked)
	}
	if len(classes[delta.ClassAdded]) != 1 {
		t.Fatalf("a replacement produced %d additions, want one: %+v", len(classes[delta.ClassAdded]), regions)
	}
	if classes[delta.ClassAdded][0] == tracked {
		t.Fatal("the covering element was given the identity of the element it covered")
	}
}

func TestAViewportChangeEndsEveryIdentity(t *testing.T) {
	differ := New()
	first := buildFrame(1, 320, 240, []panel{{x: 40, y: 60, w: 80, h: 30, value: panelValue}})
	second := buildFrame(2, 320, 240, []panel{{x: 40, y: 60, w: 80, h: 30, value: panelAlt}})
	regions, _, err := differ.Compare(first, second, defaults())
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if len(regions) == 0 {
		t.Fatal("the first comparison reported nothing")
	}
	before := regions[0].Identity

	bigger := buildFrame(3, 640, 480, []panel{{x: 300, y: 300, w: 80, h: 30, value: panelValue}})
	if _, _, err := differ.Compare(bigger, bigger, defaults()); err != nil {
		t.Fatalf("Compare after a viewport change failed: %v", err)
	}

	next := buildFrame(4, 640, 480, []panel{{x: 300, y: 300, w: 80, h: 30, value: panelAlt}})
	regions, _, err = differ.Compare(bigger, next, defaults())
	if err != nil {
		t.Fatalf("Compare after a viewport change failed: %v", err)
	}
	if len(regions) == 0 {
		t.Fatal("the change at the new size reported nothing")
	}
	for _, region := range regions {
		if region.Identity == before {
			t.Fatalf("identity %d was carried across a viewport change", before)
		}
		if region.Identity <= before {
			t.Fatalf("identity %d went backwards after identity %d, so a number was reissued", region.Identity, before)
		}
	}
}

// TestRelaxingTheMotionToleranceKeepsIdentities covers the other change: a rule change says nothing
// about where anything is, so the element keeps its identity and a caller that relaxes the tolerance
// gets the match it asked for rather than a fresh table.
func TestRelaxingTheMotionToleranceKeepsIdentities(t *testing.T) {
	differ := New()
	frames := []frame.Frame{
		buildFrame(1, 320, 240, []panel{{x: 40, y: 60, w: 80, h: 30, value: panelValue}}),
		buildFrame(2, 320, 240, []panel{{x: 40, y: 60, w: 80, h: 30, value: panelAlt}}),
		buildFrame(3, 320, 240, []panel{{x: 60, y: 60, w: 80, h: 30, value: panelValue}}),
	}

	strict := defaults()
	strict.MotionTolerancePixels = 1
	first, _, err := differ.Compare(frames[0], frames[1], strict)
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("the first comparison reported nothing")
	}

	relaxed := defaults()
	relaxed.MotionTolerancePixels = 32
	regions, _, err := differ.Compare(frames[1], frames[2], relaxed)
	if err != nil {
		t.Fatalf("Compare with a new tolerance failed: %v", err)
	}
	if len(regions) == 0 {
		t.Fatal("the move reported nothing")
	}
	carried := false
	for _, region := range regions {
		if region.Identity == first[0].Identity {
			carried = true
		}
	}
	if !carried {
		t.Fatalf("a relaxed tolerance did not continue the element's identity: %+v", regions)
	}
}

// TestResetEndsEveryComparisonAndEveryIdentity is what the engine calls when it reports a viewport
// change. Bounds are normalised, so nothing remembered at one frame size survives it.
func TestResetEndsEveryComparisonAndEveryIdentity(t *testing.T) {
	width, height := 320, 240
	differ := New()
	first := buildFrame(1, width, height, []panel{{x: 40, y: 60, w: 80, h: 30, value: panelValue}})
	second := buildFrame(2, width, height, []panel{{x: 40, y: 60, w: 80, h: 30, value: panelAlt}})
	regions, _, err := differ.Compare(first, second, defaults())
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if len(regions) == 0 {
		t.Fatal("the first comparison reported nothing")
	}
	before := regions[0].Identity

	differ.Reset()

	// Nothing is remembered, so the next comparison has no baseline and reports changed rather than
	// matching anything from before.
	next := buildFrame(3, width, height, []panel{{x: 40, y: 60, w: 80, h: 30, value: panelValue}})
	regions, _, err = differ.Compare(second, next, defaults())
	if err != nil {
		t.Fatalf("Compare after Reset failed: %v", err)
	}
	for _, region := range regions {
		if region.Identity == before {
			t.Fatalf("identity %d survived Reset", before)
		}
	}
}

// TestSignatureSamplerIsBoundedAndSeparatesContent covers the appearance measurement the identity
// layer relies on: it must cost the same for a large area as for a small one, be deterministic, and
// still tell different content apart.
func TestSignatureSamplerIsBoundedAndSeparatesContent(t *testing.T) {
	width, height := 320, 240
	plane := make([]byte, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// A left half that is dark and a right half that is bright, so two rectangles drawn on
			// either side look nothing alike.
			value := byte(20)
			if x >= width/2 {
				value = 220
			}
			plane[y*width+x] = value
		}
	}

	if _, ok := signatureOf(plane, rect(10, 10, 20, 20), width, height); !ok {
		t.Fatal("a small rectangle produced no signature")
	}
	large, ok := signatureOf(plane, rect(0, 0, width, height), width, height)
	if !ok {
		t.Fatal("a full frame rectangle produced no signature")
	}
	again, _ := signatureOf(plane, rect(0, 0, width, height), width, height)
	if large != again {
		t.Fatalf("two measurements of the same rectangle differ: %v against %v", large, again)
	}

	dark, _ := signatureOf(plane, rect(5, 5, 60, 60), width, height)
	bright, _ := signatureOf(plane, rect(width-60, 5, width-5, 60), width, height)
	if dark.Close(bright) {
		t.Fatalf("a dark area and a bright area have the same signature: %v", dark)
	}

	// A rectangle at the edge is ordinary and must not produce a signature of nothing.
	edge, ok := signatureOf(plane, rect(width-4, height-4, width+40, height+40), width, height)
	if !ok {
		t.Fatal("a rectangle past the frame edge produced no signature")
	}
	if edge.Distance(bright) > 4*identity.SignatureTolerance {
		t.Fatalf("an edge rectangle does not look like the area it covers: %v against %v", edge, bright)
	}

	// Degenerate sizes still produce something, because a one pixel element is legal.
	tiny, ok := signatureOf(plane, rect(3, 3, 4, 4), width, height)
	if !ok {
		t.Fatal("a one pixel rectangle produced no signature")
	}
	if tiny.Distance(dark) > 4*identity.SignatureTolerance {
		t.Fatalf("a one pixel rectangle in the dark half does not look dark: %v", tiny)
	}
}

// TestAPartialChangeKeepsOneIdentity is the AUD-020 regression at the engine level.
//
// The engine only learns about an element from what changes, so the sequence has to show it the whole
// panel first. After that, a change in one corner and then a change in the opposite corner are two
// changes within one element, and the second must not look like a new one. Before the footprint started
// translating and absorbing only new ground, the first patch shrank the element to that patch, and the
// second patch fell outside it and was reported as something new.
func TestAPartialChangeKeepsOneIdentity(t *testing.T) {
	width, height := 320, 240
	panelRect := panel{x: 60, y: 40, w: 200, h: 160, value: panelValue}
	leftPatch := panel{x: 70, y: 50, w: 20, h: 20, value: 240}

	frames := []frame.Frame{
		// Nothing, then the whole panel, so the engine learns how big the element is.
		buildFrame(1, width, height, nil),
		buildFrame(2, width, height, []panel{panelRect, leftPatch}),
		// A change in one corner.
		buildFrame(3, width, height, []panel{panelRect, panel{x: 70, y: 50, w: 20, h: 20, value: 100}}),
		// A change in the opposite corner.
		buildFrame(4, width, height, []panel{panelRect, panel{x: 230, y: 170, w: 20, h: 20, value: 100}}),
	}

	differ := New()
	regions, _, err := differ.Compare(frames[0], frames[1], defaults())
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if len(regions) == 0 {
		t.Fatal("the panel appearing produced no regions")
	}
	whole := regions[0].Identity
	if reported := pixelRect(regions[0].Bounds, frames[1]); reported.Dx() < 150 {
		t.Fatalf("the engine did not see the whole panel: %v", reported)
	}

	regions, _, err = differ.Compare(frames[1], frames[2], defaults())
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	for _, region := range regions {
		if region.Identity != whole {
			t.Fatalf("a change in one corner became identity %d instead of %d", region.Identity, whole)
		}
	}

	regions, _, err = differ.Compare(frames[2], frames[3], defaults())
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}
	if len(regions) == 0 {
		t.Fatal("the second patch change produced no regions")
	}
	for _, region := range regions {
		if region.Identity != whole {
			t.Fatalf("a change in the opposite corner became identity %d instead of %d", region.Identity, whole)
		}
		if region.IdentityUncertain {
			t.Fatalf("a change inside a tracked element was marked uncertain: %+v", region)
		}
	}
}
