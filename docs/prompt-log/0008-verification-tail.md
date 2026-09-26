# 0008 Verification tail: end to end tests, the consumer, the demo, and the last review round

- Date: 2026-09-27
- Author: Theo Wilenius (owner), with an agent as authoring agent
- Agent and model: dsh agent session, codex review subagent at low reasoning effort for the
  independent re-review of the identity slice
- Spec Kit command: `/speckit.implement` for the remaining tasks, `/speckit.tasks` amended as tasks
  completed
- Artifacts produced: `tests/e2e/quickstart_test.go`, `tests/e2e/compliance_test.go`, `tools/consumer`,
  `tools/demo` with its capture fixture, the `capture`, `demo`, `demo-generated` and `cross` Makefile
  targets, the report and its classified audit log, `docs/vv/evidence/functional-2026-09-27.txt`,
  `docs/vv/evidence/e2e-2026-09-27.txt`, `docs/vv/evidence/demo-2026-09-27.txt`, and the audit log in
  `docs/vv/plan.md` with 32 rows
- Related requirement IDs: FR-001 to FR-005, FR-010 to FR-016, NFR-004, NFR-007, NFR-008, SC-004, SC-005, SC-006
- Related commits: 8254adb, df54729, 2251bea, ad5a19e, 5275124, 8e38e0c, 7e2d814, 416cad9, dd181d4

## Prompt (verbatim)

The round opened with the owner's request to finish the remaining tasks in the task list and to fill the
report against the course template:

> Finish the remaining tasks: the end to end tests, the demo, the second consumer, the acceptance script,
> and the report. Verify each requirement with committed tests and evidence, and keep the repository with
> `make check-strict` green.

The verification tasks were then dispatched one at a time, each with its own brief naming the acceptance
level rather than the implementation: run the quickstart scenarios against the built command rather than
the packages; prove the no-network and no-write promise in a way that fails when the promise breaks; and
give the schema a reader that shares no code with the engine.

## Intent

Four things, in order. Close the verification tail with evidence a reviewer can check rather than with an
assertion that the tests pass. Give the pipeline demonstration real screen pixels, because every accuracy
number in the project had come from flat synthetic panels. Fill the report from the repository rather than
from memory. And run the review loop again over the slice that had just been changed, because that is where
the previous three rounds had found the most.

## What happened

**The verification tail closed at a level above the unit tests.** The end to end tests build the real
command, generate frames, and run the scenarios the quickstart documents, asserting exit codes, standard
output and the files left behind. Eight of the nine scenarios run this way; the fifth needs a specific frame
sequence rather than a corpus case and is demonstrated in the diff package instead, which the evidence file
states rather than leaving a gap in the list.

**The compliance promise became a test.** `FR-016` says the engine runs without network access and writes
only where it is told. That is now checked two ways: statically, by walking the syntax trees of every
non-test file and refusing a network import, a process execution, or a write in a file that has no business
writing; and dynamically, by running the command in a working directory of its own and comparing the tree
before and after. Both halves were falsified first: adding an import of `net/http` and a stray `os.WriteFile`
to `internal/diff` made the checks fail with the file and the call named.

**An independent consumer was written against the schema, not the code.** `tools/consumer` shares nothing
with the engine, validates every rule the schema states, and tracks each identity to fail when one reported
retired appears again. It is itself falsified in the same suite by corrupting one document.

**The pipeline demonstration got real pixels.** Neither of the owner's machines was reachable, so the
capture could not come from them. The workstation has `Xvfb` and a Playwright Chromium, so `make capture`
renders a fixture that imitates a legacy form and screenshots it: real antialiased text, real widget edges.
The demo then sustains 294.8 decisions per second against a criterion of 20, and reading the engine's output
on those pixels showed it isolating the amount field's digits, the status line's words and the clock's
seconds as separate regions.

**The re-review falsified five of the previous round's claims.** The findings were about the class of defect
rather than the instance that had been fixed, and the pattern was the author's: each fix had been aimed at
the case the review demonstrated. Six defect classes are now fixed, including the replace branch retiring an
element on geometry alone, a return reading evidence about a contained area as evidence about the whole
element, and the pixel measurement including the growth margin, whose ring never changes and put the
full-change threshold out of reach.

## What was corrected

- The requirement matrix moved ten functional requirements from `in-progress` to `verified` against a new
  evidence file that records the test run and maps each requirement to the test that carries it. The count
  is 26 of 32.
- `docs/vv/acceptance.md` was a template with an empty checklist and placeholders. It is now a real script
  with the timed scenario, six concrete scenarios, and every command verified from a clean clone. What it
  still needs is a person who is not the author, which is why NFR-007 and SC-005 remain unmet.
- The README, which still described a scaffold and pointed at a `src` directory that does not exist, now
  opens with the path a new user takes.
- The report's counts are regenerated from the repository. Two of them were wrong when first written.

## What the agent got wrong

**A silent no-op edit.** An edit to the defect table replaced nothing, because the search string did not
match the row as written, and the command reported success. The tables and the report disagreed until the
counts were verified against them. The response is a rule now applied: after changing a count, read the
table it came from and compare, rather than trusting the edit.

**Fixes aimed at instances rather than classes.** Three times in this project a review found the same defect
class at the next site along, because the fix had been written against the demonstrated case. The lesson is
recorded in the report: when a review demonstrates a case, the question to answer is what rule the case is
an instance of.

**A performance regression introduced by a correctness fix.** Measuring whether each element's pixels
changed, inside the loop over changed areas, cost the large-change profile twenty milliseconds a frame. The
measurement is a fact about the frame rather than about an area, and moving it out of the loop restored the
profile.

## Related artifacts

- `docs/vv/plan.md` holds the classified audit log, 32 rows across logical, compliance and maintainability
- `docs/vv/results.md` holds the running measurement record and the defect table
- `docs/vv/evidence/` holds the raw output of every measurement named in the report
