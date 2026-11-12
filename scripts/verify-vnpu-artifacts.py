#!/usr/bin/env python3
"""Reject vNPU artifacts that do not match the image architecture and source."""

import argparse
import hashlib
import json
from pathlib import Path
import struct
import sys

ARCHITECTURES = {"amd64": (62, "x86_64"), "arm64": (183, "aarch64")}


def verify(directory, arch, source_commit):
    """Bind the ELF machine to its build report, source revision and image leg."""
    machine, reported_arch = ARCHITECTURES[arch]
    for name in ("libvnpu.so", "ld.so.preload", "build-info.json", "SHA256SUMS"):
        path = directory / name
        if not path.is_file() or path.stat().st_size == 0:
            raise ValueError("missing or empty artifact: " + name)
    library = (directory / "libvnpu.so").read_bytes()
    if len(library) < 64 or library[:6] != b"\x7fELF\x02\x01":
        raise ValueError("libvnpu.so must be a 64-bit little-endian ELF")
    actual_machine = struct.unpack_from("<H", library, 18)[0]
    if actual_machine != machine:
        raise ValueError(
            f"libvnpu.so ELF machine {actual_machine} does not match "
            f"expected {arch} (machine {machine})"
        )
    info = json.loads((directory / "build-info.json").read_text())
    actual_arch = info.get("architecture")
    if actual_arch != reported_arch:
        raise ValueError(
            f"build-info.json architecture '{actual_arch}' does not match "
            f"expected '{reported_arch}' for {arch}"
        )
    actual_commit = info.get("source_commit")
    if actual_commit != source_commit:
        raise ValueError(
            f"build-info.json source commit '{actual_commit}' does not match "
            f"the pinned core '{source_commit}'"
        )
    actual_hash = hashlib.sha256(library).hexdigest()
    expected_hash = info.get("library", {}).get("sha256")
    if expected_hash != actual_hash:
        raise ValueError(
            f"build-info.json library SHA256 '{expected_hash}' does not match "
            f"libvnpu.so '{actual_hash}'"
        )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", type=Path, required=True)
    parser.add_argument("--arch", choices=ARCHITECTURES, required=True)
    parser.add_argument("--source-commit", required=True)
    args = parser.parse_args()
    try:
        verify(args.directory, args.arch, args.source_commit)
    except (OSError, ValueError) as error:
        print("Invalid VNPU artifacts: " + str(error), file=sys.stderr)
        return 1
    print("Verified VNPU ELF architecture and provenance for " + args.arch)
    return 0


if __name__ == "__main__":
    sys.exit(main())
