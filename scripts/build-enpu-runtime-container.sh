#!/usr/bin/env bash
set -euo pipefail

cann_version=$1
cann_env="/usr/local/Ascend/cann-$cann_version/set_env.sh"
[[ -f "$cann_env" ]] || { printf 'CANN environment not found: %s\n' "$cann_env" >&2; exit 1; }
set +u
source "$cann_env"
set -u
export ENPU_ASCEND_DRIVER_PATH=/usr/local/Ascend
[[ -n "${ASCEND_HOME_PATH:-}" ]] || { echo 'CANN did not set ASCEND_HOME_PATH' >&2; exit 1; }
for file in /usr/local/Ascend/driver/include/dcmi_interface_api.h /usr/local/Ascend/driver/lib64/driver/libdcmi.so; do
    [[ -s "$file" ]] || { printf 'Driver SDK file not found: %s\n' "$file" >&2; exit 1; }
done

mkdir -p /tmp/enpu-source
tar -xf /input/source.tar -C /tmp/enpu-source
cd /tmp/enpu-source/ubs-virt-enpu/vcann-rt
bash make_build.sh
install -m 0644 build/libvruntime.so /output/libvruntime.so
install -m 0755 build/enpu-monitor /output/enpu-monitor
printf '%s\n' /usr/local/enpu/vcann-rt/lib/libvruntime.so > /output/ld.so.preload
chmod 0644 /output/ld.so.preload
