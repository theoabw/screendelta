#!/usr/bin/env python3
"""Verify requirement traceability between specs/ and docs/traceability.md.

Checks:
  1. Every FR-###, NFR-### and SC-### identifier defined in specs/*/spec.md has a
     row in docs/traceability.md.
  2. Every identifier in the matrix is defined in some specification.
  3. A row marked in-progress or verified names the task that implements it.
  4. A row marked verified names its test and its evidence.
  5. Every status is one of the allowed values.
  6. When specs/<feature>/tasks.md exists, the feature's identifiers are
     referenced there.

Identifiers ending in -000 are treated as format examples and ignored.

Usage:
    python3 scripts/check_traceability.py [--quiet] [--require-specs]

Exit status:
    0  no errors (warnings may still be reported)
    1  at least one error
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
SPECS_DIR = REPO_ROOT / "specs"
MATRIX_PATH = REPO_ROOT / "docs" / "traceability.md"

ID_PATTERN = re.compile(r"\b(?:FR|NFR|SC)-\d{3}\b")
ROW_PATTERN = re.compile(r"^\|\s*((?:FR|NFR|SC)-\d{3})\s*\|")
COMMENT_PATTERN = re.compile(r"<!--.*?-->", re.DOTALL)
STATUSES = {"planned", "in-progress", "verified", "deferred", "withdrawn"}
COLUMNS = [
    "ID",
    "Requirement",
    "Spec",
    "User story",
    "Task",
    "Test",
    "Evidence",
    "Status",
]
TRACED_STATUSES = {"in-progress", "verified"}


class Findings:
    def __init__(self) -> None:
        self.errors: list[str] = []
        self.warnings: list[str] = []

    def error(self, message: str) -> None:
        self.errors.append(message)

    def warn(self, message: str) -> None:
        self.warnings.append(message)


def is_example(identifier: str) -> bool:
    return identifier.endswith("-000")


def without_comments(text: str) -> str:
    return COMMENT_PATTERN.sub("", text)


def relative(path: Path) -> str:
    return path.relative_to(REPO_ROOT).as_posix()


def feature_specs() -> list[Path]:
    if not SPECS_DIR.is_dir():
        return []
    return sorted(SPECS_DIR.glob("*/spec.md"))


def collect_spec_ids(findings: Findings, specs: list[Path]) -> dict[str, str]:
    """Map requirement identifier to the spec that defines it."""
    identifiers: dict[str, str] = {}
    for spec in specs:
        text = without_comments(spec.read_text(encoding="utf-8"))
        location = relative(spec)
        for match in ID_PATTERN.finditer(text):
            identifier = match.group(0)
            if is_example(identifier):
                continue
            previous = identifiers.get(identifier)
            if previous is not None and previous != location:
                findings.error(
                    f"{identifier} is defined in two specs: {previous} and {location}"
                )
                continue
            identifiers[identifier] = location
    return identifiers


def parse_matrix(findings: Findings) -> dict[str, dict[str, object]]:
    if not MATRIX_PATH.is_file():
        findings.error("docs/traceability.md is missing")
        return {}
    rows: dict[str, dict[str, object]] = {}
    lines = MATRIX_PATH.read_text(encoding="utf-8").splitlines()
    for number, line in enumerate(lines, start=1):
        match = ROW_PATTERN.match(line)
        if match is None:
            continue
        identifier = match.group(1)
        if is_example(identifier):
            continue
        cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
        if len(cells) != len(COLUMNS):
            findings.error(
                f"docs/traceability.md:{number}: row for {identifier} has "
                f"{len(cells)} columns, expected {len(COLUMNS)}"
            )
            continue
        if identifier in rows:
            findings.error(
                f"docs/traceability.md:{number}: duplicate row for {identifier}"
            )
            continue
        row: dict[str, object] = dict(zip(COLUMNS, cells))
        row["line"] = number
        rows[identifier] = row
    return rows


def check_rows(
    findings: Findings,
    spec_ids: dict[str, str],
    rows: dict[str, dict[str, object]],
) -> None:
    for identifier, spec in sorted(spec_ids.items()):
        row = rows.get(identifier)
        if row is None:
            findings.error(
                f"{identifier} defined in {spec} has no row in docs/traceability.md"
            )
            continue
        line = row["line"]
        status = str(row["Status"]).lower()
        if status not in STATUSES:
            findings.error(
                f"docs/traceability.md:{line}: {identifier} has status "
                f"'{row['Status']}', expected one of {sorted(STATUSES)}"
            )
            continue
        if status in TRACED_STATUSES and not row["Task"]:
            findings.error(
                f"docs/traceability.md:{line}: {identifier} is {status} "
                "but names no task"
            )
        if status == "verified":
            if not row["Test"]:
                findings.error(
                    f"docs/traceability.md:{line}: {identifier} is verified "
                    "but names no test"
                )
            if not row["Evidence"]:
                findings.error(
                    f"docs/traceability.md:{line}: {identifier} is verified "
                    "but names no evidence"
                )
        if not row["Spec"]:
            findings.warn(
                f"docs/traceability.md:{line}: {identifier} does not name its spec"
            )

    for identifier, row in sorted(rows.items()):
        if identifier not in spec_ids:
            findings.error(
                f"docs/traceability.md:{row['line']}: {identifier} is not defined "
                "in any spec"
            )


def check_task_coverage(
    findings: Findings,
    specs: list[Path],
    spec_ids: dict[str, str],
) -> None:
    for spec in specs:
        tasks = spec.parent / "tasks.md"
        if not tasks.is_file():
            continue
        text = without_comments(tasks.read_text(encoding="utf-8"))
        referenced = {match.group(0) for match in ID_PATTERN.finditer(text)}
        location = relative(spec)
        for identifier, owner in sorted(spec_ids.items()):
            if owner != location:
                continue
            if identifier not in referenced:
                findings.error(
                    f"{identifier} defined in {location} is not referenced in "
                    f"{relative(tasks)}"
                )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--quiet", action="store_true", help="print only errors and the summary"
    )
    parser.add_argument(
        "--require-specs",
        action="store_true",
        help="fail when no specification exists yet",
    )
    args = parser.parse_args()

    findings = Findings()
    specs = feature_specs()
    spec_ids = collect_spec_ids(findings, specs)
    rows = parse_matrix(findings)

    if not specs:
        message = (
            "no specs/*/spec.md found yet: traceability starts once the first "
            "specification exists"
        )
        if args.require_specs:
            findings.error(message)
        else:
            findings.warn(message)
    else:
        check_rows(findings, spec_ids, rows)
        check_task_coverage(findings, specs, spec_ids)

    if not args.quiet:
        for warning in findings.warnings:
            print(f"warning: {warning}")
    for error in findings.errors:
        print(f"error: {error}", file=sys.stderr)

    verified = sum(
        1 for row in rows.values() if str(row["Status"]).lower() == "verified"
    )
    print(
        f"traceability: {len(spec_ids)} requirements in {len(specs)} spec(s), "
        f"{len(rows)} matrix rows, {verified} verified, "
        f"{len(findings.errors)} error(s), {len(findings.warnings)} warning(s)"
    )
    return 1 if findings.errors else 0


if __name__ == "__main__":
    sys.exit(main())
