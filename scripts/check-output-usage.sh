#!/bin/bash
# Check for fmt.Print* usage in cmd/ files that should use output package
# This should be run as part of CI or pre-commit

set -e

ERRORS=0

# Patterns that indicate output.* should be used instead of fmt.Print*
PATTERNS=(
    'fmt\.Print.*Successfully'
    'fmt\.Print.*Warning:'
    'fmt\.Print.*Error:'
    'fmt\.Print.*✓'
    'fmt\.Print.*✗'
    'fmt\.Print.*⚠'
)

echo "Checking for fmt.Print usage that should use output package..."

for pattern in "${PATTERNS[@]}"; do
    matches=$(grep -rn "$pattern" cmd/ --include="*.go" 2>/dev/null | grep -v "_test.go" || true)
    if [ -n "$matches" ]; then
        echo ""
        echo "Found fmt.Print* that should use output.* (pattern: $pattern):"
        echo "$matches"
        ERRORS=$((ERRORS + 1))
    fi
done

# Check for progress messages (ending with ...)
progress_matches=$(grep -rn 'fmt\.Printf.*\.\.\.' cmd/ --include="*.go" 2>/dev/null | grep -v "_test.go" | grep -v "fmt.Errorf" || true)
if [ -n "$progress_matches" ]; then
    echo ""
    echo "Found progress messages that should use output.Infof:"
    echo "$progress_matches" | head -20
    echo "... and more"
    ERRORS=$((ERRORS + 1))
fi

if [ $ERRORS -gt 0 ]; then
    echo ""
    echo "=== LINT FAILED ==="
    echo "Please use the output package instead of fmt.Print* for user-facing messages:"
    echo "  - Success messages: output.Successf(...)"
    echo "  - Error messages: output.Errorf(...)"
    echo "  - Warning messages: output.Warningf(...)"
    echo "  - Progress messages: output.Infof(...)"
    echo "  - General output: output.Printf(...)"
    exit 1
fi

echo "✓ No fmt.Print* violations found"
