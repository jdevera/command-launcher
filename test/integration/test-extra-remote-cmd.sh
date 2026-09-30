#!/bin/bash

# availeble environment varibale
# CL_PATH: the path of the command launcher binary
# CL_HOME: the path of the command launcher home directory
# OUTPUT_DIR: the output folder
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
DEFAULT_REMOTE_URL=$TEST_REMOTE_BASE_URL
EXTRA_REMOTE_DIR=$OUTPUT_DIR/extra-remote-repo
rm -rf "$EXTRA_REMOTE_DIR"
mkdir -p "$EXTRA_REMOTE_DIR"
cp "$SCRIPT_DIR/../remote-repo/command-launcher-demo-2.0.0.pkg" "$EXTRA_REMOTE_DIR"
cp "$SCRIPT_DIR/../packages-src/bonjour/bonjour-v1.pkg" "$EXTRA_REMOTE_DIR/bonjour-1.0.0.pkg"
cp "$SCRIPT_DIR/../remote-repo/extra-index.json" "$EXTRA_REMOTE_DIR/index.json"

NATIVE_EXTRA_REMOTE_DIR=${EXTRA_REMOTE_DIR/\/c\//C:/}
NATIVE_EXTRA_REMOTE_DIR=${NATIVE_EXTRA_REMOTE_DIR/\/d\//D:/}
NATIVE_EXTRA_REMOTE_DIR=${NATIVE_EXTRA_REMOTE_DIR/\/e\//E:/}
NATIVE_EXTRA_REMOTE_DIR=${NATIVE_EXTRA_REMOTE_DIR/\/f\//F:/}
EXTRA_REMOTE_URL=file://${NATIVE_EXTRA_REMOTE_DIR}

# clean up the dropin folder
rm -rf $CL_HOME/dropins
mkdir -p $CL_HOME/dropins

echo "> test download default remote command"
RESULT=$($OUTPUT_DIR/cl config command_repository_base_url "$DEFAULT_REMOTE_URL")
RESULT=$($OUTPUT_DIR/cl)

echo "* should have hello command installed"
echo "$RESULT" | grep -q "hello"
if [ $? -eq 0 ]; then
  # ok
  echo "OK"
else
  echo "KO - hello command should exist"
  exit 1
fi

echo "* should contain default remote registry"
RESULT=$($CL_PATH remote list)
echo "$RESULT"
echo "$RESULT" | grep -F -q "default         : $DEFAULT_REMOTE_URL"
if [ $? -eq 0 ]; then
  # ok
  echo "OK"
else
  echo "KO - should contain default remote registry"
  exit 1
fi


echo "> test add extra remote registry"
RESULT=$($CL_PATH remote add extra1 "$EXTRA_REMOTE_URL")
RESULT=$($CL_PATH remote list)

echo "* should contain default remote registry"
echo "$RESULT" | grep -F -q "default         : $DEFAULT_REMOTE_URL"
if [ $? -eq 0 ]; then
  # ok
  echo "OK"
else
  echo "KO - should contain default remote registry"
  exit 1
fi

echo "* should contain extra remote registry"
echo "$RESULT" | grep -F -q "extra1          : $EXTRA_REMOTE_URL"
if [ $? -eq 0 ]; then
  # ok
  echo "OK"
else
  echo "KO - should contain extra remote registry"
  exit 1
fi

echo "* should contain auto-renamed command: 'hello@@command-launcher-demo@extra1'"
RESULT=$($CL_PATH)
echo "$RESULT" | grep -q "hello@@command-launcher-demo@extra1"
if [ $? -eq 0 ]; then
  # ok
  echo "OK"
else
  echo "KO - should contain auto-renamed command 'hello@@command-launcher-demo@extra1'"
  exit 1
fi

echo "* should be able to run 'hello@@command-launcher-demo@extra1'"
RESULT=$($CL_PATH hello@@command-launcher-demo@extra1)
echo "$RESULT" | grep -q "Hello World v2!"
if [ $? -eq 0 ]; then
  # ok
  echo "OK"
else
  echo "KO - should successfully run command 'hello@@command-launcher-demo@extra1'"
  exit 1
fi

echo "* should be able to run 'hello'"
RESULT=$($CL_PATH hello)
echo "$RESULT" | grep -q "Hello World!"
if [ $? -eq 0 ]; then
  # ok
  echo "OK"
else
  echo "KO - should successfully run command 'hello'"
  exit 1
fi

echo "> test remote add with --sync-policy flag"
# delete extra1 first to test adding with sync-policy
$CL_PATH remote delete extra1

RESULT=$($CL_PATH remote add extra1 "$EXTRA_REMOTE_URL" --sync-policy daily)

echo "* should have sync_policy set to 'daily' in config"
RESULT=$(cat $CL_HOME/config.json | grep -o '"sync_policy": "daily"')
if [ "$RESULT" == '"sync_policy": "daily"' ]; then
  echo "OK"
else
  echo "KO - sync_policy should be 'daily', got: $RESULT"
  exit 1
fi

echo "> test remote set --sync-policy flag"
RESULT=$($CL_PATH remote set extra1 --sync-policy weekly)

echo "* should have sync_policy updated to 'weekly' in config"
RESULT=$(cat $CL_HOME/config.json | grep -o '"sync_policy": "weekly"')
if [ "$RESULT" == '"sync_policy": "weekly"' ]; then
  echo "OK"
else
  echo "KO - sync_policy should be 'weekly', got: $RESULT"
  exit 1
fi

echo "* should print confirmation message"
RESULT=$($CL_PATH remote set extra1 --sync-policy monthly)
echo "$RESULT" | grep -q "Remote 'extra1' sync policy updated to 'monthly'"
if [ $? -eq 0 ]; then
  echo "OK"
else
  echo "KO - should print confirmation message"
  exit 1
fi

echo "> test remote set with invalid sync-policy"
RESULT=$($CL_PATH remote set extra1 --sync-policy invalid 2>&1)
echo "$RESULT" | grep -q "invalid sync policy"
if [ $? -eq 0 ]; then
  echo "OK"
else
  echo "KO - should reject invalid sync policy"
  exit 1
fi

echo "> test remote add with invalid sync-policy"
RESULT=$($CL_PATH remote add extra2 https://example.com --sync-policy invalid 2>&1)
echo "$RESULT" | grep -q "invalid sync policy"
if [ $? -eq 0 ]; then
  echo "OK"
else
  echo "KO - should reject invalid sync policy on add"
  exit 1
fi

# reset sync policy to always for subsequent tests
$CL_PATH remote set extra1 --sync-policy always

echo "> test sync policy"

echo "* should NOT have the sync.timestamp file"
RESULT=$(ls $CL_HOME/extra1 | grep -q "sync.timestamp")
if [ $? -eq 0 ]; then
  echo "KO - should NOT have the sync.timestamp file"
  exit 1
else
  echo "OK"
fi
# change the extra remote's sync policy to 'weekly' using remote set command
$CL_PATH remote set extra1 --sync-policy weekly

# now remove one package from local repository and run command launcher to sync
$CL_PATH config command_update_enabled true
rm -rf $CL_HOME/extra1/command-launcher-demo

echo "* should install new package"
RESULT=$($CL_PATH)
if [ -f "$CL_HOME/extra1/command-launcher-demo/manifest.mf" ]; then
  echo "OK"
else
  echo "$RESULT"
  echo "KO - should install new package"
  exit 1
fi

echo "* should have the sync.timestamp file after sync"
RESULT=$(ls $CL_HOME/extra1 | grep -q "sync.timestamp")
if [ $? -eq 0 ]; then
  echo "OK"
else
  echo "KO - should NOT have the sync.timestamp file"
  exit 1
fi

# now remove the package again, should not install the package again
rm -rf $CL_HOME/extra1/command-launcher-demo
echo "* should NOT install new package"
RESULT=$($CL_PATH)
if [ -e "$CL_HOME/extra1/command-launcher-demo" ]; then
  echo "$RESULT"
  echo "KO - should NOT install new package"
  exit 1
else
  echo "OK"
fi

# reset the config
$CL_PATH config command_update_enabled false

echo "> test delete extra remote registry"
RESULT=$($CL_PATH remote delete extra1)
RESULT=$($CL_PATH remote list)
echo "$RESULT" | grep -F -q "default         : $DEFAULT_REMOTE_URL"
if [ $? -eq 0 ]; then
  # ok
  echo "OK"
else
  echo "KO - should contain default remote registry"
  exit 1
fi

echo "* should NOT contain default remote registry"
echo "$RESULT" | grep -F -q "extra1          : $EXTRA_REMOTE_URL"
if [ $? -eq 0 ]; then
  echo "KO - should NOT contain extra remote registry"
  exit 1
else
  echo "OK"
fi

echo "* should NOT contain auto-renamed extra command"
RESULT=$($CL_PATH)
echo "$RESULT" | grep -q "hello@@command-launcher-demo@extra1"
if [ $? -eq 0 ]; then
  echo "KO - should NOT contain auto-renamed extra command"
  exit 1
else
  echo "OK"
fi
