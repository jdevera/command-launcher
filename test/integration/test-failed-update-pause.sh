#!/bin/bash

# Test: When updating an existing package fails, the pause mechanism should work
# This test covers the update failure case (as opposed to new installation failure)

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
source "$SCRIPT_DIR/assert.sh"

# clean up
rm -rf "$CL_HOME/dropins"
rm -rf "$CL_HOME/current"
rm -f "$CL_HOME/config.json"
mkdir -p "$CL_HOME/dropins"

# Create a local test remote repository
TEST_REMOTE_DIR=$CL_HOME/test-remote
mkdir -p "$TEST_REMOTE_DIR"

# Copy a valid package (version 1.0.0) from test assets
cp "$SCRIPT_DIR/../packages-src/bonjour/bonjour-v1.pkg" "$TEST_REMOTE_DIR/bonjour-1.0.0.pkg"

# Calculate checksum for valid package
CHECKSUM_V1=$(shasum -a 256 "$TEST_REMOTE_DIR/bonjour-1.0.0.pkg" | cut -d' ' -f1)

# Create index.json with version 1.0.0 (valid package)
cat > "$TEST_REMOTE_DIR/index.json" << EOF
[
  {
    "name": "bonjour",
    "version": "1.0.0",
    "checksum": "$CHECKSUM_V1",
    "startPartition": 0,
    "endPartition": 9
  }
]
EOF

# Configure the command launcher (updates disabled initially)
"$CL_PATH" config command_repository_base_url "file://$TEST_REMOTE_DIR" > /dev/null 2>&1
assert_status 0 $? "remote repository configuration succeeds"

################
echo "> test install valid package first"

# Enable updates and run to install the package
"$CL_PATH" config command_update_enabled true > /dev/null 2>&1
assert_status 0 $? "automatic package updates are enabled"
echo "* running first command (installing valid package 1.0.0)"
RESULT=$("$CL_PATH" 2>&1)
STATUS=$?
assert_status 0 "$STATUS" "launcher remains usable after initial installation"

# Check that the bonjour command exists (package was installed)
assert_matches "$RESULT" "bonjour.*print bonjour" "bonjour command is available after installation"

# Verify package directory exists
assert_dir_exists "$CL_HOME/current/bonjour" "package directory exists"

################
echo "> test failed update creates pause file"

# Now create a broken version 2.0.0 in the remote
echo "this is not a valid zip file" > "$TEST_REMOTE_DIR/bonjour-2.0.0.pkg"

# Update index.json to have version 2.0.0 (broken package)
cat > "$TEST_REMOTE_DIR/index.json" << EOF
[
  {
    "name": "bonjour",
    "version": "2.0.0",
    "checksum": "0000000000000000000000000000000000000000000000000000000000000000",
    "startPartition": 0,
    "endPartition": 9
  }
]
EOF

# Run command - triggers automatic update which should fail
echo "* running second command (triggers update to broken 2.0.0, expecting failure)"
RESULT=$("$CL_PATH" 2>&1)
STATUS=$?
assert_status 0 "$STATUS" "launcher remains usable after a background update failure"

# Check that update was attempted
assert_contains "$RESULT" "upgrade package 'bonjour' from version 1.0.0 to version 2.0.0" "update was attempted"

# Check that update failed
assert_matches "$RESULT" "Cannot .* the package bonjour" "update failed as expected"

# Check if pause succeeded
assert_not_contains "$RESULT" "Failed to pause update for package" "pause operation did not fail"
assert_contains "$RESULT" "has been paused due to installation failure" "package was paused after update failure"

# Run again - should skip the paused package (not retry update)
echo "* running third command (should skip paused package)"
RESULT=$("$CL_PATH" 2>&1)
STATUS=$?
assert_status 0 "$STATUS" "launcher remains usable while the package is paused"

# If pause works correctly, it should not try to update again
assert_not_contains "$RESULT" "upgrade package 'bonjour'" "paused package was skipped"

# Verify the original package is still working (version 1.0.0 should still be installed)
echo "* verifying original package still works"
RESULT=$("$CL_PATH" bonjour 2>&1)
STATUS=$?
assert_status 0 "$STATUS" "original package command exits successfully"
assert_contains "$RESULT" "bonjour!" "original version-specific command output is preserved"
