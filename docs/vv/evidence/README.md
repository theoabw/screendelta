# Verification evidence

Raw output referenced from `docs/vv/results.md` and the audit log in
`docs/vv/plan.md`.

## What belongs here

- Test run output that a reader cannot reproduce quickly, such as a full test log
  or a coverage report.
- Non-functional measurements: latency runs, concurrency runs, dependency audit
  output, coverage numbers with the command that produced them.
- Screenshots of the running system where behaviour, not code, is the evidence.

## Naming

```
YYYY-MM-DD-<what-was-run>.<ext>
```

For example `2026-10-12-pytest-full.log` or `2026-10-14-load-test-50-users.txt`.

## Rules

1. Evidence is committed, not attached to a chat or an issue, so that a reader can
   check it after the project ends.
2. Every file here is referenced from `docs/vv/results.md`, otherwise it is noise.
3. No secrets, tokens, personal data or production data in captured output. Redact
   before committing.
