#!/usr/bin/env bash
set -euo pipefail

chart=$(cd "$(dirname "${BASH_SOURCE[0]}")/../charts/ascend-device-plugin" && pwd)
helm=${HELM:-helm}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

check_mode() {
    local expected=$1
    shift
    "$helm" template mode-test "$chart" "$@" > "$work/rendered.yaml"
    grep -Fq "hamiVnpuMode: \"$expected\"" "$work/rendered.yaml"
    if grep -Eq '^[[:space:]]+(hamiVnpuCore|enpu): (true|false)' "$work/rendered.yaml"; then
        echo 'rendered config still contains a legacy mode boolean' >&2
        exit 1
    fi
}

reject() {
    if "$helm" template mode-test "$chart" "$@" > "$work/rendered.yaml" 2> "$work/error.log"; then
        echo "invalid mode values were accepted: $*" >&2
        exit 1
    fi
}

check_mode template
check_mode template --set hamiVnpuMode=template
check_mode hami-core --set hamiVnpuMode=hami-core
check_mode hami-core --set hamiVnpuMode=hamiCore
check_mode enpu --set-string 'hamiVnpuMode= ENPU '
check_mode hami-core --set hamiVnpuCore.enabled=true
check_mode template --set hamiVnpuCore.enabled=true --set hamiVnpuMode=template
check_mode enpu --set hamiVnpuCore.enabled=true --set hamiVnpuMode=enpu
grep -Fq 'hamiCorePolicy: "spatial"' "$work/rendered.yaml"
for policy in spatial dynamic fixed overdraft; do
    "$helm" template mode-test "$chart" --set "hamiVnpuCore.defaultPolicy=$policy" > "$work/rendered.yaml"
    grep -Fq "hamiCorePolicy: \"$policy\"" "$work/rendered.yaml"
done
reject --set hamiVnpuCore.defaultPolicy=elastic
reject --set hamiVnpuCore.defaultPolicy=unknown
reject --set hamiVnpuCore.defaultPolicy=false
reject --set hamiVnpuCore.defaultPolicy=
reject --set hamiVnpuMode=enup
reject --set hamiVnpuMode=false
reject --set-string hamiVnpuCore.enabled=true
reject --set enpu.enabled=true
reject --set enpu.enabled=false
reject --set hamiVnpuMode=enup --set config.create=false
reject --set hamiVnpuMode=enup --set-string 'deviceConfig=vnpus: {}'

"$helm" template mode-test "$chart" -f "$chart/../../examples/enpu/device-plugin-values.yaml" > "$work/rendered.yaml"
grep -Fq 'hamiVnpuMode: enpu' "$work/rendered.yaml"
echo 'VNPU mode and hami-core policy chart checks passed'
