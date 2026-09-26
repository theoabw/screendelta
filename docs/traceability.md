# Requirement traceability

The report is graded on the specification being the source of truth for the
implementation. This file is the evidence: every requirement defined in `specs/`
appears here, with the task that implements it, the test that verifies it and the
evidence that the test ran.

Run `make check` after every change. `scripts/check_traceability.py` fails when:

- a requirement defined in a specification has no row here;
- a row names a specification that does not define its identifier, or omits the
  specification that does, or traverses outside the repository;
- a row appears twice for the same specification and identifier;
- a specification defines the same identifier twice, or an HTML comment is left
  unterminated;
- a status is not one of the allowed values;
- a row marked `in-progress` or `verified` names no task, names something other
  than an identifier such as `T012`, names a task that is not a task entry in the
  feature's `tasks.md`, or names a task while another task entry is the one that
  references the requirement;
- a row marked `verified` names no test or no evidence, or names a path that does
  not exist, is a directory, is absolute, or resolves outside the repository;
- a requirement of a feature that has a `tasks.md` is not referenced there, unless
  its row is `deferred` or `withdrawn`;
- a matrix row's identifier cell looks like an identifier but is malformed.

It warns, rather than fails, in two cases where it cannot judge a value: an
external URL used as evidence, or a value such as `pytest -q` that looks like a
command rather than a file path. A warning is a prompt to look, not a pass, and it
never appears for a path that could have been checked and does not exist.

A requirement is owned by a task when the task entry for that identifier, together
with its continuation lines, references the identifier exactly. `NFR-001` does not
count as `FR-001`, and an entry defined as a checkbox line, a numbered line or a
heading is required: plain bullet prose is not a task entry.

What it cannot check, and what therefore remains a reviewer's job: that the named
test actually exercises the requirement, and that the evidence shows what the row
claims. The check rejects a verification claim whose test or evidence file does not
exist, and rejects a task that is not a real task entry; it cannot tell a relevant
test from an irrelevant one that happens to exist.

Requirement definitions inside fenced code blocks are ignored, because a fence in a
specification is nearly always an example. Never put a real requirement inside a
fence: it will not be checked, and it will not appear in the coverage counts.

A `planned` row may name no task and no test, by design, so a green run is a floor
rather than a statement that everything is implemented. While `specs/` holds no
specification at all, the check warns and succeeds; `make check-strict` runs the
same check with `--require-specs`, which also fails when the specifications define
no requirement at all. Use `make check-strict` from the moment the first
specification exists, and switch the `.gitlab-ci.yml` job to it at the same time.

## Identifier namespaces

Identifiers are namespaced by specification. `FR-001` in `specs/001-alpha/spec.md`
and `FR-001` in `specs/002-beta/spec.md` are different requirements, and a row is
identified by its `Spec` cell plus its `ID` cell. A specification may cite another
specification's identifier, written as `specs/002-beta/spec.md#FR-001`; only bullet
lines of the form `- **FR-001**: ...` count as definitions.

## Status values

| Status | Meaning |
|---|---|
| `planned` | Written in the spec. May name no task yet, and needs no test. |
| `in-progress` | Task started. Must name a task listed in `tasks.md`; needs no test yet. |
| `verified` | Test passes and its output is recorded. Must name a task, an existing test file and existing evidence. |
| `deferred` | Deliberately out of scope for this iteration, with the reason in the report. |
| `withdrawn` | Requirement removed after approval. The identifier is kept and never reused. |

## Conventions

- `Task` holds a task identifier from the feature's `tasks.md`, for example `T012`.
- `Test` holds a path plus a test name, for example
  `tests/test_auth.py::test_rejects_short_password`. The part before `::` must
  exist.
- `Evidence` holds a repository path, for example `docs/vv/results.md` or a file
  under `docs/vv/evidence/`. An anchor such as `docs/vv/results.md#2026-10-12` is
  allowed. Prefer repository paths over external links, because a link cannot be
  checked after the project ends.
- One row per requirement. When several tests cover a requirement, name the most
  decisive one here and list the rest in `docs/vv/results.md`.

## Matrix

The row below is a format example. The checker ignores identifiers ending in
`-000`, so delete the example row when the first real requirement appears.

| ID | Requirement | Spec | User story | Task | Test | Evidence | Status |
|---|---|---|---|---|---|---|---|
| FR-000 | Example row, delete once the first real requirement exists | specs/001-<slug>/spec.md | US1 | T001 | tests/test_example.py::test_example | docs/vv/results.md | planned |
