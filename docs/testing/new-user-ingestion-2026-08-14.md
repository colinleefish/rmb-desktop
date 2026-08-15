# New-user memory ingestion test (fresh empty DB)

Date: 2026-08-15 (report filename per brief: 2026-08-14)
Repo: `colinleefish/rmb-desktop`

## Result

**SUCCESS** — a brand-new user's conversation was ingested end-to-end on a
freshly created empty database:

| Stage | Output |
|-------|--------|
| L1 extract | **9 atoms** |
| L2 build scenes | **7 scenes** |
| L3 distill memories | **6 memories** |

Pipeline-health funnel ended at `{"sessions":1,"t1_done":1,"t2_done":1,"t3_done":1}`.

## Hard constraints honored

- Real DB **read-only**: only `cp` (backup) was performed; never written back.
- Real config **read-only**: read to copy credentials; never modified.
- No daemon on the real port `19019` (the live daemon there was left alone).
- Temp daemon used `127.0.0.1:19097`.
- `CGO_ENABLED=1` and `-tags sqlite_fts5` used for build/test.
- `api_key` values were never printed; redacted below.

## Temp paths

All test artifacts live under `/tmp/rmb-ingest/`:

| Path | Purpose |
|------|---------|
| `/tmp/rmb-ingest/backup-real.db` | read-only copy of the real DB |
| `/tmp/rmb-ingest/config.yaml` | temp config (credentials copied from real, redacted) |
| `/tmp/rmb-ingest/fresh.db` | fresh empty DB (migrations applied on first open) |
| `/tmp/rmb-ingest/rmbd` | built daemon binary |
| `/tmp/rmb-ingest/run.log` | daemon log |
| `/tmp/rmb-ingest/insert.sql` | the insertion SQL used |
| `/tmp/rmb-ingest/daemon.pid` | temp daemon PID |

## 1. Backup

```sh
mkdir -p /tmp/rmb-ingest
cp "$HOME/Library/Application Support/rmb-desktop/data/rmb.db" /tmp/rmb-ingest/backup-real.db
```

## 2. Build

```sh
cd /Users/liguanghui/Virginia/colinleefish/rmb-desktop
CGO_ENABLED=1 go build -tags sqlite_fts5 -o /tmp/rmb-ingest/rmbd ./cmd/rmbd
```

## 3–4. Fresh DB + temp config

Fresh DB path: `/tmp/rmb-ingest/fresh.db` (created empty; `db.Open` ran all
migrations on first open).

Temp config: copied the real `config.yaml`, then applied overrides with `sed` /
`perl` (keys kept, never echoed):

```yaml
addr: 127.0.0.1:19097
db_path: /tmp/rmb-ingest/fresh.db
llm:
    api_base: https://stargate.youxi123.com
    api_key: ***REDACTED***
    model: deepseek-v4-flash
    timeout: 120s
embed:
    api_base: https://open.bigmodel.cn/api/paas/v4
    api_key: ***REDACTED***
    model: embedding-3
    dimensions: 1024
pipeline:
    # ... real values kept ...
    l2_delay_after_l1: 0s
    l2_min_concurrency: 4
    l2_max_concurrency: 8
launch_at_login: false
```

## 5. Start + verify

```sh
cd /tmp/rmb-ingest
nohup ./rmbd serve --config /tmp/rmb-ingest/config.yaml > run.log 2>&1 &
```

- `GET /api/v1/version` → `{"commit":"dev","version":"0.1.20"}`
- Migrations ran to goose version **9**:
  `SELECT max(version_id) FROM goose_db_version;` → `9`
- `GET /healthz` → `{"status":"ok","sqlite":"3.53.4","vec":"v0.1.6","vec_ok":true,...}`

## 6. Schema learned

`sessions`: `id`, `session_key`, `abstract`, `created_at`, `updated_at`, `source`.
`session_turns`: `id`, `session_id`, `messages_json`, `created_at`, `l1_status`, `l1_extracted_at`.
`pipeline_state`: `session_id`, `l1_status`, `l2_status`, `l3_status`, `updated_at`,
`l1_advanced_at`, `l2_advanced_at`, `l3_advanced_at`, `l1_turns_since_advanced`,
`warmup_threshold`, `l1_last_error`, `l2_last_error`, `l3_last_error`,
`l1_started_at`, `l2_started_at`, `l3_started_at`.
`atoms`: `id`, `session_id`, `category`, `priority`, `scene_name`, `slug`,
`content`, `source_turn_ids`, `embedding`, `created_at`, `updated_at`.
`scenes`: `id`, `session_id`, `display_name`, `abstract`, `body`,
`source_atoms`, `embedding`, `created_at`, `updated_at`.
`memories`: `id`, `uri`, `category`, `slug`, `version`, `superseded_at`,
`abstract`, `body`, `source_scene_uris`, `source_correction_uris`, `embedding`,
`created_at`, `updated_at`.

## 7. Inserted conversation

Exact SQL (`/tmp/rmb-ingest/insert.sql`; `${NOW}` = epoch ms at insert time,
turns staggered −120s/−60s/−30s):

```sql
BEGIN;
INSERT INTO sessions (id, session_key, source, abstract, created_at, updated_at)
VALUES ('00000000-0000-4000-8000-000000000001', 'alice-new-user-test', 'cursor', NULL, ${NOW}, ${NOW});

INSERT INTO pipeline_state (session_id, l1_status, l2_status, l3_status, l1_turns_since_advanced, warmup_threshold, updated_at)
VALUES ('00000000-0000-4000-8000-000000000001', 'pending', 'idle', 'idle', 0, 1, ${NOW});

INSERT INTO session_turns (id, session_id, messages_json, created_at, l1_status)
VALUES ('00000000-0000-4000-8000-000000000101', '00000000-0000-4000-8000-000000000001',
'[{"role":"user","content":"Hi, I am Alice Chen, a backend engineer at Nova Labs. I work mostly in Go and PostgreSQL."},{"role":"assistant","content":"Nice to meet you, Alice. I will remember that."}]',
${T1}, 'pending');

INSERT INTO session_turns (id, session_id, messages_json, created_at, l1_status)
VALUES ('00000000-0000-4000-8000-000000000102', '00000000-0000-4000-8000-000000000001',
'[{"role":"user","content":"Right now I am leading the billing service rewrite, and I maintain an internal CLI tool called nova-cli."},{"role":"assistant","content":"Got it. Billing service rewrite and nova-cli, noted."}]',
${T2}, 'pending');

INSERT INTO session_turns (id, session_id, messages_json, created_at, l1_status)
VALUES ('00000000-0000-4000-8000-000000000103', '00000000-0000-4000-8000-000000000001',
'[{"role":"user","content":"Please always address me as Alice, prefer short answers with bullet points, and show code examples in Go."},{"role":"assistant","content":"Understood. I will address you as Alice and keep answers short."}]',
${T3}, 'pending');
COMMIT;
```

The 3 user messages fed to extraction:

1. "Hi, I am Alice Chen, a backend engineer at Nova Labs. I work mostly in Go and PostgreSQL."
2. "Right now I am leading the billing service rewrite, and I maintain an internal CLI tool called nova-cli."
3. "Please always address me as Alice, prefer short answers with bullet points, and show code examples in Go."

## 8. Pipeline run

L1 picked the session up automatically (`l1_status='pending'` + unextracted turns).
Daemon log timings:

- `l1 extracted session=alice-new-user-test turns=3 atoms=9` (llm `extract_atoms`, 261 ms, HTTP 200)
- `l2 built scenes session=alice-new-user-test count=7` (llm `build_scenes` 53.2 s, HTTP 200; `session_abstract` 3.4 s, HTTP 200)
- L3 distilled memories (llm `distill_memory`, 1.8 s, HTTP 200)

Final `pipeline_state` row: `l1_status=idle`, `l2_status=idle`, `l3_status=idle`,
all `*_last_error` NULL.

### pipeline-health (final)

```json
{"distillation_enabled":true,"tracked_sessions":1,
 "stages":{"t1":{"pending":0,"running":0,"failed":0,"idle":1,"waiting":0},
           "t2":{"pending":0,"running":0,"failed":0,"idle":1,"waiting":0},
           "t3":{"pending":0,"running":0,"failed":0,"idle":1,"waiting":0}},
 "funnel":{"sessions":1,"t1_done":1,"t2_done":1,"t3_done":1},
 "problems":[]}
```

## 9. Extracted atoms (9)

| category | priority | scene_name | slug | content |
|----------|----------|------------|------|---------|
| profile | 90 | identity | | The user's name is Alice Chen. |
| profile | 80 | career | | The user is a backend engineer at Nova Labs. |
| profile | 60 | tech-stack | | The user primarily works with Go and PostgreSQL. |
| entities | 60 | company | nova-labs | Nova Labs is the company where the user works as a backend engineer. |
| entities | 60 | tools | nova-cli | nova-cli is an internal CLI tool maintained by the user. |
| profile | 60 | projects | | The user is leading the billing service rewrite at Nova Labs. |
| preferences | 90 | ai-behavior | address-user | The user prefers to be addressed as Alice. |
| preferences | 80 | ai-behavior | answer-style | The user prefers short answers formatted with bullet points. |
| preferences | 80 | ai-behavior | code-language | The user prefers code examples in Go. |

## Distilled scenes (7)

`AI Behavior Preferences`, `Projects`, `Career`, `Tools`, `Identity`,
`Company`, `Tech Stack`.

## Distilled memories (6)

`rmb://profile`, `rmb://entities/nova-labs`, `rmb://entities/nova-cli`,
`rmb://preferences/address-user`, `rmb://preferences/answer-style`,
`rmb://preferences/code-language`.

Profile memory abstract:
"Alice Chen: backend engineer at Nova Labs, Go/PostgreSQL, leading billing service rewrite."

## Issues / notes

1. **No LLM timeouts or failures in the final clean run.** All LLM calls
   returned HTTP 200 against `stargate.youxi123.com`. One call was slow:
   `build_scenes` took **53.2 s** (within the 120 s timeout), the others were
   < 3.5 s. This is an external gateway-latency observation, not a failure.
2. **Test-harness race on first attempt (not a product failure).** During the
   first attempt I accidentally started a second `rmbd` against the same
   `fresh.db`; both L1 workers processed the same session before I noticed,
   producing duplicate atoms (15 instead of 9). I stopped everything and re-ran
   cleanly with a single daemon, which is the authoritative result above. The
   product assumes one daemon per DB file; there is no cross-process lock.
3. **PID bookkeeping.** `nohup ./rmbd ... &` reports the `nohup` wrapper PID,
   not the `rmbd` PID; the daemon was stopped via the actual `rmbd` PID from
   `pgrep`/`lsof`. The temp daemon was fully stopped and `19097` released
   before finishing.

## Cleanup

- Temp daemon killed; port `127.0.0.1:19097` confirmed free.
- `fresh.db`, `config.yaml`, `run.log`, `insert.sql` left in `/tmp/rmb-ingest/`
  for inspection.
- Real DB and real config untouched (real daemon on `19019` left running).
