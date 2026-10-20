import os
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import Mock, patch

from PIL import Image, ImageDraw, ImageFont

import render


class RenderTests(unittest.TestCase):
    def test_encoder_failure_preserves_existing_assets(self):
        for failure in (2, 3, None):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as temporary:
                out = Path(temporary)
                assets = [out / f"ecs-rollout.{extension}" for extension in ("mp4", "gif", "png")]
                for asset in assets:
                    asset.write_bytes(b"old asset")
                calls = 0

                def run(command, **kwargs):
                    nonlocal calls
                    calls += 1
                    if calls == failure:
                        raise subprocess.CalledProcessError(1, command)
                    if calls == 1:
                        Path(kwargs["env"]["UNIC_DEMO_FRAMES"]).write_text(json.dumps([
                            {"seconds": 30, "chapter": "02 / INSPECT THE ROLLOUT",
                             "caption": "test caption", "keys": "test keys", "ansi": ""}
                        ]), encoding="utf-8")
                    else:
                        Path(command[-1]).write_bytes(b"new encoded asset")

                with patch("sys.argv", ["render.py", "--output-dir", str(out), "--font", __file__]), \
                     patch.object(render.subprocess, "run", side_effect=run), \
                     patch.object(render.imageio_ffmpeg, "get_ffmpeg_exe", return_value="ffmpeg"), \
                     patch.object(render, "render", return_value=Image.new("RGB", (2, 2))):
                    if failure:
                        with self.assertRaises(subprocess.CalledProcessError):
                            render.main()
                    else:
                        render.main()
                for asset in assets:
                    if failure:
                        self.assertEqual(asset.read_bytes(), b"old asset")
                    else:
                        self.assertNotEqual(asset.read_bytes(), b"old asset")

    def test_sgr_colors(self):
        draw = ImageDraw.Draw(Image.new("RGB", (40, 30)))
        font = ImageFont.load_default()
        render.draw_terminal(draw, "\x1b[38;5;123mX\x1b[48;2;1;2;3mY", font, 0, 0)
        for codes in ("38", "48", "38;5", "48;2;1;2", "38;5;256", "48;2;256;0;0", "48;2;-1;0;0", "38;1", "48;3"):
            with self.subTest(codes=codes), self.assertRaises(ValueError):
                render.draw_terminal(draw, f"\x1b[{codes}mX", font, 0, 0)

    def test_bright_sgr_colors(self):
        font = ImageFont.load_default()
        for index in range(8):
            with self.subTest(index=index):
                draw = Mock()
                render.draw_terminal(draw, f"\x1b[{90 + index};{100 + index}mX", font, 0, 0)
                self.assertEqual(draw.text.call_args.kwargs["fill"], render.BASE[8 + index])
                self.assertEqual(draw.rectangle.call_args.kwargs["fill"], render.BASE[8 + index])

    def test_duration_validation(self):
        render.validate_duration([{"seconds": 4}, {"seconds": 26}])
        for durations in ([], [29], [0, 30], [-1, 31], [float("nan")], [float("inf")]):
            with self.subTest(durations=durations), self.assertRaises(ValueError):
                render.validate_duration([{"seconds": d} for d in durations])

    def test_missing_poster_is_an_error(self):
        poster = {"chapter": "02 / INSPECT THE ROLLOUT"}
        self.assertIs(render.select_poster([{"chapter": "intro"}, poster]), poster)
        for frames in ([], [{"chapter": "renamed chapter"}]):
            with self.subTest(frames=frames), self.assertRaises(ValueError):
                render.select_poster(frames)

    def test_capture_environment_does_not_forward_aws_credentials(self):
        caller = {"PATH": "/bin", "XDG_CONFIG_HOME": "/real/unic", "TZ": "Asia/Seoul",
                  "AWS_ACCESS_KEY_ID": "synthetic-key",
                  "AWS_SECRET_ACCESS_KEY": "synthetic-secret", "AWS_SESSION_TOKEN": "synthetic-token",
                  "AWS_PROFILE": "production", "AWS_CONFIG_FILE": "/real/config",
                  "AWS_SHARED_CREDENTIALS_FILE": "/real/credentials",
                  "AWS_CONTAINER_CREDENTIALS_FULL_URI": "http://example.invalid",
                  "AWS_CONTAINER_CREDENTIALS_RELATIVE_URI": "/synthetic/credentials",
                  "UNIC_CONTEXT": "production", "UNIC_DEMO_FRAMES": "/stale/frames.json"}
        with patch.dict(os.environ, caller, clear=True):
            env = render.capture_environment(Path("/tmp/demo"), Path("/tmp/demo/frames.json"))
            self.assertEqual(dict(os.environ), caller)
        self.assertEqual(env, {
            "PATH": "/bin", "TZ": "UTC", "XDG_CONFIG_HOME": "/tmp/demo/config",
            "UNIC_DEMO_FRAMES": "/tmp/demo/frames.json",
            "AWS_EC2_METADATA_DISABLED": "true",
            "AWS_CONFIG_FILE": "/tmp/demo/aws-config",
            "AWS_SHARED_CREDENTIALS_FILE": "/tmp/demo/aws-credentials",
        })


if __name__ == "__main__":
    unittest.main()
