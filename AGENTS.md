# Agent instructions for this repository

These rules apply to any coding agent working in this repository, including the
authoring agents. `CONTRIBUTING.md` is the human-facing version of the same
rules; this file is the order of operations an agent should follow.

## Read before writing

1. `.specify/memory/constitution.md`, the project principles.
2. `specs/<active-feature>/spec.md`, the source of truth.
3. `specs/<active-feature>/plan.md` and `tasks.md` for the work item.
4. `docs/traceability.md` to see what is already claimed as done.
5. `docs/course-context.md` when a decision depends on what the course grades.

## Hard rules

- Specification first. Do not write implementation code that no task in
  `tasks.md` covers, and do not invent requirements. If the spec is wrong, say
  so and change the spec in its own commit.
- Keep requirement identifiers stable. `FR-###`, `NFR-###` and `SC-###` are
  never renumbered or reused.
- Update `docs/traceability.md` in the same change that adds a requirement, a
  task or a test. `python3 scripts/check_traceability.py` must pass.
- Log the session in `docs/prompt-log/` using the convention in
  `docs/prompt-log/README.md`. Record what was rejected or corrected, not only
  what succeeded.
- Capture `/speckit.analyze` output into `docs/analysis/`. The command is
  read-only by design.
- Never tick a checklist item that a reviewer owns, and never edit
  `.specify/memory/constitution.md` without recording the amendment reason.
- Run `make check` before proposing a commit, and report exactly what was run
  and what it returned.

## Writing style

- Plain professional English. No em dashes and no en dashes anywhere: use
  commas, colons, parentheses or a plain hyphen.
- Documentation in this repository must be self-contained: name only files that
  exist in this repository, plus public URLs where a source is genuinely needed.
- State what changed and why it matters, not which tools were used to explore.

## Autonomy

Local, reversible work (read, edit, run tests, run checks, commit on a feature
branch) is in scope without asking. Ask the repository owner before anything
that publishes or changes shared state: pushing to a remote, opening or
commenting on a merge request, changing repository settings, or rewriting
history.

## Which commands exist where

A Spec Kit command such as `/speckit.specify` is available as:

| Agent | Location |
|---|---|
| GitHub Copilot in VS Code | `.github/prompts/speckit.specify.prompt.md` |
| opencode | `.opencode/commands/speckit.specify.md` |
| dsh | `.dsh/skills/speckit-specify/SKILL.md` |

If the command surface is missing, the canonical workflow is still defined by
`.specify/templates/` and `.specify/scripts/bash/`, which the agent can read and
run directly.
