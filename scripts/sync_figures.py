#!/usr/bin/env python3
"""Rewrite the measured figures in the report and the README from the newest recorded run.

The report and the README quote measurements that a run produces, and every run produces slightly different ones:
the demo's decision rate moves with machine load, resident memory moves with the runtime's scheduling, and the
latency percentiles move by a few tenths of a millisecond. Regenerating those figures by hand has gone wrong five
times in this repository, twice by overwriting a requirement's target with a measurement, so it is a command now.

The record is the source. This reads the newest `docs/vv/evidence/final-*.txt`, which is the file `final_verify.sh`
writes, and replaces each figure in the two documents that state one, leaving every other number alone: targets
belong to the specification and are not touched here.

Usage: python3 scripts/sync_figures.py [--check]

With --check, report what would change and exit 1 if anything would, which is what a gate wants. Without it, rewrite
the two documents and exit 0.
"""
from __future__ import annotations

import argparse
import pathlib
import re
import sys

REPO = pathlib.Path(__file__).resolve().parent.parent
DOCUMENTS = [REPO / "docs/report/report.md", REPO / "README.md"]


def newest_record() -> pathlib.Path:
    records = sorted((REPO / "docs/vv/evidence").glob("final-*.txt"), key=lambda path: path.stat().st_mtime)
    if not records:
        raise SystemExit("sync_figures: no recorded run to read; run ./scripts/final_verify.sh first")
    return records[-1]


def measured(record: str) -> dict[str, str]:
    """The figures the record states, by name."""
    values = {}
    patterns = {
        "p50": r"p50 ([\d.]+) ms",
        "p95": r"p95 ([\d.]+) ms",
        "p99": r"p99 ([\d.]+) ms",
        "throughput": r"sustained ([\d.]+) frame pairs per second",
        "heap": r"peak heap \d+ bytes \(([\d.]+) MiB\)",
        "resident": r"peak resident \d+ bytes \(([\d.]+) MiB\)",
        "decisions": r"decisions per second ([\d.]+)",
        "pairs": r"([\d,]+) frame pairs scored",
        "regions": r"([\d,]+) regions matched",
        "movements": r"of (\d+) movements reported as moved",
        "attributed": r"of \d+ movements reported as moved \(([\d.]+)",
        "f1": r"F1 ([\d.]+)",
    }
    for name, pattern in patterns.items():
        found = re.search(pattern, record)
        if found:
            values[name] = found.group(1)
    return values


THRESHOLDS = ("at least", "at most", "at or below", "no more than", "no fewer than", "above", "below")


def rewrite(text: str, values: dict[str, str]) -> str:
    """Replace every measured figure in a document, and none of the targets.

    The first version of this function carried the comment that a target has no measured phrasing beside it and then
    replaced "at least 30 frame pairs per second" with the measured rate, which is the third time in this repository
    that a sweep over the documents has overwritten a requirement's target with its own result. The rule is
    mechanical now: a figure that follows a threshold word belongs to the specification, and this leaves it alone.
    The check that compares targets against the specification is what caught this, which is why it exists.
    """
    rules = [
        (r"p50 [\d.]+ ms, p95 [\d.]+ ms, p99 [\d.]+ ms",
         f"p50 {values['p50']} ms, p95 {values['p95']} ms, p99 {values['p99']} ms"),
        (r"p95 [\d.]+ ms per 1080p frame pair", f"p95 {values['p95']} ms per 1080p frame pair"),
        (r"p95 [\d.]+ ms and p99 [\d.]+ ms", f"p95 {values['p95']} ms and p99 {values['p99']} ms"),
        (r"p95 latency [\d.]+ ms", f"p95 latency {values['p95']} ms"),
        (r"p95 at or below 12 ms", "p95 at or below 12 ms"),
        (r"met: p95 [\d.]+ ms, p99 [\d.]+ ms", f"met: p95 {values['p95']} ms, p99 {values['p99']} ms"),
        (r"sustained [\d.]+ frame pairs per second", f"sustained {values['throughput']} frame pairs per second"),
        (r"met: [\d.]+ per second", f"met: {values['throughput']} per second"),
        (r"[\d.]+ frame pairs per second in the latest run", f"{values['throughput']} frame pairs per second in the latest run"),
        (r"([\d.]+) frame pairs per second on one core", f"{values['throughput']} frame pairs per second on one core"),
        (r"([\d.]+) frame pairs per second", f"{values['throughput']} frame pairs per second"),
        (r"[\d.]+ MiB peak heap", f"{values['heap']} MiB peak heap"),
        (r"met: [\d.]+ MiB peak resident", f"met: {values['resident']} MiB peak resident"),
        (r"peak resident [\d.]+ MiB", f"peak resident {values['resident']} MiB"),
        (r"[\d.]+ MiB peak heap, [\d.]+ MiB peak resident",
         f"{values['heap']} MiB peak heap, {values['resident']} MiB peak resident"),
        (r"[\d.]+ decisions per second", f"{values['decisions']} decisions per second"),
        (r"F1 [\d.]+ over [\d,]+ frame pairs and [\d,]+ regions",
         f"F1 {values['f1']} over {values['pairs']} frame pairs and {values['regions']} regions"),
        (r"over [\d,]+ frame pairs and [\d,]+ regions",
         f"over {values['pairs']} frame pairs and {values['regions']} regions"),
    ]
    # A figure that follows a threshold belongs to the specification. Two mistakes are recorded here, both made
    # while writing this function. The guard was a window of preceding text, which crossed a cell boundary in the
    # verification table and skipped a row it should have rewritten. Then it was case-sensitive, so "At least 30",
    # which begins a sentence in the requirement table, was replaced while "at least 30" in the results table was
    # not. And the rules ran one after another, so a rule could match the output of the rule before it.
    #
    # The rules are one alternation applied in a single pass now, with a case-insensitive lookbehind on the words
    # that precede the figure, so a target is left alone whatever its capitalisation and nothing is rewritten twice.
    def guard_alternation(pattern: str) -> str:
        """A lookbehind that refuses a figure introduced by a threshold, in either capitalisation.

        A lookbehind must be fixed width, so the threshold and its following space are spelled out per word rather
        than being matched case-insensitively as a group.
        """
        variants = []
        for threshold in THRESHOLDS:
            variants.append(threshold)
            variants.append(threshold.capitalize())
            variants.append(threshold.upper())
        return "|".join(f"(?<={re.escape(variant)} )" for variant in variants)

    parts = []
    for index, (pattern, _) in enumerate(rules):
        # The digit boundary is what stops a partial match. A guard alone does not: when the lookbehind refused the
        # start of "30", the engine began one character later and matched "0", which is how "At least 30" became
        # "At least 377.9" twice while the guard was in place.
        parts.append(f"(?P<rule{index}>(?<![\\d.])(?<!{guard_alternation(pattern)}){pattern})")
    combined = re.compile("|".join(parts))

    def substitute(match: re.Match[str]) -> str:
        for index, (_, replacement) in enumerate(rules):
            if match.group(f"rule{index}") is not None:
                return replacement
        return match.group(0)

    return combined.sub(substitute, text)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="report what would change and exit non-zero if anything would")
    arguments = parser.parse_args()

    record = newest_record()
    values = measured(record.read_text())
    if not values:
        print(f"sync_figures: {record.name} states no measurement this knows how to read", file=sys.stderr)
        return 2

    changed = []
    for document in DOCUMENTS:
        original = document.read_text()
        updated = rewrite(original, values)
        if updated != original:
            changed.append(document.relative_to(REPO))
            if not arguments.check:
                document.write_text(updated)

    if arguments.check:
        if changed:
            print(f"sync_figures: {', '.join(str(path) for path in changed)} do not follow {record.name}")
            return 1
        print(f"ok    the report and the README follow {record.name}")
        return 0

    if changed:
        print(f"sync_figures: rewrote {', '.join(str(path) for path in changed)} from {record.name}")
    else:
        print(f"sync_figures: the report and the README already follow {record.name}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
