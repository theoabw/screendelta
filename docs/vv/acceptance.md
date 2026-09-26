# User acceptance script

Executed by someone who did not implement the feature. The point is not to re-run the test suite, which
already passes; it is to find out whether a person who has never seen the project can get a useful result
from it, and where they get stuck. The outcome of each scenario is recorded in this file, because section
5.1 of the report is graded on validation against a stakeholder's needs rather than on tests alone.

Two requirements are measured here and nowhere else: **NFR-007**, a new user produces a delta document for
their own screenshots within two minutes using only the README, and **SC-005**, a person unfamiliar with the
project does so in under five minutes.

## What the tester needs

- A Linux or Windows machine with Go 1.24 or newer, and `git`.
- A copy of this repository. Nothing else: no network, no accounts, no services.
- Two of their own screenshots, or the generated pair this script provides if they have none.
- A stopwatch, which is the only instrument the script requires.

The commands below were checked by the author on 2026-09-27 from a clean build, so a failure is a finding
about the documentation rather than about a missing prerequisite.

## Preparation

```bash
git clone <the repository URL from the submission>
cd swcon2627-project
make build
```

If the tester has no screenshots of their own, generate a pair with ground truth:

```bash
go run ./tools/corpusgen --case changed-label --out /tmp/frames
```

## The timed scenario

Start the clock when the tester opens `README.md`, and stop it when a delta document is on their screen. Do
not help. Record where they look first and what they try, because that is the usability data.

| Step | What the tester does | What should happen |
|---|---|---|
| 1 | Reads the README from the top and finds how to run the tool | The third section, "Use it in two minutes", gives the commands |
| 2 | Builds the command | `bin/screendelta version` prints a version and the schema version it speaks |
| 3 | Runs `diff` over two frames | One JSON document describing the change, or an empty `regions` list if the frames are the same |
| 4 | Interprets the output | The schema in `specs/001-frame-delta-engine/contracts/` names every field, and the README says where to look |
| 5 | Runs `stream` over a sequence | One document per line, one per frame |

Record the elapsed time against SC-005's five minutes and NFR-007's two minutes. Both are stated against the
README alone, so any help invalidates the measurement and the run has to be repeated by someone else.

## Scenario checklist

| Scenario | Requirement | Steps | Expected | Observed | Verdict | Tester | Date |
|---|---|---|---|---|---|---|---|
| Read the README and produce a delta for your own screenshots | NFR-007, SC-005 | The timed scenario above | A document within the stated time, without help | | | | |
| Interpret one document | FR-002, FR-004 | Say what changed, where, and by how much | The area, the class and the magnitude, using the README or the schema | | | | |
| Process a sequence and follow one element | FR-006 | `bin/screendelta stream --source <frames>` | One document per frame, and the same `identity` value while an element is tracked | | | | |
| Restrict the output to part of the screen | FR-011 | Write a configuration with `regionsOfInterest`, run `diff` with it | Only changes inside the region are reported | | | | |
| Make it fail on purpose | FR-015 | `bin/screendelta diff --previous a.png --current b.png --width 7 --height 7` | One error line naming the problem, exit code 2, nothing on standard output | | | | |
| Compare a frame against a stored fingerprint | FR-009 | `fingerprint --frame a.png --out stored.json`, then `compare` with the same frame and with another | `equal` for the same frame, `different` for the other | | | | |

Verdict values: `accepted`, `accepted with comments`, `rejected`.

## Comments from the tester

Free-form notes on usability, missing behaviour, and anything the specification did not anticipate. This is
the most valuable part of the run, so it should be written while the run is fresh rather than reconstructed
afterwards.

## What a rejection produces

A rejected scenario becomes either a defect in `docs/vv/results.md` or a change to
`specs/001-frame-delta-engine/spec.md`, and which one is recorded here with the commit that made it. A
rejection is not a failure of the run: a script that only ever records acceptance is not evidence of
anything.

## Result

Not yet run. This file is the script; the run needs a person who is not the author, which is why NFR-007 and
SC-005 are the two requirements the traceability matrix still shows as unmet. Every command in the script has
been run by the author from a clean build, so what remains to be measured is the human half: whether the
README alone is enough, and how long it takes.
