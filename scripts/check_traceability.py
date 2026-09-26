#!/usr/bin/env python3
"""Verify requirement traceability between specs/ and docs/traceability.md.

Requirement identifiers are namespaced per specification: FR-001 may exist in
specs/001-a/spec.md and again in specs/002-b/spec.md, and a matrix row is
identified by its spec path plus its identifier. A requirement is only defined
where it appears as a definition line, which is a bullet whose first element is
the bold identifier, for example "- **FR-001**: ...". Other mentions are treated
as references and are ignored, so a spec may cite another spec's identifier.

Checks:
  1. Every identifier defined in a spec has a matrix row naming that spec.
  2. Every matrix row names a spec in which its identifier is defined.
  3. Every status is one of the allowed values.
  4. A row marked in-progress or verified names a task, and that task is listed in
     the feature's tasks.md.
  5. A row marked verified names a test file that exists and evidence that exists,
     which is what makes the verification claim checkable rather than asserted.
  6. Every identifier of a feature whose tasks.md exists is referenced there,
     unless it is deferred or withdrawn.
  7. Matrix rows whose identifier cell is malformed are reported instead of being
     silently skipped.

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
DEFINITION_PATTERN = re.compile(r"^\s*[-*+]\s+\*\*((?:FR|NFR|SC)-\d{3})\*\*")
REFERENCE_PATTERN = re.compile(r"\b(?:FR|NFR|SC)-\d{3}\b")
ROW_PATTERN = re.compile(r"^\|\s*((?:FR|NFR|SC)-\d{3})\s*\|")
CELL_ID_PATTERN = re.compile(r"^\**(?:FR|NFR|SC)-\d{3}\**$")
COMMENT_PATTERN = re.compile(r"<!--.*?-->", re.DOTALL)
TASK_PATTERN = re.compile(r"^T\d{2,}$")

STATUSES = {"planned", "in-progress", "verified", "deferred", "withdrawn"}
INACTIVE_STATUSES = {"deferred", "withdrawn"}
TRACED_STATUSES = {"in-progress", "verified"}
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
FILE_SUFFIXES = {
    ".py",
    ".ts",
    ".tsx",
    ".js",
    ".jsx",
    ".java",
    ".go",
    ".rs",
    ".rb",
    ".cs",
    ".kt",
    ".sh",
    ".feature",
    ".log",
    ".txt",
    ".json",
    ".xml",
    ".html",
    ".md",
}


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


def read_text(findings: Findings, path: Path) -> str | None:
    try:
        return path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        findings.error(f"{relative(path)} is not valid UTF-8")
    except OSError as error:
        findings.error(f"{relative(path)} could not be read: {error.strerror}")
    return None


def relative(path: Path) -> str:
    try:
        return path.relative_to(REPO_ROOT).as_posix()
    except ValueError:
        return path.as_posix()


def normalise_spec(value: str) -> str:
    cleaned = value.strip().strip("`").lstrip("./")
    return cleaned


def split_row(line: str) -> list[str]:
    """Split a markdown table row on unescaped pipes and unescape cells."""
    body = line.strip()
    if body.startswith("|"):
        body = body[1:]
    if body.endswith("|"):
        body = body[:-1]
    cells: list[str] = []
    current: list[str] = []
    escaped = False
    for character in body:
        if escaped:
            current.append("|" if character == "|" else character)
            escaped = False
        elif character == "\\":
            escaped = True
        elif character == "|":
            cells.append("".join(current).strip())
            current = []
        else:
            current.append(character)
    if escaped:
        current.append("\\")
    cells.append("".join(current).strip())
    return cells


def feature_specs() -> list[Path]:
    if not SPECS_DIR.is_dir():
        return []
    return sorted(SPECS_DIR.glob("*/spec.md"))


def collect_definitions(
    findings: Findings, specs: list[Path]
) -> dict[tuple[str, str], int]:
    """Map (spec path, identifier) to the line where it is defined."""
    definitions: dict[tuple[str, str], int] = {}
    for spec in specs:
        text = read_text(findings, spec)
        if text is None:
            continue
        location = relative(spec)
        for number, line in enumerate(without_comments(text).splitlines(), start=1):
            match = DEFINITION_PATTERN.match(line)
            if match is None:
                continue
            identifier = match.group(1)
            if is_example(identifier):
                continue
            key = (location, identifier)
            if key in definitions:
                findings.error(
                    f"{location}:{number}: {identifier} is defined twice in the "
                    f"same specification, first at line {definitions[key]}"
                )
                continue
            definitions[key] = number
    return definitions


def parse_matrix(findings: Findings) -> list[dict[str, object]]:
    if not MATRIX_PATH.is_file():
        findings.error("docs/traceability.md is missing")
        return []
    text = read_text(findings, MATRIX_PATH)
    if text is None:
        return []
    rows: list[dict[str, object]] = []
    for number, line in enumerate(without_comments(text).splitlines(), start=1):
        if not line.strip().startswith("|"):
            continue
        cells = split_row(line)
        first = cells[0] if cells else ""
        strict = ROW_PATTERN.match(line)
        if strict is None:
            if CELL_ID_PATTERN.match(first):
                findings.error(
                    f"docs/traceability.md:{number}: row starts with '{first}' but "
                    "is not a well-formed row; the identifier must be the first "
                    "cell, unformatted and followed by a pipe"
                )
            continue
        identifier = strict.group(1)
        if is_example(identifier):
            continue
        if len(cells) != len(COLUMNS):
            findings.error(
                f"docs/traceability.md:{number}: row for {identifier} has "
                f"{len(cells)} columns, expected {len(COLUMNS)}"
            )
            continue
        row: dict[str, object] = dict(zip(COLUMNS, cells))
        row["line"] = number
        row["spec_path"] = normalise_spec(str(row["Spec"]))
        rows.append(row)
    return rows


def feature_of(spec_path: str) -> str:
    return spec_path.split("/")[1] if "/" in spec_path else spec_path


def check_path_evidence(
    findings: Findings,
    row: dict[str, object],
    identifier: str,
    column: str,
) -> None:
    value = str(row[column]).strip()
    line = row["line"]
    if not value:
        return
    if value.startswith(("http://", "https://")):
        findings.warn(
            f"docs/traceability.md:{line}: {identifier} uses external {column.lower()} "
            f"'{value}', which cannot be checked from the repository"
        )
        return
    candidate = value.split("::")[0].split("#")[0].strip().strip("`")
    if not candidate:
        findings.error(
            f"docs/traceability.md:{line}: {identifier} has an unusable {column.lower()} "
            f"value '{value}'"
        )
        return
    suffix = Path(candidate).suffix.lower()
    if suffix not in FILE_SUFFIXES:
        findings.warn(
            f"docs/traceability.md:{line}: {identifier} names {column.lower()} "
            f"'{value}', which does not look like a file path and cannot be checked"
        )
        return
    if not (REPO_ROOT / candidate).exists():
        findings.error(
            f"docs/traceability.md:{line}: {identifier} names {column.lower()} "
            f"'{candidate}', which does not exist"
        )


def check_rows(
    findings: Findings,
    definitions: dict[tuple[str, str], int],
    rows: list[dict[str, object]],
    task_texts: dict[str, set[str]],
) -> None:
    seen: set[tuple[str, str]] = set()

    for row in rows:
        identifier = str(row["ID"])
        line = row["line"]
        spec_path = str(row["spec_path"])
        status = str(row["Status"]).lower()

        if not spec_path:
            findings.error(
                f"docs/traceability.md:{line}: {identifier} does not name its "
                "specification, so it cannot be matched to a requirement"
            )
            continue

        key = (spec_path, identifier)
        if key in seen:
            findings.error(
                f"docs/traceability.md:{line}: duplicate row for {identifier} in {spec_path}"
            )
            continue
        seen.add(key)

        if key not in definitions:
            elsewhere = sorted(
                spec for (spec, known) in definitions if known == identifier
            )
            if elsewhere:
                findings.error(
                    f"docs/traceability.md:{line}: {identifier} is not defined in "
                    f"{spec_path}; it is defined in {', '.join(elsewhere)}"
                )
            else:
                findings.error(
                    f"docs/traceability.md:{line}: {identifier} is not defined in "
                    f"{spec_path}"
                )
            continue

        if status not in STATUSES:
            findings.error(
                f"docs/traceability.md:{line}: {identifier} has status "
                f"'{row['Status']}', expected one of {sorted(STATUSES)}"
            )
            continue

        task = str(row["Task"]).strip()
        if status in TRACED_STATUSES:
            if not task:
                findings.error(
                    f"docs/traceability.md:{line}: {identifier} is {status} but "
                    "names no task"
                )
            elif not TASK_PATTERN.match(task):
                findings.error(
                    f"docs/traceability.md:{line}: {identifier} names task "
                    f"'{task}', expected an identifier such as T012"
                )
            else:
                tasks_in_feature = task_texts.get(feature_of(spec_path))
                if tasks_in_feature is None:
                    findings.error(
                        f"docs/traceability.md:{line}: {identifier} is {status} but "
                        f"{spec_path.rsplit('/', 1)[0]}/tasks.md does not exist"
                    )
                elif task not in tasks_in_feature:
                    findings.error(
                        f"docs/traceability.md:{line}: {identifier} names task {task}, "
                        "which is not listed in that feature's tasks.md"
                    )

        if status == "verified":
            if not str(row["Test"]).strip():
                findings.error(
                    f"docs/traceability.md:{line}: {identifier} is verified but "
                    "names no test"
                )
            else:
                check_path_evidence(findings, row, identifier, "Test")
            if not str(row["Evidence"]).strip():
                findings.error(
                    f"docs/traceability.md:{line}: {identifier} is verified but "
                    "names no evidence"
                )
            else:
                check_path_evidence(findings, row, identifier, "Evidence")

    for (spec_path, identifier), number in sorted(definitions.items()):
        if (spec_path, identifier) not in seen:
            findings.error(
                f"{spec_path}:{number}: {identifier} has no row in docs/traceability.md"
            )


def check_task_coverage(
    findings: Findings,
    specs: list[Path],
    definitions: dict[tuple[str, str], int],
    rows: list[dict[str, object]],
) -> None:
    inactive = {
        (str(row["spec_path"]), str(row["ID"]))
        for row in rows
        if str(row["Status"]).lower() in INACTIVE_STATUSES
    }
    for spec in specs:
        tasks = spec.parent / "tasks.md"
        if not tasks.is_file():
            continue
        text = read_text(findings, tasks)
        if text is None:
            continue
        referenced = {
            match.group(0) for match in REFERENCE_PATTERN.finditer(without_comments(text))
        }
        location = relative(spec)
        for (spec_path, identifier), number in sorted(definitions.items()):
            if spec_path != location:
                continue
            if (spec_path, identifier) in inactive:
                continue
            if identifier not in referenced:
                findings.error(
                    f"{location}:{number}: {identifier} is not referenced in "
                    f"{relative(tasks)}"
                )


def collect_task_ids(findings: Findings, specs: list[Path]) -> dict[str, set[str]]:
    """Map feature directory name to the set of task identifiers listed."""
    task_texts: dict[str, set[str]] = {}
    for spec in specs:
        tasks = spec.parent / "tasks.md"
        if not tasks.is_file():
            continue
        text = read_text(findings, tasks)
        if text is None:
            continue
        feature = spec.parent.name
        task_texts[feature] = set(re.findall(r"\bT\d{2,}\b", without_comments(text)))
    return task_texts


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--quiet", action="store_true", help="print only errors and the summary"
    )
    parser.add_argument(
        "--require-specs",
        action="store_true",
        help="fail when no specification, or no requirement, exists yet",
    )
    args = parser.parse_args()

    findings = Findings()
    specs = feature_specs()
    definitions = collect_definitions(findings, specs)
    rows = parse_matrix(findings)
    task_texts = collect_task_ids(findings, specs)

    check_rows(findings, definitions, rows, task_texts)
    check_task_coverage(findings, specs, definitions, rows)

    if not specs:
        message = (
            "no specs/*/spec.md found yet: traceability starts once the first "
            "specification exists"
        )
        if args.require_specs:
            findings.error(message)
        else:
            findings.warn(message)
    elif not definitions and args.require_specs:
        findings.error(
            f"{len(specs)} specification(s) found but none defines a requirement; "
            "requirement lines look like '- **FR-001**: ...'"
        )

    if not args.quiet:
        for warning in findings.warnings:
            print(f"warning: {warning}")
    for error in findings.errors:
        print(f"error: {error}", file=sys.stderr)

    verified = sum(1 for row in rows if str(row["Status"]).lower() == "verified")
    print(
        f"traceability: {len(definitions)} requirements in {len(specs)} spec(s), "
        f"{len(rows)} matrix rows, {verified} verified, "
        f"{len(findings.errors)} error(s), {len(findings.warnings)} warning(s)"
    )
    return 1 if findings.errors else 0


if __name__ == "__main__":
    sys.exit(main())
