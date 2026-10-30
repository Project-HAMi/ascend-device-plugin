#!/usr/bin/env bash
# Exercises verify-chart-version-bump.sh against edited copies of the chart.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
script="$root/hack/verify-chart-version-bump.sh"
chart="$root/charts/ascend-device-plugin"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

failures=0

# case <name> <want: pass|fail> <sed expression for Chart.yaml> [sed expression for values.yaml]
case_() {
  local name="$1" want="$2" chart_edit="$3" values_edit="${4:-}"
  local head="$work/$name"
  cp -R "$chart" "$head"
  [[ -n "$chart_edit" ]] && sed -i.bak -e "$chart_edit" "$head/Chart.yaml"
  [[ -n "$values_edit" ]] && sed -i.bak -e "$values_edit" "$head/values.yaml"
  local got=pass
  "$script" "$chart" "$head" >/dev/null 2>&1 || got=fail
  if [[ "$got" == "$want" ]]; then
    echo "ok   $name"
  else
    echo "FAIL $name: want $want, got $got" >&2
    failures=$((failures + 1))
  fi
}

version="$(awk '$1 == "version:" { print $2; exit }' "$chart/Chart.yaml")"
bumped="$(echo "$version" | awk -F. '{ printf "%d.%d.%d", $1, $2, $3 + 1 }')"

case_ unchanged pass ""
case_ app-version-without-chart-bump fail 's/^appVersion: .*/appVersion: v9.9.9/'
case_ app-version-with-chart-bump pass "s/^appVersion: .*/appVersion: v9.9.9/; s/^version: .*/version: $bumped/"
case_ image-tag-without-chart-bump fail "" 's/^  tag: ""/  tag: "v9.9.9"/'
case_ image-repository-without-chart-bump fail "" 's|^  repository: [^ ]*|  repository: example.com/ascend-device-plugin|'
case_ app-version-with-chart-downgrade fail 's/^appVersion: .*/appVersion: v9.9.9/; s/^version: .*/version: 0.0.1/'
case_ chart-bump-only pass "s/^version: .*/version: $bumped/"
case_ app-version-with-prerelease-of-same-chart fail "s/^appVersion: .*/appVersion: v9.9.9/; s/^version: .*/version: $version-rc.1/"
case_ app-version-with-prerelease-of-next-chart pass "s/^appVersion: .*/appVersion: v9.9.9/; s/^version: .*/version: $bumped-rc.1/"

if [[ "$failures" -ne 0 ]]; then
  exit 1
fi
