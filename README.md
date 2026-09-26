# SWCON2627 Software Construction 2026-27, course project (Group A)

This repository is the graded artefact for the Software Construction course
project. It holds the specification, the implementation plan, the code, the
verification evidence and the experience report. Development follows
Specification-Driven Development (SDD) with [GitHub Spec Kit](https://github.com/github/spec-kit).

**Status**: scaffold complete, system not yet selected. See [Open decisions](#open-decisions).

## Deliverables

| Deliverable | Where it lives | Due |
|---|---|---|
| Course project report (35 points) | `docs/report/report.md`, exported to PDF before submission | 2026-10-25 23:59 |
| Git repository URL (15 points) | This repository, hosted on gitlab.abo.fi, URL submitted in Moodle | 2026-10-25 23:59 |

Both submissions are made for Group A in Moodle. The grading rule is that each
task (report and git activities) needs at least 50 percent of its own points,
so a strong report cannot compensate for an empty git history.

## Team

| Name | Student ID | Role in the project | Contribution |
|---|---|---|---|
| Theo Wilenius | 2202234 | Specifier and prompt engineer, repository owner | TBD |
| TBD | TBD | Verifier: test pipeline, coverage, security linting | TBD |
| TBD | TBD | Auditor: requirements quality, traceability, report | TBD |

The course recommends teams of three and warns against working alone. The
membership and the role split are recorded here and in
`docs/report/report.md` as soon as the team is settled.

## Repository layout

| Path | Purpose |
|---|---|
| `.specify/` | Spec Kit: constitution, templates, scripts, integration state. Committed in full so every member works from the same process. |
| `.specify/templates/overrides/` | Project overrides that take priority over the installed templates. |
| `specs/` | One directory per feature: spec, plan, research, data model, contracts, quickstart, tasks. Source of truth. |
| `src/`, `tests/` | Implementation and tests, created once a stack is selected. |
| `docs/report/` | The experience report that is submitted for grading, and the course template it follows. |
| `docs/prompt-log/` | Verbatim prompt records and the prompt iteration log. |
| `docs/vv/` | Verification and validation plan, results, and raw evidence. |
| `docs/analysis/` | Captured output of `/speckit.analyze`, which writes nothing to disk on its own. |
| `docs/adr/` | Architecture decision records, including every deviation from an approved plan. |
| `docs/traceability.md` | Requirement to task to test to evidence matrix. |
| `docs/course-context.md` | What the course requires, captured from Moodle. |
| `.github/`, `.opencode/`, `.dsh/` | The same Spec Kit commands rendered for Copilot in VS Code, opencode and dsh. |
| `scripts/check_traceability.py` | Fails the build when a requirement is not traced to a task and a test. |

## The workflow

Spec Kit is run in this order. Steps marked as gates must not be skipped.

```
/speckit.constitution    project rules, written once, amended deliberately
/speckit.specify         spec.md, from the feature description
/speckit.clarify         resolves open questions in spec.md          (gate)
/speckit.plan            plan.md, research.md, data-model.md, contracts/, quickstart.md
/speckit.checklist       reviewer-owned quality checklists           (gate)
/speckit.tasks           tasks.md, ordered and dependency-aware
/speckit.analyze         read-only cross-artifact consistency report (gate)
/speckit.implement       code, task by task
/speckit.converge        appends remaining work as tasks
```

`CONTRIBUTING.md` defines the gates, the commit convention and the review rules.
`AGENTS.md` defines the same rules for coding agents.

## Quickstart

Prerequisites: `git`, Python 3.11 or newer (Spec Kit requires it), and `uv`.
Spec Kit is pinned to the version this repository was scaffolded with, so every
member generates the same files.

```bash
uv tool install specify-cli --from git+https://github.com/github/spec-kit.git@v1.0.12
specify --version          # expect 1.0.12
specify integration status # shows the installed agent surfaces
```

Command surfaces installed in this repository:

| Agent | Commands |
|---|---|
| GitHub Copilot in VS Code | `.github/prompts/speckit.*.prompt.md` and `.github/agents/speckit.*.agent.md`, recommended in `.vscode/settings.json` |
| opencode | `.opencode/commands/speckit.*.md` |
| dsh | `.dsh/skills/speckit-*/SKILL.md` |

Checks:

```bash
make check    # requirement traceability: every requirement defined in specs/ has
              # a matrix row, traced rows name a task listed in tasks.md, and
              # verified rows name a test file and an evidence file that exist
make help     # list targets
```

## Open decisions

1. **Which system to build.** Not yet chosen. The course lets each team pick its
   own system, so this is the first decision to record in
   `specs/001-<slug>/spec.md`.
2. **Technology stack.** Follows from the system. The choice and its
   justification go into section 2.4 of the report.
3. **Team members two and three**, with their roles and contribution split.

## Secrets and personal data

No credentials, tokens or keys are committed. Student identifiers appear only in
the team table above and in `docs/report/report.md`, because the submission
template requires them.
