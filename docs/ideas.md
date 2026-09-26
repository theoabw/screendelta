# Candidate systems

The course does not prescribe a system, so this file narrows the choice. Every
candidate is scored against what the report actually grades rather than against
novelty or size.

## How to choose

Throughput is not the constraint: the work is done with coding agents against
specifications, so a larger system with clean module boundaries is not riskier than
a small one. What the score measures is therefore what agents cannot supply and
what the rubric rewards:

1. **Stakeholder access.** Section 5.1 grades validation by real users, and those
   users have to be people you can reach within a week. No agent substitutes.
2. **Requirement richness.** Sections 2.2 and 2.3 want a specification with real
   functional and non-functional content, including security and quality targets
   that are measurable.
3. **Verification strength.** Section 5 is graded on evidence. A system with
   provable properties, such as arithmetic invariants, timezone correctness or
   deterministic parsing, produces that evidence; one whose correctness is a matter
   of opinion does not.
4. **Demo value.** The project meetings and the report both need something that can
   be shown and understood in two minutes.
5. **Low blocker risk.** An external API, a credential, a rate limit or a privacy
   problem can stall a project that is otherwise ready. Agents cannot remove that
   risk.
6. **Agent leverage.** How well the system decomposes into independent slices with
   machine-checkable contracts. This is what makes parallel agent work pay off, and
   it is also what sections 4 and 7 ask the report to evaluate with evidence.

Avoid systems whose core value is a large language model call. The behaviour is
then hard to specify and hard to verify, which costs points in sections 2 and 5
even though the demo looks impressive.

## Scores

Each criterion is scored 1 to 5, where 5 is best. Total is out of 30.

| # | Candidate | Stakeholder access | Requirement richness | Verification strength | Demo value | Low blocker risk | Agent leverage | Total |
|---|---|---|---|---|---|---|---|---|
| B | Shared expense splitting service | 4 | 5 | 5 | 5 | 4 | 5 | 28 |
| A | Requirement traceability checker (CLI plus service) | 5 | 4 | 5 | 3 | 5 | 5 | 27 |
| D | Timezone-aware availability poll | 5 | 4 | 5 | 4 | 4 | 5 | 27 |
| C | Study plan and deadline tracker | 5 | 5 | 4 | 4 | 4 | 4 | 26 |
| G | Lending tracker for a friend group | 5 | 3 | 4 | 3 | 4 | 5 | 24 |
| K | Accessibility checker for local pages | 4 | 4 | 5 | 4 | 3 | 4 | 24 |
| E | Service inventory and health dashboard | 3 | 5 | 4 | 4 | 3 | 4 | 23 |
| H | Recipe and meal plan generator | 4 | 3 | 4 | 4 | 4 | 4 | 23 |
| F | Personal finance and receipt tracker | 3 | 4 | 5 | 3 | 3 | 4 | 22 |
| J | URL shortener with analytics | 2 | 3 | 4 | 3 | 5 | 5 | 22 |
| I | Habit tracker with encrypted export | 3 | 3 | 3 | 3 | 3 | 3 | 18 |

## A. Requirement traceability checker, CLI plus service

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
- Why it scores well: the best stakeholder access of any candidate, the lowest
  blocker risk, and a specification that decomposes into independent, contract
  checked slices. The project uses its own tool on its own specifications, so the
  reflection section has evidence rather than opinion.
- Weakness: the demo is a terminal, so the report has to carry the value.
- Scope: small headroom left, so the service mode and a second input format are
  the obvious places to add depth.

## B. Shared expense splitting service (recommended)

Groups, expenses, splits by exact amount, percentage or shares, balances, and a
settle-up suggestion that minimises the number of transfers.

- Stakeholders: friends, roommates, and any group that shares costs. The tool has
  an obvious real use, which makes recruiting testers easy.
- Sample functional requirements: create a group and add members; record an
  expense with a payer and a split policy; compute net balances; propose a minimal
  transfer list; undo an expense with an audit trail; export a monthly summary.
- Sample non-functional requirements: all money arithmetic exact to the cent with
  no floating point; balances sum to zero after every operation; balance query
  under 200 ms for 10,000 expenses; every mutation attributable to a user;
  passwords hashed with Argon2id.
- Interfaces: REST endpoints with an OpenAPI document, plus a CSV import format.
- Why it scores well: money has provable invariants, which makes property-based
  testing credible rather than decorative, and the security and audit requirements
  give section 2.3 real content. The settle-up algorithm and the audit trail give
  two independent slices that agents can build in parallel behind a contract.
- Weakness: the stakeholder set is people you have to recruit, and the balance
  arithmetic must be specified precisely before implementation, which is exactly
  the kind of work section 2 grades.
- Scope: comfortably the largest of the shortlist, which is now an advantage
  rather than a cost.

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
- Weakness: the plan generator is the interesting part and is easy to under
  specify, which pushes work into `clarify` and can stall the phase gates.
- Scope: the widest requirement surface of the shortlist, and the most dependent
  on getting the specification right early.

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
- Why it scores well: timezone and daylight saving handling is a classic source of
  subtle bugs, so both the prompt iteration log and the defect log fill themselves,
  and correctness is provable with property tests.
- Weakness: deceptively small surface, so depth has to come from notifications,
  identity handling or an administration view.

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
- Weakness: the stakeholder set is thin, and private infrastructure details must
  not reach a graded repository, so every example and fixture has to be synthetic.

## F. Personal finance and receipt tracker

Import bank exports, categorise with rules, and report monthly totals.

- Stakeholders: the owner, and anyone who tests with synthetic data.
- Sample functional requirements: import CSV with a mapping profile; apply
  ordered categorisation rules; compute monthly totals per category; detect
  duplicate imports idempotently; export a report.
- Sample non-functional requirements: 50,000 rows imported in under five seconds;
  no account number or amount in logs; imports idempotent under retry.
- Weakness: real financial data must never be committed, so the project runs on
  synthetic fixtures, which weakens the authenticity of testing.

## Shorter options

- G. Lending tracker: items, borrowers, due dates, reminders, history, optional
  ISBN lookup. Clean and easy to test, real stakeholders in a friend group, but
  thin on security and scalability.
- H. Recipe and meal plan generator: recipes, weekly plan, dietary constraints,
  aggregated shopping list with unit conversion. The constraint solver is the
  interesting, testable part.
- I. Habit tracker with encrypted export: entries, streaks, search, encrypted
  backup. Encryption is hard to verify well, which is why it scores lowest.
- J. URL shortener with analytics: redirects, click counts, custom slugs. Easy and
  blocker free, but the stakeholder and requirement story is thin, which costs
  points in section 2.
- K. Accessibility checker: fetch local pages, apply a rule set, report findings
  with a precision and recall measurement against a labelled corpus. Excellent
  verification story, heavier dependency on browser automation.

## Recommendation

Take **B, the shared expense splitting service**, if two or three people will act
as testers: it has the strongest requirement content, a demo that explains itself,
and money invariants that make the verification section credible.

Take **A, the requirement traceability checker**, if recruiting testers is
uncertain. It has the best stakeholder access, the lowest blocker risk, and the
unusual advantage that it can be run against this repository's own specification,
which turns the reflection section into a demonstration.

Take **D** if a study group is available and the timezone correctness story
appeals, since it produces a defect log almost on its own.

Whichever is chosen, record it as `docs/adr/0002-<system>.md` and write
`specs/001-<slug>/spec.md` from it, so the choice is traceable in the same way as
every other decision.

## What changed when capacity stopped being the constraint

The earlier version of this file scored "solo feasibility" and estimated hours.
With agents doing the construction, size is no longer a differentiator, so that
criterion was replaced by agent leverage: how cleanly the system splits into
independent slices with checkable contracts. The practical consequence is visible
in the ranking. B, the largest candidate on the shortlist and the one previously
penalised for its scope, now leads, because its extra size buys requirement
richness and demo value at no cost in risk.
