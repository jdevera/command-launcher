#!/bin/bash

# Small assertion helpers for integration tests. Every failed assertion exits
# the current test script so the top-level harness cannot mask it with a later
# successful command.

fail_test() {
  echo "KO - $1" >&2
  if [ $# -ge 2 ] && [ -n "$2" ]; then
    echo "Output:" >&2
    printf '%s\n' "$2" >&2
  fi
  exit 1
}

pass_test() {
  echo "OK - $1"
}

assert_status() {
  local expected=$1
  local actual=$2
  local message=$3

  if [ "$actual" -ne "$expected" ]; then
    fail_test "$message (expected status $expected, got $actual)"
  fi
  pass_test "$message"
}

assert_contains() {
  local value=$1
  local expected=$2
  local message=$3

  if ! printf '%s\n' "$value" | grep -Fq -- "$expected"; then
    fail_test "$message (missing: $expected)" "$value"
  fi
  pass_test "$message"
}

assert_matches() {
  local value=$1
  local pattern=$2
  local message=$3

  if ! printf '%s\n' "$value" | grep -Eq -- "$pattern"; then
    fail_test "$message (pattern did not match: $pattern)" "$value"
  fi
  pass_test "$message"
}

assert_not_contains() {
  local value=$1
  local unexpected=$2
  local message=$3

  if printf '%s\n' "$value" | grep -Fq -- "$unexpected"; then
    fail_test "$message (unexpected: $unexpected)" "$value"
  fi
  pass_test "$message"
}

assert_dir_exists() {
  local path=$1
  local message=$2

  if [ ! -d "$path" ]; then
    fail_test "$message (missing directory: $path)"
  fi
  pass_test "$message"
}
