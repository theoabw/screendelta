# 0002 Review round: making the traceability check real

- Date: 2026-09-26
- Author: Theo Wilenius
- Agent and model: dsh, deepseek-flash, plus a read-only Codex review agent
- Spec Kit command: none
- Artifacts produced: rework of `scripts/check_traceability.py`, corrections across the process documents, `docs/report/course-template.md`
- Related requirement IDs: none yet
- Related commit: the traceability rework commit on `main`, listed in `git log`

## Prompt (verbatim)

The review was dispatched with this brief:

> You are performing a READ-ONLY code and document review. You must not modify,
> create, or delete any file, and you must not run git commands that change
> state. [...] Review the changed files against these criteria [...] Correctness
> of `scripts/check_traceability.py` [...] Internal consistency of the
> documentation set [...] The template override at
> `.specify/templates/overrides/spec-template.md` [...] Claims that are not
> backed by the repository [...] Style: the owner bans em dashes and en dashes.

## Intent

Get an independent read of the scaffold before any specification work starts,
because the traceability checker is the mechanism the whole process rests on and a
check that passes vacuously is worse than no check.

## Expected output

Findings classified by severity with file and line references, and a verdict on
whether the scaffold is a sound starting point.

## Actual output

No critical findings. Three high findings, all valid:

1. A row marked `verified` passed with a wrong spec path, a nonexistent task, a
   nonexistent test and nonexistent evidence, because only non-empty strings were
   checked. The verification claim could therefore be entirely fictitious.
2. Task coverage accepted any occurrence of an identifier anywhere in `tasks.md`,
   and skipped the check when `tasks.md` was absent.
3. The stock templates restart identifiers at `FR-001` in every feature, while the
   checker rejected duplicates across specs, so the second feature could not have
   been specified without tripping the check.

Eight medium and six low findings followed, including checks skipped when no spec
existed, HTML-commented rows counting as coverage, escaped pipes breaking column
parsing, an overclaim that an untested requirement cannot merge, a contradiction
between the definition of done and the `deferred` and `withdrawn` states, blanket
checklist ownership that contradicts the installed `/speckit.specify` behaviour,
a course-source path outside the repository, and a false statement about where
student identifiers appear.

## What went wrong

The checker was written to enforce a process and only asserted that the fields
were filled in. That is the classic verification gap the course itself warns
about: the artifact looked rigorous while checking almost nothing. Two document
claims (the merge gate and the student identifier statement) were written from
intent rather than from what the code and files actually did.

## Correction

The checker now:

- keys requirements by specification path plus identifier, so identifiers are
  namespaced per spec and may repeat across specs;
- recognises a definition only on a bullet line whose first element is the bold
  identifier, so cross-spec references do not create false duplicates, and reports
  a duplicate definition inside one spec;
- always runs its row checks, so removing all specs cannot leave orphaned rows
  passing with only a warning;
- resolves the named task against the feature's `tasks.md` and requires that file
  to exist for `in-progress` and `verified` rows;
- requires a `verified` row's test file and evidence file to exist, treating
  external URLs as an explicit warning rather than silent acceptance;
- strips HTML comments, splits on unescaped pipes, reports malformed identifier
  cells instead of skipping them, and reports unreadable or non-UTF-8 files
  instead of crashing;
- exempts `deferred` and `withdrawn` requirements from the `tasks.md` coverage
  requirement, and fails under `--require-specs` when a spec defines no
  requirement at all.

The documents were corrected to match: the enforcement description in
`docs/traceability.md` and `docs/vv/plan.md`, the definition of done, checklist
ownership by kind, trailer applicability for process-only commits, the analysis
capture naming, and the Python version requirement. The graded template was
copied into `docs/report/course-template.md` so the repository no longer depends
on a path outside itself.

## Result

The checker was re-exercised against eleven fixtures covering each corrected
behaviour, and each produced the intended exit status: two specs reusing `FR-001`
pass, a duplicated definition fails, a verified row with a missing test file
fails, a task absent from `tasks.md` fails, a malformed row fails, an escaped pipe
passes, a commented-out row does not supply coverage, a deferred requirement
absent from `tasks.md` passes, and an empty spec fails under `--require-specs`.
The fixtures were run outside the repository, so no test scaffold is committed
before the technology stack is chosen.
