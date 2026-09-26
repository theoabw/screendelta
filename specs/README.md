# Specs

Every feature lives in its own numbered directory:

```
specs/001-<feature-slug>/
  spec.md          # created by /speckit.specify
  plan.md          # created by /speckit.plan
  research.md      # created by /speckit.plan
  data-model.md    # created by /speckit.plan, when the feature holds data
  contracts/       # created by /speckit.plan, when the feature has interfaces
  quickstart.md    # created by /speckit.plan
  tasks.md         # created by /speckit.tasks
  checklists/      # created by /speckit.specify and /speckit.checklist
```

Rules for this repository:

1. Directories are created by `/speckit.specify` through
   `.specify/scripts/bash/create-new-feature.sh`. Do not create them by hand.
2. Numbering is sequential and never reused, even if a feature is dropped.
3. `spec.md` is the source of truth. Code that is not traceable to a requirement
   in a spec is out of scope, however useful it looks.
4. Requirement identifiers (FR-###, NFR-###, SC-###) are stable once approved.
   `scripts/check_traceability.py` fails the build when an identifier is missing
   from `docs/traceability.md` or from `tasks.md`.
5. Changes to an approved spec are made by editing `spec.md` and recording the
   change in `docs/prompt-log/`, not by editing the code and back-filling later.

The first spec of the project is the system specification for the whole system.
Later specs are single features. See `docs/course-context.md` for what the
course expects each spec to contain.
