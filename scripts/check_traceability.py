#!/usr/bin/env python3
"""Verify requirement traceability between specs/ and docs/traceability.md.

Requirement identifiers are namespaced per specification: FR-001 may exist in
specs/001-a/spec.md and again in specs/002-b/spec.md, and a matrix row is
identified by its spec path plus its identifier. A requirement is only defined
where it appears as a definition line, which is a bullet whose first element is
the bold identifier, for example "- **FR-001**: ...". Other mentions are treated
as references and are ignored, so a spec may cite another spec's identifier.
Definitions inside fenced code blocks are ignored, because they are examples.

A task is a task entry, not a mention: a checkbox line, a numbered line, or a
heading whose first token is the task identifier. Plain bullet text is not a task
entry, because it cannot be told apart from prose. An entry's own line and its
continuation lines together form the text the identifier is looked for in.

What a green run means:
  - every requirement defined in a specification has a matrix row naming that
    specification;
  - every row names a specification that defines its identifier, with no duplicate
    row for the same specification and identifier;
  - every status is one of the allowed values;
  - a row marked in-progress or verified names a task that exists as a task entry
    in that feature's tasks.md, and no other task entry in that feature references
    the requirement;
  - a row marked verified names a test file and an evidence file that both exist
    inside the repository and are files, not directories;
  - a requirement of a feature that has a tasks.md is referenced there, unless its
    row is deferred or withdrawn.

What a green run does not mean:
  - that the named test actually exercises the requirement, which no static check
    can establish;
  - that the evidence proves what the row claims;
  - that every requirement is implemented. A planned row may name neither a task
    nor a test, by design.

Warnings are printed for values the checker cannot judge, such as an external URL
used as evidence or a command string used instead of a file path. Run without
--require-specs while no specification exists yet, and with it afterwards, which
also rejects a specification set that defines no requirement at all.

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
LOOSE_ID_PATTERN = re.compile(r"(?:FR|NFR|SC)-\d+")
DEFINITION_PATTERN = re.compile(r"^\s*[-*+]\s+\*\*((?:FR|NFR|SC)-\d{3})\*\*")
ROW_PATTERN = re.compile(r"^\|\s*((?:FR|NFR|SC)-\d{3})\s*\|")
CHECKBOX_TASK_PATTERN = re.compile(r"^\s*(?:[-*+]|\d+\.)\s*\[[ xX]\]\s*(T\d{2,})\b")
NUMBERED_TASK_PATTERN = re.compile(r"^\s*\d+\.\s*(T\d{2,})\b")
HEADING_TASK_PATTERN = re.compile(r"^\s*#{1,6}\s+(T\d{2,})\b")
TASK_PATTERNS = (
    CHECKBOX_TASK_PATTERN,
    NUMBERED_TASK_PATTERN,
    HEADING_TASK_PATTERN,
)
TASK_PATTERN = re.compile(r"^T\d{2,}$")
FENCE_PATTERN = re.compile(r"^\s*(`{3,}|~{3,})")

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


def identifiers_in(text: str) -> set[str]:
    """Identifiers in text, matched exactly so FR-001 is not NFR-001."""
    return {match.group(0) for match in ID_PATTERN.finditer(text)}


def relative(path: Path) -> str:
    try:
        return path.relative_to(REPO_ROOT).as_posix()
    except ValueError:
        return path.as_posix()


def read_lines(findings: Findings, path: Path) -> list[str] | None:
    try:
        text = path.read_text(encoding="utf-8")
    except UnicodeDecodeError:
        findings.error(f"{relative(path)} is not valid UTF-8")
        return None
    except OSError as error:
        findings.error(f"{relative(path)} could not be read: {error.strerror}")
        return None
    return text.splitlines()


def blank_comments(findings: Findings, lines: list[str], location: str) -> list[str]:
    """Blank HTML comments while preserving line numbering."""
    output: list[str] = []
    inside = False
    for line in lines:
        result: list[str] = []
        index = 0
        while index < len(line):
            if inside:
                end = line.find("-->", index)
                if end == -1:
                    index = len(line)
                else:
                    inside = False
                    index = end + 3
            else:
                start = line.find("<!--", index)
                if start == -1:
                    result.append(line[index:])
                    index = len(line)
                else:
                    result.append(line[index:start])
                    inside = True
                    index = start + 4
        output.append("".join(result))
    if inside:
        findings.error(
            f"{location}: unterminated HTML comment; everything after it is ignored"
        )
    return output


def blank_code_fences(lines: list[str]) -> list[str]:
    """Blank fenced code blocks, tracking delimiter character and length.

    A block opened with four backticks is only closed by four or more backticks,
    so a three backtick fence inside it is content rather than a new block.
    """
    output: list[str] = []
    opened_with: tuple[str, int] | None = None
    for line in lines:
        match = FENCE_PATTERN.match(line)
        if match is not None:
            marker = match.group(1)
            if opened_with is None:
                opened_with = (marker[0], len(marker))
            elif marker[0] == opened_with[0] and len(marker) >= opened_with[1]:
                opened_with = None
            output.append("")
            continue
        output.append("" if opened_with is not None else line)
    return output


def prepare(findings: Findings, lines: list[str], location: str) -> list[str]:
    return blank_code_fences(blank_comments(findings, lines, location))


def normalise_spec(value: str) -> str:
    cleaned = value.strip().strip("`").strip()
    if cleaned.startswith("./"):
        cleaned = cleaned[2:]
    return cleaned


def spec_path_problem(spec_path: str) -> str | None:
    if not spec_path:
        return (
            "does not name its specification, so it cannot be matched to a requirement"
        )
    if spec_path.startswith("/"):
        return "must be a repository-relative path, not an absolute one"
    if ".." in Path(spec_path).parts:
        return "must not traverse outside the repository"
    return None


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
        lines = read_lines(findings, spec)
        if lines is None:
            continue
        location = relative(spec)
        for number, line in enumerate(prepare(findings, lines, location), start=1):
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


def task_entry_id(line: str) -> str | None:
    for pattern in TASK_PATTERNS:
        match = pattern.match(line)
        if match is not None:
            return match.group(1)
    return None


def collect_task_entries(
    findings: Findings, specs: list[Path]
) -> dict[str, dict[str, tuple[int, str]]]:
    """Map feature directory name to task entries: identifier to (line, text).

    The text of an entry is its own line plus the continuation lines that follow
    it, joined, so a requirement named on a wrapped line still counts as owned by
    that task.
    """
    entries: dict[str, dict[str, tuple[int, str]]] = {}
    for spec in specs:
        tasks = spec.parent / "tasks.md"
        if not tasks.is_file():
            continue
        lines = read_lines(findings, tasks)
        if lines is None:
            continue
        feature = spec.parent.name
        found: dict[str, tuple[int, str]] = {}
        current_id: str | None = None
        current_line = 0
        buffer: list[str] = []

        def flush() -> None:
            if current_id is None:
                return
            found.setdefault(current_id, (current_line, " ".join(buffer)))

        for number, line in enumerate(prepare(findings, lines, relative(tasks)), start=1):
            entry_id = task_entry_id(line)
            if entry_id is not None:
                flush()
                current_id = entry_id
                current_line = number
                buffer = [line.strip()]
                continue
            if current_id is None:
                continue
            stripped = line.strip()
            if not stripped:
                flush()
                current_id = None
                buffer = []
                continue
            if line[:1] in (" ", "\t") or not stripped.startswith("#"):
                buffer.append(stripped)
                continue
            flush()
            current_id = None
            buffer = []
        flush()
        entries[feature] = found
    return entries


def parse_matrix(findings: Findings) -> list[dict[str, object]]:
    if not MATRIX_PATH.is_file():
        findings.error("docs/traceability.md is missing")
        return []
    lines = read_lines(findings, MATRIX_PATH)
    if lines is None:
        return []
    rows: list[dict[str, object]] = []
    for number, line in enumerate(
        prepare(findings, lines, "docs/traceability.md"), start=1
    ):
        if not line.strip().startswith("|"):
            continue
        cells = split_row(line)
        first = cells[0] if cells else ""
        strict = ROW_PATTERN.match(line)
        if strict is None:
            if LOOSE_ID_PATTERN.search(first):
                findings.error(
                    f"docs/traceability.md:{number}: row starts with '{first}' but "
                    "is not a well-formed row; the identifier must be the first "
                    "cell, unformatted, three digits, and followed by a pipe"
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


def looks_like_path(value: str) -> bool:
    return "/" in value or "\\" in value or bool(Path(value).suffix)


def check_file_value(
    findings: Findings,
    row: dict[str, object],
    identifier: str,
    column: str,
) -> None:
    """Require the named file to exist inside the repository."""
    value = str(row[column]).strip()
    if not value:
        return
    line = row["line"]
    label = column.lower()
    if value.startswith(("http://", "https://")):
        findings.warn(
            f"docs/traceability.md:{line}: {identifier} uses external {label} "
            f"'{value}', which cannot be checked from the repository"
        )
        return
    candidate = value.split("::")[0].split("#")[0].strip().strip("`").strip()
    if not candidate:
        findings.error(
            f"docs/traceability.md:{line}: {identifier} has an unusable {label} "
            f"value '{value}'"
        )
        return
    path = Path(candidate)
    if path.is_absolute():
        findings.error(
            f"docs/traceability.md:{line}: {identifier} names {label} "
            f"'{candidate}', which is an absolute path outside the repository"
        )
        return
    if ".." in path.parts:
        findings.error(
            f"docs/traceability.md:{line}: {identifier} names {label} "
            f"'{candidate}', which traverses outside the repository"
        )
        return
    resolved = REPO_ROOT / path
    if resolved.exists():
        if resolved.is_file():
            return
        findings.error(
            f"docs/traceability.md:{line}: {identifier} names {label} "
            f"'{candidate}', which is not a file"
        )
        return
    if looks_like_path(candidate):
        findings.error(
            f"docs/traceability.md:{line}: {identifier} names {label} "
            f"'{candidate}', which does not exist"
        )
        return
    findings.warn(
        f"docs/traceability.md:{line}: {identifier} names {label} '{value}', which "
        "looks like a command rather than a file path; name the file that holds "
        "the evidence"
    )


def check_rows(
    findings: Findings,
    definitions: dict[tuple[str, str], int],
    rows: list[dict[str, object]],
    task_entries: dict[str, dict[str, tuple[int, str]]],
) -> None:
    seen: set[tuple[str, str]] = set()

    for row in rows:
        identifier = str(row["ID"])
        line = row["line"]
        spec_path = str(row["spec_path"])
        status = str(row["Status"]).lower()

        problem = spec_path_problem(spec_path)
        if problem is not None:
            findings.error(f"docs/traceability.md:{line}: {identifier} {problem}")
            continue

        key = (spec_path, identifier)
        if key in seen:
            findings.error(
                f"docs/traceability.md:{line}: duplicate row for {identifier} in "
                f"{spec_path}"
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
            feature = feature_of(spec_path)
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
                entries = task_entries.get(feature)
                if entries is None:
                    findings.error(
                        f"docs/traceability.md:{line}: {identifier} is {status} but "
                        f"{spec_path.rsplit('/', 1)[0]}/tasks.md does not exist"
                    )
                elif task not in entries:
                    findings.error(
                        f"docs/traceability.md:{line}: {identifier} names task "
                        f"{task}, which is not a task entry in that feature's "
                        "tasks.md"
                    )
                elif identifier not in identifiers_in(entries[task][1]):
                    owners = sorted(
                        other
                        for other, (_, text) in entries.items()
                        if other != task and identifier in identifiers_in(text)
                    )
                    if owners:
                        findings.error(
                            f"docs/traceability.md:{line}: {identifier} names task "
                            f"{task}, but the requirement is referenced by "
                            f"{', '.join(owners)} in that feature's tasks.md"
                        )
                    else:
                        findings.warn(
                            f"docs/traceability.md:{line}: task {task} in "
                            f"{feature}/tasks.md does not reference {identifier} at "
                            "all; confirm that the mapping is right"
                        )

        if status == "verified":
            if not str(row["Test"]).strip():
                findings.error(
                    f"docs/traceability.md:{line}: {identifier} is verified but "
                    "names no test"
                )
            else:
                check_file_value(findings, row, identifier, "Test")
            if not str(row["Evidence"]).strip():
                findings.error(
                    f"docs/traceability.md:{line}: {identifier} is verified but "
                    "names no evidence"
                )
            else:
                check_file_value(findings, row, identifier, "Evidence")

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
        lines = read_lines(findings, tasks)
        if lines is None:
            continue
        location = relative(spec)
        referenced: set[str] = set()
        for line in prepare(findings, lines, relative(tasks)):
            referenced.update(identifiers_in(line))
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


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--quiet", action="store_true", help="print only errors and the summary"
    )
    parser.add_argument(
        "--require-specs",
        action="store_true",
        help=(
            "fail when no specification exists, or when the specifications define "
            "no requirement"
        ),
    )
    args = parser.parse_args()

    findings = Findings()
    specs = feature_specs()
    definitions = collect_definitions(findings, specs)
    rows = parse_matrix(findings)
    task_entries = collect_task_entries(findings, specs)

    check_rows(findings, definitions, rows, task_entries)
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
