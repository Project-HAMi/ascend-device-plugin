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
    if struct.unpack_from("<H", library, 18)[0] != machine:
        raise ValueError("libvnpu.so ELF machine does not match " + arch)
    info = json.loads((directory / "build-info.json").read_text())
    if info.get("architecture") != reported_arch:
        raise ValueError("build-info.json architecture does not match " + arch)
    if info.get("source_commit") != source_commit:
        raise ValueError("build-info.json source does not match the pinned core")
    if info.get("library", {}).get("sha256") != hashlib.sha256(library).hexdigest():
        raise ValueError("build-info.json library SHA256 does not match libvnpu.so")


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
