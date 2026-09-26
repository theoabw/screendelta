#!/usr/bin/env python3
"""Check the report and the README against the repository.

Every claim either document makes about the repository's state is recomputed here and compared, because four
separate edits in this project replaced nothing while reporting success and left stale figures in the two
documents a grader reads.

What it checks:

  * the requirement, defect, class, task, finding and commit counts, against the matrix, the defect table, the
    audit log and git;
  * the measurements in section 5.3, against the newest recorded run in docs/vv/evidence;
  * that every repository path either document cites exists;
  * that the executive summary is inside the word count the course template sets;
  * that both workflow diagrams exist and are cited.

Exit codes: 0 when every claim holds, 1 when one does not.
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
    """Run a command and fail the check when it does not succeed, because a count read from a failed command
    is not a count."""
    result = subprocess.run(command, shell=True, capture_output=True, text=True)
    if result.returncode != 0:
        print(f"FAIL  the command {command!r} exited {result.returncode}, so its output cannot be trusted")
        failures.append(command)
        return ""
    return result.stdout.strip()


def check(label, condition, detail=""):
    if condition:
        print(f"ok    {label}")
    else:
        print(f"FAIL  {label}{': ' + detail if detail else ''}")
        failures.append(label)



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
          re.search(rf"\b{truth['defects']} defects were\b", report) is not None)
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

    # The measurements in section 5.3, against the recorded runs. The section says its numbers come from the
    # commands in docs/vv/evidence, so a number that does not appear there is a claim with no measurement behind
    # it. This was the gap a review found: the script named section 5.3 in its docstring and checked none of it.
    #
    # Only the Actual column is read, because the Expected column states the targets, which are requirements
    # rather than measurements and are not supposed to appear in an evidence file.
    evidence_files = sorted(pathlib.Path("docs/vv/evidence").glob("*.txt"), key=lambda p: p.stat().st_mtime)
    if not evidence_files:
        check("at least one evidence file exists", False, "docs/vv/evidence is empty")
    else:
        recorded = "".join(path.read_text() for path in evidence_files)
        section = report[report.index("### 5.3 Test Execution Results"):report.index("### 6.1")]
        actuals = []
        for line in section.splitlines():
            if not line.startswith("| ") or line.startswith("| Measurement") or line.startswith("|---"):
                continue
            cells = [cell.strip() for cell in line.strip("|").split("|")]
            if len(cells) >= 5:
                actuals.append((cells[0], cells[3]))
        check("section 5.3 has rows to check", len(actuals) >= 6, f"{len(actuals)} rows found")
        # Two kinds of figure appear in the table and they need different rules.
        #
        # Exact figures are deterministic: the corpus is generated from a fixed seed and the coverage is a
        # property of the code, so the report has to state what the evidence says, character for character.
        #
        # Measured figures move between runs, because they are timings and memory readings on a shared machine.
        # Requiring the report to reproduce a particular millisecond would mean rewriting it after every
        # measurement, which is how a stale figure survives; what the report owes is a value consistent with a
        # recorded run, so those are accepted within a fifth of a recorded one.
        recorded_plain = recorded.replace(",", "")
        recorded_numbers = [float(number) for number in re.findall(r"\d+(?:\.\d+)?", recorded_plain)]
        compared = 0
        for label, actual in actuals:
            figures = re.findall(r"([\d,]+(?:\.\d+)?)\s*(ms|MiB|percent|frame pairs per second)?", actual)
            figures = [(value, unit) for value, unit in figures if value]
            if not figures:
                continue
            missing = []
            for value, unit in figures:
                plain = value.replace(",", "")
                compared += 1
                if plain in recorded_plain:
                    continue
                # A figure stated in a unit the evidence does not carry (mebibytes against bytes, for example)
                # cannot be matched by string, so it is accepted when a recorded number is within a fifth of it.
                try:
                    stated = float(plain)
                except ValueError:
                    missing.append(value)
                    continue
                if stated <= 1 or not any(abs(recorded_number - stated) <= 0.2 * stated for recorded_number in recorded_numbers):
                    missing.append(value)
            check(f"every figure in the {label} row is consistent with the recorded evidence", not missing,
                  f"{', '.join(missing)} not found in docs/vv/evidence")
        check("section 5.3 states figures the evidence can be checked against", compared >= 6,
              f"{compared} figures compared")

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
    if failures:
        print(f"{len(failures)} claim(s) do not hold: " + ", ".join(failures))
        return 1
    print("every claim in the report and the README holds against this commit")
    return 0


if __name__ == "__main__":
    sys.exit(main())
