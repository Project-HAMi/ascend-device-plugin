#!/usr/bin/env bash
# Fails when the chart's default image (appVersion or values image) changes but
# Chart.yaml `version` does not increase. chart-releaser skips versions that are
# already published, so Helm users would keep getting the old image.
#
# Usage: verify-chart-version-bump.sh <base-chart-dir> <head-chart-dir>
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <base-chart-dir> <head-chart-dir>" >&2
  exit 2
fi
base_dir="$1"
head_dir="$2"

chart_field() { awk -v key="$2:" '$1 == key { gsub(/"/, "", $2); print $2; exit }' "$1/Chart.yaml"; }
default_image() { helm template ascend-device-plugin "$1" | awk '$1 == "-" && $2 == "image:" { gsub(/"/, "", $3); print $3; exit }'; }

base_version="$(chart_field "$base_dir" version)"
head_version="$(chart_field "$head_dir" version)"
base_image="$(default_image "$base_dir")"
head_image="$(default_image "$head_dir")"

if [[ -z "$base_version" || -z "$head_version" || -z "$base_image" || -z "$head_image" ]]; then
  echo "failed to resolve chart version or default image" >&2
  exit 1
fi

if [[ "$base_image" == "$head_image" ]]; then
  echo "default image unchanged (${head_image})"
  exit 0
fi

# SemVer precedence: a pre-release (0.2.1-rc.1) is lower than its release (0.2.1). `sort -V` orders the
# longer string higher, so map `-` to `~`, which version sort places before the end of the string.
highest="$(printf '%s\n%s\n' "${base_version//-/\~}" "${head_version//-/\~}" | sort -V | tail -n1)"
if [[ "$head_version" == "$base_version" || "$highest" != "${head_version//-/\~}" ]]; then
  echo "default image changed from ${base_image} to ${head_image}, but chart version ${head_version} is not greater than ${base_version}" >&2
  echo "bump version in charts/ascend-device-plugin/Chart.yaml" >&2
  exit 1
fi

echo "default image changed to ${head_image}; chart version bumped ${base_version} -> ${head_version}"
