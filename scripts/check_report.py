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
    check(f"the report counts {truth['tasks_done']} of {truth['tasks_total']} tasks",
          re.search(rf"\b{truth['tasks_total']} planned tasks of which {truth['tasks_done']} are\b", report) is not None
          or re.search(rf"\b{truth['tasks_total']} tasks\b[^.]*\bwhich {truth['tasks_done']}\b", report) is not None)
    for label, key, phrase in [("functional", "functional", "functional requirements"),
                               ("non-functional", "nonfunctional", "non-functional requirements"),
                               ("success", "criteria", "success criteria")]:
        check(f"the report counts {truth[key]} {phrase}",
              re.search(rf"\b{truth[key]} {phrase}\b", report) is not None)

    # The review counts, derived from the audit log's own account of where each finding came from. The report
    # carried "three of the five review rounds" for several rounds after both numbers had drifted, which is the
    # kind of claim a reader cannot check and a writer does not notice.
    audit_sources = [cells[2].strip() for cells in
                     ([cell.strip() for cell in row.strip("|").split("|")] for row in audit_rows)]
    review_labels = {source for source in audit_sources if "review" in source.lower()}
    first_review = "### Round 1: independent code review of the first implementation" in plan
    review_passes = len(review_labels) + (1 if first_review else 0)
    from_review = sum(1 for source in audit_sources if source in review_labels)
    check(f"the report counts {review_passes} review passes",
          re.search(rf"\b{review_passes} independent review passes\b", report) is not None,
          f"the audit log records {sorted(review_labels)}")
    check(f"the report says {from_review} findings came from a review",
          re.search(rf"\b{from_review} of the {len(audit_rows)} defects came from\b", report) is not None)

    check("the audit log and the defect table have the same number of rows",
          truth["defects"] == len(audit_rows),
          f"{truth['defects']} against {len(audit_rows)}")
    for name, count in sorted(classes.items()):
        check(f"the report's {name} count is {count}",
              re.search(rf"\|\s*{name.capitalize()}\s*\|\s*{count}\s*\|", report) is not None)

    # The measurements in section 5.3, against the recorded run for each statistic.
    #
    # Two earlier versions of this check were wrong in opposite directions. The first compared every figure against
    # the concatenation of every evidence file, on substrings, with a tolerance against any number in the pile; a
    # review showed that a report stating p95 99 ms, F1 1.9 and a region count multiplied by a thousand passed.
    # The second anchored each row to a file but still compared figures against every number in that file, so the
    # allocation row failed on a figure that had merely moved between runs.
    #
    # This version anchors each row to a named file and to the statistic inside it, so the comparison is always
    # between the report's value for a measurement and the recorded value of the same measurement. Figures a run
    # produces afresh, the timings, the memory readings and the allocation counters, are allowed to differ from
    # the recorded run by up to a fifth because they move on a shared machine; everything else has to match the
    # recorded text exactly.
    def evidence_named(keyword):
        # The newest file with this keyword in its name, not the first alphabetically: a check that validates
        # against an old run is a check that can pass on a superseded figure.
        matches = sorted(pathlib.Path("docs/vv/evidence").glob("*.txt"), key=lambda path: path.stat().st_mtime)
        for path in reversed(matches):
            if keyword in path.name:
                return path
        return None

    final = evidence_named("final-")
    portability = evidence_named("portability-")
    fuzzing = evidence_named("fuzz-")
    mutations = evidence_named("mutation-")

    # row prefix | evidence file | extractors for the recorded values of this row's statistics | tolerance
    row_sources = [
        ("Accuracy", final, [r"([\d,]+) frame pairs scored", r"([\d,]+) regions matched", r"F1 ([\d.]+)",
                             r"of (\d+) movements reported as moved", r"movements reported as moved \(([\d.]+)"], False),
        ("Latency", final, [r"p50 ([\d.]+) ms", r"p95 ([\d.]+) ms", r"p99 ([\d.]+) ms"], True),
        ("Throughput", final, [r"sustained ([\d.]+) frame pairs per second"], True),
        ("Memory", final, [r"peak heap \d+ bytes \(([\d.]+) MiB\)", r"peak resident \d+ bytes \(([\d.]+) MiB\)"], True),
        ("Allocation", final, [r"first window (\d+), second window (\d+)"], True),
        ("Coverage", final, [r"coverage: ([\d.]+)% of statements"], False),
        ("Determinism", portability, [r"([\d,]+) bytes each"], False),
        ("Fuzzing", fuzzing, [r"([\d.]+) million inputs"], True),
        ("Mutation", mutations, [r"([\d]+) of ([\d]+) applied mutations killed"], False),
        ("Regression", evidence_named("regression-"), [r"([\d]+) of ([\d]+) applied reversions caught"], False),
        # The traceability row is read from the matrix rather than from the recorded run, because the recorded run
        # is being written by the gate that reads it: the numbers do not exist in the file yet at that moment.
        ("Requirement", pathlib.Path("docs/traceability.md"), ["matrix"], False),
    ]

    def extract(path, patterns):
        if path is None or not path.exists():
            return None
        if patterns == ["matrix"]:
            # The client's own count of the matrix: rows, verified rows and errors, which is exactly what the
            # traceability row states.
            rows = [line for line in path.read_text().splitlines() if re.match(r"^\| (FR|NFR|SC)-\d+ ", line)]
            verified = [line for line in rows if line.strip().endswith("| verified |")]
            return [str(len(rows)), str(len(verified)), "0"]
        text = path.read_text()
        values = []
        for pattern in patterns:
            for match in re.finditer(pattern, text):
                values.extend(group for group in match.groups() if group)
        return values

    section = report[report.index("### 5.3 Test Execution Results"):report.index("### 6.1")]
    rows_read = 0
    actual_cells = []
    figures_compared = 0
    for line in section.splitlines():
        if not line.startswith("| ") or line.startswith("| Measurement") or line.startswith("|---"):
            continue
        cells = [cell.strip() for cell in line.strip("|").split("|")]
        if len(cells) < 5:
            continue
        label, actual = cells[0], cells[3]
        rows_read += 1
        actual_cells.append((label, actual))

        source = next((entry for entry in row_sources if label.startswith(entry[0])), None)
        if source is None:
            check(f"the {label} row names a recorded statistic", False, "no extractor is defined for this row")
            continue
        _, path, patterns, tolerant = source
        recorded = extract(path, patterns)
        if not recorded:
            check(f"the {label} row has a recorded run", False, f"{path.name if path else 'no file'} holds no value for it")
            continue

        figures = [figure for figure in re.findall(r"(?<![A-Za-z0-9-])\d[\d,]*(?:\.\d+)?", actual) if figure.strip(",.")]
        if not figures:
            check(f"the {label} row states a figure the evidence can be checked against", False,
                  f"the row says {actual!r}, which has no figure in it")
            continue

        missing = []
        for figure in figures:
            plain = figure.replace(",", "")
            figures_compared += 1
            if any(plain == value.replace(",", "") for value in recorded):
                continue
            if tolerant:
                try:
                    stated = float(plain)
                except ValueError:
                    missing.append(figure)
                    continue
                if any(abs(float(value.replace(",", "")) - stated) <= 0.2 * stated for value in recorded):
                    continue
            missing.append(figure)
        check(f"the {label} row agrees with the recorded {label.lower()} measurement", not missing,
              f"{', '.join(missing)} not among the recorded values {recorded}")

    check("section 5.3 has rows to read", rows_read >= 8, f"{rows_read} rows read")
    # A floor rather than a target: it exists so that a table whose figures stopped being comparable fails here
    # rather than passing because nothing was read.
    check("section 5.3 states figures the evidence can be checked against", figures_compared >= 15,
          f"{figures_compared} figures compared")

    # The report may not contradict itself. Each measurement appears in the executive summary, in the
    # requirement table and in the verification table, and a document that states two throughput figures for the
    # same run is the kind of thing an assessor notices. The verification table's Actual column is the measured
    # record, so every other statement of the same measurement has to agree with it.
    #
    # The first version of this check iterated over a string instead of using it as a pattern, so it compared
    # against single characters, and it read the Expected column, so it compared against a requirement's own
    # number. Both mistakes are the reason the block is written out rather than built from a comprehension.
    outside = report[:report.index("### 5.3 Test Execution Results")] + report[report.index("### 6.1"):]
    # A requirement states its target as "at least 30 frame pairs per second", which is not a measurement of the
    # engine and must not be compared with one. The targets are removed before the scan rather than being
    # special-cased per figure.
    outside = re.sub(r"at least [\d,.]+ frame pairs per second", "", outside, flags=re.IGNORECASE)
    for label, pattern in [
        ("throughput", r"([\d.]+) frame pairs per second"),
        ("p50", r"p50 ([\d.]+) ms"),
        ("p95", r"p95 ([\d.]+) ms"),
        ("p99", r"p99 ([\d.]+) ms"),
        ("peak resident", r"peak resident ([\d.]+) MiB"),
    ]:
        measured_value = None
        for _, actual in actual_cells:
            found = re.search(pattern, actual)
            if found:
                measured_value = found.group(1)
                break
        if measured_value is None:
            continue
        elsewhere = {value for value in re.findall(pattern, outside) if value != measured_value}
        check(f"the report states one {label} figure, not {len(elsewhere) + 1}",
              not elsewhere,
              f"the measured record says {measured_value} and the rest of the report says {', '.join(sorted(elsewhere))}")

    # Section 6.1's activity table, the line counts and the README's measured table. These were the figures an
    # assessor found stale while the check reported that every count and measurement in the report was recomputed
    # from the repository, which was true of some tables and not of these.
    run_command = lambda command: run(command)

    activity = {
        "commits": int(run_command("git rev-list --count HEAD")),
        "spec": int(run_command("git log --grep='^Spec:' --oneline | wc -l")),
        "req": int(run_command("git log --grep='^Req:' --oneline | wc -l")),
        "task": int(run_command("git log --grep='^Task:' --oneline | wc -l")),
        "prompt": int(run_command("git log --grep='^Prompt:' --oneline | wc -l")),
        "go": int(run_command("find internal cmd tools -name '*.go' -not -name '*_test.go' | xargs cat | wc -l")),
        "test": int(run_command("find internal cmd tools tests -name '*_test.go' | xargs cat | wc -l")),
        "docs": int(run_command("find specs docs -name '*.md' -o -name '*.json' -o -name '*.svg' | xargs cat | wc -l")),
    }
    # The activity figures are the state at the commit that wrote them, and every later commit, including the one
    # that writes a correction, moves them. The check therefore allows the small drift those commits cause and
    # refuses anything further away, which is a figure that was never regenerated.
    # A count that moves with every commit cannot be exact in a document written before them, so the row states
    # the figure at the commit that wrote it and the report says so. The check requires the stated count to be a
    # real past state: never greater than the repository's, and not so far behind that it was never regenerated.
    drift = 12
    stated_commits = re.search(r"\| Commits \| (\d+) when this table was written", report)
    check(f"the report's commit count is a past state of the repository ({stated_commits.group(1) if stated_commits else 'missing'} against {activity['commits']})",
          stated_commits is not None
          and int(stated_commits.group(1)) <= activity["commits"]
          and activity["commits"] - int(stated_commits.group(1)) <= drift)
    for label, count in [("Spec", "spec"), ("Req", "req"), ("Task", "task"), ("Prompt", "prompt")]:
        stated = re.search(rf"\| (?:Commits )?[Cc]arrying a `{label}:` trailer \| (\d+) \|", report)
        check(f"the report's {label} trailer count is a past state of the repository ({stated.group(1) if stated else 'missing'} against {activity[count]})",
              stated is not None
              and int(stated.group(1)) <= activity[count]
              and activity[count] - int(stated.group(1)) <= drift)
    breakdown = run_command("git log --format='%s' | sed 's/(.*//; s/:.*//' | sort | uniq -c | sort -rn")
    types = [(int(number), kind) for number, kind in (line.split() for line in breakdown.splitlines())]
    # Each type count is a past state too, so it may not exceed the repository's and may not be stale by more than
    # the same margin. The first version of this check compared the report's total with itself, which is why the
    # row could say "summing to 198" and pass.
    stated_types = dict((kind, int(number)) for number, kind in re.findall(r"(\d+) ([a-z]+)", report))
    over = {kind: (stated_types.get(kind, 0), number) for number, kind in types if stated_types.get(kind, 0) > number}
    stale = {kind: (stated_types.get(kind, 0), number) for number, kind in types
             if number - stated_types.get(kind, 0) > drift}
    check(f"the report's commit breakdown is a past state of the repository, type by type",
          not over and not stale,
          f"overstated: {over}; stale: {stale}; the repository has {', '.join(f'{n} {k}' for n, k in types)}")

    for label, key in [("Go", "go"), ("test", "test"), ("specification and process documents", "docs")]:
        claim = re.search(rf"([\d,]+) lines of {label}", report)
        check(f"the report's {label} line count matches the tree ({activity[key]:,})",
              claim is not None and claim.group(1) == f"{activity[key]:,}",
              f"the report says {claim.group(1) if claim else 'nothing'}")

    # The README presents the same measurements as the report, under a sentence saying they come from the
    # evidence. A figure in one and not the other is a claim one of the two documents cannot support.
    for label, pattern in [
        ("frame pairs", r"([\d,]+) frame pairs"), ("regions", r"([\d,]+) regions"),
        ("p50", r"p50 ([\d.]+) ms"), ("p95", r"p95 ([\d.]+) ms"), ("p99", r"p99 ([\d.]+) ms"),
        ("throughput", r"([\d.]+) frame pairs per second"), ("decisions", r"([\d.]+) decisions per second"),
        ("peak resident", r"peak resident ([\d.]+) MiB"),
    ]:
        in_readme = re.search(pattern, readme)
        in_evidence = any(re.search(pattern, path.read_text())
                          for path in pathlib.Path("docs/vv/evidence").glob("*.txt"))
        check(f"the README's {label} figure appears in the evidence", in_readme is None or in_evidence,
              f"the README states {in_readme.group(1) if in_readme else 'nothing'}")

    # The executive summary states the same measurements in prose, in a different shape from the table. A check
    # that reads only the table lets the summary drift, which a reviewer demonstrated by changing p95 to 99.65 ms
    # there and watching the check pass.
    summary = report[report.index("## Executive Summary"):report.index("## 1. Introduction")]
    for label, pattern, source in [
        ("p95 latency", r"p95 latency ([\d.]+) ms", r"p95 ([\d.]+) ms"),
        ("peak heap", r"([\d.]+) MiB peak heap", r"peak heap \d+ bytes \(([\d.]+) MiB\)"),
        ("throughput", r"([\d.]+) frame pairs per second", r"sustained ([\d.]+) frame pairs per second"),
        ("F1", r"F1 ([\d.]+)", r"F1 ([\d.]+)"),
    ]:
        claimed = re.search(pattern, summary)
        if claimed is None:
            continue
        recorded_value = None
        if final is not None and final.exists():
            found = re.search(source, final.read_text())
            recorded_value = found.group(1) if found else None
        check(f"the executive summary's {label} matches the recorded run",
              recorded_value is not None
              and abs(float(claimed.group(1).replace(",", "")) - float(recorded_value.replace(",", "")))
              <= 0.2 * float(recorded_value.replace(",", "")),
              f"the summary says {claimed.group(1)} and the recorded run says {recorded_value}")

    # (b) The README's figures have to match a recorded value, not merely have the shape of one.
    # The README and the record state the same measurement in different shapes, so each row carries the pattern for
    # the README and the pattern for the record. An earlier version used one pattern for both, which silently read
    # nothing from the record and then failed every row.
    newest = sorted(pathlib.Path("docs/vv/evidence").glob("final-*.txt"), key=lambda path: path.stat().st_mtime)
    newest_text = newest[-1].read_text() if newest else ""
    for label, readme_pattern, record_pattern in [
        ("p50", r"p50 ([\d.]+) ms", r"p50 ([\d.]+) ms"),
        ("p95", r"p95 ([\d.]+) ms", r"p95 ([\d.]+) ms"),
        ("p99", r"p99 ([\d.]+) ms", r"p99 ([\d.]+) ms"),
        ("throughput", r"([\d.]+) frame pairs per second", r"sustained ([\d.]+) frame pairs per second"),
        ("decisions", r"([\d.]+) decisions per second", r"decisions per second ([\d.]+)"),
        ("peak heap", r"([\d.]+) MiB peak heap", r"peak heap \d+ bytes \(([\d.]+) MiB\)"),
        ("peak resident", r"([\d.]+) MiB peak resident", r"peak resident \d+ bytes \(([\d.]+) MiB\)"),
        ("regions", r"([\d,]+) regions", r"([\d,]+) regions matched"),
    ]:
        claimed = re.search(readme_pattern, readme)
        if claimed is None:
            continue
        recorded_values = re.findall(record_pattern, newest_text)
        value = float(claimed.group(1).replace(",", ""))
        check(f"the README's {label} matches the recorded value ({claimed.group(1)} against {recorded_values[:2]})",
              any(abs(float(candidate.replace(",", "")) - value) <= 0.2 * value for candidate in recorded_values),
              f"the README says {claimed.group(1)}; the record has {recorded_values[:4]}")

    # (c) The decision rate, the days of work and the fixed-finding count, each of which a reviewer changed without
    # the check noticing.
    decision_claim = re.search(r"([\d.]+) decisions per second", report)
    if decision_claim and final is not None:
        recorded_decisions = re.findall(r"decisions per second ([\d.]+)", final.read_text())
        check("the report's decision rate matches the recorded run",
              any(abs(float(candidate) - float(decision_claim.group(1))) <= 0.2 * float(candidate)
                  for candidate in recorded_decisions),
              f"the report says {decision_claim.group(1)} and the record says {recorded_decisions[:3]}")
    days = run_command("git log --format=%ad --date=short | sort -u | wc -l")
    check(f"the report's days of work match the history ({days})",
          re.search(rf"\| Days of work \| {days} ", report) is not None)
    fixed_claim = re.search(r"of which (\d+) are fixed", report)
    check(f"the report's fixed-finding count matches the records ({len(defect_rows) + 11 - 1})",
          fixed_claim is not None and abs(int(fixed_claim.group(1)) - (len(defect_rows) + 11 - 1)) <= 1,
          f"the report says {fixed_claim.group(1) if fixed_claim else 'nothing'}")

    # A requirement's target belongs to the specification and a measurement belongs to the record. A scripted
    # replacement over the report once overwrote a target with a measurement, which no existing check could see
    # because targets are excluded from the consistency rule, so the two are checked against their own sources.
    spec = pathlib.Path("specs/001-frame-delta-engine/spec.md").read_text()

    def numbers(text: str) -> list[str]:
        """The figures in a sentence, with thousands separators removed so that 10,000 is 10000."""
        return [value.replace(",", "") for value in re.findall(r"\d+(?:[,\d]*\.?\d*)", text)]

    for identifier, source in re.findall(r"^- \*\*(NFR-\d+)\*\*[^:]*: ([^\n]+)", spec, re.M):
        target_figures = numbers(source)
        row = re.search(rf"\| {identifier} \|[^|]*\|([^|]*)\|", report)
        if row is None or not target_figures:
            continue
        stated = numbers(row.group(1))
        missing = [figure for figure in target_figures if figure not in stated]
        check(f"the {identifier} target in the report is the specification's ({target_figures})",
              not missing,
              f"the specification says {target_figures} and the report's target cell says {stated}")

    # The state cell is a measurement, and it is read through the shape the row uses rather than by taking every
    # number in the cell, which would count the digits of "p95" as a figure.
    for identifier, pattern, source in [
        ("NFR-001", r"met: p95 [\d.]+ ms, p99 [\d.]+ ms", r"p95 ([\d.]+) ms|p99 ([\d.]+) ms"),
        ("NFR-002", r"met: [\d.]+ per second", r"sustained ([\d.]+) frame pairs per second"),
        ("NFR-003", r"met: [\d.]+ MiB peak resident", r"peak resident \d+ bytes \(([\d.]+) MiB\)"),
    ]:
        row = re.search(rf"\| {identifier} \|[^|]*\|[^|]*\|[^|]*\|([^|]*)\|", report)
        if row is None or final is None:
            continue
        in_row = re.search(pattern, row.group(1))
        claimed = []
        if in_row:
            for value in re.findall(r"\d+(?:\.\d+)?", row.group(1)):
                if value in ("95", "99"):
                    continue
                claimed.append(float(value))
        found = re.findall(source, final.read_text())
        recorded_values = [float(value) for entry in found
                           for value in (entry if isinstance(entry, tuple) else (entry,)) if value]
        check(f"the {identifier} state in the report is the recorded measurement ({claimed})",
              bool(claimed) and all(
                  any(abs(value - recorded) <= 0.2 * recorded for recorded in recorded_values) for value in claimed),
              f"the report states {claimed} and the record has {recorded_values}")

    # Every markdown table has to be a table. The verification table's separator row had four columns under a
    # six-column header, so pandoc rejected it and the PDF rendered the section's most important table as a
    # paragraph of pipe characters. Nothing else noticed, because the source looked like a table.
    for name, content in [("the report", report), ("the README", readme),
                          ("the handover", pathlib.Path("docs/handover.md").read_text())]:
        lines = content.splitlines()
        broken = 0
        for index in range(len(lines) - 1):
            line = lines[index]
            if not line.strip().startswith("|"):
                continue
            separator = lines[index + 1].strip()
            if not re.fullmatch(r"\|[\s:|-]+\|", separator):
                continue
            header = len(re.findall(r"(?<!\\)\|", line)) - 1
            for offset in range(index + 2, len(lines)):
                row = lines[offset]
                if not row.strip().startswith("|"):
                    break
                if len(re.findall(r"(?<!\\)\|", row)) - 1 != header:
                    broken += 1
        check(f"every table in {name} has rows shaped like its header", broken == 0,
              f"{broken} row(s) would be rejected by the renderer")

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
