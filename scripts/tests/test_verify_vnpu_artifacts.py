"""Architecture checks use minimal ELF headers, without an SDK or an NPU."""

import hashlib
import importlib.util
import json
from pathlib import Path
import struct
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / "verify-vnpu-artifacts.py"
spec = importlib.util.spec_from_file_location("verify_vnpu_artifacts", SCRIPT)
verifier = importlib.util.module_from_spec(spec)
spec.loader.exec_module(verifier)
SOURCE = "a" * 40


class ArtifactTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.directory = Path(temporary.name)

    def bundle(self, arch):
        machine, reported = verifier.ARCHITECTURES[arch]
        elf = bytearray(64)
        elf[:6] = b"\x7fELF\x02\x01"
        struct.pack_into("<H", elf, 18, machine)
        (self.directory / "libvnpu.so").write_bytes(elf)
        info = {"architecture": reported, "source_commit": SOURCE,
                "library": {"sha256": hashlib.sha256(elf).hexdigest()}}
        (self.directory / "build-info.json").write_text(json.dumps(info))
        (self.directory / "ld.so.preload").write_text("/hami-vnpu-core/libvnpu.so\n")
        (self.directory / "SHA256SUMS").write_text("fixture\n")

    def test_matching_architectures(self):
        for arch in verifier.ARCHITECTURES:
            with self.subTest(arch=arch):
                self.bundle(arch)
                verifier.verify(self.directory, arch, SOURCE)

    def test_rejects_arm_library_in_amd64_image(self):
        self.bundle("arm64")
        with self.assertRaisesRegex(ValueError, "ELF machine"):
            verifier.verify(self.directory, "amd64", SOURCE)

    def test_rejects_amd64_library_in_arm_image(self):
        self.bundle("amd64")
        with self.assertRaisesRegex(ValueError, "ELF machine"):
            verifier.verify(self.directory, "arm64", SOURCE)

    def test_rejects_mislabelled_build_report(self):
        self.bundle("amd64")
        info = self.directory / "build-info.json"
        data = json.loads(info.read_text())
        data["architecture"] = "aarch64"
        info.write_text(json.dumps(data))
        with self.assertRaisesRegex(ValueError, "architecture"):
            verifier.verify(self.directory, "amd64", SOURCE)

    def test_rejects_changed_library(self):
        self.bundle("amd64")
        with (self.directory / "libvnpu.so").open("ab") as stream:
            stream.write(b"changed")
        with self.assertRaisesRegex(ValueError, "SHA256"):
            verifier.verify(self.directory, "amd64", SOURCE)

    def test_rejects_wrong_revision(self):
        self.bundle("amd64")
        with self.assertRaisesRegex(ValueError, "pinned core"):
            verifier.verify(self.directory, "amd64", "b" * 40)

    def test_rejects_truncated_elf(self):
        self.bundle("amd64")
        (self.directory / "libvnpu.so").write_bytes(b"\x7fELF")
        with self.assertRaisesRegex(ValueError, "64-bit"):
            verifier.verify(self.directory, "amd64", SOURCE)

    def test_rejects_missing_preload(self):
        self.bundle("amd64")
        (self.directory / "ld.so.preload").unlink()
        with self.assertRaisesRegex(ValueError, "ld.so.preload"):
            verifier.verify(self.directory, "amd64", SOURCE)


if __name__ == "__main__":
    unittest.main()
