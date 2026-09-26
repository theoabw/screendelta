# Prompt iteration log

This table is the short form of the prompt log and is pasted into section 3.3 of
the report. One row per prompt that had to be corrected. The full record stays in
the dated entries beside this file.

The first row is the example given by the course, kept as a format reference. The
checker ignores it because its identifiers contain `000`.

| Initial prompt or specification | Failure mode in the output | Refined prompt or specification | Resulting quality | Verified by |
|---|---|---|---|---|
| Example: "Write a database model for user registration." | Passwords stored in plain text, validation skipped. | "Implement the User model following schema.json, ensuring password hashing using bcrypt in a pre-save hook." | Passwords hashed, input validated. | Course example, replaced by a real row |
| | | | | |

## How to fill a row

1. Paste the original prompt or the specification version it acted on.
2. Name the failure precisely: a hallucinated dependency, an ignored constraint,
   a security flaw, a wrong data model, code that passes the wrong test.
3. Paste the corrected prompt, or name the specification change.
4. State how the corrected output was verified, with a test name or a command.
5. Link the dated entry in `docs/prompt-log/NNNN-<topic>.md` and the commit.
