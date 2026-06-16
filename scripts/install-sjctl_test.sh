#!/usr/bin/env bash
# Regression tests for install-sjctl.sh's tag parsing.
#
# Guards against the bug where the bash installer extracted the wrong field on
# Linux while the PowerShell installer worked: GitHub serves the release JSON
# minified (one line), so the old `grep | cut -f4` sliced the "url" value into
# the download URL and every download 404'd.
set -euo pipefail

# Source the installer for its functions only (it returns early when sourced).
SJCTL_SKIP_COSIGN=1 source "$(dirname "$0")/install-sjctl.sh"

fail=0
check() {
  local name="$1" expected="$2" got="$3"
  if [ "$got" = "$expected" ]; then
    echo "ok   - $name"
  else
    echo "FAIL - $name: expected '$expected', got '$got'" >&2
    fail=1
  fi
}

# Minified, as GitHub actually returns it — note "url" precedes "tag_name".
minified='{"url":"https://api.github.com/repos/o/r/releases/338660781","tag_name":"v0.1.0","name":"v0.1.0"}'
check "minified single-line JSON" "v0.1.0" "$(printf '%s' "$minified" | extract_tag_name)"

# Pretty-printed, one key per line — must keep working too.
pretty='{
  "url": "https://api.github.com/repos/o/r/releases/338660781",
  "tag_name": "v2.3.4",
  "name": "v2.3.4"
}'
check "pretty-printed JSON" "v2.3.4" "$(printf '%s' "$pretty" | extract_tag_name)"

exit "$fail"
