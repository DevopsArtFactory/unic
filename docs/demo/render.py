#!/usr/bin/env python3
"""Render the real unic views from an offline, test-only ECS walkthrough."""

import argparse
import json
import math
import os
from pathlib import Path
import re
import subprocess
import tempfile

from PIL import Image, ImageDraw, ImageFont
import imageio_ffmpeg


ROOT = Path(__file__).resolve().parents[2]
SGR = re.compile(r"\x1b\[([0-9;]*)m")
BG = "#0b111b"
TEXT = "#e1e8f0"
MUTED = "#9aaec3"
ACCENT = "#65d5ce"
BASE = ["#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#c0c0c0",
        "#808080", "#ff0000", "#00ff00", "#ffff00", "#0000ff", "#ff00ff", "#00ffff", "#ffffff"]


def indexed_color(n):
    if not 0 <= n <= 255:
        raise ValueError("SGR color index must be between 0 and 255")
    if n < 16:
        return BASE[n]
    if n >= 232:
        return (8 + (n - 232) * 10,) * 3
    n -= 16
    levels = [0, 95, 135, 175, 215, 255]
    return levels[n // 36], levels[(n // 6) % 6], levels[n % 6]


def draw_terminal(draw, text, font, x, y):
    """Preserve SGR colors/underlines from Lip Gloss; no heuristic recoloring."""
    cell = font.getlength("M")
    for row, line in enumerate(text.splitlines()):
        fg, bg, underline = TEXT, None, False
        column = 0
        parts = SGR.split(line)
        for index, part in enumerate(parts):
            if index % 2 == 0:
                for char in part:
                    px, py = x + column * cell, y + row * 22
                    if bg is not None:
                        draw.rectangle((px, py, px + cell, py + 22), fill=bg)
                    draw.text((px, py), char, font=font, fill=fg)
                    if underline:
                        draw.line((px, py + 21, px + cell, py + 21), fill=fg)
                    column += 1
                continue
            codes = [int(v or 0) for v in part.split(";")]
            i = 0
            while i < len(codes):
                code = codes[i]
                if code == 0:
                    fg, bg, underline = TEXT, None, False
                elif code == 4:
                    underline = True
                elif code == 24:
                    underline = False
                elif code == 39:
                    fg = TEXT
                elif code == 49:
                    bg = None
                elif 30 <= code <= 37:
                    fg = BASE[code - 30]
                elif 40 <= code <= 47:
                    bg = BASE[code - 40]
                elif code in (38, 48):
                    if i + 1 >= len(codes):
                        raise ValueError("truncated SGR color sequence")
                    if codes[i + 1] == 5:
                        if i + 2 >= len(codes):
                            raise ValueError("truncated SGR indexed color")
                        color = indexed_color(codes[i + 2])
                        i += 2
                    elif codes[i + 1] == 2:
                        if i + 4 >= len(codes):
                            raise ValueError("truncated SGR RGB color")
                        color = tuple(codes[i + 2:i + 5])
                        if any(channel > 255 for channel in color):
                            raise ValueError("SGR RGB channels must be between 0 and 255")
                        i += 4
                    else:
                        raise ValueError("unsupported SGR color")
                    if code == 38:
                        fg = color
                    else:
                        bg = color
                i += 1


def render(frame, font_path):
    image = Image.new("RGB", (1600, 1200), BG)
    draw = ImageDraw.Draw(image)
    font = ImageFont.truetype(font_path, 18)
    small = ImageFont.truetype(font_path, 17)
    heading = ImageFont.truetype(font_path, 33)
    draw.text((60, 29), "unic", font=heading, fill=ACCENT)
    draw.text((180, 41), "AWS operations, in your terminal.", font=small, fill=MUTED)
    draw.text((60, 91), frame["chapter"], font=small, fill=ACCENT)
    draw.text((60, 120), frame["caption"], font=font, fill=TEXT)
    draw.rounded_rectangle((56, 174, 1544, 1064), radius=16, fill="#101923", outline="#2a3b4e", width=2)
    for i, color in enumerate(("#e37478", "#e8bd6e", "#6fc1a1")):
        draw.ellipse((79 + i * 22, 195, 89 + i * 22, 205), fill=color)
    draw.text((165, 187), "unic / ECS rollout", font=small, fill=MUTED)
    draw.text((1080, 187), "SYNTHETIC DATA / REAL TUI VIEWS", font=small, fill=MUTED)
    draw.line((57, 221, 1543, 221), fill="#2a3b4e", width=1)
    draw_terminal(draw, frame["ansi"], font, 84, 238)
    draw.text((60, 1091), frame["keys"], font=font, fill=ACCENT)
    draw.text((60, 1144), "github.com/DevopsArtFactory/unic", font=small, fill=MUTED)
    draw.text((1035, 1144), "Offline fixture | No AWS calls", font=small, fill=MUTED)
    return image


def capture_environment(tmp, frames_path):
    env = {k: v for k, v in os.environ.items() if not k.startswith("AWS_")}
    env.update(UNIC_DEMO_FRAMES=str(frames_path), TZ="UTC",
               AWS_EC2_METADATA_DISABLED="true", XDG_CONFIG_HOME=str(tmp / "config"),
               AWS_CONFIG_FILE=str(tmp / "aws-config"),
               AWS_SHARED_CREDENTIALS_FILE=str(tmp / "aws-credentials"))
    return env


def validate_duration(frames):
    durations = [f["seconds"] for f in frames]
    total = sum(durations)
    if any(not math.isfinite(d) or d <= 0 for d in durations) or abs(total - 30) >= 0.01:
        raise ValueError(f"expected 30 seconds of positive finite frames, got {total}")


def select_poster(frames):
    for frame in frames:
        if frame["chapter"] == "02 / INSPECT THE ROLLOUT":
            return frame
    raise ValueError("missing rollout chapter for the still image")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-dir", type=Path, default=ROOT / "docs/assets")
    parser.add_argument("--font", type=Path, help="Path to a monospace TrueType font")
    args = parser.parse_args()
    candidates = [args.font, Path("/System/Library/Fonts/Menlo.ttc"),
                  Path("/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf")]
    font = next((p for p in candidates if p and p.is_file()), None)
    if font is None:
        parser.error("provide --font /path/to/monospace.ttf")
    out = args.output_dir.resolve()
    out.mkdir(parents=True, exist_ok=True)
    ffmpeg = imageio_ffmpeg.get_ffmpeg_exe()
    with tempfile.TemporaryDirectory(prefix="unic-product-demo-") as temporary:
        tmp = Path(temporary)
        frames_path = tmp / "frames.json"
        env = capture_environment(tmp, frames_path)
        subprocess.run(["go", "test", "-tags=productdemo", "./internal/app", "-run", "^TestProductDemo$", "-count=1"],
                       cwd=ROOT, env=env, check=True)
        frames = json.loads(frames_path.read_text(encoding="utf-8"))
        validate_duration(frames)
        poster = select_poster(frames)
        timeline = []
        for i, frame in enumerate(frames):
            image = render(frame, str(font))
            image.save(tmp / f"frame-{i:02d}.png")
            if frame is poster:
                image.save(out / "ecs-rollout.png")
            timeline.extend([f"file 'frame-{i:02d}.png'", f"duration {frame['seconds']}"])
        timeline.append(f"file 'frame-{len(frames) - 1:02d}.png'")
        (tmp / "timeline.txt").write_text("\n".join(timeline) + "\n", encoding="utf-8")
        video = out / "ecs-rollout.mp4"
        subprocess.run([ffmpeg, "-y", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", str(tmp / "timeline.txt"),
                        "-t", "30", "-vf", "fps=30", "-c:v", "libx264", "-preset", "slow", "-crf", "18",
                        "-pix_fmt", "yuv420p", "-movflags", "+faststart", str(video)], check=True)
        subprocess.run([ffmpeg, "-y", "-loglevel", "error", "-i", str(video), "-filter_complex",
                        "fps=10,scale=1200:-1:flags=lanczos,split[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse=dither=none",
                        "-loop", "0", str(out / "ecs-rollout.gif")], check=True)
        print(f"Rendered 30-second MP4, GIF and PNG in {out}")


if __name__ == "__main__":
    main()
