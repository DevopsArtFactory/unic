# ECS rollout walkthrough

[30-second MP4](../assets/ecs-rollout.mp4) · [Animated preview](../assets/ecs-rollout.gif) · [Still image](../assets/ecs-rollout.png)

## What you are seeing

The production TUI renders synthetic ECS API results. The capture drives the actual service filter, Enter navigation, and rollout rendering in a 120×36 terminal. No live account, credentials, or customer resource data is used. Editorial captions and the terminal frame are added outside the TUI.

This is a deterministic rendered walkthrough, not a live terminal recording or a performance benchmark. Load commands are deliberately not executed. It does not introduce a public `unic demo` command (#383), connected cross-service investigation (#385), or report sharing (#386).

| Time | Action | What to notice |
| --- | --- | --- |
| 0–4s | Start in an ECS cluster's service list | `checkout-api` has fewer running tasks than desired. |
| 4–8s | `/`, type `checkout`, Enter | Filtering isolates one of four services. A second Enter opens its rollout. |
| 8–17s | Read rollout detail | Service `ACTIVE` is different from deployment `FAILED`; the fixture reports four failed tasks and a circuit-breaker reason. |
| 17–26s | Read the evidence | Inspect the image version and timestamped service events. They report failure to start; they do not establish the underlying root cause. |
| 26–30s | First-use prompt | Open the same screen with your own profile, region, cluster, and service. |

On a real account: `unic --profile your-profile --region us-east-1`. Skip the splash with Space, then press `P` from the context picker, search `ECS Browser`, and select ECS Browser & Exec → cluster → service. This uses your CLI-selected profile/region without requiring a saved context. Sign into SSO first if your profile requires it. `?` opens help; smaller terminals support `↓`/`PgDn` scrolling. Account permissions and data determine which results you can see.

## Reproduce the assets

Requirements: the repository's Go toolchain, Python 3.10+, and a monospace TrueType font (Menlo on macOS or DejaVu Sans Mono on Linux). Use a clean virtual environment outside the repository:

```bash
python3 -m venv /tmp/unic-demo-venv
/tmp/unic-demo-venv/bin/pip install Pillow==12.0.0 imageio-ffmpeg==0.6.0
/tmp/unic-demo-venv/bin/python docs/demo/render.py
```

`imageio-ffmpeg` supplies the encoder on supported platforms; alternatively point `IMAGEIO_FFMPEG_EXE` to an installed FFmpeg executable. Pass `--font /path/to/monospace.ttf` if automatic font selection fails. Use `--output-dir /tmp/unic-demo-preview` to preview without replacing checked-in media. The default output intentionally regenerates the three `docs/assets/ecs-rollout.*` files.

The script runs only `TestProductDemo` with the `productdemo` build tag, writes frames to a temporary directory, renders PNGs, and encodes a 1600×1200 H.264 MP4 plus a smaller GIF. It strips inherited AWS environment variables and points AWS config/credential paths at the temporary directory. The tagged test is not shipped in either binary. It never calls `Init` or runs returned commands, so fixture navigation cannot start AWS queries. It asserts the filter result, detail navigation, expected evidence, terminal dimensions, and 30-second duration. Times render in UTC; font rasterization/encoder differences can change media bytes between platforms.

The source is [product_demo_test.go](../../internal/app/product_demo_test.go) and [render.py](render.py). The existing tmux harness under `.agents/skills/unic-ux-review/scripts/` can capture an actual local terminal when tmux is available; this fixture renderer also works without it.

After changing the fixture or rendering code, regenerate and inspect the video from start to finish, confirm captions match the screen, check small-size legibility, and run:

```bash
/tmp/unic-demo-venv/bin/python -m unittest discover -s docs/demo -p 'test_*.py'
/tmp/unic-demo-venv/bin/python -O -m unittest discover -s docs/demo -p 'test_*.py'
make test
make build
```

## First-impression check — pending

The five-reader check in #384 requires real first-time participants. No responses or usability improvements are claimed by this PR. Show the README opening and video for 30 seconds, then ask each participant:

1. What problem would you use unic to solve?
2. What would you do first after installing it?
3. Was the difference between service status and deployment state clear?

Record anonymized observations and points of confusion in #384, not AWS details or personal information. Keep #384 open until this check is completed. The broader repeat-use and sharing study remains #389.
