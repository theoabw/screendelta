# Course project report template

The graded template, captured from the course page on 2026-09-26, so that this
repository is self-contained and a reader can check the report against the
requirements it was written for. `docs/report/report.md` follows this structure
section by section.

Title: An Experience Report on a Specification-Driven, AI-Assisted Software
Construction Project.

## Team details

Student name, student ID, role in the project, contribution percentage, one row
per member.

## Executive summary

Required. 200 to 300 words, stating the system built, the formal or semi-formal
specification techniques used, the AI models and tools used, the SDD workflow
followed, the SDD tool kit used, and the verification and validation method that
shows the generated code implements the specification.

## 1. Introduction and paradigm shift (3 points)

- 1.1 Project overview and objectives.
- 1.2 The vibe coding: what it is, and how the team approached development with it.
- 1.3 The SDD methodology: what SDD is, how the team applied it, which tool was
  used and why, the specification scope, and the development timeline. A workflow
  diagram for both approaches is expected.

## 2. Requirements and system specification (6 points)

The template calls this the most critical section, because it documents the source
of truth fed to the AI.

- 2.1 Stakeholders, and why each was selected.
- 2.2 Functional requirements.
- 2.3 Non-functional requirements: performance, security, usability and
  reliability constraints, each measurable.
- 2.4 Technology stack, including the AI generation and orchestration tools, with
  the reasoning for the choice.
- 2.5 Interface and API specifications.
- 2.6 How the requirements and specifications were created with SDD AI tools.

## 3. Vibe coding and the project (5 points)

The approach, the division of work among members, any promptbook used, the
modules the project was divided into and how they were combined.

- 3.1 Prompting framework: chain of thought, agentic workflows, or other.
- 3.2 Master prompts and system prompts, with reasoning, expectation, outcome and
  revision.
- 3.3 Prompt iteration log, including a failure and its resolution, with the
  initial prompt, the failure mode, the refined prompt and the resulting quality.

## 4. Specification Driven Development and the project (7 points)

- 4.1 SDD tool kit and why it was chosen.
- 4.2 Development environment: system, IDE and coding agents, and why.
- 4.3 Deriving the requirements with the agents: software requirements
  specification, constraints, interfaces, acceptance criteria, compliance.
- 4.4 SDD workflow and implementation, including any workflow the team proposed.
- 4.5 Prompts used during SDD, with reasoning, expectation, outcome and revision.

## 5. Verification and validation (6 points)

- 5.1 V&V strategy: the technique used, the threats to validity, and the levels
  covered, from unit testing to user acceptance and compliance testing.
- 5.2 Code audit log: security, compliance and logical flaws found in generated
  output, and how each was resolved.
- 5.3 Test execution results: expected and actual results with pass or fail
  status, defects found and their resolution, and testing metrics such as code
  coverage.

## 6. Project management and AI toolchain workflows (3 points)

- 6.1 Version control and prompt integration: whether commit messages link code
  changes to prompt iterations and specification changes, and how the workspace
  was configured.
- 6.2 Team contribution breakdown: how roles shifted from coding to specifying,
  prompting, auditing and orchestrating, with each member's contribution visible
  in git.

## 7. Reflections on specification-driven development (5 points)

- 7.1 Benefits of AI-assisted SDD.
- 7.2 Core bottlenecks and challenges, with how each was overcome.
- 7.3 Lessons learned and best practices, evidenced by concrete examples from the
  project rather than preference.
- 7.4 A proposed SDD workflow for future developers: objectives, activities,
  inputs, outputs and deliverables per phase, where AI assistance belongs and
  where human oversight remains essential, justified by the project's evidence,
  with a process diagram.
