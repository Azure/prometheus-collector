import copy
import unittest
from unittest.mock import patch

from discovery_log import emit, read


class DiscoveryLogTest(unittest.TestCase):
    def test_roundtrip_rejects_bad_checksum_build_source_images_and_frames(self):
        images = {"production": {"ref": "mcr.microsoft.com/image@sha256:" + "a" * 64},
                  "main": {"ref": "mcr.microsoft.com/image@sha256:" + "b" * 64}}
        discovery = {"verdict": "DISCOVERY_ONLY", "images": images,
                     "productionFindings": [{"id": f"CVE-2026-{index}", "package": "example"}
                                            for index in range(126)]}
        sha = "c" * 40
        text = emit(discovery, 131371, sha, images)
        timestamped = "\n".join("2026-11-09T01:02:03.123Z " + line for line in text.splitlines())
        self.assertEqual(read(timestamped, 131371, sha, images), discovery)
        with self.assertRaisesRegex(ValueError, "checksum"):
            read(text.replace(text.split()[1], "0" * 64), 131371, sha, images)
        altered = copy.deepcopy(images)
        altered["main"]["ref"] = "mcr.microsoft.com/image@sha256:" + "d" * 64
        mutable = copy.deepcopy(images)
        mutable["main"]["ref"] = "mcr.microsoft.com/image:latest"
        for build, source, expected_images in ((131372, sha, images), (131371, "d" * 40, images),
                                                (131371, sha, altered), (131371, sha, mutable),
                                                (131371, sha, {})):
            with self.subTest(build=build, source=source, images=expected_images):
                with self.assertRaisesRegex(ValueError, "binding"):
                    read(text, build, source, expected_images)
        for broken in (text + text, "\n".join(text.splitlines()[:-1]), "TF400813: not authorized"):
            with self.assertRaisesRegex(ValueError, "frame"):
                read(broken, 131371, sha, images)
        with patch("discovery_log.MAX_BYTES", 10):
            with self.assertRaisesRegex(ValueError, "limit"):
                emit(discovery, 131371, sha, images)
