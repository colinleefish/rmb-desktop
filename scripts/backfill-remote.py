#!/usr/bin/env python3
"""Upload local rmb-desktop session turns to the remote rmb CS server.

Reads turns from the local SQLite DB and POSTs any missing turns to
RMB_URL (from ~/.rmb/config.yaml). Skips turns already on the server by
comparing created_at against the server's last_turn_at per session.

Usage:
  ./scripts/backfill-remote.py [--days N] [--dry-run]
"""
from __future__ import annotations

import argparse
import base64
import json
import os
import re
import sqlite3
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timedelta, timezone
from pathlib import Path

DEFAULT_DB = Path.home() / "Library/Application Support/rmb-desktop/data/rmb.db"
CONFIG_YAML = Path.home() / ".rmb/config.yaml"


def load_config(path: Path) -> tuple[str, str, str]:
    text = path.read_text()
    url = re.search(r'url:\s*"?([^"\n]+)"?', text)
    user = re.search(r'username:\s*([^\n]+)', text)
    pw = re.search(r'password:\s*"?([^"\n]+)"?', text)
    if not url or not user or not pw:
        raise SystemExit(f"could not parse client url/auth from {path}")
    return url.group(1).strip(), user.group(1).strip(), pw.group(1).strip()


def api_request(
    base_url: str,
    auth_header: str,
    method: str,
    path: str,
    body: bytes | None = None,
    timeout: int = 120,
) -> tuple[int, str]:
    req = urllib.request.Request(
        f"{base_url.rstrip('/')}{path}",
        data=body,
        method=method,
        headers={
            "Authorization": auth_header,
            "Content-Type": "application/json",
            "User-Agent": "rmb-backfill-remote",
        },
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, resp.read().decode("utf-8", errors="replace")
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode("utf-8", errors="replace")


def main() -> int:
    parser = argparse.ArgumentParser(description="Backfill local turns to remote rmb server")
    parser.add_argument("--days", type=int, default=2, help="look back N days (default: 2)")
    parser.add_argument("--db", type=Path, default=DEFAULT_DB, help="local rmb-desktop SQLite path")
    parser.add_argument("--config", type=Path, default=CONFIG_YAML, help="remote client config.yaml")
    parser.add_argument("--dry-run", action="store_true", help="print plan only, do not upload")
    args = parser.parse_args()

    if not args.db.is_file():
        raise SystemExit(f"local database not found: {args.db}")

    base_url, user, password = load_config(args.config)
    auth_header = "Basic " + base64.b64encode(f"{user}:{password}".encode()).decode()

    status, raw = api_request(base_url, auth_header, "GET", "/api/v1/browse/sessions?limit=500")
    if status != 200:
        raise SystemExit(f"list sessions failed: HTTP {status}: {raw[:500]}")

    server = {it["session_key"]: it for it in json.loads(raw).get("items", [])}

    cutoff_ms = int((datetime.now(timezone.utc) - timedelta(days=args.days)).timestamp() * 1000)
    db = sqlite3.connect(args.db)
    rows = db.execute(
        """
        SELECT s.session_key, t.created_at, t.messages_json
        FROM sessions s JOIN session_turns t ON t.session_id = s.id
        WHERE t.created_at >= ?
        ORDER BY s.session_key, t.created_at
        """,
        (cutoff_ms,),
    ).fetchall()

    to_upload: list[tuple[str, int, list[dict]]] = []
    for session_key, created_at, messages_json in rows:
        st = server.get(session_key)
        if st and st.get("last_turn_at"):
            last_ms = int(
                datetime.fromisoformat(st["last_turn_at"].replace("Z", "+00:00")).timestamp() * 1000
            )
            if created_at <= last_ms:
                continue
        try:
            messages = json.loads(messages_json)
        except json.JSONDecodeError as e:
            print(f"skip {session_key}: bad messages_json: {e}", file=sys.stderr)
            continue
        if not messages:
            continue
        to_upload.append((session_key, created_at, messages))

    print(f"remote: {base_url}")
    print(f"local turns in last {args.days}d: {len(rows)}")
    print(f"turns to upload (after last_turn_at filter): {len(to_upload)}")

    if args.dry_run:
        for session_key, created_at, messages in to_upload[:10]:
            ts = datetime.fromtimestamp(created_at / 1000, tz=timezone.utc).isoformat()
            print(f"  would upload {session_key} @ {ts} ({len(messages)} msgs)")
        if len(to_upload) > 10:
            print(f"  ... and {len(to_upload) - 10} more")
        return 0

    ok, fail = 0, 0
    for i, (session_key, created_at, messages) in enumerate(to_upload, 1):
        started_at = datetime.fromtimestamp(created_at / 1000, tz=timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
        payload = json.dumps({"started_at": started_at, "messages": messages}).encode("utf-8")
        path = f"/api/v1/sessions/{session_key}/upload"
        status, body = api_request(base_url, auth_header, "POST", path, payload)
        ts = datetime.fromtimestamp(created_at / 1000, tz=timezone.utc).isoformat()
        if status in (200, 202):
            ok += 1
            print(f"[{i}/{len(to_upload)}] ok {session_key} @ {ts}")
        else:
            fail += 1
            print(f"[{i}/{len(to_upload)}] FAIL {session_key} @ {ts}: HTTP {status} {body[:200]}", file=sys.stderr)
        time.sleep(0.05)

    print(f"done: uploaded={ok} failed={fail}")
    return 1 if fail else 0


if __name__ == "__main__":
    raise SystemExit(main())
