#!/usr/bin/env bash
# Run Go tests with coverage and print final outcome (pass/fail + total coverage).
# Usage: ./scripts/coverage.sh [path-to-module] [-html]
#   path-to-module: optional; default is current directory.
#   -html: open HTML coverage report after running (macOS: open).
# Env: COVERAGE_VERBOSE=1 to pass -v to go test; COVER_PROFILE to override coverage file path.

SHOW_HTML=""
MODULE_PATH=""
for arg in "$@"; do
  case "$arg" in
    -html) SHOW_HTML=1 ;;
    *)     [ -z "$MODULE_PATH" ] && MODULE_PATH="$arg" ;;
  esac
done

if [ -n "$MODULE_PATH" ]; then
  if [ ! -d "$MODULE_PATH" ]; then
    echo "Error: not a directory: $MODULE_PATH" >&2
    exit 1
  fi
  cd "$MODULE_PATH"
fi

if [ ! -f go.mod ]; then
  echo "Error: go.mod not found (run from Go module root or pass path)" >&2
  exit 1
fi

COVER_PROFILE="${COVER_PROFILE:-coverage.out}"
GO_TEST_ARGS=("-coverprofile=$COVER_PROFILE" "./...")
[ -n "${COVERAGE_VERBOSE}" ] && GO_TEST_ARGS=("-v" "${GO_TEST_ARGS[@]}")

echo "Running tests..."
go test "${GO_TEST_ARGS[@]}"
TEST_EXIT=$?
if [ "$TEST_EXIT" -eq 0 ]; then
  TEST_RESULT="PASS"
else
  TEST_RESULT="FAIL"
fi

COVERAGE_LINE=""
if [ -f "$COVER_PROFILE" ] && [ -s "$COVER_PROFILE" ]; then
  COVER_OUT=$(go tool cover -func="$COVER_PROFILE" 2>/dev/null) || true
  if [ -n "$COVER_OUT" ]; then
    COVERAGE_LINE=$(echo "$COVER_OUT" | grep '^total:' | awk '{print $3}')
    [ -n "${COVERAGE_VERBOSE}" ] && echo "$COVER_OUT"
  fi
fi

echo "Tests: $TEST_RESULT"
if [ -n "$COVERAGE_LINE" ]; then
  echo "Code coverage: $COVERAGE_LINE"
else
  echo "No coverage data"
fi

if [ -n "$SHOW_HTML" ] && [ -f "$COVER_PROFILE" ] && [ -s "$COVER_PROFILE" ]; then
  HTML_FILE="${COVER_PROFILE%.out}.html"
  go tool cover -html="$COVER_PROFILE" -o "$HTML_FILE" 2>/dev/null || true
  if [ -f "$HTML_FILE" ]; then
    if command -v open >/dev/null 2>&1; then
      open "$HTML_FILE"
    elif command -v xdg-open >/dev/null 2>&1; then
      xdg-open "$HTML_FILE"
    else
      echo "HTML report: $HTML_FILE"
    fi
  fi
fi

exit "$TEST_EXIT"
