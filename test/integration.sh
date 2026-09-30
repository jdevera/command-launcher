#!/bin/bash

SCRIPT_DIR=${SCRIPT_DIR:-$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )}
# replace \ to / for windows
SCRIPT_DIR=${SCRIPT_DIR//\\//}
echo "integration test directory: $SCRIPT_DIR"

KEEP_OUTPUT=${KEEP_OUTPUT:-"no"}

EXIT_CODE=0
TEST_COUNT=0
FAILURE_COUNT=0

# create output folder
OUTPUT_DIR=$SCRIPT_DIR/output
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

##
# build the binary
##
cd "$SCRIPT_DIR" || exit 1
if ! go build -o "$OUTPUT_DIR/cl" -ldflags='-X main.version=integration-test -X main.buildNum=1 -X main.appName=cl -X "main.appLongName=Command Launcher"' "$SCRIPT_DIR/../main.go"; then
  echo "failed to build integration test binary" >&2
  exit 1
fi

# specify the app home
export CL_HOME=$OUTPUT_DIR/home

# unlock the file vault without depending on ~/.ssh existing (CI runners don't have one)
export CL_VAULT_SECRET=very_secret

# Use the checked-in registry for deterministic integration tests. The separate
# remote HTTPS smoke test exercises the tagged public fixture and system trust
# store. Callers can still override this for an explicit integration run.
if [ -z "${TEST_REMOTE_BASE_URL:-}" ]; then
  REMOTE_FIXTURE_DIR="$SCRIPT_DIR/../examples/remote-repo"
  if command -v cygpath >/dev/null 2>&1; then
    REMOTE_FIXTURE_DIR=$(cygpath -m "$REMOTE_FIXTURE_DIR")
  fi
  TEST_REMOTE_BASE_URL="file://$REMOTE_FIXTURE_DIR"
fi
export TEST_REMOTE_BASE_URL

if [ $# -ne 0 ]; then
  # in case pass test as arguments, run test from the arguments
  for test in "$@"; do
    echo "------------------------------------------------------------"
    echo "- test/integration/${test}.sh"
    echo "------------------------------------------------------------"

    # clean up for fresh start
    rm -rf "$CL_HOME/dropins"
    rm -rf "$CL_HOME/current"
    rm -f "$CL_HOME/config.json"
    mkdir -p "$CL_HOME/dropins"

    TEST_COUNT=$((TEST_COUNT + 1))

    if env \
      OUTPUT_DIR="$OUTPUT_DIR" \
      CL_PATH="$OUTPUT_DIR/cl" \
      CL_HOME="$CL_HOME" \
      "$SCRIPT_DIR/integration/${test}.sh"; then
      echo "- PASS"
    else
      echo "- FAIL"
      EXIT_CODE=1
      FAILURE_COUNT=$((FAILURE_COUNT + 1))
    fi

    echo ""
  done
else
  # otherwise run all tests in integration/ folder
  TESTS=("$SCRIPT_DIR"/integration/test-*.sh)
  echo "find all tests in integration folder:"
  printf '%s\n' "${TESTS[@]}"
  for f in "${TESTS[@]}"; do
    echo "------------------------------------------------------------"
    echo "- $f"
    echo "------------------------------------------------------------"

    # cleanup
    rm -rf "$CL_HOME/dropins"
    rm -rf "$CL_HOME/current"
    rm -f "$CL_HOME/config.json"
    mkdir -p "$CL_HOME/dropins"

    TEST_COUNT=$((TEST_COUNT + 1))

    if env \
      OUTPUT_DIR="$OUTPUT_DIR" \
      CL_PATH="$OUTPUT_DIR/cl" \
      CL_HOME="$CL_HOME" \
      "$f"; then
      echo "- PASS"
    else
      echo "- FAIL"
      EXIT_CODE=1
      FAILURE_COUNT=$((FAILURE_COUNT + 1))
    fi

    echo ""
  done
fi

##
# remove the output folder
##
if [ "$KEEP_OUTPUT" == "yes" ]; then
  echo "KEEP_OUTPUT = yes, skip clean up"
else
  echo "clean up"
  rm -rf "$OUTPUT_DIR"
fi

echo ""
echo "Total test suits $TEST_COUNT, failure $FAILURE_COUNT"
exit $EXIT_CODE
