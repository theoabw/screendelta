# 0003 Second review round: closing the remaining traceability gaps

- Date: 2026-09-26
- Author: Theo Wilenius
- Agent and model: dsh, deepseek-flash, plus a second read-only Codex review round
- Spec Kit command: none
- Artifacts produced: `scripts/check_traceability.py`, documentation alignment, CI note
- Related requirement IDs: none yet
- Related commit: the fix commit that follows, listed in `git log`

## Prompt (verbatim)

The second review was dispatched with this brief, after the first round of fixes:

> Verify, one by one, whether the previously reported findings are actually
> resolved, and check for regressions or new defects introduced by the rework.
> [...] Then attempt to falsify the new checker on its own terms: construct
> adversarial fixtures in a scratch copy under /tmp and try to make it pass
> something it should reject, or reject something it should accept.

## Intent

The first round fixed the obvious holes. This round asked the reviewer to try to
break the fixed checker rather than to re-read it, because a check that only
enforces "the field is not empty" passes its own documentation and fails the
purpose.

## Expected output

A per-finding verdict, plus successful falsification attempts with fixture content
and observed output.

## Actual output

No critical findings. Two high findings, both demonstrated with fixtures:

1. Task resolution still accepted prose. A `tasks.md` containing only
   "T001 is not a task. FR-001 is not implemented." satisfied a verified row.
   A row could also name the wrong task: when `T002` was the task that referenced
   `FR-001`, a row naming `T001` still passed. Task identifiers could additionally
   leak across features through reference lines.
2. File existence was only checked for a whitelist of extensions. A nonexistent
   `tests/missing.cpp` passed with a warning, a directory named `tests/test_a.py`
   passed silently, and a path such as `../valid/tests/test_a.py` outside the
   repository passed.

Six medium findings followed: malformed rows were still skipped when the
identifier cell used backticks or fewer than three digits; an unterminated HTML
comment hid everything after it while still passing; fenced code examples were
counted as requirement definitions; `.lstrip("./")` normalised `../specs/...`
into a valid path; and several documents still claimed more enforcement than the
code provided. Five low findings covered line numbers in diagnostics, a stale
analysis filename convention, and incomplete trailer rules.

## What went wrong

The first fix treated the checker as a string validator and the documentation as
prose. Both drifted from what the code did: the checker accepted any identifier
occurrence as a task and any path with a familiar extension, and three documents
described a gate stronger than the implementation. The second falsification round
found all of it within one pass, which is the argument for adversarial review over
re-reading.

## Correction

The checker now:

- parses task entries, not mentions: a checkbox line, a numbered line or a heading
  that begins with a task identifier, so prose cannot satisfy a task reference;
- resolves the named task inside the feature's own `tasks.md`, and fails when the
  requirement is referenced by a different task entry;
- requires a verified row's test and evidence to resolve to existing files inside
  the repository, treating directories, absolute paths and parent-relative paths
  as errors, and warning only for values it genuinely cannot judge, such as an
  external URL or a command string;
- reports malformed identifier cells for backticked, bold and short forms;
- errors on an unterminated HTML comment instead of silently passing;
- ignores fenced code blocks when looking for requirement definitions;
- keeps diagnostic line numbers correct by blanking comments in place rather than
  deleting them.

The documents were then aligned with the code: `README.md`, `CONTRIBUTING.md`,
`docs/traceability.md`, `docs/vv/plan.md`, `specs/README.md`,
`docs/adr/0001-adopt-spec-kit.md` and the workflow diagram now describe what the
check does, including that a `planned` row may name no task or test, that some
values produce warnings rather than failures, and that CI should be switched to
`--require-specs` once the first specification exists.

## Result

Twelve adversarial fixtures from the review now behave as intended: prose tasks,
wrong-task mapping, cross-feature task references, unknown extensions,
directories in place of files, paths outside the repository, parent-relative spec
paths, backticked and short identifiers, unterminated comments and fenced
examples. The regression suite still passes: two specifications reusing `FR-001`,
escaped pipes, deferred and withdrawn exemptions, `--require-specs` on an empty
specification, and the repository's own empty state. Diagnostic line numbers were
confirmed against the physical file. The remaining known limits, which no static
check can close, are stated in `docs/traceability.md`: a test that exists but does
not exercise the requirement, and an edit to requirement text that leaves the
identifier unchanged.
