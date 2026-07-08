#!/usr/bin/env bash
# Diff the pinned upstream revision against upstream HEAD for packages/ai,
# bucketing changed files by area so they can be mapped to Go packages via
# docs/PORTING.md. Usage: upstream/sync.sh [ref]   (default: origin/main)
#
# Env overrides (all optional):
#   UPSTREAM_LOCK_FILE   path to the lock file (default: $here/UPSTREAM.lock)
#   UPSTREAM_PINNED_SHA  overrides the lock file's commit=, letting a
#                        deliberately stale SHA rehearse the non-empty-diff
#                        path (see upstream/sync_test.sh, and the weekly CI
#                        job in .github/workflows/upstream-sync.yml)
#   UPSTREAM_CLONE_DIR   where to clone/reuse the upstream checkout
#                        (default: $here/.upstream-clone)
#   GITHUB_OUTPUT        when set (GitHub Actions sets this automatically),
#                        appends "changed=true"/"changed=false" so a workflow
#                        step can branch on it
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
lock="${UPSTREAM_LOCK_FILE:-$here/UPSTREAM.lock}"
repo=$(grep '^repo=' "$lock" | cut -d= -f2)
pinned="${UPSTREAM_PINNED_SHA:-$(grep '^commit=' "$lock" | cut -d= -f2)}"
ref="${1:-origin/main}"

workdir="${UPSTREAM_CLONE_DIR:-$here/.upstream-clone}"
if [ ! -d "$workdir/.git" ]; then
  git clone --filter=blob:none "$repo" "$workdir"
fi
git -C "$workdir" fetch origin

report_changed() {
  # report_changed <true|false>
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    echo "changed=$1" >>"$GITHUB_OUTPUT"
  fi
}

echo "== upstream: $pinned..$ref (packages/ai) =="
changed=$(git -C "$workdir" diff --name-only "$pinned..$ref" -- packages/ai || true)
if [ -z "$changed" ]; then
  echo "No upstream changes since the pinned revision."
  report_changed false
  exit 0
fi
report_changed true

bucket() { echo; echo "-- $1 --"; grep -E "$2" <<<"$changed" | { [ -n "$3" ] && grep -Ev "$3" || cat; } || true; }

bucket "Catalog (re-run tools/export-catalog only)" 'src/providers/.*\.models\.ts|src/models\.generated\.ts|src/image-models\.generated\.ts' ''
bucket "API adapters (ai/apis/*)" 'packages/ai/src/api/' ''
bucket "Provider bindings (ai/providers)" 'packages/ai/src/providers/' '\.models\.ts'
bucket "Auth (ai/auth)" 'packages/ai/src/(auth/|oauth\.ts|env-api-keys\.ts|utils/oauth/|utils/provider-env\.ts)' ''
bucket "Core types/models (ai)" 'packages/ai/src/(types\.ts|models\.ts)' ''
bucket "Utils (ai, ai/internal/*)" 'packages/ai/src/utils/' 'utils/oauth|utils/provider-env'
bucket "Images (ai/images)" 'packages/ai/src/(images|image-models)' '\.generated\.ts'
bucket "Tests (port alongside)" 'packages/ai/test/' ''
bucket "Generator/scripts" 'packages/ai/scripts/' ''

echo
echo "Full diff: git -C $workdir diff $pinned..$ref -- packages/ai"
echo "After porting, update commit= in $lock"
