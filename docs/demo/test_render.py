import os
from pathlib import Path
import unittest
from unittest.mock import patch

from PIL import Image, ImageDraw, ImageFont

import render


class RenderTests(unittest.TestCase):
    def test_sgr_colors(self):
        draw = ImageDraw.Draw(Image.new("RGB", (40, 30)))
        font = ImageFont.load_default()
        render.draw_terminal(draw, "\x1b[38;5;123mX\x1b[48;2;1;2;3mY", font, 0, 0)
        for codes in ("38", "48", "38;5", "48;2;1;2", "38;5;256", "48;2;256;0;0"):
            with self.subTest(codes=codes), self.assertRaises(ValueError):
                render.draw_terminal(draw, f"\x1b[{codes}mX", font, 0, 0)

    def test_duration_validation(self):
        render.validate_duration([{"seconds": 4}, {"seconds": 26}])
        for durations in ([], [29], [0, 30], [-1, 31], [float("nan")], [float("inf")]):
            with self.subTest(durations=durations), self.assertRaises(ValueError):
                render.validate_duration([{"seconds": d} for d in durations])

    def test_capture_environment_does_not_forward_aws_credentials(self):
        caller = {"PATH": "/bin", "AWS_ACCESS_KEY_ID": "synthetic-key",
                  "AWS_SECRET_ACCESS_KEY": "synthetic-secret", "AWS_SESSION_TOKEN": "synthetic-token",
                  "AWS_PROFILE": "production", "AWS_CONFIG_FILE": "/real/config",
                  "AWS_SHARED_CREDENTIALS_FILE": "/real/credentials",
                  "AWS_CONTAINER_CREDENTIALS_FULL_URI": "http://example.invalid"}
        with patch.dict(os.environ, caller, clear=True):
            env = render.capture_environment(Path("/tmp/demo"), Path("/tmp/demo/frames.json"))
            self.assertEqual(dict(os.environ), caller)
        self.assertEqual(env["PATH"], "/bin")
        self.assertEqual({k: v for k, v in env.items() if k.startswith("AWS_")}, {
            "AWS_EC2_METADATA_DISABLED": "true",
            "AWS_CONFIG_FILE": "/tmp/demo/aws-config",
            "AWS_SHARED_CREDENTIALS_FILE": "/tmp/demo/aws-credentials",
        })


if __name__ == "__main__":
    unittest.main()
