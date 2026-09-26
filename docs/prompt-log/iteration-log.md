# Prompt iteration log

This table is the short form of the prompt log and is pasted into section 3.3 of
the report. One row per prompt that had to be corrected. The full record stays in
the dated entries beside this file.

The first row is the example given by the course, kept as a format reference for
the first real entry, which replaces it. Nothing checks this file automatically:
it is evidence for section 3.3 of the report, and its value is in naming what
failed, not in the number of rows.

| Initial prompt or specification | Failure mode in the output | Refined prompt or specification | Resulting quality | Verified by |
|---|---|---|---|---|
| "Build the accuracy and memory harnesses, run them, and record the numbers as evidence" (round of 2026-09-26) | The harness passed while measuring almost nothing: 27 scored pairs instead of 5,000, no classification checked, and an answer key that contradicted the requirement; a 512 byte per frame leak survived both memory tests | "Score every adjacent pair, derive the answer key from the rendered pixels with an oracle independent of the engine, assert the classes each case states, require the sample size the specification names, and measure resident memory with a bound a half kilobyte per frame leak cannot survive. Then try to pass it with a deliberately wrong implementation before believing it." | 27,203 pairs scored at F1 1.0000 with every asserted class correct; 22.5 MiB peak resident over 10,000 frames; the deliberate leak now fails both memory tests; three real defects in the diff core surfaced and were fixed | `make accuracy`, `make memcheck`, the leak introduced and reverted, `go test ./...`; recorded in `docs/vv/results.md` as AUD-004 to AUD-007 | 91ff96b, 914fe49, aeb503f, 7d4d0d1 |
| Example: "Write a database model for user registration." | Passwords stored in plain text, validation skipped. | "Implement the User model following schema.json, ensuring password hashing using bcrypt in a pre-save hook." | Passwords hashed, input validated. | Course example, replaced by a real row |
| | | | | |

## How to fill a row

1. Paste the original prompt or the specification version it acted on.
2. Name the failure precisely: a hallucinated dependency, an ignored constraint,
   a security flaw, a wrong data model, code that passes the wrong test.
3. Paste the corrected prompt, or name the specification change.
4. State how the corrected output was verified, with a test name or a command.
5. Link the dated entry in `docs/prompt-log/NNNN-<topic>.md` and the commit.
