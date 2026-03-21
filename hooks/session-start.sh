#!/bin/bash
# Check if the project has .spec files and inject speclang skill awareness.

SPEC_COUNT=$(find "${CLAUDE_PROJECT_ROOT:-.}" -name "*.spec" -not -path "*/testdata/*" -not -path "*/node_modules/*" 2>/dev/null | wc -l | tr -d ' ')

if [ "$SPEC_COUNT" -gt 0 ]; then
  cat <<EOF
{
  "hookSpecificOutput": {
    "additionalContext": "This project uses speclang specifications ($SPEC_COUNT .spec files found). Two skills are available: speclang:author (convert requirements to specs) and speclang:verify (run verification before merging). Use /spec and /verify-spec commands as shortcuts."
  }
}
EOF
fi
