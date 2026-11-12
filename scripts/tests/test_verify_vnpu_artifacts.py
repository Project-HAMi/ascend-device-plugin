"""Architecture checks use minimal ELF headers, without an SDK or an NPU."""

import hashlib
import importlib.util
import json
from pathlib import Path
import struct
import shutil
import subprocess
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
        self.write_manifest()

    def write_manifest(self):
        lines = []
        for name in ("libvnpu.so", "build-info.json"):
            digest = hashlib.sha256((self.directory / name).read_bytes()).hexdigest()
            lines.append(f"{digest}  {name}\n")
        (self.directory / "SHA256SUMS").write_text("".join(lines))

    def test_bundle_entrypoint_checks_complete_local_output(self):
        self.bundle("amd64")
        core = self.directory / "libvnpu"
        subprocess.run(["git", "init", str(core)], check=True, capture_output=True)
        subprocess.run(["git", "-C", str(core), "-c", "user.name=Test",
                        "-c", "user.email=test@example.invalid", "commit",
                        "--allow-empty", "-m", "fixture"], check=True, capture_output=True)
        commit = subprocess.check_output(["git", "-C", str(core), "rev-parse", "HEAD"],
                                         text=True).strip()
        info = self.directory / "build-info.json"
        data = json.loads(info.read_text())
        data["source_commit"] = commit
        info.write_text(json.dumps(data))
        self.write_manifest()
        scripts = self.directory / "scripts"
        scripts.mkdir()
        for name in ("verify-vnpu-artifacts.py", "verify-vnpu-bundle.sh"):
            shutil.copyfile(SCRIPT.parent / name, scripts / name)
        argv = ["bash", "scripts/verify-vnpu-bundle.sh", "amd64", "."]
        result = subprocess.run(argv, cwd=self.directory, text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        (self.directory / "ld.so.preload").unlink()
        result = subprocess.run(argv, cwd=self.directory, text=True, capture_output=True)
        self.assertEqual(result.returncode, 1)
        self.assertIn("missing or empty artifact: ld.so.preload", result.stderr)

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
