#!/bin/bash

# Test: When a new package installation fails, the pause mechanism should work
# Bug: When a new package fails to install, the .update file is not created
#      because the package is not yet in the repoIndex.packageDirs map.
#      This causes repeated installation attempts on subsequent updates.

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
source "$SCRIPT_DIR/assert.sh"

# clean up
rm -rf "$CL_HOME/dropins"
rm -rf "$CL_HOME/current"
rm -f "$CL_HOME/config.json"
mkdir -p "$CL_HOME/dropins"

# Create a local test remote repository with a broken package
TEST_REMOTE_DIR=$CL_HOME/test-remote
mkdir -p "$TEST_REMOTE_DIR"

# Create a broken package file (invalid zip - just text content)
echo "this is not a valid zip file" > "$TEST_REMOTE_DIR/broken-package-1.0.0.pkg"

# Create index.json pointing to the broken package
cat > "$TEST_REMOTE_DIR/index.json" << 'EOF'
[
  {
    "name": "broken-package",
    "version": "1.0.0",
    "checksum": "0000000000000000000000000000000000000000000000000000000000000000",
    "startPartition": 0,
    "endPartition": 9
  }
]
EOF

# Configure the command launcher to use our test remote repository
"$CL_PATH" config command_repository_base_url "file://$TEST_REMOTE_DIR"
assert_status 0 $? "remote repository configuration succeeds"
"$CL_PATH" config command_update_enabled true
assert_status 0 $? "automatic package updates are enabled"

################
echo "> test failed installation creates pause file"

# First run - triggers automatic update which should fail to install the broken package
echo "* running first command (triggers automatic update, expecting installation failure)"
RESULT=$("$CL_PATH" 2>&1)
STATUS=$?
assert_status 0 "$STATUS" "launcher remains usable after a background install failure"

# Check that installation was attempted
assert_contains "$RESULT" "install new package 'broken-package'" "installation was attempted"

# Check that installation failed (could be "Cannot get" or "Cannot install")
assert_matches "$RESULT" "Cannot .* the package broken-package" "installation failed as expected"

# Check if pause succeeded (fix works) or failed (bug exists)
# The pause message could be either:
# - "has been paused due to installation failure" (success)
# - "Failed to pause update for package" (pause attempted but failed)
# - Neither (pause not even attempted - bug for early failures)
assert_not_contains "$RESULT" "Failed to pause update for package" "pause operation did not fail"
assert_contains "$RESULT" "has been paused due to installation failure" "package was paused after installation failure"

# Second run - should skip the paused package (not retry installation)
echo "* running second command (should skip paused package)"
RESULT=$("$CL_PATH" 2>&1)
STATUS=$?
assert_status 0 "$STATUS" "launcher remains usable while the package is paused"

# If pause works correctly, it should not try to install again
assert_not_contains "$RESULT" "install new package 'broken-package'" "paused package was skipped"
