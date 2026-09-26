# Course context

Captured from Moodle on 2026-09-26. Re-check the course page before every
project meeting, because deadlines and instructions can change.

## Course

| Item | Value |
|---|---|
| Code and name | SWCON2627 Software Construction 2026-27 |
| Scope | Period I, 5 ECTS |
| Teachers | Ivan Porres, AKM Bahalul Haque |
| Course page | https://moodle.abo.fi/course/view.php?id=13701 |
| Project section | Description of the course project report, group selection, report submission, repository submission |

## Assessment

| Task | Points | Rule |
|---|---|---|
| Project Report | 35 | At least 50 percent of the task's points required to pass |
| Git Activities | 15 | At least 50 percent of the task's points required to pass |

Grade bands: no grade below 25 points or below 50 percent in either task, then
1 at 25 to 30, 2 at 31 to 35, 3 at 36 to 40, 4 at 41 to 45, and 5 at 46 to 50.

## Deadlines

| Item | Date |
|---|---|
| Course Project Report submission | Sunday 2026-10-25, 23:59 |
| Git repository submission | Sunday 2026-10-25, 23:59 |
| Project meeting 1 of 3 | Monday 2026-09-28, 12:30 |
| Project meeting 2 of 3 | Monday 2026-10-05, 12:30 |
| Project meeting 3 of 3 | Listed as 2026-10-02, 12:30, which contradicts the order of the meetings. Confirm with the lecturer. |

Both submissions are made for Group A. The group choice activity lists Group A
with a single member: this project is done solo, which the course discourages.
The consequences are recorded in `README.md` and section 6.2 of the report, and
the compensating controls are independent agent review, role rotation per phase,
and external testers for user acceptance.

## What the course requires of the project

- Teams of up to three participants. Working alone is explicitly discouraged.
- Specification-Driven Development, using GitHub Spec Kit as the tool kit.
- The repository must contain the whole project, a README, and visible
  contributions from every member. Modifications, issues and discussions belong
  in git.
- The report follows a fixed template and must include an executive summary of
  200 to 300 words, the specification techniques used, the AI models and tools
  used, the SDD workflow, the Spec Kit usage, and the verification and
  validation method.
- The system to build is not prescribed: each team chooses it.

## Report sections and points

| Section | Points | Focus |
|---|---|---|
| 1. Introduction and paradigm shift | 3 | Project overview, vibe coding, SDD methodology, timeline |
| 2. Requirements and system specification | 6 | Stakeholders, functional requirements, non-functional requirements, technology stack, interface and API specification, how the requirements were derived with SDD tools |
| 3. Vibe coding and the project | 5 | Prompting framework, master prompts, prompt iteration log with a real failure and its fix |
| 4. Specification Driven Development and the project | 7 | Tool kit choice, development environment, deriving requirements with agents, workflow, prompts |
| 5. Verification and validation | 6 | V&V strategy and threats to validity, code audit log, test execution results |
| 6. Project management and AI toolchain workflows | 3 | Version control and prompt integration, workspace configuration, contribution breakdown |
| 7. Reflections | 5 | Benefits, bottlenecks, lessons learned, and a proposed SDD workflow with a process diagram |

## Course resources that constrain this repository

| Resource | What it fixes |
|---|---|
| GitHub Spec Kit, https://github.com/github/spec-kit | The tool kit. This repository is scaffolded with version 1.0.12. |
| Spec Kit installation guide linked from the course page | The VS Code and Copilot setup path |
| Spec-driven development example linked from the course page | A worked example of the workflow |
| gitlab.abo.fi | The hosting for the submitted repository, signed in with the university account |

## Consequences for this repository

1. `docs/report/report.md` mirrors the graded template section by section.
2. `docs/prompt-log/` exists because sections 3 and 4 are graded on prompt
   iteration, including failures, which no tool records automatically.
3. `docs/vv/` exists because section 5 is graded on test evidence with expected
   and actual results.
4. `docs/traceability.md` and `scripts/check_traceability.py` exist because
   section 2 is graded on the requirements being the source of truth, and
   `/speckit.analyze` computes that coverage without writing it down.
5. `CONTRIBUTING.md` records the commit convention that links code changes to
   prompts and specification changes, which section 6 asks about directly.
