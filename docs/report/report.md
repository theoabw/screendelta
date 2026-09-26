# An Experience Report on a Specification-Driven, AI-Assisted Software Construction Project

**Status**: working draft. Every placeholder below is replaced as the project
proceeds. Evidence lives in the repository paths named in each section, so the
report can be assembled from the repository rather than from memory.

## Team Details (Team A)

| Student Name | Student ID | Role in the project | Contribution (%) |
|---|---|---|---|
| Theo Wilenius | 2202234 | All four roles, rotated per phase: specifier, prompt engineer, verifier, auditor | 100 |

The project is done solo, against the course recommendation of teams of three.
State that plainly here and, in section 6.2, how the four roles were separated in
time and how independent review and external user acceptance testing substituted
for a second and third member.

## Executive Summary

Target length is 200 to 300 words. It must state, explicitly:

- [ ] the system that was built,
- [ ] the formal or semi-formal specification techniques used,
- [ ] the AI models and tools used,
- [ ] the SDD workflow followed,
- [ ] the SDD tool kit used, which is GitHub Spec Kit,
- [ ] the verification and validation method that shows the generated code
      implements the specification.

## 1. Introduction and Paradigm Shift (3 points)

### 1.1 Project Overview and Objectives

What the system is, who it serves, and what problem it solves. Record the
selection decision and the alternatives that were rejected in `docs/adr/`.

### 1.2 The Vibe Coding

What vibe coding is, and how this team used it: which parts of the system were
explored in an unstructured way, which prompts drove that exploration, and what
was kept from it. Source material: `docs/prompt-log/`.

### 1.3 The SDD Methodology

Why Specification-Driven Development, why GitHub Spec Kit rather than another
kit, what the specification scope was, and the development timeline. State the
persistence model used for the specification (this repository follows the
flow-forward model: the spec is updated deliberately and deviations are recorded
as ADRs). Diagram: `docs/process/sdd-workflow.svg`, referenced by section 7.4.

## 2. Requirements and System Specification (6 points)

The course template calls this the most critical section, because it documents the
source of truth that was fed to the agents.

### 2.1 Stakeholders

Each stakeholder, what they need from the system, and why they were selected.

### 2.2 Functional Requirements

How functional requirements are written, then the requirements themselves,
summarised by user story. Do not paste the whole specification: cite
`specs/001-<slug>/spec.md` and summarise. Requirement counts by category and by
priority belong here.

### 2.3 Non-Functional Requirements

The quality attributes in scope (performance, security, usability, reliability,
and any others), each with its measurable target and the way it is measured. The
identifiers are `NFR-###` in `specs/001-<slug>/spec.md`, with the measurement
method recorded in `docs/vv/plan.md`.

### 2.4 Technology Stack

The stack, and the reasoning for choosing it over the alternatives. Include the
AI generation and orchestration tools: Spec Kit version, the coding agents and
models, and the reason each was chosen.

### 2.5 Interface and API Specifications

The interfaces the system exposes and consumes, with the contract artifacts from
`specs/001-<slug>/contracts/`.

### 2.6 How These Requirements and Specifications Were Created

The prompts and commands used to derive them, and which parts were rejected or
rewritten. Source material: `docs/prompt-log/` and the analyze reports in
`docs/analysis/`.

## 3. Vibe Coding and the Project (5 points)

The approach taken, how work was divided among members, how the parts were
combined, and what the unstructured phase contributed or cost.

### 3.1 Prompting Framework

The prompting techniques used: chain of thought, role prompting, structured
agentic workflows, or a mix. Justify the choice.

### 3.2 Master Prompts and System Prompts

The system prompts used for each agent surface, quoted verbatim, with the
reasoning, the expected result, the actual result, and the later revisions.
Source material: `docs/prompt-log/`.

### 3.3 Prompt Iteration Log

The table from `docs/prompt-log/iteration-log.md`, including at least one case
where a prompt failed or hallucinated and how the refinement fixed it.

## 4. Specification Driven Development and the Project (7 points)

### 4.1 SDD Tool Kit

GitHub Spec Kit, the version pinned by this repository, and why it was chosen
over the alternatives. The decision record is `docs/adr/0001-adopt-spec-kit.md`.

### 4.2 Development Environment

The operating systems, editors, coding agents and models used, and why. Include
the three command surfaces configured in this repository, and say why a solo
project still needs all three: the specification workflow stays usable from any
agent, and the independent review pass runs in a different one from the authoring
pass, which is what keeps a single author from reviewing their own blind spots.

### 4.3 Deriving the Requirements Using the IDE and Coding Agents

How the functional and non-functional requirements, constraints, interfaces,
acceptance criteria and compliance requirements were produced with the agents.
Include the prompts used.

### 4.4 SDD Workflow and Implementation

The workflow as actually followed, step by step, with the real command order and
the gates that were enforced. `.specify/` holds the constitution, templates and
scripts that implement it.

### 4.5 Prompts

The prompts used during SDD, with reasoning, expectations, what did not go to
plan, and the modifications made. Source material: `docs/prompt-log/`.

## 5. Verification and Validation (6 points)

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
rationale are in `CONTRIBUTING.md` and `AGENTS.md`. Report the measured result:
counts of commits carrying `Spec:` and `Req:` trailers, for example.

### 6.2 Team Contribution Breakdown

How human work shifted from coding to specifying, prompting, auditing and
orchestrating. For each member, name the contribution and the git evidence for it.
Working solo, this section instead shows how the four roles were separated in
time, why each phase was finished before the next started, and how the missing
second and third reviewers were replaced: independent agent review in a fresh
context, self-review against `CONTRIBUTING.md`, and external testers for user
acceptance. Cite the review records in `docs/prompt-log/` and the audit log in
`docs/vv/plan.md`.

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
