# Requirement traceability

The report is graded on the specification being the source of truth for the
implementation. This file is the evidence: every requirement defined in `specs/`
appears here, with the task that implements it, the test that verifies it and the
evidence that the test ran.

Run `make check` after every change. `scripts/check_traceability.py` fails when:

- a requirement defined in a specification has no row here;
- a row names a specification that does not define its identifier, or omits the
  specification that does;
- a row appears twice for the same specification and identifier;
- a status is not one of the allowed values;
- a row marked `in-progress` or `verified` names no task, names a task that is not
  an identifier such as `T012`, or names a task that the feature's `tasks.md` does
  not list;
- a row marked `verified` names no test or no evidence, or names a test file or an
  evidence file that does not exist in the repository;
- a requirement of a feature that has a `tasks.md` is not referenced there, unless
  its row is `deferred` or `withdrawn`.

What it cannot check, and what therefore remains a reviewer's job: that the named
test actually exercises the requirement, and that the evidence really shows what
the row claims. The check makes a false claim fail the build; only review catches
a misleading one.

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
