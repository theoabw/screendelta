package stream

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/fielderr"
	"github.com/theoabw/screendelta/internal/frame"
)

// stubDiffer records what it was asked and returns whatever the test configured, so
// the loop can be tested without the image code.
type stubDiffer struct {
	regions        []delta.Region
	conditions     []delta.Condition
	compareCalls   int
	seenPrevious   []string
	seenCurrent    []string
	err            error
	fingerprintErr error
}

func (s *stubDiffer) Compare(previous, current frame.Frame, _ config.Config) ([]delta.Region, []delta.Condition, error) {
	s.compareCalls++
	s.seenPrevious = append(s.seenPrevious, checksum(previous.Pixels))
	s.seenCurrent = append(s.seenCurrent, checksum(current.Pixels))
	if s.err != nil {
		return nil, nil, s.err
	}
	return s.regions, s.conditions, nil
}

func (s *stubDiffer) Fingerprint(current frame.Frame, cfg config.Config) (delta.Fingerprint, error) {
	if s.fingerprintErr != nil {
		return delta.Fingerprint{}, s.fingerprintErr
	}
	size := cfg.Fingerprint.GridSize
	return delta.Fingerprint{
		Algorithm:  "stub-1",
		GridSize:   size,
		Cells:      make([]int, size*size),
		StrictHash: checksum(current.Pixels)[:16],
	}, nil
}

func checksum(pixels []byte) string {
	sum := sha256.Sum256(pixels)
	return hex.EncodeToString(sum[:])
}

func testFrame(sequence uint64, width, height int, fill byte) frame.Frame {
	pixels := bytes.Repeat([]byte{fill}, width*height*4)
	return frame.Frame{
		Sequence:    sequence,
		Width:       width,
		Height:      height,
		Format:      frame.FormatRGBA8,
		ScaleFactor: 1,
		Pixels:      pixels,
	}
}

func newEngine(t *testing.T, differ Differ) *Engine {
	t.Helper()
	engine, err := New(config.Defaults(), differ)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	return engine
}

func TestFirstFrameCarriesTheConditionAndNoRegions(t *testing.T) {
	differ := &stubDiffer{}
	engine := newEngine(t, differ)

	document, err := engine.Push(testFrame(1, 8, 8, 0x10))
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	if len(document.Regions) != 0 {
		t.Fatalf("the first frame reported %d regions, want none", len(document.Regions))
	}
	if len(document.Conditions) != 1 || document.Conditions[0] != delta.ConditionFirstFrame {
		t.Fatalf("conditions = %v, want [first-frame]", document.Conditions)
	}
	if differ.compareCalls != 0 {
		t.Fatalf("the differ was called %d times for a first frame, want 0", differ.compareCalls)
	}
}

func TestSecondFrameIsCompared(t *testing.T) {
	differ := &stubDiffer{
		regions: []delta.Region{
			{Identity: 1, Class: delta.ClassChanged, Bounds: delta.Bounds{X: 0, Y: 0, W: 0.5, H: 0.5}, Magnitude: 0.5, AreaPixels: 16},
		},
	}
	engine := newEngine(t, differ)

	if _, err := engine.Push(testFrame(1, 8, 8, 0x10)); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	document, err := engine.Push(testFrame(2, 8, 8, 0x20))
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	if differ.compareCalls != 1 {
		t.Fatalf("the differ was called %d times, want 1", differ.compareCalls)
	}
	if len(document.Regions) != 1 {
		t.Fatalf("regions = %d, want 1", len(document.Regions))
	}
	if document.Frame.Sequence != 2 {
		t.Fatalf("document names frame %d, want 2", document.Frame.Sequence)
	}
}

func TestViewportChangeSkipsTheComparison(t *testing.T) {
	differ := &stubDiffer{regions: []delta.Region{{Identity: 1, Class: delta.ClassChanged, Bounds: delta.Bounds{W: 0.5, H: 0.5}, Magnitude: 0.5, AreaPixels: 16}}}
	engine := newEngine(t, differ)

	if _, err := engine.Push(testFrame(1, 8, 8, 0x10)); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	larger := testFrame(2, 16, 8, 0x10)
	document, err := engine.Push(larger)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	if differ.compareCalls != 0 {
		t.Fatalf("the differ was called %d times across a viewport change, want 0", differ.compareCalls)
	}
	if len(document.Regions) != 0 {
		t.Fatalf("a viewport change reported %d regions, want none", len(document.Regions))
	}
	if len(document.Conditions) != 1 || document.Conditions[0] != delta.ConditionViewportChanged {
		t.Fatalf("conditions = %v, want [viewport-changed]", document.Conditions)
	}
}

func TestSequenceMustIncrease(t *testing.T) {
	engine := newEngine(t, &stubDiffer{})
	if _, err := engine.Push(testFrame(5, 8, 8, 0x10)); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	for _, sequence := range []uint64{5, 4, 1} {
		if _, err := engine.Push(testFrame(sequence, 8, 8, 0x10)); err == nil {
			t.Fatalf("Push accepted sequence %d after 5", sequence)
		}
	}
}

func TestCallerMayReuseItsBufferImmediately(t *testing.T) {
	differ := &stubDiffer{}
	engine := newEngine(t, differ)

	firstPixels := bytes.Repeat([]byte{0x11}, 8*8*4)
	first := frame.Frame{Sequence: 1, Width: 8, Height: 8, Format: frame.FormatRGBA8, ScaleFactor: 1, Pixels: firstPixels}
	if _, err := engine.Push(first); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// The caller overwrites its own buffer, which is allowed because the engine copied
	// it. The previous frame the differ sees must still be the original content.
	for index := range firstPixels {
		firstPixels[index] = 0xEE
	}

	second := testFrame(2, 8, 8, 0x22)
	if _, err := engine.Push(second); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	want := checksum(bytes.Repeat([]byte{0x11}, 8*8*4))
	if differ.seenPrevious[0] != want {
		t.Fatalf("the engine retained the caller's buffer instead of copying it")
	}
}

func TestStreamOfConstantGeometryDoesNotKeepAllocating(t *testing.T) {
	engine := newEngine(t, &stubDiffer{})

	for sequence := uint64(1); sequence <= 200; sequence++ {
		if _, err := engine.Push(testFrame(sequence, 32, 32, byte(sequence))); err != nil {
			t.Fatalf("Push failed at frame %d: %v", sequence, err)
		}
	}
	if allocations := engine.BufferAllocations(); allocations != 2 {
		t.Fatalf("allocated %d pixel buffers over 200 frames, want 2", allocations)
	}
	if held := engine.BufferBytes(); held != 2*32*32*4 {
		t.Fatalf("holding %d buffer bytes, want %d", held, 2*32*32*4)
	}

	// A geometry change is allowed to allocate, and only then.
	if _, err := engine.Push(testFrame(201, 64, 64, 0x0F)); err != nil {
		t.Fatalf("Push failed after the geometry change: %v", err)
	}
	if allocations := engine.BufferAllocations(); allocations <= 2 {
		t.Fatalf("a geometry change allocated nothing, which cannot be right")
	}
}

func TestPushRejectsAnInvalidFrame(t *testing.T) {
	engine := newEngine(t, &stubDiffer{})
	broken := testFrame(1, 8, 8, 0x10)
	broken.Pixels = broken.Pixels[:10]
	if _, err := engine.Push(broken); err == nil {
		t.Fatal("Push accepted a frame whose buffer does not match its dimensions")
	}
}

func TestCloseIsIdempotentAndPushAfterCloseFails(t *testing.T) {
	engine := newEngine(t, &stubDiffer{})
	if _, err := engine.Push(testFrame(1, 8, 8, 0x10)); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	if err := engine.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := engine.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
	if _, err := engine.Push(testFrame(2, 8, 8, 0x10)); err == nil {
		t.Fatal("Push succeeded after Close")
	}
	if engine.BufferBytes() != 0 {
		t.Fatalf("Close left %d buffer bytes behind", engine.BufferBytes())
	}
}

func TestDocumentsAreDeterministicAcrossEngines(t *testing.T) {
	run := func() []byte {
		engine := newEngine(t, &stubDiffer{
			regions: []delta.Region{
				{Identity: 2, Class: delta.ClassChanged, Bounds: delta.Bounds{X: 0.5, Y: 0.5, W: 0.1, H: 0.1}, Magnitude: 0.3, AreaPixels: 1},
				{Identity: 1, Class: delta.ClassChanged, Bounds: delta.Bounds{X: 0.1, Y: 0.1, W: 0.1, H: 0.1}, Magnitude: 0.2, AreaPixels: 1},
			},
			conditions: []delta.Condition{delta.ConditionViewportChanged, delta.ConditionOutOfOrderTimestamp},
		})
		var out bytes.Buffer
		for sequence := uint64(1); sequence <= 5; sequence++ {
			document, err := engine.Push(testFrame(sequence, 8, 8, byte(sequence)))
			if err != nil {
				t.Fatalf("Push failed: %v", err)
			}
			if err := document.Encode(&out, false); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
		}
		return out.Bytes()
	}

	if !bytes.Equal(run(), run()) {
		t.Fatal("two runs over the same input produced different bytes")
	}
}

func TestDifferErrorsAreAttributedToTheFrame(t *testing.T) {
	differ := &stubDiffer{err: errors.New("comparison exploded")}
	engine := newEngine(t, differ)
	if _, err := engine.Push(testFrame(1, 8, 8, 0x10)); err != nil {
		t.Fatalf("Push failed on the first frame: %v", err)
	}
	_, err := engine.Push(testFrame(2, 8, 8, 0x10))
	if err == nil {
		t.Fatal("Push hid a differ error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("frame 2")) {
		t.Fatalf("error does not name the frame: %q", err.Error())
	}
}

func TestNewRejectsAMissingDiffer(t *testing.T) {
	if _, err := New(config.Defaults(), nil); err == nil {
		t.Fatal("New accepted a nil differ")
	}
}

// reusingDiffer behaves like a streaming implementation that pools its own output
// storage, which is what the ownership rule has to survive.
type reusingDiffer struct {
	regions     []delta.Region
	cells       []int
	previous    delta.Bounds
	frameNumber int
	shared      *fielderr.Error
}

func (d *reusingDiffer) Compare(previous, current frame.Frame, cfg config.Config) ([]delta.Region, []delta.Condition, error) {
	d.frameNumber = int(current.Sequence)
	if d.shared != nil {
		return nil, nil, d.shared
	}
	d.previous = delta.Bounds{X: float64(d.frameNumber) / 100, Y: 0, W: 0.1, H: 0.1}
	d.regions = []delta.Region{{
		Identity:       1,
		Class:          delta.ClassMoved,
		Bounds:         delta.Bounds{X: 0, Y: 0, W: 0.1, H: 0.1},
		PreviousBounds: &d.previous,
		Magnitude:      float64(d.frameNumber) / 10,
		AreaPixels:     1,
	}}
	return d.regions, nil, nil
}

func (d *reusingDiffer) Fingerprint(current frame.Frame, cfg config.Config) (delta.Fingerprint, error) {
	size := cfg.Fingerprint.GridSize
	if d.cells == nil {
		d.cells = make([]int, size*size)
	}
	d.cells[0] = int(current.Sequence)
	return delta.Fingerprint{
		Algorithm:  "reuse-1",
		GridSize:   size,
		Cells:      d.cells,
		StrictHash: checksum(current.Pixels)[:16],
	}, nil
}

func TestRetainedDocumentsDoNotAliasDifferStorage(t *testing.T) {
	differ := &reusingDiffer{}
	engine := newEngine(t, differ)

	var documents []delta.Document
	for sequence := uint64(1); sequence <= 3; sequence++ {
		document, err := engine.Push(testFrame(sequence, 8, 8, byte(sequence)))
		if err != nil {
			t.Fatalf("Push failed: %v", err)
		}
		documents = append(documents, document)
	}

	if len(documents[0].Regions) != 0 {
		t.Fatalf("the first frame reported %d regions, want none", len(documents[0].Regions))
	}
	if got := documents[0].Fingerprint.Cells[0]; got != 1 {
		t.Fatalf("the first document's fingerprint now reads %d, so it aliases the differ's storage", got)
	}
	if got := documents[1].Regions[0].Magnitude; got != 0.2 {
		t.Fatalf("the second document's magnitude now reads %v, so it aliases the differ's storage", got)
	}
	if got := documents[1].Regions[0].PreviousBounds.X; got != 0.02 {
		t.Fatalf("the second document's previous bounds now read %v, so they alias the differ's storage", got)
	}
	if got := documents[2].Regions[0].Magnitude; got != 0.3 {
		t.Fatalf("the third document's magnitude now reads %v", got)
	}
}

func TestDifferErrorsNameTheFailingOperation(t *testing.T) {
	cases := []struct {
		name         string
		differ       *stubDiffer
		operation    string
		expectFrame  string
		failsOnFirst bool
	}{
		{name: "compare", differ: &stubDiffer{err: errors.New("comparison exploded")}, operation: "compare", expectFrame: "frame 2"},
		// The fingerprint is computed for every frame, including the first, so this
		// failure is reported against frame 1.
		{name: "fingerprint", differ: &stubDiffer{fingerprintErr: errors.New("fingerprint exploded")}, operation: "fingerprint", expectFrame: "frame 1", failsOnFirst: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := newEngine(t, tc.differ)
			_, firstErr := engine.Push(testFrame(1, 8, 8, 0x10))
			if tc.failsOnFirst {
				if firstErr == nil {
					t.Fatal("Push hid the fingerprint error on the first frame")
				}
			} else if firstErr != nil {
				t.Fatalf("Push failed on the first frame: %v", firstErr)
			}

			var err error
			if tc.failsOnFirst {
				err = firstErr
			} else {
				_, err = engine.Push(testFrame(2, 8, 8, 0x10))
			}
			if err == nil {
				t.Fatal("Push hid a differ error")
			}
			if !strings.Contains(err.Error(), tc.operation) {
				t.Fatalf("error does not name the failing operation %q: %q", tc.operation, err.Error())
			}
			if !strings.Contains(err.Error(), tc.expectFrame) {
				t.Fatalf("error does not name %s: %q", tc.expectFrame, err.Error())
			}
		})
	}
}

func TestWrappingADifferErrorDoesNotRewriteIt(t *testing.T) {
	shared := &fielderr.Error{Op: "differ.Compare", Subject: "frame", Sequence: 7, Field: "tile", Problem: "shared failure"}
	differ := &stubDiffer{}
	engine := newEngine(t, differ)
	if _, err := engine.Push(testFrame(1, 8, 8, 0x10)); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// The same error value is handed back for a later frame, which a pooled differ may
	// legitimately do. The message must name the frame that actually failed.
	differ.err = shared
	if _, err := engine.Push(testFrame(2, 8, 8, 0x10)); err == nil {
		t.Fatal("Push hid the shared error")
	}
	if shared.Sequence != 7 {
		t.Fatalf("wrapping rewrote the caller's error to frame %d", shared.Sequence)
	}

	differ.err = nil
	if _, err := engine.Push(testFrame(3, 8, 8, 0x10)); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	differ.err = shared
	_, err := engine.Push(testFrame(4, 8, 8, 0x10))
	if err == nil {
		t.Fatal("Push hid the shared error")
	}
	// The engine knows which frame it handed to the differ, so that is the frame the
	// message must name, however stale the error value's own number is.
	if !strings.Contains(err.Error(), "frame 4") {
		t.Fatalf("error names a stale frame: %q", err.Error())
	}
	if shared.Sequence != 7 {
		t.Fatalf("wrapping rewrote the caller's error to frame %d", shared.Sequence)
	}
}

func TestDifferErrorKeepsItsCause(t *testing.T) {
	sentinel := errors.New("disk on fire")
	engine := newEngine(t, &stubDiffer{err: sentinel})
	if _, err := engine.Push(testFrame(1, 8, 8, 0x10)); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	_, err := engine.Push(testFrame(2, 8, 8, 0x10))
	if err == nil {
		t.Fatal("Push hid the error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("errors.Is no longer finds the cause: %v", err)
	}
}
