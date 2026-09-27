# Handover

Everything an agent can do to this repository is done. This file is the short list of what is left, who has to do
it, and how to check the result. It is written for the repository owner on the day of submission.

## State

| | |
|---|---|
| Commits | 169 on `main`, signed, no remote configured |
| Requirements | 33 defined, 29 verified with a committed test and recorded output |
| Tasks | 54 planned, 53 complete |
| Defects | 44 recorded: 43 fixed, 1 documented as inherent with its reason, none open |
| Gates | `make check-strict` (which runs the test suite, the traceability check and the report check), `./scripts/regression_check.sh` and `./scripts/final_verify.sh` exit 0. `./scripts/mutation_check.sh` exits 0 with one survivor, the equivalent mutant its list records |
| Report | `docs/report/report.md`, exported to `docs/report/report.pdf` |

The four requirements that are not verified need something this machine does not have, and the traceability matrix
says which is which:

| Requirement | What it needs |
|---|---|
| NFR-004 determinism | A second physical host. Byte-identical output is demonstrated across two toolchains, two userlands, two thread counts and two collector settings, all on this machine |
| NFR-007 usability | A person who is not the author, following the README while being timed |
| NFR-008 portability | A Windows machine. Three targets build and the suite passes on two Linux userlands |
| SC-005 a person unfamiliar with the project | The same run as NFR-007 |

## Decisions already taken

| Question | Answer | What it means here |
|---|---|---|
| Push the repository | Local for now | The 154 signed commits stay on this machine. Nothing has been pushed and no remote is configured |
| The acceptance run | The owner runs it and reports the result | `docs/vv/acceptance.md` is ready; the timed scenario and six short ones take about ten minutes with a second person. NFR-007 and SC-005 move to verified once the result is recorded |
| The second host | Retry the owner's own machines later | The desktop, the laptop and its WSL instance were all unreachable when attempted, so NFR-004's cross-host claim and NFR-008's Windows runtime half stay unmet with the residual recorded |

## One thing to enable in a fresh clone

```bash
git config core.hooksPath .githooks
```

That pre-commit hook refuses to commit while a check is applying a mutation to the tree, which is how a broken form
of a fix was once committed and left the suite red while the process gate reported green. `make hooks` reports
whether it is enabled. The three guards together are described in `CONTRIBUTING.md`.

## What the owner has to do

### 1. Push the repository (about five minutes)

Nothing has been pushed. Create an empty project on `gitlab.abo.fi`, then:

```bash
cd /path/to/swcon2627-project
git remote add origin <the project URL>
git push -u origin main
```

Then submit that URL in Moodle, as the course requires. The commits are signed and the history is the graded
artefact, so push rather than squash: the granularity is part of what is being assessed.

### 2. Run the acceptance test (about ten minutes, needs a second person)

`docs/vv/acceptance.md` is the script. It has one timed scenario and six short ones. Give it to someone who has not
seen the project, with the README and nothing else, and record what happens in the table at the end of the script.
Then:

```bash
# after filling in the script's result section
git add docs/vv/acceptance.md
git commit -m "docs(vv): record the acceptance run

<what the tester did, how long the timed scenario took, and any comments>"
```

If the timed run takes under five minutes with no help, NFR-007 and SC-005 are met and their traceability rows move
to verified. If it does not, the run has found a usability defect, which is a better outcome than not running it:
record it in `docs/vv/results.md` as a defect and fix the README.

### 3. Optional: close the other two, when a machine is reachable

Both need a machine this one cannot reach. If either becomes available, the commands are:

```bash
# A second Linux host, for cross-host determinism (NFR-004)
git clone <the pushed URL> && cd swcon2627-project && go test ./... -count=1

# A Windows machine, for the runtime half of NFR-008
go build ./... && go test ./... -count=1
```

Record the result in `docs/vv/evidence/` and move the requirement's row in `docs/traceability.md` to verified, in the
same commit.

## Before submitting the report

The PDF is a build artefact of the markdown, so regenerate it after any change to the report:

```bash
make report-pdf
git add docs/report/report.pdf && git commit -m "docs(report): re-export the PDF"
```

Then check everything once more, from a clean tree:

```bash
make check-strict     # traceability and every claim in the report against the repository
make verify           # the whole measurement suite, with the exit code of each command
go test ./... -count=1
```

## Where the graded evidence lives

| Artefact | Path |
|---|---|
| The report, and the template it follows | `docs/report/` |
| Requirement to task to test to evidence | `docs/traceability.md` |
| Verification plan, classified audit log, results, raw output | `docs/vv/` |
| Prompt records, one per session, and the failure table | `docs/prompt-log/` |
| Analyzer and convergence reports | `docs/analysis/` |
| Decisions and deviations | `docs/adr/`, `specs/001-frame-delta-engine/plan.md` |
| The specification, plan, contracts, quickstart and tasks | `specs/001-frame-delta-engine/` |

## What was deliberately not done

- **No frame size limit.** An oversized frame is not refused explicitly, because a limit is a configuration surface
  the specification does not ask for. `docs/analysis/converge-2026-09-27.md` records it as a gap with that reason.
- **One inherent boundary**, AUD-019: an element that disappears from exactly the footprint it occupied is reported
  as a change, because the evidence that would justify a removal also fires on a subtle repaint, and a false removal
  is the failure the noise corpus exists to catch.
- **No merge requests or issues.** One author worked on one branch with no remote until now. The review record takes
  their place, and the report says so rather than inventing retrospective issues.
