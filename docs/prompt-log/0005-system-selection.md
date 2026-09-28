# 0005 System selection and the first specification

- Date: 2026-09-26
- Author: Theo Wilenius
- Agent and model: dsh, deepseek-flash
- Spec Kit command: `/speckit.specify` performed by hand against the project template
- Artifacts produced: `.specify/memory/constitution.md`, `docs/adr/0002-frame-delta-engine.md`, `specs/001-frame-delta-engine/spec.md`, `docs/traceability.md`
- Related requirement IDs: FR-001 to FR-016, NFR-001 to NFR-010, SC-001 to SC-006
- Related commit: the specification commit on `main`, listed in `git log`

## Prompt (verbatim)

The selection ran over several turns. The prompts that moved it:

> These feel so bureaucratic in a way

> Anything that would be useful for me and that I could publish on GitHub as a real
> project after the course? [redacted: a reference to the author's own home-automation tooling]

> None of these feel particularly nice

> Let's pivot away from the agent sandboxes maybe

> What could be nice is an universal GUI parser that could analyze edges and all of
> that to determine really fast what it's looking at and where everything is. Not
> sure if I would need to train a model on it though and if that's acceptable for
> the course

> Nvm this becomes maybe too complex and I could do it outside the course instead

> Pick the one idea that could serve as a small component in a bigger pipeline that
> handles computer GUI parsing in the future. The biggest need for this is that AI
> models nowadays still aren't that fast at navigating legacy GUIs on desktops. It
> doesn't need to do the whole process, but at least do one part of it really well
> and so that it could be chained with other stuff

## Intent

Choose a system for the course project that is worth finishing: interesting to
build, publishable afterwards, and small enough that the specification and the
report are not rushed.

## Expected output

A shortlist that converges in one or two turns, then a specification for the chosen
system.

## Actual output

Six proposals were rejected before the system was chosen:

1. Process and traceability tooling was rejected as bureaucratic.
2. A set of ideas drawn from the author's own infrastructure tooling was rejected as not nice, after a tool for splitting shared expenses was rejected as a solved consumer problem.
3. Agent-sandbox and agent-tooling ideas were rejected outright.
4. The full GUI parser the owner proposed was rejected as too large, and the owner
   agreed, keeping it as a side project.

What finally worked was extracting two criteria and one constraint instead of
listing domains: which technical core is enjoyable (performance and optimisation,
compilers and parsers, VMs), what the finished project must be (fun to finish well),
and where the component sits (one stage, chainable, not the whole pipeline).

## What went wrong

The early shortlists were built from what the rubric rewards and what is
publishable, and they ignored whether the work would be enjoyable for three weeks.
That produced a run of plausible but lifeless suggestions, and each rejection cost
a turn. The GUI parser proposal was good but unbounded: "universal" plus an optional
trained model made it a research project rather than a course project.

## Correction

Three changes to how the options were generated:

1. Ask what is enjoyable before proposing domains, not after three rejections.
2. Cut the proposed idea to a component instead of abandoning it, which kept the
   owner's motivation and removed the scope that made it infeasible.
3. State the pipeline position in the specification itself, so the component is
   defined by its contract with the stages around it rather than by a feature list.

The chosen component, a deterministic frame delta and element identity engine, is
the front half of GUI navigation: it removes the redundant work that makes model
driven navigation slow, and it chains forward to any detector and back in after an
action to answer whether the action did anything.

## Result

The constitution fixes five principles, of which two shape every requirement: never
fabricate a result, and keep the interface chainable. The specification defines 16
functional requirements, 10 non-functional requirements and 6 success criteria, all
traced as `planned` in `docs/traceability.md`, and `make check-strict` reports 32
requirements, 32 matrix rows and zero errors. The technology stack is deliberately
left to the plan, constrained by the constitution to single-command distribution,
predictable numeric behaviour and a native image path within the latency budget.

## Redactions

Two passages were redacted before this repository was published, because they named systems and private projects of
the author's rather than anything the course asked about. The list of rejected ideas lost the names of those tools, and
one quoted prompt that referred to two of them is marked as redacted in place. The reasoning, the sequence and the
outcome are unchanged, and nothing else in this log was edited.
