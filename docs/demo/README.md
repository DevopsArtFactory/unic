# ECS rollout walkthrough

[30-second MP4](../assets/ecs-rollout.mp4) · [Animated preview](../assets/ecs-rollout.gif) · [Still image](../assets/ecs-rollout.png)

## What you are seeing

The production TUI starts with an empty context picker, drives the Add Context wizard, and saves a synthetic profile/region to a temporary config. It then renders synthetic ECS API results, driving the actual service filter, Enter navigation, and rollout rendering in a 120×36 terminal. No live account, credentials, or customer resource data is used. Editorial captions and the terminal frame are added outside the TUI.

This is a deterministic rendered walkthrough, not a live terminal recording or a performance benchmark. The local context save is real but isolated to the test's temporary directory. Authentication and AWS evidence are simulated; AWS/load commands are never executed. The recording shows the shipped connected `i` investigation and sanitized `m` Markdown preview, but does not execute clipboard or export commands. The displayed export path is a fixed synthetic placeholder, not the machine's current directory. The public `unic demo` remains a separate, rollout-only experience.

| Time | Action | What to notice |
| --- | --- | --- |
| 0–4s | Empty context picker → `a` → credential | Reuse an existing AWS profile, without entering access keys. |
| 4–10s | Name, region, profile | Enter `demo-readonly`, `us-east-1`, and `demo-profile`; optional fields are skipped with Enter. |
| 10–14s | Review, save, return to picker | The new context is persisted locally. Selecting a real context can authenticate; this demo does not run authentication. |
| 14–17s | Cut to an ECS cluster's service list | Synthetic results: `checkout-api` has fewer running tasks than desired. |
| 17–20s | `/`, type `checkout`, Enter | Filtering isolates one of four services. A second Enter opens its rollout. |
| 20–23s | Read rollout detail | Service `ACTIVE` differs from deployment `FAILED`; the fixture reports failed tasks and a circuit-breaker reason. |
| 23–26s | `i` → page through related evidence | Group-wide target health and a stopped-task failure are observations, not proof of a common cause. |
| 26–28s | Read unavailable logs and nearby change | The synthetic task has an image-pull failure before startup; logs are unavailable. A nearby `UpdateService` event does not establish cause. |
| 28–30s | `m` → sanitized Markdown preview | Identifiers default to OFF. Copy/save is explicit and is not executed in this recording. |

On a real account: `unic --profile your-profile --region us-east-1`. Skip the splash with Space, press `a`, choose credential, and enter a name, region, and existing AWS profile. Skip unneeded optional fields with Enter, confirm the save, and select the saved context. Follow the [complete first-context instructions](../../README.md#add-your-first-context). There is no `unic context add` CLI subcommand; this is the TUI wizard. Then press `P`, search `ECS Browser`, and select ECS Browser & Exec → cluster → service. Alternatively, the `P` route works directly from the initial picker without a saved context. Sign into SSO first if your profile requires it. `?` opens help; smaller terminals support `↓`/`PgDn` scrolling. Account permissions and data determine which results you can see.

## Reproduce the assets

Requirements: the repository's Go toolchain, Python 3.10+, and a monospace TrueType font (Menlo on macOS or DejaVu Sans Mono on Linux). Use a clean virtual environment outside the repository:

```bash
python3 -m venv /tmp/unic-demo-venv
/tmp/unic-demo-venv/bin/pip install Pillow==12.0.0 imageio-ffmpeg==0.6.0
/tmp/unic-demo-venv/bin/python docs/demo/render.py
```

`imageio-ffmpeg` supplies the encoder on supported platforms; alternatively point `IMAGEIO_FFMPEG_EXE` to an installed FFmpeg executable. Pass `--font /path/to/monospace.ttf` if automatic font selection fails. Use `--output-dir /tmp/unic-demo-preview` to preview without replacing checked-in media. The default output intentionally regenerates the three `docs/assets/ecs-rollout.*` files.

The script runs only `TestProductDemo` with the `productdemo` build tag, writes frames to a temporary directory, renders PNGs, and encodes a 1600×1200 H.264 MP4 plus a smaller GIF. It strips inherited `AWS_*` and `UNIC_*` environment variables, then sets only fixture-specific overrides and points AWS config/credential paths at the temporary directory. The tagged test is not shipped in either binary. The only command executed by the TUI fixture is the inspected local `saveContext` command against a `t.TempDir()` config path; the renderer also runs the documented Go test and two FFmpeg subprocesses. It never calls `Init` or executes authentication/AWS, clipboard, or export commands. Assertions cover persisted context fields, selection/filter/detail navigation, investigation entry, paged evidence reachability, redacted preview, terminal dimensions, and 30-second duration. Times render in UTC; font rasterization/encoder differences can change media bytes between platforms.

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

Record anonymized observations and points of confusion in #384, not AWS details or personal information. Keep #384 open until this check is completed. The broader repeat-use and sharing study remains #389; use the [pilot protocol](../user-pilot.md) for consent, task scoring, and follow-up. No participant results have been collected here.
