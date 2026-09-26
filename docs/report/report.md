# An Experience Report on a Specification-Driven, AI-Assisted Software Construction Project

**Status**: working draft. Sections 1 to 4 are written from the repository; section 5 is written from
the running V&V record; the user acceptance testing in `docs/vv/acceptance.md` is outstanding and is
marked as such where it belongs rather than being implied complete.

## Team Details (Team A)

| Student Name | Student ID | Role in the project | Contribution (%) |
|---|---|---|---|
| Theo Wilenius | 2202234 | All four roles, rotated per phase: specifier, prompt engineer, verifier, auditor | 100 |

The project is done solo, against the course recommendation of teams of three. Section 6.2 says how the
four roles were separated in time, and how independent agent review in a fresh context, self-review
against `CONTRIBUTING.md`, and planned external acceptance testing substitute for a second and third
member.

## Executive Summary

The system built is **ScreenDelta**, a deterministic frame delta and element identity engine written in
Go. It takes consecutive screen frames at 1920 by 1080, reports which rectangular areas changed, assigns
each area an element identity that stays stable while that element is tracked, and emits one
self-contained JSON document per frame. It is deliberately one stage of a larger pipeline rather than a
whole tool: it does not classify, recognise text or drive a pointer. What it adds is a contract a later
stage can plan against, including an explicit uncertainty marker for identities the engine has evidence
for but will not claim.

Specification was done with **GitHub Spec Kit v1.0.12** in the flow-forward model: a constitution, a
feature specification with 16 functional requirements, 10 non-functional requirements and 6 success
criteria, an implementation plan, a task list of 53 tasks, interface contracts including a JSON Schema,
and a requirement traceability matrix whose rows are checked by a script that runs in CI. Deviations from
the plan are recorded as architecture decision records rather than edited away.

The AI tools were a **DeepSeek-backed agent harness** (dsh) for authoring, with the same repository
reachable from three command surfaces: GitHub Copilot prompts, opencode commands and dsh skills. Every
authoring session is logged in `docs/prompt-log/`, including what failed. Independent review was run in a
**separate agent context** configured for falsification rather than reading, which produced five review
rounds and 36 findings.

Verification is measurement rather than assertion. The engine scores F1 1.0000 with zero false removals on
27,211 generated frame pairs, holds p95 latency of 9.56 ms per frame pair on one CPU core against a 12 ms
requirement, sustains 74.9 frame pairs per second against 30, and holds a 25.2 MiB peak heap over 10,000
frames against a 128 MiB ceiling. Twelve of 32 requirements are verified with committed tests and
recorded output; the rest are in progress, and the report says which and why. Twenty-six defects were
found and recorded, of which 23 are fixed, one is documented as inherent to a pixel-only stage, and two
remain open.

## 1. Introduction and Paradigm Shift (3 points)

### 1.1 Project Overview and Objectives

ScreenDelta serves a planner: a program that decides what to click or type next by looking at a screen.
Modern models are good at deciding and slow at looking, because finding out what changed between two
frames of a legacy interface is a solved problem that nobody has packaged with an honest contract. The
engine therefore does one thing well and chains: frames in, a versioned delta document out.

The objective was set by the course constraint that the component must be small enough to finish
properly and useful enough to sit in a pipeline. Eleven candidate systems were scored on stakeholder
access, requirement richness, verification strength, demonstration value, blocker risk and how much
leverage an agent team gives, and the scores are in `docs/ideas.md`. The selection decision and the
alternatives rejected are in `docs/adr/0002-frame-delta-engine.md`; the choice of Go over Rust, C++ and
Python is `docs/adr/0003-go-implementation.md`, decided on build and test speed across Linux and Windows,
a standard library that covers image decoding without a dependency, and a cross-compilation story that
makes the Windows requirement cheap rather than expensive.

The engine deliberately excludes what the pipeline can do better: no machine learning, no OCR, no element
classification, no capture implementation beyond reading files and standard input. Each exclusion is a
decision in `specs/001-frame-delta-engine/research.md` with the reason recorded.

### 1.2 The Vibe Coding

Vibe coding, in the sense used here, is prompting an agent with an outcome and accepting what it produces
without a specification to check it against. It was used for exactly three things, each deliberately
bounded:

- **exploration before the specification existed**, where an agent was asked what a chainable GUI change
  detector could usefully promise, which produced the candidate list in `docs/ideas.md` and was thrown
  away afterwards except for the scores;
- **repository and process scaffolding**, where the first session produced the Spec Kit layout, the
  process documents and the traceability script from a description of what the course grades;
- **wording**, where error messages, documentation and commit messages were drafted by an agent and
  corrected by hand.

What was kept from the unstructured phase is small and auditable: the idea scores, the process layer and
the wording. What was not kept is the code: no implementation line was written before the requirement it
implements existed, which the repository enforces rather than promises, because
`scripts/check_traceability.py` fails when a requirement has no task and a task has no test.

The cost of the unstructured phase was real and is recorded in `docs/prompt-log/0005-system-selection.md`:
four candidate systems were explored in detail before the fifth was chosen, and the first two were
rejected for reasons that a single question about the course grading would have surfaced immediately. The
lesson, applied since, is that the exploration prompt has to name the constraint it is optimising for.

### 1.3 The SDD Methodology

Specification-Driven Development was chosen because the failure mode it prevents is the one this project
would otherwise have: an agent produces plausible code quickly, nothing records what it was supposed to
do, and the first real test becomes the specification. The chosen kit is GitHub Spec Kit v1.0.12, pinned
by `uv tool install specify-cli --from git+https://github.com/github/spec-kit.git@v1.0.12`, for three
reasons recorded in `docs/adr/0001-adopt-spec-kit.md`: it is the kit the course names, it is
command-driven rather than document-driven so the same repository works from three agent surfaces, and
its templates are overridable, which let the constitution and the report skeleton live in the repository
rather than in the tool.

The specification scope is one feature, `specs/001-frame-delta-engine/`, with three user stories in
priority order: a deterministic delta that a planner can act on, element identity across a stream, and a
fingerprint that lets a plan be cached. The persistence model is flow-forward: the specification is
updated deliberately when reality contradicts it, and the update is a commit of its own with the reason in
the message, rather than the specification being regenerated from the code. Deviations that change the
design rather than the wording become ADRs; deviations that change a requirement are spec commits.

The development timeline so far is two working days of agent-assisted construction, 2026-09-26 and
2026-09-27, against a submission deadline of 2026-10-25. That is worth stating plainly, because the report
is graded on what the process produced rather than on how long it took: 68 commits, 5,409 lines of Go and
4,755 lines of test, with the specification, plan and task list written before the code they describe.
The feature workflow this repository commits to is drawn in `docs/process/sdd-workflow.svg` and described
in section 7.4.

## 2. Requirements and System Specification (6 points)

### 2.1 Stakeholders

| Stakeholder | What they need | Why selected |
|---|---|---|
| The pipeline author | One stage whose output can be planned against, with a schema that does not change under them | They are the immediate consumer, and the reason the engine is a stage rather than a tool |
| The course assessment | Traceable requirements, visible process, and a history that shows the work | The repository is a graded artefact, so the process is a deliverable |
| A future maintainer | The reason behind each non-obvious rule, available without reading the code | Recorded as decision records and research notes, because the interesting decisions are the ones that look arbitrary later |
| An agent working on the code later | Rules it can follow and checks it cannot pass without satisfying | `AGENTS.md`, `CONTRIBUTING.md` and the traceability check exist for this reader specifically |
| A screencast tool author | Frames in, deltas out, with no opinion about capture | Excluded from scope deliberately: capture stays outside, so the engine can be embedded |

The stakeholder list drives the requirements rather than decorating them: FR-011 (regions of interest)
and FR-012 (configuration) exist because the pipeline author needs to bound the work, FR-014 and NFR-010
exist because a schema that changes silently breaks the future maintainer, and FR-016 exists because a
pipeline stage that writes files or opens sockets is not embeddable.

### 2.2 Functional Requirements

Functional requirements are written as testable statements with stable identifiers, each naming a
behaviour rather than an implementation, and each traced to a task in `docs/traceability.md`. The full
set is `specs/001-frame-delta-engine/spec.md`; what follows is the shape of it rather than a copy.

By user story and priority:

| Story | Priority | Functional requirements | What it promises |
|---|---|---|---|
| US1: a deterministic delta | P1 | FR-001 to FR-005, FR-010 to FR-016 (12) | Region detection with bounds, class, magnitude and area; a noise floor that is documented and configurable; one self-contained document per frame; explicit failure without a partial document |
| US2: element identity | P2 | FR-006, FR-007 (2) | Identifiers stable across a stream and never reused; identity confidence reported, and an identity that cannot be re-established marked uncertain rather than silently matched |
| US3: a fingerprint for plan caching | P3 | FR-008, FR-009 (2) | A per-frame fingerprint stable under sub-threshold noise, and comparison against a stored fingerprint reporting equal or different |

Counts by category: 16 functional requirements, of which 12 are priority 1, 2 are priority 2 and 2 are
priority 3; 10 non-functional requirements; 6 success criteria. Each functional requirement names the
observable behaviour and the input that produces it, and none of them names a data structure, a package or
an algorithm, which is what keeps the agents from inventing an implementation inside a requirement.

The requirements that carry the most weight are the ones that constrain failure rather than success:
FR-005 (no regions when the difference is below the floor), FR-007 (an identity that cannot be
re-established is marked, not guessed), FR-015 (fail explicitly rather than emit a partial document) and
FR-016 (no network, no writes outside the declared path). They are what make the engine usable as a stage,
and each has a boundary case in the test suite rather than only a happy path.

### 2.3 Non-Functional Requirements

Each non-functional requirement has a number, a target and a method, and the method is recorded in
`docs/vv/plan.md` before the measurement is taken, so the target cannot be adjusted afterwards to fit the
result.

| ID | Attribute | Target | How measured | State |
|---|---|---|---|---|
| NFR-001 | Performance | p95 at or below 12 ms, p99 at or below 25 ms per 1080p frame pair on one CPU core | `make perf`, per-pair timing, percentiles, GOMAXPROCS=1 | met: p95 9.56 ms, p99 10.74 ms |
| NFR-002 | Throughput | At least 30 frame pairs per second at 1080p on one core | `make perf`, sustained over 300 pairs | met: 74.9 per second |
| NFR-003 | Memory | At most 128 MB over 10,000 frames, no growth with stream length | `make memcheck`, resident and heap sampled every 1,000 frames | met: 22.5 MiB peak resident |
| NFR-004 | Determinism | Byte-identical output, independent of host, thread count and scheduling | Two runs encoded and compared; GOMAXPROCS 1 and 4 | met in part: identical across thread counts on this host; cross-host is untested |
| NFR-005 | Reliability | Zero false removals on the noise corpus | `make accuracy`, noise case, every pair | met: zero |
| NFR-006 | Accuracy | Region F1 at or above 0.98 on 5,000 or more generated pairs | `make accuracy`, corpus with generated ground truth | met: 27,211 pairs, F1 1.0000 |
| NFR-007 | Usability | A new user produces a delta from their own screenshots within two minutes, from the README alone | Task timing with an external tester | not yet measured; the acceptance script exists and is not run |
| NFR-008 | Portability | CPU only, no GPU, no network at runtime, Linux and Windows | Cross-build in CI, no-network test | partly met: cross-build done, no-network test outstanding |
| NFR-009 | Maintainability | At least 80 percent line coverage on the geometry and identity modules | `go test -cover` | met: diff 80.8, identity 86.3 |
| NFR-010 | Compatibility | Schema changes versioned, and a consumer can reject a version it does not understand | Decode-time rejection test, exit code 3 from the command line | met |

Two of the targets were met only after the first measurement failed, and the failures are recorded rather
than replaced: latency measured 25.26 ms p95 before three optimisations (AUD-008), and the memory
measurement passed while measuring nothing until it was rebuilt (AUD-007). The second case is the reason
section 5.3 records what was wrong with the measurement and not only what the engine scored.

### 2.4 Technology Stack

| Layer | Choice | Reason, and what was rejected |
|---|---|---|
| Language | Go 1.24 (toolchain 1.26) | Fast builds and tests, a standard library that decodes PNG without a dependency, and cross-compilation that makes the Windows requirement cheap. Rust was rejected for compile time against a two-day budget, C++ for build complexity, Python for the performance requirement |
| Images | Standard library only | One dependency fewer, and the formats in scope are PNG and raw RGBA, which the standard library covers |
| Contracts | JSON Schema (draft 2020-12) plus a hand-written Go validator | The schema is the consumer-facing contract and the validator enforces it in the engine, so a document that encodes is a document the schema accepts |
| Specification | GitHub Spec Kit v1.0.12 | The course names it, it is command-driven, and its templates are overridable |
| Authoring agent | DeepSeek-backed agent harness (dsh) | Long-context authoring with repository reach, and the same repository reachable from three command surfaces |
| Review agent | Codex-based agent at low reasoning effort, in a separate context | Independence is the point: it sees the diff and the specification and is briefed to falsify rather than to agree |
| Hosting | GitLab at `gitlab.abo.fi` | Course requirement. The repository is local until the owner pushes |
| CI | GitLab CI, `verify` and `test` stages | Requirement traceability and the Go test suite run on every push; the jobs are checked locally before they are committed |

The agent surfaces are the reason the stack section belongs in a report about specification: the same
specification is reachable as Copilot prompts in `.github/prompts/`, opencode commands in
`.opencode/commands/` and dsh skills in `.dsh/skills/`, so an agent can be changed without changing the
process. Section 4.2 says what each surface is for.

### 2.5 Interface and API Specifications

The engine has three interfaces, each with a contract artefact in
`specs/001-frame-delta-engine/contracts/`:

| Interface | Contract | What it fixes |
|---|---|---|
| The delta document | `delta-document.schema.json` | Field names, types, required fields, the class-specific rules for `previousBounds`, the ordering of regions and conditions, and the identity fields FR-007 requires |
| The configuration document | `config.schema.json` | Every key, its range, its default and its meaning. An unknown key is an error with the offending key named, because a silently ignored setting is a wrong result that looks right |
| The library and command line | `interfaces.md` | The two Go interfaces, the exit codes (0 success, 1 usage, 2 input, 3 contract), what each subcommand can classify, and the consumer-facing rules for identity, covers and returns |

The document is the important one, because it is what a later pipeline stage reads. It declares a schema
version, carries no timing fields (so two runs are byte-identical, which is NFR-004), and states its
ordering as part of the contract rather than as an implementation detail: regions sorted by top edge,
then left edge, then identifier, then size, so a total order exists even for two regions that share a
position and an identity.

### 2.6 How These Requirements and Specifications Were Created

The order was: constitution, then feature specification, then clarification, then plan, then contracts,
then tasks, then implementation, with `analyze` run between plan and tasks. The commands and their
outputs are recorded in `docs/analysis/` and the sessions in `docs/prompt-log/`.

What the agents produced and what was rejected:

- `/speckit.specify` produced the three user stories and the requirement skeleton in one pass. It was
  rewritten by hand in two respects: the success criteria were made measurable (the first version said
  "fast", the second said "p95 at or below 12 ms per 1080p frame pair on one core"), and an invented
  requirement about exporting to a database was deleted as out of scope.
- `/speckit.clarify` was the most valuable single command, because it asked the questions the first
  specification had left open. It produced, among others, the decision that an empty region-of-interest
  list means report nothing while an absent one means unrestricted, and that a viewport change is a
  condition rather than content.
- `/speckit.analyze` found two inconsistencies: a requirement traced to a task that did not exist, and a
  success criterion with no measurement. Both are fixed, and the reports are kept in `docs/analysis/`.
- The contract schema was written by an agent and then corrected by hand in three places the validator
  and the schema disagreed, which is exactly the class of defect the traceability check cannot see and
  the integration test can.

The rule applied throughout is the one in `AGENTS.md`: do not write implementation code that no task
covers, and do not invent requirements. Where the specification turned out to be wrong about what the
engine can know, the specification was changed in its own commit with the reason: the second acceptance
scenario of user story 2 originally asked for an element hidden behind an overlay to keep its identity,
which a pixel-only stage cannot do, and it now states the two events the engine can distinguish. That
change is commit `17900ed`, and `docs/prompt-log/0007-measurement-review.md` records why the first
implementation could not satisfy the original wording.

### 3.1 Prompting Framework

The framework is a mix, chosen per activity rather than per project:

| Activity | Technique | Why |
|---|---|---|
| Exploration before the specification | Role prompting with a named constraint ("you are choosing a system that a single agent team can finish and that a course can grade on process") | The first attempts failed by exploring systems with no constraint to optimise for, which section 1.2 records |
| Writing the specification | Structured, one Spec Kit command per step, with the command's own template as the prompt | The template asks for the fields the process needs; a free-form prompt produced a specification with no measurable criteria |
| Implementation | Chain of thought inside a task-bounded brief: goal, files, constraints, existing patterns, how to verify | An agent given a task from `tasks.md` has a bounded problem, and the brief carries the conventions it would otherwise guess |
| Verification | Adversarial role prompting: "your job is to falsify the claims below, not to comment on style" | A review prompt that asks for correctness returns code style; the one that asks for a counterexample returns defects |
| Measurement | No prompting: the harness is code, and the agent's role is to build and then try to defeat it | The most valuable findings in this project came from an agent trying to pass a measurement with a deliberately wrong implementation |

The single most important choice is the last one. Three of the five review rounds found defects in the
measurement rather than in the engine: a success criterion that could be satisfied by an engine with
broken classification, a memory guard that passed with a deliberate per-frame leak, and a latency run
inside the full test suite whose tail belonged to the machine rather than to the engine. Each is recorded
as a defect with the fix, because a measurement that reports success while measuring nothing is worse than
one that fails.

### 3.2 Master Prompts and System Prompts

The agent surfaces are configured in the repository rather than in a chat history, so the prompts are
reviewable artifacts. The three that matter:

**The repository constitution** (`.specify/memory/constitution.md`) is the system prompt for every
authoring session. Its operative rules, quoted from the file: requirements are written before code;
requirement identifiers are stable; a deviation is recorded rather than edited away; every emitted
document declares its schema version; the engine fails explicitly rather than emitting a partial document;
and the engine never reaches the network. It is amended only with the reason recorded, and it has been
amended once, for the identity fields FR-007 requires.

**The authoring brief** (`AGENTS.md`) is the working agreement. It fixes the order of operations (read the
constitution, then the specification, then the plan and tasks, then the traceability matrix), the hard
rules (no code without a task, update the matrix in the same change, log the session, run `make check`
before proposing a commit), and the autonomy boundary (local and reversible work needs no permission;
anything that publishes or changes shared state does). It exists because an agent that has to be told the
conventions each session will eventually forget one.

**The review brief** is not stored as a file but as a fixed shape, because it is sent fresh each round:
name the repository and the commit range, state that the pass is read-only, list the claims to falsify
numbered one by one, require a demonstrating case for each finding with a severity and a file and line,
and ask for a verdict on whether the acceptance claim can be made. The prompt that produced round five is
quoted in `docs/prompt-log/0007-measurement-review.md`, along with the two earlier rounds' briefs.

The expected result was that independent review would find defects the authoring pass could not. The actual
result was stronger and less comfortable: it found that the *evidence* was not what it claimed. The later
revisions to the brief ask about the measurement first and the code second.

### 3.3 Prompt Iteration Log

The full table is `docs/prompt-log/iteration-log.md`. One case, in the form the course asks for:

| Original prompt | Failure it caused | Corrected prompt | Result | Verified by |
|---|---|---|---|---|
| "Build the accuracy and memory harnesses, run them, and record the numbers as evidence" | The harness passed while measuring almost nothing: 27 scored pairs instead of the 5,000 the criterion names, no classification checked, an answer key that contradicted the requirement, and a memory guard that a 512 byte per frame leak survived | "Score every adjacent pair, derive the answer key from the rendered pixels with an oracle independent of the engine, assert the classes each case states, require the sample size the specification names, and measure resident memory with a bound a half kilobyte per frame leak cannot survive. Then try to pass it with a deliberately wrong implementation before believing it." | 27,211 pairs scored at F1 1.0000 with every asserted class correct; 22.5 MiB peak resident over 10,000 frames; the deliberate leak now fails both memory tests | `make accuracy`, `make memcheck`, the leak introduced and reverted, and the round recorded as AUD-007 in `docs/vv/results.md` |

The general lesson, which the corrected prompt states as a rule: a measurement is not finished until an
attempt to pass it with a deliberately wrong implementation has failed. Three of the 26 recorded defects
were found by exactly that, and none of them by reading the code.

### 4.1 SDD Tool Kit

GitHub Spec Kit v1.0.12, pinned in the repository so the workflow cannot drift under it. The decision
record is `docs/adr/0001-adopt-spec-kit.md`. It was chosen over three alternatives: a bespoke markdown
process, which would have had no templates and no integration; a document-driven generator, which would
have kept the specification outside the repository; and writing the process as prompts only, which is what
parts of this project do in addition rather than instead.

What the kit contributes, and what was overridden:

| Spec Kit artifact | Used for | Overridden |
|---|---|---|
| `.specify/memory/constitution.md` | The project rules every session reads | Amended once, with the reason recorded |
| `.specify/templates/` | The specification, plan, tasks and checklist shapes | Partly: the report skeleton and the V&V plan are repository documents, and the traceability matrix is generated by a script rather than a template |
| `/speckit.specify`, `clarify`, `plan`, `checklist`, `tasks`, `analyze`, `implement` | The workflow, one command per step | `analyze` output is captured to `docs/analysis/` because the command is read-only by design |
| `.specify/scripts/bash/` | Creating the feature directory, resolving templates | Used unchanged |

### 4.2 Development Environment

One Linux workstation (Ubuntu, Go 1.26 toolchain, 10 cores) with the repository reachable from three agent
surfaces, all three configured in this repository:

- **GitHub Copilot prompts** in `.github/prompts/`, for editing inside an editor;
- **opencode commands** in `.opencode/commands/`, for a terminal agent with the repository in reach;
- **dsh skills** in `.dsh/skills/`, the harness used for the authoring rounds in this project.

A solo project still needs all three for one reason that is worth stating: the authoring pass and the
review pass must run in *different* contexts, or the review is the author agreeing with themselves.
Changing surface is the cheapest way to guarantee that, and it is what kept the five review rounds from
inheriting the assumptions of the code they were reading. The review rounds in this project ran in a
separate harness with no access to the authoring conversation, given only the repository, the commit range
and the claims to falsify.

The models are named in the prompt log per session: a DeepSeek-backed agent for authoring, and a
Codex-based agent at low reasoning effort for review. The reasoning effort is deliberate: the review task
is to construct counterexamples against a specification, which is bounded work, and the value comes from
the questions asked rather than from the model's depth.

### 4.3 Deriving the Requirements Using the IDE and Coding Agents

The functional requirements came from the user stories by asking, for each story, what a consumer would
have to be told and what the engine could be wrong about. That second question is where most of the
interesting requirements come from: FR-007 (an identity that cannot be re-established is marked, not
guessed), FR-015 (fail explicitly without a partial document) and NFR-005 (zero false removals on noise)
are all answers to "how would this fail silently?".

The prompts that produced them, and what was rejected:

- "Write the functional requirements for a frame delta engine as observable behaviours, not as
  implementation" produced the skeleton in one pass. The agent's first version included "use a tile grid
  for comparison", which is a design decision that would have made the later measured removal of the tile
  grid a requirement change. It was rewritten as "report changed regions with bounds and magnitude".
- "What can this engine know for certain, and what can it only infer?" produced the distinction the whole
  design rests on: bounds and classes are observations, identity is an inference, and the uncertainty
  marker exists because the second is not the first.
- "List the ways a document consumer could be misled" produced the multiplicity rule (one element may be
  two regions), the ordering guarantee, and the rule that no timing field appears in a document.

The non-functional requirements were harder and were derived by asking what would be measured and by what
command, before the command existed. That order matters: three of the ten targets failed their first
measurement, and because the method was fixed in `docs/vv/plan.md` first, the response was to fix the
engine rather than to adjust the target.

### 4.4 SDD Workflow and Implementation

The workflow as actually followed, with the gate that ends each step:

| Step | Command | Output | Gate |
|---|---|---|---|
| 1 | `/speckit.constitution` | `.specify/memory/constitution.md` | The rules are quoted in `AGENTS.md` and enforced by review |
| 2 | `/speckit.specify` | `specs/001-frame-delta-engine/spec.md` | Every success criterion measurable, checked by hand |
| 3 | `/speckit.clarify` | The requirements notes and research decisions | Open questions in the specification brief are answered or recorded as research items |
| 4 | `/speckit.plan` | `plan.md`, `research.md`, `data-model.md`, `contracts/` | The contracts exist before the code they constrain |
| 5 | `/speckit.checklist` | `checklists/requirements.md` | Reviewer-owned items are never ticked by the author |
| 6 | `/speckit.analyze` | `docs/analysis/analyze-2026-09-26.md` | Five findings, all resolved or recorded |
| 7 | `/speckit.tasks` | `tasks.md`, 53 tasks | Every requirement has at least one task; checked by `scripts/check_traceability.py` |
| 8 | `/speckit.implement`, task by task | Code, tests, evidence | `make check` (traceability, format, vet, tests) before any commit; an independent review before a slice is called done |
| 9 | `/speckit.converge` (not yet used) | A gap list | Planned for the endgame, once the acceptance testing has run |

Two deviations from the comfortable path are worth recording. First, the implementation started before the
whole task list was written: the frame, error and document types existed while tasks 20 onwards were still
being refined, because the interfaces they define had to be real before the plan could say anything true
about them. The task list was updated in the same round. Second, the review gate was added after the fact:
eleven code commits went in without an independent review, which the audit log records as a process
failure, and the standing rule since is that a slice is not finished until a review in a separate context
has tried to break it. That rule has since found 36 findings across five rounds.

### 4.5 Prompts

The prompts used during the specification and implementation rounds are in `docs/prompt-log/`, one entry
per session, each with the verbatim prompt, the intent, what happened, what was corrected in the
specification or the documents, and what the agent got wrong. Seven entries exist. The ones that carry the
most information for a reader are:

- `0005-system-selection.md`: the selection round, including the four systems explored and rejected first;
- `0006-implementation-rounds.md`: the first implementation round, including the missing review step;
- `0007-measurement-review.md`: the round that falsified its own measurement, and the prompt change it
  caused.

### 5.1 V&V Strategy

The strategy, the threats to validity, and who verified what: the full plan is
`docs/vv/plan.md`. Name the levels used (unit, integration, functional,
requirement verification, user acceptance, compliance), or state which were
omitted and why.

### 5.2 Code Audit Log

Each flaw caught in review of generated code, classified as security,
compliance or logical, with the resolution technique. Record entries as they
happen in `docs/vv/plan.md` under the audit log heading, because they are hard to
reconstruct later.

### 5.3 Test Execution Results

The executed cases with expected result, actual result and pass or fail status,
plus coverage and any defect that remains open. The running record is
`docs/vv/results.md`; raw output is kept under `docs/vv/evidence/`.

## 6. Project Management and AI Toolchain Workflows (3 points)

### 6.1 Version Control and Prompt Integration

How commit messages link code changes to specification changes and prompt
iterations, and how the workspace was configured. The convention and its
rationale are in `CONTRIBUTING.md`, and the agent-facing version is in
`AGENTS.md`.

Git activity carries 15 of the 50 course points, and the course asks for
modifications, issues and discussions to be visible in git, so report the process
evidence rather than only the convention:

- Commit counts by phase and by type, and the fact that the history was kept to
  one logical change per commit by design (`CONTRIBUTING.md`, commit granularity).
- Counts of commits carrying each trailer, which is what links code to
  specification changes and prompt iterations. These are reproducible:
  `git log --oneline --grep='Req:'`, `git log --format='%s' | cut -d'(' -f1 | sort | uniq -c`.
- Issues raised and closed, with the commit that closed each one, and the merge
  requests where review happened, including the independent agent review findings
  and their resolutions.
- The workspace configuration itself: the Spec Kit version pin, the three agent
  command surfaces, the constitution, the gates in `CONTRIBUTING.md`, and the
  traceability check that runs in CI.
- One honest note about what the single-author history cannot show, which is a
  second human reviewer, and what was put in its place.

State where the evidence lives and how to reproduce it, so a reader can verify the
counts instead of taking them on trust.

### 6.2 Team Contribution Breakdown

How human work shifted from coding to specifying, prompting, auditing and
orchestrating. For each member, name the contribution and the git evidence for it.
Working solo, this section instead shows how the four roles were separated in
time, why each phase was finished before the next started, and how the missing
second and third reviewers were replaced: independent agent review in a fresh
context, self-review against `CONTRIBUTING.md`, and external testers for user
acceptance. Cite the review records in `docs/prompt-log/` and the audit log in
`docs/vv/plan.md`.

Quantify the leverage rather than asserting it: tasks closed per agent session,
commits per phase, requirements specified per hour of human work, and lines of the
specification written per line of code changed. Say where the human was the
bottleneck, which is where the honest part of this section lives. One person
directing agents can produce the output of a much larger team, but the specification
gates, the review passes and the acceptance testing stay serial and human, and
saying so is worth more than a claim about team size.

## 7. Reflections on Specification-Driven Development (5 points)

### 7.1 Benefits of AI-Assisted SDD

What worked, with evidence, and how much time the construction phase gained.

### 7.2 Core Bottlenecks and Challenges

Where the paradigm struggled: context limits, drift from the specification,
hallucinated dependencies, inconsistency across the codebase. For each, how it
was overcome.

### 7.3 Lessons Learned and Best Practices

Critically evaluate the process, support each claim with a concrete example from
this project, and compare vibe coding with SDD using evidence rather than
preference. End with actionable practices for the next team.

### 7.4 Proposed Specification-Driven Development Workflow

A reusable workflow, not a description of this project. For each phase state the
objective, activities, inputs, outputs and deliverables, mark where AI assistance
belongs and where human judgement is required, and justify the design from the
evidence above. Include the process diagram required by the template.

## Appendix: Evidence Index

| Evidence | Location |
|---|---|
| Specification, plan, tasks | `specs/001-<slug>/` |
| Prompt records | `docs/prompt-log/` |
| Analyze reports | `docs/analysis/` |
| V&V plan, results, raw output | `docs/vv/` |
| Requirement traceability | `docs/traceability.md` |
| Decisions and deviations | `docs/adr/` |
| Process definition | `.specify/memory/constitution.md`, `CONTRIBUTING.md`, `AGENTS.md` |
