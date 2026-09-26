# Requirement traceability

The report is graded on the specification being the source of truth for the
implementation. This file is the evidence: every requirement identifier from
`specs/` appears here exactly once, with the task that implements it and the test
that verifies it.

Run `make check` after every change. `scripts/check_traceability.py` fails the
check when a requirement is missing here, when an identifier here no longer
exists in a spec, when a traced requirement has no task or no test, or when a
row marked `verified` has no evidence.

## Status values

| Status | Meaning |
|---|---|
| `planned` | Written in the spec, not yet implemented |
| `in-progress` | Task started, test not passing yet |
| `verified` | Test passes and evidence is recorded in `docs/vv/results.md` |
| `deferred` | Deliberately out of scope for this iteration, with a reason in the report |
| `withdrawn` | Requirement removed after approval; the identifier is kept and never reused |

## Conventions

- `Task` holds the task identifier from `tasks.md` of that feature, for example `T012`.
- `Test` holds a path plus a test name, for example `tests/test_auth.py::test_rejects_short_password`.
- `Evidence` holds a reference into `docs/vv/results.md` or a file under `docs/vv/evidence/`.
- One row per requirement. If a requirement is covered by several tests, list the
  most decisive one here and the rest in the V&V results.

## Matrix

Replace the example row with the first real requirement as soon as
`specs/001-<slug>/spec.md` exists. The example row is ignored by the checker only
because its identifier contains `000`.

| ID | Requirement | Spec | User story | Task | Test | Evidence | Status |
|---|---|---|---|---|---|---|---|
| FR-000 | Example row: delete this once the first real requirement exists | specs/001-<slug>/spec.md | US1 | T001 | tests/test_example.py::test_example | docs/vv/results.md | planned |
