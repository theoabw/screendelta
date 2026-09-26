# Specification Quality Checklist: Frame Delta and Element Identity Engine

**Purpose**: Validate specification completeness and quality before planning and implementation.
**Created**: 2026-09-26
**Feature**: [spec.md](../spec.md)

This is the built-in specification-quality checklist, maintained by the specification
and clarification steps and re-evaluated when the specification changes. Custom
checklists generated later by the checklist step are reviewer-owned and an agent
must leave their items unticked.

## Content quality

- [x] No implementation details leak into the requirements. The specification names no
      language, library or data structure; the technology stack is decided in `plan.md`
      and `docs/adr/0003-go-implementation.md`.
- [x] Focused on what the system must do rather than how. Interface details appear only
      where the interface is the requirement, such as the schema version field.
- [ ] Written for a non-technical reader. Not satisfied, deliberately: this is a
      component contract for a pipeline, so latency percentiles and normalised bounds
      are the vocabulary. A non-technical summary would lose the precision the
      verification depends on.
- [x] All mandatory sections present: user scenarios with acceptance scenarios, edge
      cases, functional requirements, non-functional requirements, key entities,
      success criteria and assumptions.

## Requirement quality

- [x] Every requirement is testable or measurable. The functional requirements are
      written as observable behaviour, and each non-functional requirement carries a
      number and a measurement method.
- [x] No `[NEEDS CLARIFICATION]` markers remain.
- [x] Success criteria are measurable and stated as outcomes: F1 at or above 0.98 with
      zero false removals, p95 and p99 latency, a memory ceiling, a decision rate.
- [x] Success criteria are technology-agnostic. They name no tool, library or platform.
- [x] Acceptance scenarios are present for all three user stories, in given, when, then
      form.
- [x] Edge cases are enumerated, including identical frames, a theme change, a window
      drag, a scale change, a moving cursor, an overlay, out-of-order timestamps and a
      single-frame stream.
- [x] Scope is bounded, and the boundaries are explicit: no OCR, no element
      classification, no natural-language grounding, rectangles only.
- [x] Dependencies and assumptions are recorded, including the assumption that ground
      truth comes from generated content rather than hand labelling.
- [x] Requirement identifiers are stable and unique within this specification, and the
      rule for referencing another specification's identifier is written down.

## Verification readiness

- [x] Every requirement maps to at least one task in `tasks.md`, enforced by
      `scripts/check_traceability.py`.
- [x] Every requirement has a row in `docs/traceability.md`.
- [x] The measurement method for each non-functional requirement is fixed in
      `research.md` before the first measurement.
- [x] The quickstart scenarios in `quickstart.md` name the requirement each one
      exercises, so a failure report starts with an identifier.

## Notes

- The one unticked item is a deliberate, recorded exception rather than an omission.
  It is revisited in the report when the audience for the specification is discussed.
- Re-evaluate this checklist whenever `spec.md` changes, and record the reason for any
  new exception here rather than in a commit message.
