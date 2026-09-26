# Analyze reports

`/speckit.analyze` is read-only by design: it prints a cross-artifact
consistency report and writes nothing to disk. Without capturing it, the
strongest quality gate in the workflow leaves no trace, and section 2 of the
report depends on exactly that evidence.

## Convention

Save each run as:

```
NNN-<feature-slug>-analyze-YYYY-MM-DD.md
```

`NNN` is the feature number as it appears in `specs/NNN-`. When the same feature
is analyzed more than once on one day, append `-2`, `-3` and so on, so that no run
overwrites an earlier one. Include the date, the command used, the full report
text, and the actions taken for each finding. A finding that is accepted as a
deliberate deviation is recorded as an entry in `docs/adr/`, not silently ignored.

## What to do with the findings

| Finding severity | Action |
|---|---|
| Critical or high | Fix the specification or the plan before implementing, in its own commit |
| Medium | Record the decision: fix now, or defer with a reason in `docs/vv/plan.md` |
| Low | Record and continue |
