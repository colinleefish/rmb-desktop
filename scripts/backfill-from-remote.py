#!/usr/bin/env python3
"""将远端 rmb CS 会话迁移到本地 rmb-desktop（并恢复原始时间戳）。

本脚本一次完成服务端 → 桌面端迁移的两步：

1. 通过 POST /api/v1/sessions/{key}/upload 上传远端 turns
2. 把本地 SQLite 的 created_at/updated_at 改写为远端提交时间
   （已发布的桌面版会忽略 upload 的 started_at，因此需要这一步）

兼容已分发的桌面版，无需升级应用。

模式：
  （默认迁移）上传本地缺失的会话，并写入远端时间戳
  --timestamps-only  仅刷新已有本地会话的时间戳

用法：
  ./scripts/backfill-from-remote.py --dry-run --skip-existing
  ./scripts/backfill-from-remote.py --skip-existing
  ./scripts/backfill-from-remote.py --skip-existing -y
  ./scripts/backfill-from-remote.py --timestamps-only
  ./scripts/backfill-from-remote.py --timestamps-only --dry-run --limit-sessions 3
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
import urllib.parse
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

CONFIG_YAML = Path.home() / ".rmb/config.yaml"
DEFAULT_LOCAL = "http://127.0.0.1:19019"
DEFAULT_DB = Path.home() / "Library/Application Support/rmb-desktop/data/rmb.db"
PAGE_LIMIT = 200


def load_remote_config(path: Path) -> tuple[str, str, str]:
    text = path.read_text()
    url = re.search(r'url:\s*"?([^"\n]+)"?', text)
    user = re.search(r'username:\s*([^\n]+)', text)
    pw = re.search(r'password:\s*"?([^"\n]+)"?', text)
    if not url or not user or not pw:
        raise SystemExit(f"无法从配置文件解析 url/用户名/密码: {path}")
    return url.group(1).strip(), user.group(1).strip(), pw.group(1).strip()


def resolve_remote(args: argparse.Namespace) -> tuple[str, str, str]:
    url = (args.remote_url or os.environ.get("RMB_URL") or "").strip()
    user = (args.username or os.environ.get("RMB_USERNAME") or "").strip()
    password = (args.password or os.environ.get("RMB_PASSWORD") or "").strip()
    if url and user and password:
        return url, user, password
    if args.config.is_file():
        return load_remote_config(args.config)
    raise SystemExit(
        "需要远端凭据：请传入 --remote-url/--username/--password "
        "（或环境变量 RMB_URL/RMB_USERNAME/RMB_PASSWORD），或提供 --config"
    )


def api_request(
    base_url: str,
    method: str,
    path: str,
    *,
    auth_header: str | None = None,
    body: bytes | None = None,
    timeout: int = 120,
    retries: int = 4,
) -> tuple[int, str]:
    headers = {
        "Content-Type": "application/json",
        "User-Agent": "rmb-backfill-from-remote",
    }
    if auth_header:
        headers["Authorization"] = auth_header
    url = f"{base_url.rstrip('/')}{path}"
    last_err: Exception | None = None
    for attempt in range(retries + 1):
        req = urllib.request.Request(url, data=body, method=method, headers=headers)
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                return resp.status, resp.read().decode("utf-8", errors="replace")
        except urllib.error.HTTPError as e:
            # Retry transient 5xx; return other HTTP errors to caller.
            if e.code >= 500 and attempt < retries:
                time.sleep(min(2 ** attempt, 8))
                last_err = e
                continue
            return e.code, e.read().decode("utf-8", errors="replace")
        except urllib.error.URLError as e:
            last_err = e
            if attempt < retries:
                time.sleep(min(2 ** attempt, 8))
                continue
            return 0, f"connection error: {e}"
    return 0, f"connection error: {last_err}"


def parse_jsonl_messages(raw: str) -> list[dict]:
    out: list[dict] = []
    for line in raw.splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            obj = json.loads(line)
        except json.JSONDecodeError:
            continue
        role = str(obj.get("role") or "").strip()
        content = obj.get("content")
        if content is None:
            content = ""
        if not isinstance(content, str):
            content = json.dumps(content, ensure_ascii=False)
        if not role:
            continue
        out.append({"role": role, "content": content})
    return out


def parse_rfc3339_ms(value: str | None) -> int | None:
    if not value:
        return None
    try:
        dt = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None
    return int(dt.timestamp() * 1000)


def fmt_ms(ms: int) -> str:
    return datetime.fromtimestamp(ms / 1000, tz=timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def started_at_from_ms(turn_ms: int) -> str:
    dt = datetime.fromtimestamp(turn_ms / 1000, tz=timezone.utc)
    return dt.strftime("%Y-%m-%dT%H:%M:%S.") + f"{turn_ms % 1000:03d}Z"


def list_all_sessions(base_url: str, auth_header: str | None = None) -> list[dict]:
    items: list[dict] = []
    offset = 0
    total = None
    while True:
        path = f"/api/v1/browse/sessions?limit={PAGE_LIMIT}&offset={offset}&sort=created&order=asc"
        status, raw = api_request(base_url, "GET", path, auth_header=auth_header)
        if status != 200:
            raise SystemExit(f"拉取会话列表失败: HTTP {status}: {raw[:500]}")
        payload = json.loads(raw)
        batch = payload.get("items") or []
        if total is None:
            total = int(payload.get("total") or 0)
        items.extend(batch)
        offset += len(batch)
        if not batch or (total is not None and offset >= total):
            break
    return items


def list_local_session_keys(local_url: str) -> set[str]:
    return {
        (item.get("session_key") or "").strip()
        for item in list_all_sessions(local_url)
        if (item.get("session_key") or "").strip()
    }


def local_turn_count(local_url: str, session_key: str) -> int:
    path = f"/api/v1/browse/sessions/{urllib.parse.quote(session_key, safe='')}"
    status, raw = api_request(local_url, "GET", path)
    if status == 404:
        return 0
    if status != 200:
        print(f"警告: 本地读取会话 {session_key} 失败: HTTP {status}", file=sys.stderr)
        return 0
    return len(json.loads(raw).get("turns") or [])


def open_db(db_path: Path) -> sqlite3.Connection:
    if not db_path.is_file():
        raise SystemExit(f"未找到本地数据库: {db_path}")
    db = sqlite3.connect(str(db_path))
    db.execute("PRAGMA busy_timeout = 5000")
    return db


def stamp_turn(db: sqlite3.Connection, turn_id: str, created_ms: int) -> None:
    db.execute(
        "UPDATE session_turns SET created_at = ? WHERE id = ?",
        (created_ms, turn_id),
    )


def stamp_session(
    db: sqlite3.Connection,
    session_key: str,
    created_ms: int | None,
    updated_ms: int | None,
) -> None:
    if created_ms is None and updated_ms is None:
        return
    if created_ms is not None and updated_ms is not None:
        db.execute(
            "UPDATE sessions SET created_at = ?, updated_at = ? WHERE session_key = ?",
            (created_ms, updated_ms, session_key),
        )
    elif created_ms is not None:
        db.execute(
            "UPDATE sessions SET created_at = ? WHERE session_key = ?",
            (created_ms, session_key),
        )
    else:
        db.execute(
            "UPDATE sessions SET updated_at = ? WHERE session_key = ?",
            (updated_ms, session_key),
        )


def sync_timestamps_for_session(
    db: sqlite3.Connection,
    session_key: str,
    remote_detail: dict,
    *,
    dry_run: bool,
) -> tuple[int, bool, str | None]:
    """Match local/remote turns by order; rewrite created_at. Returns (turns_updated, session_touched, skip_reason)."""
    remote_turns = remote_detail.get("turns") or []
    remote_session = remote_detail.get("session") or {}

    row = db.execute(
        "SELECT id, created_at, updated_at FROM sessions WHERE session_key = ?",
        (session_key,),
    ).fetchone()
    if not row:
        return 0, False, "not local"

    session_id, local_created, local_updated = row
    local_turns = db.execute(
        """
        SELECT id, created_at FROM session_turns
        WHERE session_id = ?
        ORDER BY created_at ASC, id ASC
        """,
        (session_id,),
    ).fetchall()

    if len(local_turns) != len(remote_turns):
        return 0, False, f"turn count local={len(local_turns)} remote={len(remote_turns)}"

    pairs: list[tuple[str, int, int]] = []
    for (turn_id, old_ms), remote_turn in zip(local_turns, remote_turns):
        new_ms = parse_rfc3339_ms(str(remote_turn.get("created_at") or ""))
        if new_ms is None:
            continue
        if int(old_ms) != new_ms:
            pairs.append((turn_id, int(old_ms), new_ms))

    sess_created = parse_rfc3339_ms(str(remote_session.get("created_at") or ""))
    sess_updated = parse_rfc3339_ms(str(remote_session.get("updated_at") or ""))
    if remote_turns:
        first_ms = parse_rfc3339_ms(str(remote_turns[0].get("created_at") or ""))
        last_ms = parse_rfc3339_ms(str(remote_turns[-1].get("created_at") or ""))
        if first_ms is not None:
            sess_created = first_ms
        if last_ms is not None:
            sess_updated = last_ms

    session_needs = False
    if sess_created is not None and int(local_created) != sess_created:
        session_needs = True
    if sess_updated is not None and int(local_updated) != sess_updated:
        session_needs = True

    if not pairs and not session_needs:
        return 0, False, None

    if dry_run:
        if pairs:
            print(f"  将回写 {len(pairs)} 条 turn 时间，例如 {fmt_ms(pairs[0][1])} -> {fmt_ms(pairs[0][2])}")
        if session_needs:
            print("  将回写会话 created_at/updated_at")
        return len(pairs), session_needs, None

    for turn_id, _old, new_ms in pairs:
        stamp_turn(db, turn_id, new_ms)
    if session_needs:
        stamp_session(db, session_key, sess_created, sess_updated)
    db.commit()
    return len(pairs), session_needs, None


def confirm_or_exit(prompt: str = "是否继续？[y/N] ") -> None:
    try:
        answer = input(prompt).strip().lower()
    except EOFError:
        print("已取消（无法读取交互输入）", file=sys.stderr)
        raise SystemExit(1)
    if answer not in ("y", "yes"):
        print("已取消")
        raise SystemExit(0)


def sum_turn_counts(sessions: list[dict]) -> int | None:
    """Sum turn_count from browse list rows when every row has it."""
    total = 0
    for s in sessions:
        if "turn_count" not in s or s.get("turn_count") is None:
            return None
        try:
            total += int(s["turn_count"])
        except (TypeError, ValueError):
            return None
    return total


def print_migrate_preflight(
    *,
    remote_url: str,
    local_url: str,
    db_path: Path | None,
    sessions: list[dict],
    skip_existing: bool,
    local_session_count: int | None,
    stamp_timestamps: bool,
) -> None:
    turns = sum_turn_counts(sessions)
    print()
    print("=== 预检信息 ===")
    print(f"远端服务:     {remote_url}")
    print(f"本地桌面:     {local_url}")
    if db_path is not None and stamp_timestamps:
        print(f"本地数据库:   {db_path}")
    if skip_existing and local_session_count is not None:
        print(f"本地已有会话: {local_session_count}")
    print(f"待导出并恢复的会话数: {len(sessions)}")
    if turns is not None:
        print(f"待导出并恢复的 turn 数: {turns}")
    print(f"回写时间戳:   {'是' if stamp_timestamps else '否'}")
    print("================")


def print_timestamps_preflight(
    *,
    remote_url: str,
    db_path: Path,
    session_count: int,
    scope: str,
) -> None:
    print()
    print("=== 预检信息 ===")
    print(f"远端服务:     {remote_url}")
    print(f"本地数据库:   {db_path}")
    print(f"范围:         {scope}")
    print(f"待恢复时间戳的会话数: {session_count}")
    print("================")


def run_timestamps_only(args: argparse.Namespace, remote_url: str, auth_header: str) -> int:
    db = open_db(args.db)
    if args.all_equal_count:
        rows = db.execute(
            "SELECT session_key FROM sessions ORDER BY created_at"
        ).fetchall()
        scope = "全部（turn 数一致时）"
    else:
        rows = db.execute(
            "SELECT session_key FROM sessions WHERE source = ? ORDER BY created_at",
            (args.source,),
        ).fetchall()
        scope = f"source={args.source}"

    keys = [r[0] for r in rows if r[0]]
    if args.session:
        want = {k.strip() for k in args.session}
        keys = [k for k in keys if k in want]
    if args.limit_sessions > 0:
        keys = keys[: args.limit_sessions]

    print_timestamps_preflight(
        remote_url=remote_url,
        db_path=args.db,
        session_count=len(keys),
        scope=scope,
    )
    if not args.dry_run and not args.yes:
        confirm_or_exit("确认恢复以上会话的时间戳？[y/N] ")
    elif args.dry_run:
        print("（演练模式 dry-run — 无需确认）")

    turns_updated = 0
    sessions_touched = 0
    skipped = 0
    failed = 0

    for i, session_key in enumerate(keys, 1):
        path = f"/api/v1/browse/sessions/{urllib.parse.quote(session_key, safe='')}"
        status, raw = api_request(remote_url, "GET", path, auth_header=auth_header)
        if status != 200:
            failed += 1
            print(f"[{i}/{len(keys)}] 失败 {session_key}: HTTP {status} {raw[:160]}", file=sys.stderr)
            continue

        detail = json.loads(raw)
        n, touched, reason = sync_timestamps_for_session(
            db, session_key, detail, dry_run=args.dry_run
        )
        if reason:
            skipped += 1
            print(f"[{i}/{len(keys)}] 跳过 {session_key}: {reason}")
            continue
        turns_updated += n
        if touched:
            sessions_touched += 1
        print(f"[{i}/{len(keys)}] {session_key}: 已回写 {n} 条 turn"
              + (" + 会话时间" if touched else ""))

    print(
        f"完成: 已更新 turn={turns_updated} 已更新会话={sessions_touched} "
        f"跳过={skipped} 失败={failed}"
        + ("（演练模式）" if args.dry_run else "")
    )
    return 1 if failed else 0


def run_migrate(args: argparse.Namespace, remote_url: str, auth_header: str) -> int:
    status, raw = api_request(args.local_url, "GET", "/api/v1/browse/overview")
    if status != 200:
        raise SystemExit(
            f"无法连接本地 rmbd: {args.local_url} (HTTP {status})。"
            "请先启动 RMB Desktop / rmbd。"
        )

    db: sqlite3.Connection | None = None
    if args.stamp_timestamps and not args.dry_run:
        db = open_db(args.db)

    local_keys: set[str] = set()
    if args.skip_existing:
        local_keys = list_local_session_keys(args.local_url)

    if args.session:
        sessions = [{"session_key": k} for k in args.session]
    else:
        sessions = list_all_sessions(remote_url, auth_header)

    remote_total_before_filter = len(sessions)
    if args.skip_existing:
        sessions = [
            s for s in sessions
            if (s.get("session_key") or "").strip() not in local_keys
        ]

    if args.limit_sessions > 0:
        sessions = sessions[: args.limit_sessions]

    print_migrate_preflight(
        remote_url=remote_url,
        local_url=args.local_url,
        db_path=args.db if args.stamp_timestamps else None,
        sessions=sessions,
        skip_existing=args.skip_existing,
        local_session_count=len(local_keys) if args.skip_existing else None,
        stamp_timestamps=args.stamp_timestamps,
    )
    if args.skip_existing:
        print(
            f"（已过滤仅远端会话: {len(sessions)} / {remote_total_before_filter}）"
        )
    if len(sessions) == 0:
        print("没有需要处理的会话")
        return 0
    if not args.dry_run and not args.yes:
        confirm_or_exit("确认将这些会话导出并恢复到本地桌面端？[y/N] ")
    elif args.dry_run:
        print("（演练模式 dry-run — 无需确认）")

    planned = 0
    uploaded = 0
    skipped = 0
    failed = 0
    turns_stamped = 0
    sessions_stamped = 0

    for i, sess in enumerate(sessions, 1):
        session_key = (sess.get("session_key") or "").strip()
        if not session_key:
            continue

        path = f"/api/v1/browse/sessions/{urllib.parse.quote(session_key, safe='')}"
        status, raw = api_request(remote_url, "GET", path, auth_header=auth_header)
        if status != 200:
            failed += 1
            print(f"[{i}/{len(sessions)}] 拉取失败 {session_key}: HTTP {status} {raw[:200]}", file=sys.stderr)
            continue

        detail = json.loads(raw)
        turns = detail.get("turns") or []
        already = 0 if args.dry_run else local_turn_count(args.local_url, session_key)

        print(
            f"[{i}/{len(sessions)}] {session_key}: "
            f"远端 {len(turns)} 条 turn，本地已有 {already} 条"
        )

        uploaded_ms: list[int] = []
        for idx, turn in enumerate(turns):
            if idx < already:
                skipped += 1
                continue

            messages = parse_jsonl_messages(turn.get("messages_jsonl") or "")
            if not messages:
                skipped += 1
                continue

            created_at = turn.get("created_at")
            turn_ms = parse_rfc3339_ms(str(created_at) if created_at else None)
            started_at = started_at_from_ms(turn_ms) if turn_ms is not None else None

            planned += 1
            if args.dry_run:
                print(
                    f"  将上传 turn {turn.get('turn_index', idx)} @ {created_at} "
                    f"（{len(messages)} 条消息）"
                    + (" + 回写时间戳" if args.stamp_timestamps and turn_ms is not None else "")
                )
                continue

            payload_obj: dict = {"messages": messages, "source": args.source}
            if started_at:
                payload_obj["started_at"] = started_at
            payload = json.dumps(payload_obj).encode("utf-8")
            up_path = f"/api/v1/sessions/{urllib.parse.quote(session_key, safe='')}/upload"
            status, body = api_request(args.local_url, "POST", up_path, body=payload)
            if status not in (200, 202):
                failed += 1
                print(
                    f"  上传失败 turn {turn.get('turn_index', idx)} @ {created_at}: "
                    f"HTTP {status} {body[:200]}",
                    file=sys.stderr,
                )
                time.sleep(args.sleep)
                continue

            uploaded += 1
            already += 1
            stamped = ""
            if db is not None and turn_ms is not None:
                try:
                    resp = json.loads(body)
                    turn_id = resp.get("turn_id")
                    if turn_id:
                        stamp_turn(db, turn_id, turn_ms)
                        db.commit()
                        turns_stamped += 1
                        uploaded_ms.append(turn_ms)
                        stamped = "（已回写时间戳）"
                except (json.JSONDecodeError, sqlite3.Error) as e:
                    print(f"  警告: 回写时间戳失败: {e}", file=sys.stderr)

            print(f"  成功 turn {turn.get('turn_index', idx)} @ {created_at}{stamped}")
            time.sleep(args.sleep)

        if db is not None and uploaded_ms:
            stamp_session(db, session_key, min(uploaded_ms), max(uploaded_ms))
            db.commit()
            sessions_stamped += 1

    print(
        f"完成: 计划={planned} 已上传={uploaded} 跳过={skipped} 失败={failed}"
        + (f" 已回写 turn={turns_stamped} 已回写会话={sessions_stamped}" if args.stamp_timestamps else "")
        + ("（演练模式）" if args.dry_run else "")
    )
    print("接下来: 等待本地流水线 L1→L2→L3 完成（可在 UI / browse/pipeline-health 查看）。")
    return 1 if failed else 0


def main() -> int:
    parser = argparse.ArgumentParser(
        description="将远端 rmb 会话迁移到本地桌面端（并恢复时间戳）"
    )
    parser.add_argument("--config", type=Path, default=CONFIG_YAML, help="可选的远端配置 ~/.rmb/config.yaml")
    parser.add_argument("--remote-url", default="", help="远端 rmb 地址（或 RMB_URL）")
    parser.add_argument("--username", default="", help="远端 basic-auth 用户名（或 RMB_USERNAME）")
    parser.add_argument("--password", default="", help="远端 basic-auth 密码（或 RMB_PASSWORD）")
    parser.add_argument("--local-url", default=DEFAULT_LOCAL, help="本地 rmbd 地址")
    parser.add_argument("--db", type=Path, default=DEFAULT_DB, help="本地 rmb-desktop SQLite 路径")
    parser.add_argument("--session", action="append", default=[], help="仅处理指定 session_key")
    parser.add_argument("--limit-sessions", type=int, default=0, help="最多处理多少个会话（0=全部）")
    parser.add_argument(
        "--skip-existing",
        action="store_true",
        help="仅迁移本地尚不存在的会话",
    )
    parser.add_argument(
        "--timestamps-only",
        action="store_true",
        help="不上传，仅把远端时间戳回写到本地数据库",
    )
    parser.add_argument(
        "--all-equal-count",
        action="store_true",
        help="配合 --timestamps-only：同步所有 turn 数一致的会话",
    )
    parser.add_argument(
        "--no-stamp-timestamps",
        action="store_true",
        help="迁移时不回写 SQLite 时间戳",
    )
    parser.add_argument(
        "--sleep",
        type=float,
        default=0.05,
        help="两次上传之间的间隔（秒）",
    )
    parser.add_argument("--dry-run", action="store_true", help="仅打印计划，不实际写入")
    parser.add_argument(
        "-y",
        "--yes",
        action="store_true",
        help="预检后跳过交互确认",
    )
    parser.add_argument("--source", default="migrate", help="写入本地会话的 source 标签")
    args = parser.parse_args()
    args.stamp_timestamps = not args.no_stamp_timestamps

    remote_url, user, password = resolve_remote(args)
    auth_header = "Basic " + base64.b64encode(f"{user}:{password}".encode()).decode()

    if args.timestamps_only:
        return run_timestamps_only(args, remote_url, auth_header)
    return run_migrate(args, remote_url, auth_header)


if __name__ == "__main__":
    raise SystemExit(main())
