#!/usr/bin/env bash
# Diff the pinned upstream revision against upstream HEAD for packages/ai,
# bucketing changed files by area so they can be mapped to Go packages via
# PORTING.md. Usage: upstream/sync.sh [ref]   (default: origin/main)
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
lock="$here/UPSTREAM.lock"
repo=$(grep '^repo=' "$lock" | cut -d= -f2)
pinned=$(grep '^commit=' "$lock" | cut -d= -f2)
ref="${1:-origin/main}"

workdir="${UPSTREAM_CLONE_DIR:-$here/.upstream-clone}"
if [ ! -d "$workdir/.git" ]; then
  git clone --filter=blob:none "$repo" "$workdir"
fi
git -C "$workdir" fetch origin

echo "== upstream: $pinned..$ref (packages/ai) =="
changed=$(git -C "$workdir" diff --name-only "$pinned..$ref" -- packages/ai || true)
if [ -z "$changed" ]; then
  echo "No upstream changes since the pinned revision."
  exit 0
fi

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
