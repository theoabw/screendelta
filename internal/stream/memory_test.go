package stream

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/theoabw/screendelta/internal/config"
	"github.com/theoabw/screendelta/internal/delta"
	"github.com/theoabw/screendelta/internal/diff"
	"github.com/theoabw/screendelta/internal/frame"
)

// cheapDiffer measures the stream loop rather than the comparison. Its fingerprint is a fixed
// value and it allocates nothing, so what the measurement sees is the engine's own buffers and
// bookkeeping.
type cheapDiffer struct {
	cells []int
}

func (d *cheapDiffer) Reset() {}

func (d *cheapDiffer) Compare(frame.Frame, frame.Frame, config.Config) ([]delta.Region, []delta.Condition, error) {
	return nil, nil, nil
}

func (d *cheapDiffer) Fingerprint(_ frame.Frame, cfg config.Config) (delta.Fingerprint, error) {
	// The document requires exactly gridSize squared cells, and the engine copies them, so the
	// differ allocates this once and hands over the same slice every frame.
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

// residentBytes reads the process's resident set size.
//
// HeapAlloc is not resident memory: it counts live heap objects and misses stacks, runtime
// metadata and anything the allocator has not returned. The requirement is stated in resident
// memory, so the measurement reads resident memory. On a platform without the file the
// measurement reports that it could not be taken rather than passing quietly.
func residentBytes() (uint64, bool) {
	file, err := os.Open("/proc/self/statm")
	if err != nil {
		return 0, false
	}
	defer file.Close()

	fields := strings.Fields(readLine(file))
	if len(fields) < 2 {
		return 0, false
	}
	pages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return pages * uint64(os.Getpagesize()), true
}

func readLine(file *os.File) string {
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return ""
	}
	return scanner.Text()
}

// TestAllocationsDoNotGrowWithStreamLength is the guard behind NFR-003: a stream of constant
// geometry must reach a steady state rather than allocate a little more each frame. Two
// consecutive windows are compared, so a slow leak shows up as growth even though a single
// window looks harmless.
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

	// The first window includes the buffers the engine allocates once, so it is discarded as
	// warm-up and the next two are compared.
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

// TestMemoryCeiling is the memory half of NFR-003: at most 128 MB resident while streaming
// 10,000 frames of 1920x1080, with no growth attributable to stream length.
//
// The comparison is stubbed, and that is stated rather than hidden: what this measures is the
// engine's retention over a long stream, which is where a per-frame leak would live. The real
// pipeline's retention is measured separately by TestRealPipelineRetention.
func TestMemoryCeiling(t *testing.T) {
	if testing.Short() {
		t.Skip("the memory ceiling measurement is skipped in short mode")
	}

	const (
		width, height = 1920, 1080
		frames        = 10000
		ceilingBytes  = 128 << 20
		sampleEvery   = 1000
		// A per-frame leak of half a kilobyte, which is small enough to look like noise in a
		// single sample, accumulates to more than four megabytes over the stream. One megabyte
		// of growth over nine thousand frames is therefore a leak and not jitter.
		growthAllowed = 1 << 20
	)

	cfg := config.Defaults()
	cfg.Fingerprint.GridSize = 8

	engine, err := New(cfg, &cheapDiffer{})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	pixels := make([]byte, width*height*4)
	started := time.Now()
	var peakAlloc, peakResident, firstAlloc, lastAlloc uint64
	residentAvailable := true

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
		if resident, ok := residentBytes(); ok {
			if resident > peakResident {
				peakResident = resident
			}
		} else {
			residentAvailable = false
		}
		if sequence == sampleEvery {
			firstAlloc = stats.HeapAlloc
		}
		lastAlloc = stats.HeapAlloc
	}

	t.Logf("streamed %d frames at %dx%d in %s", frames, width, height, time.Since(started).Round(time.Millisecond))
	t.Logf("peak heap %d bytes (%.1f MiB), engine buffers %d bytes", peakAlloc, float64(peakAlloc)/(1<<20), engine.BufferBytes())
	if residentAvailable {
		t.Logf("peak resident %d bytes (%.1f MiB)", peakResident, float64(peakResident)/(1<<20))
	} else {
		t.Logf("resident memory could not be read on this platform, so only the heap is claimed")
	}
	t.Logf("heap at frame %d: %d bytes, at frame %d: %d bytes", sampleEvery, firstAlloc, frames, lastAlloc)

	if peakAlloc > ceilingBytes {
		t.Fatalf("peak heap %d bytes exceeds the %d byte ceiling", peakAlloc, ceilingBytes)
	}
	if residentAvailable && peakResident > ceilingBytes {
		t.Fatalf("peak resident %d bytes exceeds the %d byte ceiling", peakResident, ceilingBytes)
	}
	if lastAlloc > firstAlloc+growthAllowed {
		t.Fatalf("heap grew %d bytes between frame %d and frame %d, which is more than the %d bytes allowed and suggests state accumulating per frame",
			lastAlloc-firstAlloc, sampleEvery, frames, growthAllowed)
	}
	if engine.BufferBytes() != 2*width*height*4 {
		t.Fatalf("engine holds %d buffer bytes, want %d", engine.BufferBytes(), 2*width*height*4)
	}
}

// TestRealPipelineRetention streams the real differ and fingerprint over a long sequence and
// asserts that nothing accumulates. The frames are smaller than in the ceiling test because the
// subject here is retention rather than the size of the working set, and running the real
// comparison ten thousand times at 1080p would take minutes without measuring anything further.
func TestRealPipelineRetention(t *testing.T) {
	if testing.Short() {
		t.Skip("the retention measurement is skipped in short mode")
	}

	const (
		width, height = 320, 240
		frames        = 10000
		sampleEvery   = 1000
		growthAllowed = 1 << 20
	)

	cfg := config.Defaults()
	engine, err := New(cfg, diff.New())
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer engine.Close()

	pixels := make([]byte, width*height*4)
	for index := range pixels {
		pixels[index] = uint8(index % 251)
	}

	started := time.Now()
	var firstAlloc, lastAlloc, peakResident uint64
	regions := 0

	for sequence := uint64(1); sequence <= frames; sequence++ {
		// Every frame shifts one band, so the differ has real work: changes to find, regions to
		// remember and identities to allocate.
		for x := 0; x < width; x++ {
			offset := ((int(sequence)%height)*width + x) * 4
			pixels[offset] = uint8((int(sequence) * 7) % 251)
		}
		f := frameAt(sequence, width, height, pixels)
		document, err := engine.Push(f)
		if err != nil {
			t.Fatalf("Push failed at frame %d: %v", sequence, err)
		}
		regions += len(document.Regions)

		if sequence%sampleEvery != 0 {
			continue
		}
		runtime.GC()
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		if sequence == sampleEvery {
			firstAlloc = stats.HeapAlloc
		}
		lastAlloc = stats.HeapAlloc
		if resident, ok := residentBytes(); ok && resident > peakResident {
			peakResident = resident
		}
	}

	t.Logf("streamed %d frames through the real pipeline in %s, %d regions reported",
		frames, time.Since(started).Round(time.Millisecond), regions)
	t.Logf("heap at frame %d: %d bytes, at frame %d: %d bytes, peak resident %d bytes",
		sampleEvery, firstAlloc, frames, lastAlloc, peakResident)

	if lastAlloc > firstAlloc+growthAllowed {
		t.Fatalf("the real pipeline grew %d bytes between frame %d and frame %d, more than the %d bytes allowed",
			lastAlloc-firstAlloc, sampleEvery, frames, growthAllowed)
	}
}
