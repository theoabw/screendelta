# Candidate systems

The course does not prescribe a system, so this file narrows the choice. It is
written for a solo project with three weeks of build time, and every candidate is
scored against what the report actually grades rather than against novelty.

## How to choose

The report is graded on stakeholders (2.1), functional and non-functional
requirements (2.2, 2.3), interface specifications (2.5), verification and
validation including user acceptance (5.1, 5.3), and reflection with evidence
(7). The git history is graded separately.

That rewards a system that:

1. has real users you can reach within a week, because user acceptance testing
   needs testers who are not you;
2. decomposes into requirements that are unambiguous and testable, so
   `FR-###` and `NFR-###` can be written and checked mechanically;
3. contains at least one property that a test can prove rather than a screenshot,
   such as arithmetic invariants, timezone correctness, or deterministic parsing;
4. has measurable non-functional targets, such as latency, throughput, coverage,
   dependency count or memory ceiling;
5. fits in roughly 60 to 80 hours of solo work, including the report;
6. can be demonstrated in two minutes at a project meeting.

Avoid systems whose core value is a large language model call, because the
behaviour is then hard to specify and hard to verify, which works against sections
2 and 5.

## Scores

Each criterion is scored 1 to 5, where 5 is best. Total is out of 30.

| # | Candidate | Stakeholder access | Requirement richness | Verification strength | Solo feasibility | Demo value | Low blocker risk | Total |
|---|---|---|---|---|---|---|---|---|
| A | Requirement traceability checker (CLI plus service) | 5 | 4 | 5 | 5 | 3 | 5 | 27 |
| B | Shared expense splitting service | 4 | 5 | 5 | 4 | 5 | 4 | 27 |
| D | Timezone-aware availability poll | 5 | 4 | 5 | 4 | 4 | 4 | 26 |
| C | Study plan and deadline tracker | 5 | 5 | 4 | 3 | 4 | 4 | 25 |
| G | Lending tracker for a friend group | 5 | 3 | 4 | 5 | 3 | 4 | 24 |
| E | Service inventory and health dashboard | 3 | 5 | 4 | 4 | 4 | 3 | 23 |
| H | Recipe and meal plan generator | 4 | 3 | 4 | 4 | 4 | 4 | 23 |
| K | Accessibility checker for local pages | 4 | 4 | 5 | 3 | 4 | 3 | 23 |
| F | Personal finance and receipt tracker | 3 | 4 | 5 | 4 | 3 | 3 | 22 |
| J | URL shortener with analytics | 2 | 3 | 4 | 5 | 3 | 5 | 22 |
| I | Habit tracker with encrypted export | 3 | 3 | 3 | 4 | 3 | 3 | 19 |

## A. Requirement traceability checker, CLI plus service (recommended)

A tool that reads Markdown specifications, extracts `FR-###`, `NFR-###` and
`SC-###` identifiers, and detects orphans, duplicates and coverage gaps. A service
mode serves the resulting matrix as JSON and HTML.

- Stakeholders: classmates taking the same course, other student teams, and this
  repository itself, which is the first user.
- Sample functional requirements: parse a specification directory; detect an
  identifier with no task; detect a task with no requirement; emit a matrix as
  JSON; fail a build with a non-zero exit status; accept a path filter.
- Sample non-functional requirements: parse a 10,000 line specification in under
  one second; deterministic output byte for byte; no network access at runtime;
  zero runtime dependencies beyond the standard library; usable on Windows as well
  as Linux.
- Interfaces: a command line contract, a documented JSON schema, and an HTTP
  route set for the service mode.
- Why it is strong for the report: the project uses its own tool on its own
  specifications, so the reflection section has real evidence, and verification is
  cheap because the logic is deterministic and property-testable.
- Main risk: the demo is a terminal, so the report must carry the value.
- Effort: roughly 50 hours including the service mode.

## B. Shared expense splitting service

Groups, expenses, splits by exact amount, percentage or shares, balances, and a
settle-up suggestion that minimises the number of transfers.

- Stakeholders: friends, roommates, and any group that shares costs.
- Sample functional requirements: create a group and add members; record an
  expense with a payer and a split policy; compute net balances; propose a minimal
  transfer list; undo an expense with an audit trail; export a monthly summary.
- Sample non-functional requirements: all money arithmetic exact to the cent with
  no floating point; balances sum to zero after every operation; balance query
  under 200 ms for 10,000 expenses; every mutation attributable to a user;
  passwords hashed with Argon2id.
- Interfaces: REST endpoints with an OpenAPI document, plus an import format for
  CSV.
- Why it is strong for the report: money has provable invariants, which makes
  property-based testing credible, and the security and audit requirements give
  section 2.3 real content.
- Main risk: authentication and a usable interface both take time, so the scope
  must stay at one split policy first.
- Effort: roughly 70 hours.

## C. Study plan and deadline tracker

Course deadlines, estimated effort per task, a generated weekly plan, reminders
and calendar export.

- Stakeholders: students, including anyone who will test it, and course staff.
- Sample functional requirements: import deadlines; record estimated effort;
  generate a weekly plan respecting available hours; mark work done; export an ICS
  calendar; warn about overloaded weeks.
- Sample non-functional requirements: plan generation under one second for 200
  tasks; calendar files valid per RFC 5545; no deadline lost on restart;
  destructive actions reversible for 24 hours.
- Main risk: the plan generator is the interesting part and is easy to under
  specify, which pushes work into `clarify`.
- Effort: roughly 75 hours.

## D. Timezone-aware availability poll

A Doodle-like tool: propose candidate slots, collect votes, resolve a winner, and
export a calendar entry.

- Stakeholders: study groups, project teams, families.
- Sample functional requirements: create a poll with candidate slots; vote
  available, maybe or unavailable; resolve the winner with tie rules; export the
  outcome as ICS; close a poll.
- Sample non-functional requirements: correct across daylight saving transitions
  for at least three zones; concurrent votes never lost; poll page interactive
  under two seconds; created timestamps stored in UTC.
- Why it is strong for the report: timezone and daylight saving handling is a
  classic source of subtle bugs, so both the prompt iteration log and the defect
  log fill themselves, and correctness is provable with property tests.
- Main risk: deceptively small surface, so extra scope such as notifications may
  be needed to reach a satisfying size.
- Effort: roughly 60 hours.

## E. Service inventory and health dashboard

A registry of services with health probes, relationships, and a read-only status
page.

- Stakeholders: the operator, and household members who consume the status page.
- Sample functional requirements: register a service with an endpoint and tags;
  probe it on an interval; record availability history; render a status page;
  notify on transition; resolve a dependency graph.
- Sample non-functional requirements: probe timeout under three seconds; the probe
  loop survives a hung target; the status endpoint is anonymous and leaks no
  internal addresses; notification deduplicated within an incident.
- Main risk: private infrastructure details must not reach a graded repository, so
  all examples and fixtures must be synthetic.
- Effort: roughly 60 hours.

## F. Personal finance and receipt tracker

Import bank exports, categorise with rules, and report monthly totals.

- Stakeholders: the owner, and anyone who tests with synthetic data.
- Sample functional requirements: import CSV with a mapping profile; apply
  ordered categorisation rules; compute monthly totals per category; detect
  duplicate imports idempotently; export a report.
- Sample non-functional requirements: 50,000 rows imported in under five seconds;
  no account number or amount in logs; imports idempotent under retry.
- Main risk: real financial data must never be committed, so the project runs on
  synthetic fixtures, which weakens the authenticity of testing.
- Effort: roughly 55 hours.

## Shorter options

- G. Lending tracker: items, borrowers, due dates, reminders, history, optional
  ISBN lookup. Small, clean, easy to test, real stakeholders in a friend group,
  but thin on security and scalability.
- H. Recipe and meal plan generator: recipes, weekly plan, dietary constraints,
  aggregated shopping list with unit conversion. The constraint solver is the
  interesting, testable part.
- I. Habit tracker with encrypted export: entries, streaks, search, encrypted
  backup. Encryption is hard to verify well in the time available.
- J. URL shortener with analytics: redirects, click counts, custom slugs. Easy and
  blocker-free, but the stakeholder and requirement story is thin, which costs
  points in section 2.
- K. Accessibility checker: fetch local pages, apply a rule set, report findings
  with a precision and recall measurement against a labelled corpus. Excellent
  verification story, heavier dependency on browser automation.

## Recommendation

Take **A, the requirement traceability checker**, if the priority is a project that
is certain to finish and whose evidence writes itself, because it can be used on
this repository's own specifications. Take **B, a shared-expense tool**, if there is
a friend group available to test with and the priority is a visible demo with
provable invariants. Take **D** if the timezone correctness story appeals and a
study group can act as testers.

Whichever is chosen, record it as `docs/adr/0002-<system>.md` and write
`specs/001-<slug>/spec.md` from it, so the choice is traceable in the same way as
every other decision.
