#!/bin/bash
# Verify vNPU artifacts architecture and provenance
set -euo pipefail

ARCH="${1:-}"
DIRECTORY="${2:-}"

if [ -z "$ARCH" ] || [ -z "$DIRECTORY" ]; then
    echo "Usage: $0 <arch> <directory>" >&2
    echo "Example: $0 amd64 ./artifacts" >&2
    exit 1
fi

# Verify checksums
if command -v sha256sum >/dev/null 2>&1; then
    (cd "$DIRECTORY" && sha256sum -c SHA256SUMS)
else
    (cd "$DIRECTORY" && shasum -a 256 -c SHA256SUMS)
fi

# Verify architecture and provenance
SOURCE_COMMIT=$(git -C libvnpu rev-parse HEAD)
python3 scripts/verify-vnpu-artifacts.py \
    --arch "$ARCH" \
    --directory "$DIRECTORY" \
    --source-commit "$SOURCE_COMMIT"
