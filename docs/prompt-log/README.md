# Prompt log

Sections 3 and 4 of the graded report ask for the prompts used during
development, what was expected from them, what went wrong, and how the prompt or
the specification was changed afterwards. This directory is that record.

## Naming

```
NNNN-<short-topic>.md
```

`NNNN` is a four-digit sequence, starting at `0001`, incremented per entry, never
reused. One entry per session or per significant prompt exchange.

## Required fields

Every entry starts with the template below. Do not skip the failure fields: an
entry that reports only successes is not evidence of prompt engineering, and the
report explicitly asks for a case where a prompt failed or hallucinated.

```markdown
# NNNN <title>

- Date:
- Author:
- Agent and model:
- Spec Kit command:
- Artifacts produced:
- Related requirement IDs:
- Related commit:

## Prompt (verbatim)

<the exact prompt text>

## Intent

<what the prompt was meant to achieve, and why it was phrased that way>

## Expected output

<what was expected to come back>

## Actual output

<what came back, quoted where it matters>

## What went wrong

<errors, omissions, hallucinations, spec drift, invented dependencies,
unnecessary complexity, or "nothing, this prompt worked" plus the reason it worked>

## Correction

<the refined prompt or specification change, and why that change addresses the
failure>

## Result

<what the corrected version produced, and how it was verified>
```

## Rules

1. Paste prompts verbatim. Summarising them destroys the value of the record.
2. Log the prompt that failed, not only the one that worked.
3. Reference the requirement identifiers the prompt produced or changed, so the
   prompt log lines up with `docs/traceability.md`.
4. Add the same reference to the commit message (`Prompt: docs/prompt-log/NNNN-...md`).
5. Entries are append-only. If a later prompt supersedes an earlier one, write a
   new entry and reference the old one.
