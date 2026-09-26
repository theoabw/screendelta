# Architecture decision records

One file per decision that changes the specification, the plan, or the process.

```
NNNN-<short-title>.md
```

Record a decision when it deviates from an approved plan or specification, when a
technology is chosen among alternatives, or when the process itself changes. The
report cites these records as evidence in sections 2.4, 4.1 and 7.3.

Required fields: title, status (proposed, accepted, superseded), date, deciders,
context, decision, consequences, alternatives considered. Superseding an earlier
decision means writing a new record and marking the old one `superseded by NNNN`.

Decisions taken so far:

| Record | Decision |
|---|---|
| [0001](0001-adopt-spec-kit.md) | Adopt GitHub Spec Kit 1.0.12 with three agent surfaces and mechanical traceability |
| [0002](0002-frame-delta-engine.md) | Build ScreenDelta, a bounded frame delta and element identity engine, as one stage of a GUI-navigation pipeline |
