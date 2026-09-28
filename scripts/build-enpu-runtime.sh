#!/usr/bin/env bash
set -euo pipefail
export GIT_NO_REPLACE_OBJECTS=1

release=1.0.0
commit=46b6d55a892429008b1e02f96b0a0f81d1176db8
repository=https://gitcode.com/openeuler/ubs-virt.git
builder=swr.cn-north-4.myhuaweicloud.com/ubscore/ubs-virt:oe2403-v1
cann_version=9.1.0
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
output_dir="$script_dir/../enpu-runtime-assets"
driver_path=
source_dir=

usage() {
    cat <<EOF
Usage: $0 [options]

Build official ENPU $release ($commit) with CANN $cann_version.
  --output-dir PATH    Asset destination (default: enpu-runtime-assets)
  --driver-path PATH   Override the image's driver SDK with a read-only host mount
  --source-dir PATH    Clean local Git repository containing official tag $release
  --builder-image REF  Official builder tag or digest (default: $builder)
  -h, --help          Show this help
EOF
}

fail() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

while (($#)); do
    case "$1" in
        --output-dir|--driver-path|--source-dir|--builder-image)
            (($# >= 2)) && [[ -n "$2" ]] || fail "Missing value for $1"
            case "$1" in
                --output-dir) output_dir=$2 ;;
                --driver-path) driver_path=$2 ;;
                --source-dir) source_dir=$2 ;;
                --builder-image) builder=$2 ;;
            esac
            shift 2
            ;;
        -h|--help) usage; exit 0 ;;
        *) fail "Unknown option: $1" ;;
    esac
done

case "$builder" in
    swr.cn-north-4.myhuaweicloud.com/ubscore/ubs-virt:*|swr.cn-north-4.myhuaweicloud.com/ubscore/ubs-virt@sha256:*) ;;
    *) fail 'Use an official ubscore/ubs-virt builder image' ;;
esac
for tool in git docker python3; do
    command -v "$tool" >/dev/null || fail "Required command not found: $tool"
done
if [[ -n "$driver_path" ]]; then
    [[ -s "$driver_path/include/dcmi_interface_api.h" ]] || fail "Driver header not found: $driver_path/include/dcmi_interface_api.h"
    [[ -s "$driver_path/lib64/driver/libdcmi.so" ]] || fail "Driver library not found: $driver_path/lib64/driver/libdcmi.so"
    driver_path=$(cd "$driver_path" && pwd -P)
fi
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/enpu-release-build.XXXXXX")
trap 'rm -rf "$work_dir"' EXIT
mkdir -p "$work_dir/input" "$work_dir/output"

if [[ -n "$source_dir" ]]; then
    source_dir=$(cd "$source_dir" && pwd -P)
    [[ -z "$(git -C "$source_dir" status --porcelain --untracked-files=normal)" ]] || fail 'Local source repository must be clean'
else
    source_dir="$work_dir/source"
    git init -q "$source_dir"
    git -C "$source_dir" fetch --depth=1 "$repository" "refs/tags/$release:refs/tags/$release"
fi
actual_commit=$(git -C "$source_dir" rev-parse "refs/tags/$release^{commit}")
[[ "$actual_commit" == "$commit" ]] || fail "Tag $release resolves to $actual_commit; expected $commit"
git -C "$source_dir" archive --format=tar "$commit" ubs-virt-enpu/vcann-rt > "$work_dir/input/source.tar"
cp "$script_dir/build-enpu-runtime-container.sh" "$work_dir/input/build.sh"

if ! docker image inspect "$builder" > "$work_dir/image.json" 2>/dev/null; then
    docker pull --platform linux/arm64 "$builder"
    docker image inspect "$builder" > "$work_dir/image.json"
fi
image_id=$(python3 - "$work_dir/image.json" <<'PY'
import json
import sys

image = json.load(open(sys.argv[1]))[0]
if (image["Os"], image["Architecture"]) != ("linux", "arm64"):
    raise SystemExit("Official ENPU build requires a linux/arm64 builder image")
print(image["Id"])
PY
)
docker_args=(run --rm --platform linux/arm64 --network none)
if [[ -n "$driver_path" ]]; then
    docker_args+=(--mount "type=bind,src=$driver_path,dst=/usr/local/Ascend/driver,readonly")
fi
docker "${docker_args[@]}" \
    --mount "type=bind,src=$work_dir/input,dst=/input,readonly" \
    --mount "type=bind,src=$work_dir/output,dst=/output" \
    --entrypoint /bin/bash "$image_id" /input/build.sh "$cann_version"

python3 - "$work_dir" "$repository" "$release" "$commit" "$builder" "$cann_version" "$driver_path" <<'PY'
import hashlib
import json
import pathlib
import struct
import sys

work, repository, release, commit, builder, cann, driver = sys.argv[1:]
work = pathlib.Path(work)
output = work / "output"
image = json.loads((work / "image.json").read_text())[0]

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

for name in ("libvruntime.so", "enpu-monitor"):
    header = (output / name).read_bytes()[:20]
    if len(header) < 20 or header[:6] != b"\x7fELF\x02\x01" or struct.unpack("<H", header[18:20])[0] != 183:
        raise SystemExit(f"Expected an AArch64 ELF binary: {name}")
assets = {name: digest(output / name) for name in ("libvruntime.so", "enpu-monitor", "ld.so.preload")}
manifest = {
    "source": {"repository": repository, "tag": release, "commit": commit,
               "archive_sha256": digest(work / "input/source.tar")},
    "builder": {"image": builder, "id": image["Id"], "repo_digests": image.get("RepoDigests", []),
                "os": image["Os"], "architecture": image["Architecture"]},
    "cann_version": cann,
    "driver_sdk": {"source": "host" if driver else "builder-image",
                   "path": driver or "/usr/local/Ascend/driver"},
    "assets": assets,
}
(output / "build-info.json").write_text(json.dumps(manifest, indent=2) + "\n")
assets["build-info.json"] = digest(output / "build-info.json")
(output / "SHA256SUMS").write_text("".join(f"{checksum}  {name}\n" for name, checksum in assets.items()))
PY

mkdir -p "$output_dir"
for name in libvruntime.so enpu-monitor ld.so.preload build-info.json SHA256SUMS; do
    temp_asset=$(mktemp "$output_dir/.${name}.XXXXXX")
    cp "$work_dir/output/$name" "$temp_asset"
    chmod 0644 "$temp_asset"
    [[ "$name" != enpu-monitor ]] || chmod 0755 "$temp_asset"
    mv -f "$temp_asset" "$output_dir/$name"
done
printf 'ENPU %s assets: %s\n' "$release" "$(cd "$output_dir" && pwd -P)"
cat "$output_dir/SHA256SUMS"
