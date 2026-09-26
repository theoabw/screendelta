# 0004 Third review round: exact identifiers, real fences, real paths

- Date: 2026-09-26
- Author: Theo Wilenius
- Agent and model: dsh, deepseek-flash, plus a third read-only Codex review round
- Spec Kit command: none
- Artifacts produced: `scripts/check_traceability.py`, `Makefile`, documentation alignment
- Related requirement IDs: none yet
- Related commit: the commit that follows, listed in `git log`

## Prompt (verbatim)

> Your job is to verify the claimed fixes and to try once more to falsify the
> checker [...] Priorities for falsification: the definition-versus-reference
> distinction, per-spec namespacing with the same identifier in several specs,
> task-to-tasks.md resolution, the existence checks for test and evidence paths,
> the malformed-row detection, and the comment stripping.

## Intent

Third adversarial pass, on the assumption that two earlier passes each looked
fixed and each still had a hole. The specific worry was identifier matching,
because `FR-001` is a substring of `NFR-001`, and fence handling, because a
specification is full of Markdown examples.

## Expected output

Per-claim verification and any successful falsification, with fixture content.

## Actual output

Three more high and medium defects, each demonstrated:

1. Task ownership used substring membership, so a task entry mentioning
   `NFR-001` satisfied a row for `FR-001`, and a requirement referenced on a
   wrapped continuation line was attributed to the wrong task.
2. A value containing a space was classified as a command before its path was
   examined, so `tests/missing file.py`, `/tmp/missing file.py` and
   `../missing file.py` all passed with warnings.
3. Fence handling toggled on any fence-looking line, so a four-backtick block
   containing three backticks both hid a real requirement outside the block and
   exposed an example inside it.

Six further findings covered a plain bullet being accepted as a task entry while
a heading was rejected, four documentation overclaims, a contradiction between
the V&V plan and the solo review policy, a stale plural-team rationale in the
report skeleton, and qualifications missing from the enforcement description.

## What went wrong

Two things. The checker compared identifiers as text rather than as identifiers,
which is the kind of bug that survives a reading and dies to a fixture. And the
documentation was edited in the same pass as the code, so each fix arrived with
prose describing the intended behaviour rather than the implemented one. The
lesson recorded for the report: after tightening a check, re-read every sentence
that describes it, because the description drifts faster than the code.

## Correction

The checker now matches identifiers exactly, so `NFR-001` never satisfies
`FR-001`; attributes a requirement to a task entry together with its continuation
lines; requires a task entry to be a checkbox line, a numbered line or a heading,
in that documented order; validates absolute, parent-relative and
space-containing paths as errors rather than warnings, leaving a warning only for
a value with no path shape at all; and tracks fence character and length, so a
longer fence is only closed by a fence at least as long.

The Makefile gained `check-strict`, which runs the same check with
`--require-specs`, and the documents now describe the check, its warnings and its
three inherent limits rather than a stronger gate: `README.md`,
`CONTRIBUTING.md`, `docs/traceability.md`, `docs/vv/plan.md`,
`docs/report/report.md` and the workflow diagram.

## Result

A harness of 32 fixtures now covers every falsification from all three review
rounds plus the positive cases: exact identifier ownership, continuation lines,
prose tasks, heading tasks, space-containing paths, directories, outside-repo
paths, four-backtick fences, malformed identifier cells, unterminated comments,
line numbering, per-spec namespacing, cross-feature task leakage, escaped pipes,
deferred and withdrawn exemptions, planned rows, wrong tasks, unknown statuses,
duplicate definitions, orphan rows, and strict mode with an empty or absent
specification. All 32 behaved as intended, and the harness lives outside the
repository, so no test scaffold is committed before the stack is chosen.

The remaining known limits, which no static check can close, are stated in
`docs/traceability.md`: a test that exists but does not exercise the requirement,
evidence that does not show what the row claims, a `planned` row that names
nothing, and a real requirement hidden inside a code fence.
