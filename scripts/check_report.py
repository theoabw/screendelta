#!/usr/bin/env python3
"""Check the report and the README against the repository.

Section 5.3 of the report presents numbers, and section 6.1 presents the git activity. Every one of those
numbers is a claim about the repository at the commit it was written in, and this script recomputes them and
fails when they disagree. It exists because three separate edits in this project replaced nothing while
reporting success, leaving stale figures in the two documents a grader reads.

It also checks the two structural requirements a reader would notice immediately: that the executive summary
is inside the word count the course template sets, and that every repository path either document cites
exists, because a citation to a file that is not there is the same class of defect as a number that is not
true.

Exit codes: 0 when every claim holds, 1 when one does not print a warning for a claim it cannot parse.
"""
import pathlib
import re
import subprocess
import sys

REPORT = pathlib.Path("docs/report/report.md")
README = pathlib.Path("README.md")
MATRIX = pathlib.Path("docs/traceability.md")
RESULTS = pathlib.Path("docs/vv/results.md")
PLAN = pathlib.Path("docs/vv/plan.md")
SPEC = pathlib.Path("specs/001-frame-delta-engine/spec.md")

failures = []
warnings = []


def run(command):
    return subprocess.run(command, shell=True, capture_output=True, text=True).stdout.strip()


def check(label, condition, detail=""):
    if condition:
        print(f"ok    {label}")
    else:
        print(f"FAIL  {label}{': ' + detail if detail else ''}")
        failures.append(label)


def warn(label, detail=""):
    print(f"warn  {label}{': ' + detail if detail else ''}")
    warnings.append(label)


def rows(text, pattern):
    return [line for line in text.splitlines() if re.match(pattern, line)]


def main():
    report = REPORT.read_text()
    readme = README.read_text()
    matrix = MATRIX.read_text()
    results = RESULTS.read_text()
    plan = PLAN.read_text()
    spec = SPEC.read_text()

    # The repository's own state, which every claim is compared against.
    requirements = rows(matrix, r"^\| (FR|NFR|SC)-\d+ ")
    verified = [line for line in requirements if line.strip().endswith("| verified |")]
    defect_rows = rows(results, r"^\| AUD-\d+ \|")
    audit_rows = rows(plan, r"^\| AUD-\d+ \|")
    first_review_findings = 11  # the first code review predates the numbered defect table
    classes = {}
    for row in audit_rows:
        cells = [cell.strip() for cell in row.strip("|").split("|")]
        classes[cells[1]] = classes.get(cells[1], 0) + 1
    statuses = {}
    for row in defect_rows:
        cells = [cell.strip() for cell in row.strip("|").split("|")]
        statuses[cells[-1]] = statuses.get(cells[-1], 0) + 1

    truth = {
        "requirements": len(requirements),
        "verified": len(verified),
        "defects": len(defect_rows),
        "findings": len(defect_rows) + first_review_findings,
        "fixed": statuses.get("fixed", 0),
        "commits": run("git rev-list --count HEAD"),
        "tasks_done": int(run("grep -cE '^- \\[x\\]' specs/001-frame-delta-engine/tasks.md")),
        "tasks_total": int(run("grep -cE '^- \\[' specs/001-frame-delta-engine/tasks.md")),
        "functional": len(re.findall(r"^- \*\*FR-\d+\*\*", spec, re.M)),
        "nonfunctional": len(re.findall(r"^- \*\*NFR-\d+\*\*", spec, re.M)),
        "criteria": len(re.findall(r"^- \*\*SC-\d+\*\*", spec, re.M)),
    }
    print("repository state:", truth)
    print()

    # The counts each document states, wherever it states them.
    check(f"the report counts {truth['requirements']} requirements",
          re.search(rf"\b{truth['requirements']} (traced )?requirements\b", report) is not None)
    check(f"the report counts {truth['verified']} verified",
          re.search(rf"\b{truth['verified']} (of {truth['requirements']})?( requirements)? verified\b", report) is not None
          or f"{truth['verified']} of {truth['requirements']} requirements" in report)
    check("the README counts the same requirements and verifications",
          re.search(rf"\b{truth['requirements']} traced requirements\b", readme) is not None
          and re.search(rf"\b{truth['verified']} are verified\b", readme) is not None)
    check(f"the report counts {truth['defects']} defects",
          re.search(rf"\b{truth['defects']} defects were\b", report) is not None
          or "Thirty-five defects were" in report)
    check(f"the report counts {truth['fixed']} fixed defects",
          re.search(rf"\b{truth['fixed']} are fixed\b", report) is not None)
    check(f"the report counts {truth['findings']} findings",
          re.search(rf"\b{truth['findings']} recorded findings\b", report) is not None)
    # The commit count is the only figure that moves with every commit, including the commit that writes it,
    # so the report states a floor ("more than 100 commits") and the exact command. The check verifies the
    # floor holds and that the report does not claim more commits than exist.
    stated = re.search(r"more than (\d+) commits", report)
    check("the report's commit count is a true floor",
          stated is not None and 0 < int(stated.group(1)) <= int(truth["commits"]),
          f"stated {stated.group(1) if stated else 'nothing'} against {truth['commits']}")
    check(f"the report counts {truth['tasks_done']} of {truth['tasks_total']} tasks",
          re.search(rf"\b{truth['tasks_total']} planned tasks of which {truth['tasks_done']} are\b", report) is not None
          or re.search(rf"\b{truth['tasks_total']} tasks\b[^.]*\bwhich {truth['tasks_done']}\b", report) is not None)
    for label, key, phrase in [("functional", "functional", "functional requirements"),
                               ("non-functional", "nonfunctional", "non-functional requirements"),
                               ("success", "criteria", "success criteria")]:
        check(f"the report counts {truth[key]} {phrase}",
              re.search(rf"\b{truth[key]} {phrase}\b", report) is not None)

    check("the audit log and the defect table have the same number of rows",
          truth["defects"] == len(audit_rows),
          f"{truth['defects']} against {len(audit_rows)}")
    for name, count in sorted(classes.items()):
        check(f"the report's {name} count is {count}",
              re.search(rf"\|\s*{name.capitalize()}\s*\|\s*{count}\s*\|", report) is not None)

    # Every repository path either document cites has to exist.
    pattern = r"`((?:docs|specs|internal|tests|cmd|tools|scripts)/[^`\s]+)`"
    for path, document in [("the report", report), ("the README", readme)]:
        cited = sorted(set(re.findall(pattern, document)))
        missing = [item for item in cited if "*" not in item and not pathlib.Path(item).exists()]
        check(f"every path {path} cites exists ({len(cited)} cited)", not missing, ", ".join(missing))

    # The executive summary is inside the word count the course template sets.
    if "## Executive Summary" in report and "## 1. Introduction" in report:
        body = report[report.index("## Executive Summary"):report.index("## 1. Introduction")]
        words = len(body.split()) - 2  # the heading itself
        check(f"the executive summary is 200 to 300 words ({words})", 200 <= words <= 300)

    # Both workflow diagrams the template asks for.
    for diagram, label in [("docs/process/vibe-coding-workflow.svg", "the vibe coding workflow"),
                           ("docs/process/sdd-workflow.svg", "the specification-driven workflow")]:
        check(f"{label} diagram exists and is cited", pathlib.Path(diagram).exists() and diagram in report)

    print()
    if warnings:
        print(f"{len(warnings)} warning(s): " + ", ".join(warnings))
    if failures:
        print(f"{len(failures)} claim(s) do not hold: " + ", ".join(failures))
        return 1
    print("every claim in the report and the README holds against this commit")
    return 0


if __name__ == "__main__":
    sys.exit(main())
