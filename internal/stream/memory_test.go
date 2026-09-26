package stream

import (
	"runtime"
	"testing"
	"time"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/frame"
)

// cheapDiffer exists because these tests measure the stream loop, not the comparison. It
// returns no regions and a fingerprint it reuses, so the cost being measured is the
// engine's buffers and bookkeeping rather than hashing.
type cheapDiffer struct {
	cells []int
}

func (d *cheapDiffer) Compare(frame.Frame, frame.Frame, config.Config) ([]delta.Region, []delta.Condition, error) {
	return nil, nil, nil
}

func (d *cheapDiffer) Fingerprint(_ frame.Frame, cfg config.Config) (delta.Fingerprint, error) {
	size := cfg.Fingerprint.GridSize
	if d.cells == nil {
		d.cells = make([]int, size*size)
	}
	return delta.Fingerprint{
		Algorithm:  "cheap-1",
		GridSize:   size,
		Cells:      d.cells,
		StrictHash: "0123456789abcdef",
	}, nil
}

func frameAt(sequence uint64, width, height int, buffer []byte) frame.Frame {
	return frame.Frame{
		Sequence:    sequence,
		Width:       width,
		Height:      height,
		Format:      frame.FormatRGBA8,
		ScaleFactor: 1,
		Pixels:      buffer,
	}
}

// TestAllocationsDoNotGrowWithStreamLength is the guard behind NFR-003: a stream of
// constant geometry must reach a steady state rather than allocate a little more each
// frame. Two consecutive windows are compared, so a slow leak shows up as growth even
// though any single window looks small.
func TestAllocationsDoNotGrowWithStreamLength(t *testing.T) {
	if testing.Short() {
		t.Skip("allocation measurement is skipped in short mode")
	}

	const (
		width, height = 320, 240
		window        = 1000
	)
	engine, err := New(config.Defaults(), &cheapDiffer{})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	pixels := make([]byte, width*height*4)
	sequence := uint64(0)
	runWindow := func() uint64 {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		for index := 0; index < window; index++ {
			sequence++
			if _, err := engine.Push(frameAt(sequence, width, height, pixels)); err != nil {
				t.Fatalf("Push failed at frame %d: %v", sequence, err)
			}
		}
		runtime.ReadMemStats(&after)
		return (after.TotalAlloc - before.TotalAlloc) / window
	}

	// The first window includes the buffers the engine allocates once, so it is discarded
	// as warm-up and the next two are compared.
	_ = runWindow()
	first := runWindow()
	second := runWindow()

	t.Logf("bytes allocated per frame: first window %d, second window %d", first, second)
	if second > first+first/20+512 {
		t.Fatalf("allocation per frame grew from %d to %d bytes, so the stream is not in a steady state", first, second)
	}
	if engine.BufferAllocations() != 2 {
		t.Fatalf("the engine allocated %d pixel buffers, want exactly 2 for a stream of constant geometry", engine.BufferAllocations())
	}
}

// TestMemoryCeiling is NFR-003: at most 128 MB resident over 10,000 frames, with no growth
// attributable to stream length.
func TestMemoryCeiling(t *testing.T) {
	if testing.Short() {
		t.Skip("the memory ceiling measurement is skipped in short mode")
	}

	const (
		width, height = 1920, 1080
		frames        = 10000
		ceilingBytes  = 128 << 20
		sampleEvery   = 1000
	)

	cfg := config.Defaults()
	// A small fingerprint grid keeps the measurement about the stream's pixel buffers;
	// the comparison cost belongs to the diff package's own benchmarks.
	cfg.Fingerprint.GridSize = 8

	engine, err := New(cfg, &cheapDiffer{})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	pixels := make([]byte, width*height*4)
	started := time.Now()
	var peakAlloc, peakSys, firstSample, lastSample uint64

	for sequence := uint64(1); sequence <= frames; sequence++ {
		if _, err := engine.Push(frameAt(sequence, width, height, pixels)); err != nil {
			t.Fatalf("Push failed at frame %d: %v", sequence, err)
		}
		if sequence%sampleEvery != 0 {
			continue
		}
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		if stats.HeapAlloc > peakAlloc {
			peakAlloc = stats.HeapAlloc
		}
		if stats.Sys > peakSys {
			peakSys = stats.Sys
		}
		if sequence == sampleEvery {
			firstSample = stats.HeapAlloc
		}
		lastSample = stats.HeapAlloc
	}

	elapsed := time.Since(started)
	t.Logf("streamed %d frames at %dx%d in %s", frames, width, height, elapsed.Round(time.Millisecond))
	t.Logf("peak heap %d bytes, peak system %d bytes, engine buffers %d bytes",
		peakAlloc, peakSys, engine.BufferBytes())
	t.Logf("heap at frame %d: %d bytes, at frame %d: %d bytes", sampleEvery, firstSample, frames, lastSample)

	if peakAlloc > ceilingBytes {
		t.Fatalf("peak heap %d bytes exceeds the %d byte ceiling", peakAlloc, ceilingBytes)
	}
	// Growth attributable to stream length: the last sample must not be materially larger
	// than the first, since the buffers are fixed.
	if lastSample > firstSample+8<<20 {
		t.Fatalf("heap grew from %d to %d bytes over the stream, which suggests state accumulating per frame", firstSample, lastSample)
	}
	if engine.BufferBytes() != 2*width*height*4 {
		t.Fatalf("engine holds %d buffer bytes, want %d", engine.BufferBytes(), 2*width*height*4)
	}
}
