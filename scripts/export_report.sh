#!/usr/bin/env bash
# Export the report to PDF, which is the form the course submissions are made in.
#
# The styling lives in docs/report/report.css so the exported document is reproducible from the source rather
# than from a manual export step. Requires pandoc and a CSS-to-PDF renderer.
set -euo pipefail

cd "$(dirname "$0")/.."

title="An Experience Report on a Specification-Driven, AI-Assisted Software Construction Project"

if ! command -v pandoc >/dev/null; then
  echo "export_report: pandoc is required" >&2
  exit 1
fi
renderer=""
for candidate in weasyprint wkhtmltopdf; do
  if command -v "$candidate" >/dev/null; then renderer="$candidate"; break; fi
done
if [ -z "$renderer" ]; then
  echo "export_report: weasyprint or wkhtmltopdf is required" >&2
  exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
cp docs/report/report.css "$work/report.css"

# pagetitle rather than title: it sets the document title without rendering a title block, because the report
# already begins with its own heading and the PDF would otherwise print the title twice.
pandoc docs/report/report.md -f gfm -t html5 --standalone \
  --metadata pagetitle="$title" -c report.css -o "$work/report.html"

if [ "$renderer" = "weasyprint" ]; then
  weasyprint "$work/report.html" docs/report/report.pdf 2>/dev/null
else
  wkhtmltopdf --enable-local-file-access "$work/report.html" docs/report/report.pdf >/dev/null 2>&1
fi

echo "export_report: wrote docs/report/report.pdf"
