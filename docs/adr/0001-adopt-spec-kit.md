# 0001 Adopt GitHub Spec Kit and this workflow

- Status: accepted
- Date: 2026-09-26
- Deciders: repository owner, on behalf of Group A

## Context

The course requires Specification-Driven Development and names GitHub Spec Kit as
the tool kit. The report is graded on the specification being the source of
truth, on prompt iteration being documented, and on verification evidence, and
the git history is graded separately.

Spec Kit provides the command workflow and the specification templates, but it
writes no prompt log, no traceability matrix, no verification record, and no
report. It also does not run `git init` and creates no root `.gitignore`.

## Decision

1. Use GitHub Spec Kit, pinned to version 1.0.12, installed with
   `uv tool install specify-cli --from git+https://github.com/github/spec-kit.git@v1.0.12`.
   The pin keeps every team member on the same generated file layout.
2. Install three command surfaces so that no member is blocked by their choice
   of agent: Copilot in VS Code (`.github/prompts`, `.github/agents`), opencode
   (`.opencode/commands`) and dsh (`.dsh/skills`). Verified that their target
   directories do not overlap.
3. Add what Spec Kit does not produce, under `docs/`: prompt log, traceability
   matrix, verification plan and results, analyze reports, ADRs and the graded
   report.
4. Enforce traceability mechanically with `scripts/check_traceability.py`, run by
   `make check` and by CI, rather than trusting review alone.
5. Follow the flow-forward persistence model: the specification is edited
   deliberately and deviations are recorded as ADRs, rather than letting the
   specification drift towards whatever the code became.
6. Add non-functional requirements with `NFR-###` identifiers through a project
   template override at `.specify/templates/overrides/spec-template.md`, because
   the stock template produces functional requirements only and the report
   grades non-functional requirements explicitly.

## Consequences

- Every artifact the report cites exists as a file in the repository, so the
  report is assembled from evidence rather than recalled.
- The specification set and the traceability matrix must be updated together;
  `make check` fails otherwise.
- Three command surfaces mean more files to keep in step after a Spec Kit
  upgrade, mitigated by `specify integration upgrade <key>`.
- `specify integration status` reports an advisory finding because the Copilot
  integration is IDE-scoped rather than multi-install safe. Accepted knowingly;
  the target directories do not collide.
- Pinning the tool kit means a Spec Kit upgrade is a deliberate, recorded change.

## Alternatives considered

| Alternative | Why not |
|---|---|
| Hand-written specification process without a kit | The course names Spec Kit, and a bespoke process would have to reimplement the same templates and scripts |
| Another SDD kit | Less documentation, no course support, and no equivalent of the analyze gate |
| Installing only the Copilot surface | Team members using opencode or dsh would have to translate commands by hand, which weakens the process evidence |
| Tracking traceability in a spreadsheet | It cannot fail a build, so the check would depend on discipline alone |
