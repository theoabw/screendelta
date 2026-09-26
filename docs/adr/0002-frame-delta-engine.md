# 0002 Build a frame delta and element identity engine

- Status: accepted
- Date: 2026-09-26
- Deciders: repository owner, on behalf of Group A

## Context

The course requires one system, built through specification-driven development,
with stakeholders, measurable quality targets and machine-checked verification.

The motivating problem is specific. Agents that drive legacy desktop interfaces
are slow because each step sends a whole screenshot to a model and asks what
changed and where things are. Most of that work is redundant: between two frames,
a few regions changed, and the rest of the screen is identical. A component that
computes that difference cheaply, and that can say which element is which across
frames, removes the redundancy for every later stage in the pipeline.

Several candidate systems were considered and rejected in `docs/ideas.md`. Process
and compliance tooling was rejected as uninteresting to build. Consumer
applications such as expense splitting were rejected as already solved by MobilePay,
Revolut and similar. A general GUI parser that recovers a full element tree from
pixels, optionally with a trained model, was rejected as too large for a three-week
project, and it is being kept as a separate side project.

## Decision

Build ScreenDelta, a deterministic frame delta and element identity engine, as one
bounded stage of a larger GUI-navigation pipeline.

1. **Scope.** Rectangular regions on desktop screenshots. The engine consumes
   frames from any source, emits changed, added, removed and moved regions with
   normalized bounds and a change magnitude, assigns stable identifiers per
   session, and produces a screen fingerprint for plan caching.
2. **Boundaries.** No OCR, no element classification, no natural-language
   grounding, no model training, no capture implementation beyond file and stdin
   adapters. Those are later stages, and the engine must not depend on them.
3. **Interface.** A versioned JSON delta document, newline-delimited when
   streaming, plus a CLI and a library exposing the same schema. Externally
   supplied regions of interest are accepted so that later stages can constrain the
   engine rather than replace it.
4. **Failure semantics.** Never fabricate: no removal reported below the noise
   floor, no identifier reuse within a session, and explicit uncertainty rather
   than a guessed identity match.
5. **Verification.** Ground truth is generated rather than hand-labelled, by
   rendering pages in a headless browser and reading element boxes from the DOM,
   and by synthesizing frame sequences with known motion, appearance and
   disappearance. Accuracy is reported as F1 against that corpus, latency and
   memory as percentiles from a committed benchmark.
6. **Language.** Deferred to the plan. The constraints are stated in the
   constitution: single-command distribution, predictable numeric behaviour, and a
   native image path fast enough for the latency budget.

## Consequences

- The course project is small enough to finish well: no training loop, no labelling
  campaign, and no floating accuracy ceiling.
- Requirements come from a schema the project defines, so traceability from
  requirement to test is direct.
- The component is useful on its own, for visual regression testing, session
  recording and remote desktop delta encoding, which makes it publishable
  separately from the pipeline it was written for.
- The rest of the pipeline is not built. The engine's value depends on later stages
  existing, so the README must state the pipeline position honestly rather than
  implying an end-to-end product.
- Element identity across occlusion is the hardest part and the most likely to need
  scope reduction during implementation. The uncertainty marker exists precisely so
  that a partial capability is still honest.

## Alternatives considered

| Alternative | Why not |
|---|---|
| Full GUI parser with optional trained model | Too large for three weeks, and a learned core makes requirement traceability statistical rather than deterministic. Kept as a side project. |
| Linear-time regex engine with a ReDoS scanner | Bounded and verifiable, but it competes directly with mature tools and does not address the motivating problem. |
| Shared expense splitting service | Solved by existing consumer products, so the motivation is weak. |
| Traceability or compliance tooling | Strong course fit, rejected because it is not interesting to build. |
| Vectorised SQL engine | Highest portfolio value, highest risk of not finishing, and its verification story is already well served by existing differential-testing literature. |
