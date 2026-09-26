# 0001 Repository scaffold and course context capture

- Date: 2026-09-26
- Author: Theo Wilenius
- Agent and model: dsh, deepseek-flash, with one research subagent on Spec Kit 1.0.12
- Spec Kit command: none directly; `specify init` plus three integration installs
- Artifacts produced: the repository scaffold, `docs/course-context.md`, process documents
- Related requirement IDs: none yet
- Related commit: scaffold commit on `main`

## Prompt (verbatim)

> pls get any context about the software construction project and create a
> scaffold project here in an appropriate dir to start doing it

Followed by:

> use the browser to make sure you have the latest details

## Intent

Start the course project with a repository that already matches the process the
course grades, instead of building the process later. Two kinds of context were
needed first: what the course requires (deadlines, deliverables, report template,
tool kit) and what the tool kit itself produces on this version.

## Expected output

A repository in an appropriate directory, scaffolded with GitHub Spec Kit, plus
the course-specific material that Spec Kit does not create: report skeleton,
prompt log, verification and validation layout, traceability matrix and the
checks that enforce them.

## Actual output

Context came from two sources. The local scrape under
`assignments/software-construction/2026-09-08_22-01-58--Software Construction
2026-27/` supplied the course description, the report template and the lecture
material. The live Moodle course page supplied what the scrape could not: the
current deadlines (2026-10-25 23:59 for both the report and the repository), the
group assignment (Group A), the requirement to host the repository on
gitlab.abo.fi, and the fact that the system to build is not prescribed.

A research subagent established the current Spec Kit surface: version 1.0.12
installed through `uv`, `specify init --integration <key>`, and the file layout
each integration writes.

## What went wrong

1. The first browser attempt failed because the SSH agent on the workstation held
   no identities, so the browser host was unreachable even though the TCP
   connection succeeded. Solved by asking the owner to unlock the key, then
   retrying. The lesson recorded here: check the agent before blaming the host.
2. The scaffold was nearly built from an outdated understanding of Spec Kit. The
   remembered interface (`--ai claude`, `.claude/commands/`, `/speckit.specify`
   as a slash command file) was removed in version 0.10.0 and replaced by
   `--integration` plus per-agent skill directories. Reading the installed source
   and the changelog caught this before anything was written.
3. Installing three integrations produced an advisory finding: `specify
   integration status` reports that the set is not all declared multi-install
   safe, because the Copilot integration is IDE-scoped. Verified that the three
   target directories do not overlap, accepted the finding, and documented it in
   `CONTRIBUTING.md` rather than dropping a surface a teammate may need.

## Correction

The scaffold was built by running the real CLI rather than by hand, so the layout
is the layout the tool produces. The course-specific additions were then written
on top of it: `.specify/templates/overrides/spec-template.md` adds non-functional
requirements with `NFR-###` identifiers, which the stock template does not
produce and the report grades in section 2.3.

## Result

`specify integration status` reports all managed files present and unmodified.
The three command surfaces are installed. The traceability check runs and passes
on an empty specification set, and is designed to become strict as soon as the
first spec exists.
