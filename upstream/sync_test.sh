#!/usr/bin/env bash
# Offline test for upstream/sync.sh's diff-detection logic and its
# UPSTREAM_LOCK_FILE / UPSTREAM_PINNED_SHA / GITHUB_OUTPUT overrides.
#
# Builds a throwaway local git repo standing in for upstream (no network
# access needed), pins sync.sh at it via a fixture UPSTREAM.lock, and
# asserts:
#   - no diff between pin and ref => "changed=false" + the "no changes"
#     message, and no bucket output.
#   - a diff introduced after the pin => "changed=true" + the file listed
#     under its bucket.
#   - UPSTREAM_PINNED_SHA overrides the lock file's commit=, so a
#     deliberately stale SHA can be used to rehearse the "non-empty diff"
#     path against an already-synced checkout (the epic 14 issue 02
#     acceptance criterion) without waiting for a real upstream change.
#
# Run directly: bash upstream/sync_test.sh
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
sync_sh="$here/sync.sh"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# ---- Build a throwaway "upstream" repo (no network) ----
fake_upstream="$work/fake-upstream"
git init -q "$fake_upstream"
git -C "$fake_upstream" config user.email "test@example.com"
git -C "$fake_upstream" config user.name "test"
mkdir -p "$fake_upstream/packages/ai/src/utils"
echo "v1" >"$fake_upstream/packages/ai/src/utils/retry.ts"
git -C "$fake_upstream" add -A
git -C "$fake_upstream" commit -q -m "pinned revision"
pinned_sha="$(git -C "$fake_upstream" rev-parse HEAD)"

echo "v2" >"$fake_upstream/packages/ai/src/utils/retry.ts"
git -C "$fake_upstream" add -A
git -C "$fake_upstream" commit -q -m "a semantic change"
head_sha="$(git -C "$fake_upstream" rev-parse HEAD)"

write_lock() {
  # write_lock <path> <commit>
  cat >"$1" <<EOF
repo=$fake_upstream
commit=$2
package=packages/ai
version=0.0.0-test
EOF
}

# ---- Case 1: no changes since the pin ----
lock1="$work/UPSTREAM-nochange.lock"
write_lock "$lock1" "$head_sha"

output1="$work/gh-output-1"
: >"$output1"

UPSTREAM_LOCK_FILE="$lock1" UPSTREAM_CLONE_DIR="$work/clone-nochange" GITHUB_OUTPUT="$output1" \
  bash "$sync_sh" "$head_sha" >"$work/stdout-1.txt" 2>&1 ||
  fail "sync.sh exited non-zero on no-change case"

grep -q "No upstream changes" "$work/stdout-1.txt" ||
  fail "no-change case: missing 'No upstream changes' message"
grep -qx "changed=false" "$output1" ||
  fail "no-change case: GITHUB_OUTPUT missing changed=false"

# ---- Case 2: a change lands after the pin ----
lock2="$work/UPSTREAM-changed.lock"
write_lock "$lock2" "$pinned_sha"

output2="$work/gh-output-2"
: >"$output2"

UPSTREAM_LOCK_FILE="$lock2" UPSTREAM_CLONE_DIR="$work/clone-changed" GITHUB_OUTPUT="$output2" \
  bash "$sync_sh" "$head_sha" >"$work/stdout-2.txt" 2>&1 ||
  fail "sync.sh exited non-zero on changed case"

grep -q "src/utils/retry.ts" "$work/stdout-2.txt" ||
  fail "changed case: changed file not listed"
grep -q -- "-- Utils (ai, ai/internal/\*) --" "$work/stdout-2.txt" ||
  fail "changed case: missing Utils bucket header"
grep -qx "changed=true" "$output2" ||
  fail "changed case: GITHUB_OUTPUT missing changed=true"

# ---- Case 3: UPSTREAM_PINNED_SHA overrides the lock file's commit= ----
# lock3's commit= claims we're already synced (head_sha); the override
# deliberately supplies the stale pinned_sha instead, proving the override
# path used to rehearse a non-empty diff is live code.
lock3="$work/UPSTREAM-uptodate.lock"
write_lock "$lock3" "$head_sha"

output3="$work/gh-output-3"
: >"$output3"

UPSTREAM_LOCK_FILE="$lock3" UPSTREAM_PINNED_SHA="$pinned_sha" UPSTREAM_CLONE_DIR="$work/clone-override" \
  GITHUB_OUTPUT="$output3" bash "$sync_sh" "$head_sha" >"$work/stdout-3.txt" 2>&1 ||
  fail "sync.sh exited non-zero on pinned-sha override case"

grep -q "src/utils/retry.ts" "$work/stdout-3.txt" ||
  fail "override case: changed file not listed"
grep -qx "changed=true" "$output3" ||
  fail "override case: GITHUB_OUTPUT missing changed=true"

echo "ok  upstream/sync_test.sh"
