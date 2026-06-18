#!/usr/bin/env python3
"""Capture reproducible unic TUI screenshots with an isolated config."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import shutil
import shlex
import subprocess
import sys
import tempfile
import time


WIDTH = 120
HEIGHT = 36


CONFIG_YAML = """current: dev-sandbox

defaults:
  region: ap-northeast-2

favorites:
  services:
    - ECS
    - EKS
    - RDS

ui:
  boot_splash: false
  last_boot_splash_version: 0.1.3

contexts:
  - name: dev-sandbox
    order: 10
    profile: dev-sandbox
    region: ap-northeast-2
    auth_type: credential

  - name: prod-readonly
    order: 20
    profile: prod-readonly
    region: us-east-1
    auth_type: assume_role
    role_arn: arn:aws:iam::123456789012:role/ReadOnly

  - name: platform-sso
    order: 30
    profile: platform-sso
    region: ap-northeast-2
    auth_type: sso
    sso_start_url: https://example.awsapps.com/start
    sso_account_id: "123456789012"
    sso_role_name: DeveloperRole
"""


def repo_root_from_script() -> Path:
    return Path(__file__).resolve().parents[4]


def run(cmd: list[str], cwd: Path, *, capture: bool = False) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        cmd,
        cwd=str(cwd),
        text=True,
        stdout=subprocess.PIPE if capture else None,
        stderr=subprocess.PIPE if capture else None,
        check=True,
    )


def describe_command_error(error: subprocess.CalledProcessError) -> str:
    cmd = error.cmd if isinstance(error.cmd, list) else [str(error.cmd)]
    lines = [f"command failed ({error.returncode}): {shlex.join(cmd)}"]
    output = (error.stderr or error.stdout or "").strip()
    if output:
        lines.append(output)
    return "\n".join(lines)


def write_fixture_config(config_root: Path) -> None:
    unic_dir = config_root / "unic"
    unic_dir.mkdir(parents=True, exist_ok=True)
    (unic_dir / "config.yaml").write_text(CONFIG_YAML, encoding="utf-8")
    cache = {"version": "0.1.3", "checked_at": "2999-01-01T00:00:00Z"}
    (unic_dir / "update-check.json").write_text(json.dumps(cache), encoding="utf-8")


def wait_for_capture(session: str, repo_root: Path, expected: str, timeout: float = 5.0) -> str:
    deadline = time.time() + timeout
    last = ""
    while time.time() < deadline:
        last = capture_pane(session, repo_root)
        if expected in last:
            return last
        time.sleep(0.15)
    raise TimeoutError(f"expected {expected!r} was not found in tmux output after {timeout:.1f}s")


def capture_pane(session: str, repo_root: Path) -> str:
    result = run(["tmux", "capture-pane", "-pt", session, "-J"], repo_root, capture=True)
    return result.stdout


def render_png(src: Path, dest: Path, width: int, height: int) -> bool:
    try:
        from PIL import Image, ImageDraw, ImageFont
    except ImportError as error:
        print(f"Pillow is not available, skipping PNG render: {error}", file=sys.stderr)
        return False

    font_candidates = [
        "/System/Library/Fonts/SFNSMono.ttf",
        "/System/Library/Fonts/Menlo.ttc",
        "/System/Library/Fonts/Supplemental/Andale Mono.ttf",
        "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
        "/usr/share/fonts/dejavu-sans-mono-fonts/DejaVuSansMono.ttf",
    ]
    font_path = next((p for p in font_candidates if Path(p).exists()), None)
    if font_path is None:
        print("No supported monospace font found, skipping PNG render", file=sys.stderr)
        return False

    try:
        font = ImageFont.truetype(font_path, 18)
    except OSError as error:
        print(f"Failed to load font {font_path}: {error}", file=sys.stderr)
        return False
    cell_w = int(round(font.getlength("M")))
    bbox = font.getbbox("Ag")
    line_h = int((bbox[3] - bbox[1]) * 1.45)
    pad_x = 20
    pad_y = 18

    colors = {
        "bg": (18, 22, 27),
        "bar": (43, 48, 56),
        "text": (220, 225, 232),
        "dim": (142, 150, 160),
        "accent": (109, 211, 255),
        "selected": (255, 214, 102),
        "border": (105, 113, 124),
        "green": (103, 232, 153),
    }

    lines = src.read_text(encoding="utf-8", errors="replace").splitlines()
    lines = (lines + [""] * height)[:height]
    img = Image.new("RGB", (width * cell_w + pad_x * 2, height * line_h + pad_y * 2), colors["bg"])
    draw = ImageDraw.Draw(img)

    def line_color(raw: str, index: int) -> tuple[int, int, int]:
        stripped = raw.strip()
        if stripped in {"Select Context", "Select AWS Service"}:
            return colors["accent"]
        if stripped.startswith(">") or stripped.startswith("│ >"):
            return colors["selected"]
        if raw.startswith("╭") or raw.startswith("╰") or raw.startswith("│"):
            return colors["border"]
        if "*" in raw and any(name in raw for name in ("ECS", "EKS", "RDS")):
            return colors["green"]
        if "favorites first" in raw or "No contexts" in raw:
            return colors["dim"]
        return colors["text"]

    for index, line in enumerate(lines):
        y = pad_y + index * line_h
        stripped = line.strip()
        if (index == 0 and stripped.startswith("[")) or "↑/↓" in line or stripped.startswith("esc:"):
            draw.rectangle((0, y - 2, img.width, y + line_h - 2), fill=colors["bar"])
        draw.text((pad_x, y), line, font=font, fill=line_color(line, index))

    try:
        img.save(dest)
    except OSError as error:
        print(f"Failed to save PNG {dest}: {error}", file=sys.stderr)
        return False
    return True


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", type=Path, default=repo_root_from_script())
    parser.add_argument("--out-dir", type=Path, default=None)
    parser.add_argument("--width", type=int, default=WIDTH)
    parser.add_argument("--height", type=int, default=HEIGHT)
    parser.add_argument("--skip-build", action="store_true")
    parser.add_argument("--keep-session", action="store_true")
    args = parser.parse_args()

    repo_root = args.repo_root.resolve()
    out_dir = args.out_dir or Path(tempfile.mkdtemp(prefix="unic-ux-review-", dir=tempfile.gettempdir()))
    out_dir.mkdir(parents=True, exist_ok=True)
    config_root = out_dir / "xdg-config"
    write_fixture_config(config_root)

    if shutil.which("tmux") is None:
        print("tmux is required for real TUI capture", file=sys.stderr)
        return 2

    if not args.skip_build:
        try:
            run(["make", "build"], repo_root)
        except subprocess.CalledProcessError as error:
            print(f"Build failed:\n{describe_command_error(error)}", file=sys.stderr)
            return 1

    binary = repo_root / "unic"
    if not binary.exists():
        print(f"missing binary: {binary}", file=sys.stderr)
        return 2

    session = f"unic_ux_{os.getpid()}"
    command = " ".join(
        [
            "TERM=xterm-256color",
            f"XDG_CONFIG_HOME={shlex.quote(str(config_root))}",
            "AWS_EC2_METADATA_DISABLED=true",
            shlex.quote(str(binary)),
        ]
    )

    created = False
    outputs: list[Path] = []
    try:
        try:
            run(
                ["tmux", "new-session", "-d", "-s", session, "-x", str(args.width), "-y", str(args.height), command],
                repo_root,
            )
        except subprocess.CalledProcessError as error:
            print(f"Failed to create tmux session:\n{describe_command_error(error)}", file=sys.stderr)
            return 1
        created = True

        try:
            context_text = wait_for_capture(session, repo_root, "Select Context")
        except (subprocess.CalledProcessError, TimeoutError) as error:
            print(f"Failed to capture context picker: {error}", file=sys.stderr)
            return 1
        context_path = out_dir / "unic-context.txt"
        context_path.write_text(context_text, encoding="utf-8")
        outputs.append(context_path)

        try:
            run(["tmux", "send-keys", "-t", session, "Escape"], repo_root)
            service_text = wait_for_capture(session, repo_root, "Select AWS Service")
        except (subprocess.CalledProcessError, TimeoutError) as error:
            print(f"Failed to capture service picker: {error}", file=sys.stderr)
            return 1
        service_path = out_dir / "unic-service.txt"
        service_path.write_text(service_text, encoding="utf-8")
        outputs.append(service_path)

        for txt_path in (context_path, service_path):
            png_path = txt_path.with_suffix(".png")
            if render_png(txt_path, png_path, args.width, args.height):
                outputs.append(png_path)

    finally:
        if created and not args.keep_session:
            subprocess.run(["tmux", "kill-session", "-t", session], cwd=str(repo_root), check=False)

    print(f"out_dir={out_dir}")
    for path in outputs:
        print(path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
