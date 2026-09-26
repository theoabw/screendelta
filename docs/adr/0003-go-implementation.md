# 0003 Implement ScreenDelta in Go

- Status: accepted
- Date: 2026-09-26
- Deciders: repository owner

## Context

The constitution fixes the constraints for the implementation language but not the
language itself: single-command distribution, predictable numeric behaviour, and a
native image path fast enough for a p95 of 12 ms and a p99 of 25 ms per 1080p frame
pair on one CPU core, with a 128 MB ceiling over a 10,000 frame stream.

The component runs on the machine that captures the screen, which for legacy GUI
navigation means Windows desktops as well as Linux. It must be distributable as one
binary with no runtime dependencies, because users install it next to the thing it
observes rather than into a managed environment.

## Decision

Implement ScreenDelta in Go.

1. **Toolchain already present.** Go is installed on the development workstation,
   together with `golangci-lint` and `govulncheck`, which supply linting and
   dependency-vulnerability evidence for the verification chapter at no extra cost.
2. **Cross-compilation.** `GOOS=windows GOARCH=amd64` produces the Windows binary
   from Linux with no toolchain juggling, which matters because Windows is the
   platform where legacy GUI navigation happens.
3. **Packaging.** `nfpm` is already available for building redistributable
   packages, which supports the publish-after-the-course goal.
4. **Language familiarity.** The author already maintains a Go daemon with a
   documented wire protocol, so the effort goes into the engine rather than into
   learning a language.
5. **Latency budget is reachable.** The workload is tile-wise comparison and small
   connected-component work over a 1920 by 1080 buffer. With pooled buffers, no
   per-frame allocation growth, and `GOMAXPROCS=1` for the measured configuration,
   the tail latency target is achievable without fighting a collector.

## Consequences

- Garbage collection is the main technical risk to the p99 target. Mitigations are
  fixed-size buffer pools owned by the stream, reused region slices, and a
  benchmark that fails the build when allocation per frame grows. If the p99 target
  proves unreachable, the recorded alternative is a Rust rewrite of the hot
  package, and the JSON contract means that rewrite would not disturb consumers.
- Go's image decoders live in the standard library and `golang.org/x/image`, so the
  dependency surface stays small and auditable.
- Property testing will use a dedicated library rather than hand-rolled generators,
  because the identity and moved-region rules are exactly the kind of invariants
  that benefit from generated inputs.
- Benchmarks run with `benchstat` comparisons, which gives the report reproducible
  performance evidence instead of a single timing.

## Alternatives considered

| Alternative | Why not |
|---|---|
| Rust | Stronger tail-latency story because there is no collector, and the more common choice for this kind of tool, but the toolchain is not installed on the workstation, iteration is slower, and the correctness benefit for tile comparison and small connected components is small. Kept as the documented fallback for the hot package. |
| Python with NumPy | Fastest to write and the corpus tooling is Python anyway, but PNG decode plus per-frame interpreter overhead makes the p95 target tight, and the "performance" story of the project would rest on a library rather than on the work. |
| C or C++ | Best possible control over memory and vectorisation, but manual memory safety management adds a class of defects that the verification budget would have to absorb. |
| Reuse an existing diffing library | No maintained library exposes session-stable element identity, uncertainty semantics, or a versioned delta document, so the interesting part would remain unwritten. |
