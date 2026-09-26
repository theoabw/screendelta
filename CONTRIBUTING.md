# Contributing

The course grades the report and the git history separately, and each needs at
least half of its own points. That means the process has to be visible in the
repository, not only in the final code.

## The mandated command order

Run Spec Kit commands in this order, one feature at a time. `/speckit.specify`
is the only strictly required step before `/speckit.plan`; the other gates exist
because the report is graded on exactly what they produce.

| Step | Command | Produces | Gate |
|---|---|---|---|
| 1 | `/speckit.constitution` | `.specify/memory/constitution.md` | Once per project, amended only through an ADR |
| 2 | `/speckit.specify` | `specs/NNN-slug/spec.md` | No |
| 3 | `/speckit.clarify` | `spec.md` with open questions resolved | Yes: no `NEEDS CLARIFICATION` may survive into `plan.md` |
| 4 | `/speckit.plan` | `plan.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md` | Yes: the constitution check in `plan.md` must pass |
| 5 | `/speckit.checklist` | `specs/NNN-slug/checklists/*.md` | Yes: reviewer-owned, see below |
| 6 | `/speckit.tasks` | `tasks.md` | No |
| 7 | `/speckit.analyze` | An analysis report, which the command does not write to disk | Yes: paste the report into `docs/analysis/` |
| 8 | `/speckit.implement` | Code, task by task | No |
| 9 | `/speckit.converge` | New tasks appended to `tasks.md` | No |

Between steps 7 and 8, every requirement identifier must appear in
`docs/traceability.md` with a task and a test.

## Rules

1. **Specification first.** No implementation without `tasks.md`. If a change is
   needed that no requirement covers, change the spec first, in its own commit.
2. **Stable identifiers.** `FR-###`, `NFR-###` and `SC-###` are never renumbered
   after approval. Withdrawn requirements keep their identifier and are marked
   `[WITHDRAWN]` with the reason.
3. **Traceability is part of done.** A task is complete when its row in
   `docs/traceability.md` names the test and the evidence, not when the code
   runs.
4. **Checklists are reviewer-owned.** Ticking a checklist item means a reviewer
   judged the requirement quality, not that coding finished. Never tick your own
   implementation work.
5. **Prompts are logged.** Every substantive agent session gets an entry in
   `docs/prompt-log/`, using `docs/prompt-log/README.md`. Rejected outputs and
   corrections are the valuable part of that record.
6. **`make check` passes before every commit.** It compares requirement
   identifiers in `specs/` with `docs/traceability.md` and `tasks.md`.
7. **Analyze reports are captured.** `/speckit.analyze` writes nothing, so its
   output is pasted into `docs/analysis/NNNN-<feature>-analyze.md` in the same
   session.

## Branches, merges and review

- One branch per feature, named exactly like the spec directory: `001-<slug>`.
- Merge through a GitLab merge request, not by pushing to `main`. The repository
  default branch carries only reviewed work.
- At least one other member reviews every merge request. Record what was
  reviewed, and any AI review findings, in the report's code audit log
  (`docs/vv/plan.md` links the audit evidence).
- Keep the merge request small enough that the reviewer reads the spec
  difference and the code difference together.

## Commit convention

```
<type>(<scope>): <summary>

Spec: specs/001-<slug>/spec.md
Req: FR-003, NFR-002
Task: T012
Prompt: docs/prompt-log/0004-specify-first-feature.md
```

`type` is one of `spec`, `plan`, `task`, `feat`, `fix`, `test`, `docs`, `chore`,
`ci`. The `Spec`, `Req`, `Task` and `Prompt` trailers are what make the git
history answer the report question about linking code to specification changes
and prompt iterations. A commit that implements behaviour without a `Req` line
is a process failure.

## What never goes into the repository

- Credentials, tokens, private keys, `.env` files with real values.
- Generated build output, dependency directories, editor state.
- Personal data beyond the student identifiers the report template requires.

## Agent surfaces

Three Spec Kit integrations are installed so that every member and every agent
sees the same commands: Copilot in VS Code (`.github/`), opencode
(`.opencode/commands`) and dsh (`.dsh/skills`). After a Spec Kit upgrade, refresh
them with `specify integration upgrade <key>`.

`specify integration status` reports an advisory finding because the Copilot
integration is marked IDE-scoped rather than multi-install safe. It does not
affect the workflow. `specify integration use <key>` changes the default surface
without removing the others.
