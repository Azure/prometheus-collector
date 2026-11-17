import hashlib
import io
import tarfile
import unittest
from unittest.mock import patch

import native_image


IMAGE = "mcr.microsoft.com/example/image@sha256:" + "1" * 64


def layer(entries):
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w:gz") as archive:
        for path, content in entries.items():
            member = tarfile.TarInfo(path)
            if content is None:
                member.type = tarfile.DIRTYPE
                archive.addfile(member)
                continue
            if isinstance(content, tuple):
                member.type = tarfile.SYMTYPE
                member.linkname = content[0]
                archive.addfile(member)
                continue
            member.size = len(content)
            archive.addfile(member, io.BytesIO(content))
    return output.getvalue()


class NativeImageTest(unittest.TestCase):
    def test_final_bytes_and_unsafe_layers(self):
        original = layer({"usr": None, "usr/sbin": None,
                          **{path: name.encode() for name, path in native_image.PATHS.items()}})
        replacement = layer({"usr": None, "usr/sbin": None, "usr/sbin/mdsd": b"replacement"})
        deletion = layer({"usr/sbin/.wh.mdsd": b""})
        opaque = layer({"usr/sbin/.wh..wh..opq": b""})
        noncanonical = layer({"usr/sbin/../sbin/mdsd": b"unexpected"})
        for last, expected in ((replacement, None), (deletion, "whiteout"),
                               (opaque, "whiteout"), (noncanonical, "Noncanonical")):
            with self.subTest(expected=expected):
                blobs = {"sha256:" + hashlib.sha256(raw).hexdigest(): raw for raw in (original, last)}
                manifest = {"config": {"digest": "sha256:" + "2" * 64},
                            "layers": [{"digest": digest} for digest in blobs]}
                with patch.object(native_image, "registry_json", side_effect=[
                        (manifest, ""), ({"os": "linux", "architecture": "amd64"}, "")]), \
                        patch.object(native_image, "urlopen", side_effect=lambda url, timeout: (
                            io.BytesIO(blobs[url.rsplit("/", 1)[1]]))):
                    if expected:
                        with self.assertRaisesRegex(ValueError, expected):
                            native_image.binary_hashes(IMAGE, "linux/amd64")
                    else:
                        self.assertEqual(native_image.binary_hashes(IMAGE, "linux/amd64"), {
                            "metricsext2": hashlib.sha256(b"metricsext2").hexdigest(),
                            "azure-mdsd": hashlib.sha256(b"replacement").hexdigest(),
                        })
        inherited = layer({path: b"new bytes" for path in native_image.PATHS.values()})
        blobs = {"sha256:" + hashlib.sha256(raw).hexdigest(): raw for raw in (original, inherited)}
        manifest = {"config": {"digest": "sha256:" + "2" * 64},
                    "layers": [{"digest": digest} for digest in blobs]}
        with patch.object(native_image, "registry_json", side_effect=[
                (manifest, ""), ({"os": "linux", "architecture": "amd64"}, "")]), \
                patch.object(native_image, "urlopen", side_effect=lambda url, timeout: (
                    io.BytesIO(blobs[url.rsplit("/", 1)[1]]))):
            self.assertEqual(native_image.binary_hashes(IMAGE, "linux/amd64"), {
                name: hashlib.sha256(b"new bytes").hexdigest() for name in native_image.PATHS
            })
        symlink = layer({"usr": None, "usr/sbin": ("elsewhere",)})
        blobs = {"sha256:" + hashlib.sha256(raw).hexdigest(): raw for raw in (symlink, inherited)}
        manifest["layers"] = [{"digest": digest} for digest in blobs]
        with patch.object(native_image, "registry_json", side_effect=[
                (manifest, ""), ({"os": "linux", "architecture": "amd64"}, "")]), \
                patch.object(native_image, "urlopen", side_effect=lambda url, timeout: (
                    io.BytesIO(blobs[url.rsplit("/", 1)[1]]))):
            with self.assertRaisesRegex(ValueError, "unsupported ancestor"):
                native_image.binary_hashes(IMAGE, "linux/amd64")
        recreated = layer({"usr": None, "usr/sbin": None})
        blobs = {"sha256:" + hashlib.sha256(raw).hexdigest(): raw for raw in (original, symlink, recreated)}
        manifest["layers"] = [{"digest": digest} for digest in blobs]
        with patch.object(native_image, "registry_json", side_effect=[
                (manifest, ""), ({"os": "linux", "architecture": "amd64"}, "")]), \
                patch.object(native_image, "urlopen", side_effect=lambda url, timeout: (
                    io.BytesIO(blobs[url.rsplit("/", 1)[1]]))):
            with self.assertRaisesRegex(ValueError, "unsupported ancestor"):
                native_image.binary_hashes(IMAGE, "linux/amd64")
        manifest["layers"] = [{"digest": "sha256:" + "3" * 64}]
        with patch.object(native_image, "registry_json", side_effect=[
                (manifest, ""), ({"os": "linux", "architecture": "amd64"}, "")]), \
                patch.object(native_image, "urlopen", return_value=io.BytesIO(original)):
            with self.assertRaisesRegex(ValueError, "digest mismatch"):
                native_image.binary_hashes(IMAGE, "linux/amd64")
        for invalid in ([], {}, {"layers": [], "config": {"digest": []}}):
            with self.subTest(manifest=invalid), \
                    patch.object(native_image, "registry_json", return_value=(invalid, "")):
                with self.assertRaises(ValueError):
                    native_image.binary_hashes(IMAGE, "linux/amd64")


if __name__ == "__main__":
    unittest.main()
