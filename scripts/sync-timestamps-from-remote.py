#!/usr/bin/env python3
"""已弃用的兼容入口 — 请改用 backfill-from-remote.py --timestamps-only。"""
from __future__ import annotations

import runpy
import sys
from pathlib import Path

print(
    "提示: sync-timestamps-from-remote.py 已合并进 backfill-from-remote.py；"
    "正在转发为 --timestamps-only",
    file=sys.stderr,
)

script = Path(__file__).with_name("backfill-from-remote.py")
sys.argv = [str(script), "--timestamps-only", *sys.argv[1:]]
runpy.run_path(str(script), run_name="__main__")
