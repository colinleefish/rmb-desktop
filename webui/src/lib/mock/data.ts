/**
 * Dev-only deterministic mock dataset for the full-app mock mode (see
 * lib/mockMode.ts). Content mirrors the real product's domain (sessions →
 * atoms → scenes → memories pyramid, skills, corrections) so every page has
 * believable data to render. Seeded PRNG keeps the dataset stable across
 * reloads/HMR. No browser APIs — the node selftest (mock/selftest.ts)
 * imports this module directly.
 */
import { DEFAULT_PIPELINE } from "../pipelineDefaults";
import type {
  AtomRow,
  ConfigView,
  CorrectionRow,
  MemoryRow,
  Overview,
  PipelineHealth,
  PipelineState,
  SceneRow,
  SessionRow,
  SkillDetail,
  SkillRow,
  TurnRow,
} from "../types";

const NOW = Date.now();
const DAY = 86_400_000;
const HOUR = 3_600_000;

/** mulberry32 — tiny seeded PRNG. Rebuilt data re-seeds it, so every
 *  buildData() call (dev server, HMR, selftest) yields identical datasets. */
const PRNG_SEED = 20260912;
let prngState = PRNG_SEED;

function rand(): number {
  prngState = (prngState + 0x6d2b79f5) | 0;
  let t = Math.imul(prngState ^ (prngState >>> 15), 1 | prngState);
  t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
}

const between = (min: number, max: number) => min + Math.floor(rand() * (max - min + 1));

const ago = (days: number, jitterHours = 6): number =>
  NOW - days * DAY - Math.floor(rand() * jitterHours) * HOUR;

const iso = (ms: number): string => new Date(ms).toISOString();

/**
 * Pipeline stage of a session — drives t1/t2/t3 statuses, atoms and scenes.
 * "captured" = raw turns only; "t2done"/"done" have atoms/scenes.
 */
type Stage =
  | "captured"
  | "t1running"
  | "t1done"
  | "t2running"
  | "t2done"
  | "t3running"
  | "done"
  | "failed1"
  | "failed2";

interface Topic {
  source: string;
  slug: string;
  title: string;
  abstract: string;
  stage: Stage;
  daysAgo: number;
  open?: boolean;
  /** Hand-written dialogue; [user, assistant, user, ...] */
  talk: string[];
}

const TOPICS: Topic[] = [
  {
    source: "zcode",
    slug: "zcode-hooks-recall",
    title: "zcode hook-submit wiring",
    abstract: "Wired the zcode Stop hook to rmb hook-submit and verified turns land in rmbd.",
    stage: "done",
    daysAgo: 0.2,
    open: true,
    talk: [
      "The zcode integration should push every Stop event into rmbd via hook-submit. Where does the payload shape come from?",
      "The hook payload is built in internal/hook — it marshals the transcript tail as NDJSON and POSTs to /api/v1/turns. The F07 payload was inferred from the docs, which is exactly why #61/#62 happened.",
      "Right — let's add a contract test that feeds a real zcode transcript through the hook builder and asserts the stored turns match.",
      "Agreed. I'll pin the payload shape with a golden file so future zcode upgrades can't silently drift it.",
    ],
  },
  {
    source: "zcode",
    slug: "webui-mock-mode",
    title: "webui full-app mock mode",
    abstract: "Added VITE_MOCK full-app mock mode so the SPA runs with no daemon for fast frontend tweaks.",
    stage: "t2running",
    daysAgo: 0.1,
    open: true,
    talk: [
      "I want to tweak the frontend without booting the daemon. Can we serve the whole SPA from mock data?",
      "There are already per-feature mocks (setup, pipeline, onboarding) but Sessions/Memories/Skills hit /api directly. I'll add a central mock router behind VITE_MOCK and seed it with a realistic dataset.",
      "Make it deterministic — same data every reload — and give me a badge so I can tell mock and real apart at a glance.",
    ],
  },
  {
    source: "claude-code",
    slug: "retrieval-audit-p0",
    title: "Memory retrieval audit P0s",
    abstract: "Worked the retrieval audit remediation plan: recency decay and FTS/vector fusion P0 fixes.",
    stage: "t2done",
    daysAgo: 0.6,
    open: true,
    talk: [
      "The audit says recency is basically ignored in fusion. What are the P0s?",
      "Three P0s: apply time-decay before fusion, fix the FTS ranking normalization, and stop double-counting corrections in the vector lane. The time-traveler timeline doc has the evidence.",
      "Then let's re-run the recall eval gate after each fix so we can see the delta per change.",
    ],
  },
  {
    source: "codex",
    slug: "modern-go-idioms",
    title: "Modern Go idioms sweep",
    abstract: "Repo-wide sweep to errors.Is, range-int loops and sync.WaitGroup.Go; merged via PR #69.",
    stage: "done",
    daysAgo: 1.1,
    talk: [
      "Sweep the repo for pre-1.22 idioms: errors.Is over error compares, range-over-int, WaitGroup.Go.",
      "Sixty-nine files touched, all mechanical. make check stayed green the whole way — vet is the gate of record here.",
      "Merge with a merge commit, no squash — the parallel-work plan requires traceable history.",
    ],
  },
  {
    source: "cursor",
    slug: "b04-flaky-daemon-log-poll",
    title: "B04 flaky daemon log poll",
    abstract: "TestSpawnedDaemonStdioIsLogFdNotPipe flakes under load; 5s file-poll window times out.",
    stage: "failed2",
    daysAgo: 0.4,
    open: true,
    talk: [
      "make check flaked again on TestSpawnedDaemonStdioIsLogFdNotPipe — third time.",
      "Root cause: the test waits a fixed 5s for the fake daemon's daemon-mark write; under ~30 parallel packages the child lands past the deadline. The *os.File assertion itself never failed.",
      "Make detection tolerant of an 8s delayed write and keep the fd assertion untouched. I proved it with RMB_TEST_DAEMON_WRITE_DELAY=8s — deterministic FAIL at 5.0s.",
    ],
  },
  {
    source: "zcode",
    slug: "pr-ci-gate-fixture",
    title: "Hermetic CI fixture for pr-check",
    abstract: "fakeRMBHome fixture makes the CI run hermetic; PR #67 green on Linux in 2m10s.",
    stage: "t1done",
    daysAgo: 0.8,
    talk: [
      "CI can't assume a real ~/.rmb. What does the fixture fake?",
      "fakeRMBHome redirects HOME to a temp dir with a minimal config and empty DB — every test that touches rmbd paths now runs hermetic, locally and on Linux.",
    ],
  },
  {
    source: "claude-code",
    slug: "distill-pyramid-tuning",
    title: "Distillation pyramid tuning",
    abstract: "Tuned l1_every_n, idle windows and batch caps so T2 latency dropped without starveing T3.",
    stage: "t2done",
    daysAgo: 1.4,
    talk: [
      "T2 lags hours behind T1. Which knobs matter?",
      "l1_every_n=8 and the 600s idle window gate T1 flushes; l2_delay_after_l1=90s spaces the fan-out. Dropping l1_max_turns_per_batch to 8 kept prompts under the context ceiling.",
      "Watch T3 starvation though — the backlog only drains every 300s.",
    ],
  },
  {
    source: "cursor",
    slug: "onboarding-wizard-ux",
    title: "Onboarding wizard UX pass",
    abstract: "Three-step onboarding (language → models → agents) with skippable agent step and demo mode.",
    stage: "t1done",
    daysAgo: 1.8,
    talk: [
      "First run should not show an empty dashboard. Gate the app behind onboarding until the marker file exists.",
      "Three steps: language, models (with live connection test), agents. The agent step is skippable and there's a VITE_MOCK_ONBOARDING demo path for UI work.",
    ],
  },
  {
    source: "claude-code",
    slug: "tauri-to-go-shell",
    title: "Tauri → Go shell migration",
    abstract: "Replaced the Tauri shell with a Go menu-bar app; plan moved to plan/done after ship.",
    stage: "done",
    daysAgo: 3,
    talk: [
      "Tauri doubled our bundle and we never used the webview glue. Can rmb-app be pure Go?",
      "Yes — systray + embedded webui served by rmbd. The migration plan is complete and archived under plan/done; the old app/ files must never come back.",
    ],
  },
  {
    source: "zcode",
    slug: "fts-vector-hybrid",
    title: "Hybrid FTS + vector recall",
    abstract: "Fused FTS5 ranking with pgvector cosine scores; reciprocal-rank fusion beat either lane alone.",
    stage: "t2done",
    daysAgo: 2.2,
    talk: [
      "Pure FTS misses paraphrases, pure vectors miss exact identifiers. Fuse them?",
      "Reciprocal-rank fusion at k=60 works well. Normalize FTS bm25 ranks per-query first, then fuse with the cosine lane and apply the recency decay last.",
    ],
  },
  {
    source: "workbuddy",
    slug: "pgpour-cdc-lag",
    title: "pgpour CDC replication lag",
    abstract: "starlink-dev-all-in-one slot lag spiked to 40GB; traced to a long-running tx blocking consumption.",
    stage: "t1done",
    daysAgo: 2.6,
    talk: [
      "The CDC slot is lagging 40GB and Kafka topics are starving. Where do I look first?",
      "Check pg_replication_slots for active_pid and restart_lsn, then pg_stat_activity for the oldest xact holding xmin. A stuck batch job was pinning the slot for 6 hours.",
    ],
  },
  {
    source: "cursor",
    slug: "starmap-pipeline-ab",
    title: "Starmap AB-test pipeline",
    abstract: "Mapped solutions → ways → tags → mini bundles flow and documented the Starmap dependency order.",
    stage: "t2done",
    daysAgo: 3.4,
    talk: [
      "Document the AB pipeline: what depends on what?",
      "Solutions own ways; tags attach to ways; mini bundles snapshot ways+tags. Starmap consumes the bundle snapshot, so tag edits must re-snapshot before the next ship window.",
    ],
  },
  {
    source: "codex",
    slug: "postgres-replica-lag",
    title: "Postgres replica lag incident",
    abstract: "Hot standby fell 20min behind during a vacuum-heavy batch; max_slot_wal_keep_size too small.",
    stage: "failed1",
    daysAgo: 4,
    talk: [
      "Standby is 20 minutes behind and the app reads are stale.",
      "vacuum on the events table generated a WAL burst; the slot's wal_keep_size capped retention and the standby had to re-fetch. Raise max_slot_wal_keep_size and move the batch off-peak.",
    ],
  },
  {
    source: "zcode",
    slug: "release-notarization",
    title: "Release pipeline + notarization",
    abstract: "make release builds, signs and uploads; credentials live in ~/.rmb/release.env, never in the repo.",
    stage: "t1done",
    daysAgo: 4.5,
    talk: [
      "Where do signing credentials come from?",
      "~/.rmb/release.env only — the Makefile sources it at release time. If SIGN_KEYCHAIN_PASS is missing you get an ad-hoc signed local build, like the 0.2.10-dev.1 install.",
    ],
  },
  {
    source: "pi",
    slug: "jumpserver-audit",
    title: "JumpServer access audit",
    abstract: "Quarterly audit of jump.hs99.vip: pruned stale accounts, enforced MFA on the audit group.",
    stage: "captured",
    daysAgo: 5,
    talk: [
      "Quarterly JumpServer audit — what's the checklist?",
      "Stale accounts, group membership vs oncall roster, MFA coverage, and command-recording retention. Export the diff before pruning so we can restore if anyone screams.",
    ],
  },
  {
    source: "workbuddy",
    slug: "orbstack-k8s-migration",
    title: "OrbStack K8s dev clusters",
    abstract: "Standardized dev Kubernetes on OrbStack; context naming and per-task namespaces documented.",
    stage: "t1running",
    daysAgo: 5.5,
    talk: [
      "Docker Desktop is slow on the M-series. Move dev k8s to OrbStack?",
      "OrbStack runs kind clusters natively and the contexts are stable. Convention: one cluster per task, namespace = task slug, torn down at clock-out.",
    ],
  },
  {
    source: "cursor",
    slug: "sentry-noise-triage",
    title: "Sentry noise triage",
    abstract: "Grouped recurring client-side noise with fingerprint rules; alert budget down 70%.",
    stage: "t1done",
    daysAgo: 6,
    talk: [
      "sentry.hs99.vip drowns us in network-noise events. Triage?",
      "Fingerprint the retry storms, set a release-health baseline, and route game-client noise to a low-priority project. Alert budget dropped ~70% in a week.",
    ],
  },
  {
    source: "cursor",
    slug: "solutionhub-cache-headers",
    title: "SolutionHub cache headers",
    abstract: "Static assets in the Vue SPA now send Cache-Control: no-cache so deploys land instantly.",
    stage: "t2done",
    daysAgo: 6.5,
    talk: [
      "SolutionHub users see stale bundles after deploys.",
      "nginx was caching index.html hard. Set no-cache on static files, keep immutable hashing for the chunk filenames themselves.",
    ],
  },
  {
    source: "claude-code",
    slug: "airflow-dag-standards",
    title: "Airflow DAG standards",
    abstract: "Codified DAG review standards: idempotent tasks, SLAs on the starlink ETLs, no top-level IO.",
    stage: "captured",
    daysAgo: 7,
    talk: [
      "DAGs keep breaking on backfills. Standards?",
      "Idempotent tasks only, SLA on the ETL roots, no IO at module top-level. Backfill mode gets an explicit flag and the reviewer checklist enforces it.",
    ],
  },
  {
    source: "workbuddy",
    slug: "aliyun-cost-attribution",
    title: "Aliyun monthly cost attribution",
    abstract: "Tag-based cost attribution across the caikuai-cn account; the monthly runbook lands in OSS.",
    stage: "t2running",
    daysAgo: 7.5,
    talk: [
      "Finance wants per-team cost splits from the Aliyun bill.",
      "Instance tags map to teams; the attribution script joins the bill export with the tag inventory and writes the monthly sheet. Un-tagged resources get a 'needs-owner' bucket so the number is honest.",
    ],
  },
  {
    source: "codex",
    slug: "windmill-script-migration",
    title: "Windmill script migration",
    abstract: "Moved cron scripts into Windmill workspaces with reviewed promotions and locked dependencies.",
    stage: "t1done",
    daysAgo: 8,
    talk: [
      "The old cron box is a snowflake. Move scripts to Windmill?",
      "Workspace per environment, promotion path dev → prod with review, dependencies pinned per script. Schedules mirror the old crontab one-to-one so nothing is silently dropped.",
    ],
  },
  {
    source: "zcode",
    slug: "harbor-image-cleanup",
    title: "Harbor image retention",
    abstract: "Retention policies prune untagged images after 14 days; registry disk usage halved.",
    stage: "t2done",
    daysAgo: 8.6,
    talk: [
      "Harbor is at 80% disk. Cleanup policy?",
      "Retention: keep the last 10 tagged per repo, purge untagged after 14 days, never touch release tags. Ran it dry first and halved usage.",
    ],
  },
  {
    source: "pi",
    slug: "devops-interview-loop",
    title: "DevOps interview loop Q3",
    abstract: "Refreshed the interview loop: k8s troubleshooting scenario, scored rubric, calibration notes.",
    stage: "t1done",
    daysAgo: 9,
    talk: [
      "Refresh the DevOps interview loop for Q3 hiring.",
      "Hands-on k8s troubleshooting scenario (CrashLoopBackOff + bad probe), a scored rubric, and calibration notes so different interviewers grade the same. Probation plan template links from the same doc.",
    ],
  },
  {
    source: "cursor",
    slug: "blockblast-match-latency",
    title: "BlockBlast match latency",
    abstract: "Profiled match-service p99 spikes to GC pauses; pooled boards and trimmed allocations.",
    stage: "captured",
    daysAgo: 9.5,
    talk: [
      "p99 on the match service spikes 200ms every few minutes.",
      "GC pauses from per-request board allocations. Pool the boards, preallocate the shuffle buffers, and the spikes flatten.",
    ],
  },
  {
    source: "workbuddy",
    slug: "kong-rate-limit-tuning",
    title: "Kong rate-limit tuning",
    abstract: "Redis-backed rate limiting on the game gateway; per-tier limits replace the global cap.",
    stage: "t2done",
    daysAgo: 10,
    talk: [
      "The global Kong rate limit punishes our own batch jobs.",
      "Move to the Redis-backed plugin with per-consumer tiers: game clients tight, internal services loose but metered. Policy: local for latency, redis for accuracy across nodes.",
    ],
  },
  {
    source: "zcode",
    slug: "rmb-recall-eval-gate",
    title: "Recall eval gate in make check",
    abstract: "make check runs the recall eval against golden queries; a regression fails the build.",
    stage: "t3running",
    daysAgo: 11,
    talk: [
      "Recall changes keep regressing quality silently. Gate it?",
      "The eval runs golden queries through the hybrid ranker and compares against expected hits; a drop beyond tolerance fails make check. It's the L3 layer of the pipeline now.",
    ],
  },
  {
    source: "cursor",
    slug: "sessions-page-pagination",
    title: "Sessions page pagination",
    abstract: "Cursor-free limit/offset paging with total counts; search hits abstracts and session keys.",
    stage: "done",
    daysAgo: 12,
    talk: [
      "Sessions page slows with thousands of rows. Paginate it?",
      "limit/offset with a total count is fine at this scale; q filters on abstract + session_key, sort stays updated desc. Server does the slicing.",
    ],
  },
  {
    source: "claude-code",
    slug: "menu-bar-shell-swift",
    title: "Menu-bar shell behaviors",
    abstract: "rmb-app menu-bar states: daemon up/down, update available, quick recall from the tray.",
    stage: "t1done",
    daysAgo: 13,
    talk: [
      "The tray icon should mean something: what states?",
      "Green daemon healthy, amber updating, red down with a 'start rmbd' action. Tooltip shows version + uptime; menu has quick recall and open webui.",
    ],
  },
  {
    source: "codex",
    slug: "embed-dimensions-migration",
    title: "Embedding dimensions migration",
    abstract: "Switching embed dimensions forces a full re-embed; the config API reports reembed_started.",
    stage: "failed2",
    daysAgo: 14,
    talk: [
      "What happens when I change embed dimensions from 768 to 1024?",
      "Vectors are incompatible, so rmbd starts a full re-embed and PUT /config answers reembed_started:true. Abort mid-flight is safe — rows just stay unembedded until the next batch.",
    ],
  },
  {
    source: "zcode",
    slug: "correction-flow-design",
    title: "Correction flow design",
    abstract: "Corrections attach to memory URIs and fold into recall at query time; retract is a DELETE.",
    stage: "t2done",
    daysAgo: 15,
    talk: [
      "How do corrections override a memory without editing it?",
      "Corrections are first-class rows targeting memory URIs; recall injects them alongside the target. Retract = DELETE by uri, history preserved.",
    ],
  },
  {
    source: "pi",
    slug: "svn-to-git-freeze",
    title: "SVN freeze window",
    abstract: "svn.youxi123.com freeze during the git migration week; changelist inventory archived first.",
    stage: "captured",
    daysAgo: 16,
    talk: [
      "We need a freeze window for the SVN → git cutover.",
      "Inventory changelists and branches first, archive the dump, then freeze 48h. Mirror authors mapping before the freeze or history comes out mangled.",
    ],
  },
  {
    source: "workbuddy",
    slug: "workbuddy-minutes-todo",
    title: "WorkBuddy minutes → TODO flow",
    abstract: "Meeting minutes extract action items; the TODO skill files them with owners and due dates.",
    stage: "t1done",
    daysAgo: 18,
    talk: [
      "Action items from meeting minutes keep evaporating.",
      "The minutes skill extracts owner+deadline pairs and files them as TODOs; the weekly digest lists anything unsigned. Filing is idempotent on the minute item id.",
    ],
  },
  {
    source: "codex",
    slug: "tutu-zsh-prompt",
    title: "tutu zsh prompt theme",
    abstract: "Prompt shows git branch/status, k8s context and last-command duration; async for speed.",
    stage: "t2done",
    daysAgo: 20,
    talk: [
      "My prompt is slow in big repos. Make tutu async?",
      "Render the static part immediately and fill git status from an async worker. K8s context only when a kubeconfig is present, duration only for >1s commands.",
    ],
  },
  {
    source: "claude-code",
    slug: "pbp-market-pipeline",
    title: "PBP market data pipeline",
    abstract: "Multi-source A-share quotes synced into Postgres; gap detection backfills missing bars nightly.",
    stage: "captured",
    daysAgo: 23,
    talk: [
      "Quote sources disagree on adjusted prices. Which wins?",
      "Upstream-adjusted from the primary source, secondary fills gaps. A nightly gap detector backfills missing bars and flags sources that drift beyond tolerance.",
    ],
  },
  {
    source: "cursor",
    slug: "game-ci-shard-split",
    title: "Game CI shard split",
    abstract: "Split the client test suite into shards by package weight; wall time down from 22 to 9 minutes.",
    stage: "t1running",
    daysAgo: 26,
    talk: [
      "Client CI takes 22 minutes and blocks merges.",
      "Shard by historical package weight — the heavy suites get their own runners. Cache the asset pipeline between runs; wall time dropped to ~9 minutes.",
    ],
  },
  {
    source: "zcode",
    slug: "rmb-schema-atoms",
    title: "Atoms schema design",
    abstract: "Atoms carry category+priority+source_turn_ids; scene distillation consumes them as a set.",
    stage: "done",
    daysAgo: 30,
    talk: [
      "What does an atom row need for scene distillation to work?",
      "Category, priority, and source_turn_ids so a scene can cite its evidence. Scenes reference atoms by uri — deleting an atom orphans the scene, so distill-time GC checks backreferences.",
    ],
  },
];

const STAGE_STATUS: Record<Stage, [string, string, string]> = {
  captured: ["idle", "idle", "idle"],
  t1running: ["running", "idle", "idle"],
  t1done: ["done", "waiting", "idle"],
  t2running: ["done", "running", "idle"],
  t2done: ["done", "done", "pending"],
  t3running: ["done", "done", "running"],
  done: ["done", "done", "done"],
  failed1: ["failed", "idle", "idle"],
  failed2: ["done", "failed", "idle"],
};

/** Atoms distilled per session, derived from its topic. */
function atomsFor(topic: Topic, sessionId: string): AtomRow[] {
  if (!["t2done", "t3running", "done"].includes(topic.stage)) return [];
  const categories = ["decision", "fact", "preference", "insight", "task"] as const;
  const angles = [
    `Chose the staged approach for ${topic.title.toLowerCase()} — reversible per step`,
    `Key constraint surfaced in "${topic.title}": verify with the eval gate before merging`,
    `Operational note from "${topic.title}": keep the dry-run path before enforcing anything`,
    `Preference recorded in "${topic.title}": evidence over confidence, check green before ship`,
    `Follow-up from "${topic.title}": promote the rule into arch-rules when it recurs`,
  ];
  const n = between(2, 5);
  const created = ago(topic.daysAgo + 0.05);
  return Array.from({ length: n }, (_, i) => {
    const id = `a-${sessionId}-${i + 1}`;
    return {
      id,
      session_id: sessionId,
      category: categories[i % categories.length],
      priority: between(1, 3),
      scene_name: `${topic.slug}-scene-${(i % 2) + 1}`,
      slug: topic.slug,
      content: angles[i % angles.length],
      source_turn_ids: [`t-${sessionId}-1`, `t-${sessionId}-${Math.min(i + 2, topic.talk.length)}`],
      created_at: iso(created),
      updated_at: iso(created + between(0, 3) * HOUR),
      uri: `rmb://atoms/${id}`,
    };
  });
}

function scenesFor(topic: Topic, sessionId: string, atoms: AtomRow[]): SceneRow[] {
  if (atoms.length === 0) return [];
  const n = between(1, 2);
  return Array.from({ length: n }, (_, i) => {
    const id = `sc-${sessionId}-${i + 1}`;
    const mine = atoms.filter((_, ai) => ai % n === i);
    const searched = rand() < 0.6;
    return {
      id,
      session_id: sessionId,
      display_name: `${topic.title} — part ${i + 1}`,
      abstract: topic.abstract,
      body: `The session walked through ${topic.title.toLowerCase()}: ${topic.talk[1] ?? topic.abstract} Distilled ${mine.length} atom${mine.length === 1 ? "" : "s"} covering the decisions and operational follow-ups.`,
      source_atoms: mine.map((a) => a.uri!),
      created_at: iso(ago(Math.max(0, topic.daysAgo - 0.02))),
      updated_at: iso(ago(topic.daysAgo)),
      uri: `rmb://scenes/${id}`,
      recall_stats: searched
        ? {
            uri: `rmb://scenes/${id}`,
            search_count: between(1, 25),
            cat_count: between(0, 8),
            meta_count: between(0, 5),
            last_searched_at: iso(ago(between(0, 10))),
            updated_at: iso(ago(between(0, 10))),
          }
        : null,
    };
  });
}

export interface MockData {
  sessions: SessionRow[];
  turns: Map<string, TurnRow[]>;
  atoms: Map<string, AtomRow[]>;
  scenes: Map<string, SceneRow[]>;
  pipelineStates: Map<string, PipelineState>;
  memories: MemoryRow[];
  skills: SkillRow[];
  skillDetails: Map<string, SkillDetail>;
  corrections: CorrectionRow[];
  config: ConfigView;
  version: { version: string; commit: string };
}

function buildData(): MockData {
  prngState = PRNG_SEED;
  resetCorrectionSeq();
  const sessions: SessionRow[] = [];
  const turns = new Map<string, TurnRow[]>();
  const atoms = new Map<string, AtomRow[]>();
  const scenes = new Map<string, SceneRow[]>();
  const pipelineStates = new Map<string, PipelineState>();

  TOPICS.forEach((topic, idx) => {
    const key = `${topic.source}:${topic.slug}`;
    const id = `s-${String(idx + 1).padStart(3, "0")}`;
    const [t1, t2, t3] = STAGE_STATUS[topic.stage];
    const lastTurn = ago(topic.daysAgo);
    const created = ago(topic.daysAgo + 0.4);

    const sessionTurns: TurnRow[] = topic.talk.map((content, i) => ({
      id: `t-${id}-${i + 1}`,
      turn_index: i + 1,
      uri: `rmb://turns/t-${id}-${i + 1}`,
      messages_jsonl: JSON.stringify({
        role: i % 2 === 0 ? "user" : "assistant",
        content,
      }),
      created_at: iso(created + i * 7 * 60_000),
      updated_at: iso(created + i * 7 * 60_000),
    }));
    turns.set(key, sessionTurns);

    const sessionAtoms = atomsFor(topic, id);
    atoms.set(key, sessionAtoms);
    const sessionScenes = scenesFor(topic, id, sessionAtoms);
    scenes.set(key, sessionScenes);

    sessions.push({
      id,
      session_key: key,
      source: topic.source,
      status: topic.open ? "open" : rand() < 0.7 ? "closed" : "open",
      abstract: topic.abstract,
      turn_count: sessionTurns.length,
      atom_count: sessionAtoms.length,
      scene_count: sessionScenes.length,
      t1_status: t1,
      t2_status: t2,
      t3_status: t3,
      uri: `rmb://sessions/${key}`,
      created_at: iso(created),
      updated_at: iso(lastTurn),
      last_turn_at: iso(lastTurn - between(1, 30) * 60_000),
    });

    pipelineStates.set(key, {
      session_id: id,
      t1_status: t1,
      t2_status: t2,
      t3_status: t3,
      t1_advanced_at: t1 === "idle" ? null : iso(ago(Math.max(0, topic.daysAgo - 0.1))),
      t2_advanced_at: t2 === "idle" ? null : iso(ago(Math.max(0, topic.daysAgo - 0.08))),
      t3_advanced_at: t3 === "idle" || t3 === "pending" ? null : iso(ago(Math.max(0, topic.daysAgo - 0.05))),
      t1_turns_since_advanced: between(0, 9),
      warmup_threshold: DEFAULT_PIPELINE.l1_every_n,
      updated_at: iso(lastTurn),
    });
  });

  // ----- memories -----------------------------------------------------------
  const memories: MemoryRow[] = [
    ...[
      ["colin-identity", 3, "Colin (李广慧) — Beijing-based DevOps/SRE lead", "Goes by Colin; GitHub colinleefish; runs the AI platform integration/architecture group at HungryStudio."],
      ["work-role", 2, "Tech lead of the architecture group, product R&D center", "Leads Starlink/StarOrigin SRE as super admin (role 888); runs DevOps interviews, probation plans, and onboarding; mentors rather than formally manages."],
      ["devices-setup", 2, "Primary machine: colin-mbp13-2026 (M5 Pro, 48GB)", "Daily stack: zsh + Oh My Zsh (tutu theme), iTerm2, Cursor + Claude Code, OrbStack for Docker/K8s; work MBP is colin-hs-mbp2023."],
      ["contact-channels", 1, "Work email liguanghui@hungrystudio.com; personal colinleefish@gmail.com", "Workcode/SSO sub 001232; DingTalk staff id 1646383526189462; mainland-China network needs the local proxy for GitHub."],
      ["apple-dev-id", 1, "Apple Developer ID: GUANGHUI LI / N4YPJBRBN4", "Personal (non-org) account; used to sign rmb-desktop release builds."],
    ].map(([slug, version, abstract, body]) => memory("profile", slug as string, version as number, abstract as string, body as string)),
    ...[
      ["merge-modern-go-pr69", 1, "modern-go refactor merged via PR #69 (merge commit 3ab60a5)", "One-commit sweep across 69 files: errors.Is, range-int loops, sync.WaitGroup.Go. Merged with a merge commit per the parallel-work plan; local make check was the gate of record (pr-check workflow not yet landed)."],
      ["b04-flaky-diagnosis", 1, "B04: TestSpawnedDaemonStdioIsLogFdNotPipe flakes under suite load", "Fixed 5s file-poll deadline loses to ~30 parallel packages; causality proven by injecting an 8s child-write delay (deterministic FAIL at 5.0s). Fix = tolerant detection, fd assertion untouched."],
      ["pr67-ci-fixture", 1, "PR #67 adds the hermetic fakeRMBHome CI fixture", "Linux CI passed 2m10s. Until it merges, PRs carry no CI runs and local make check is the gate."],
      ["f07-zcode-shipped", 1, "F07 zcode integration shipped at 9b70cb3, VERSION 0.2.10-dev.1", "Local ad-hoc-signed install (no notarization — SIGN_KEYCHAIN_PASS unavailable); the inferred payload-shape risk materialized as #61/#62."],
      ["tauri-migration-done", 1, "Tauri shell replaced by the Go menu-bar app; plan archived", "app/package.json and app/src-tauri must never be resurrected — the shell is rmb-app + embedded webui served by rmbd."],
      ["retrieval-audit-p0s", 1, "Retrieval audit produced three P0 remediations", "Time-decay before fusion, FTS rank normalization, de-duplicated corrections in the vector lane; re-run the recall eval after each fix."],
      ["webui-refactor-kickoff", 1, "WebUI refactor is IA-first (F03 audit → F04 shell → F05 settings)", "Plan lives in plan/webui-refactor.md; strangler pattern for settings, no big-bang rewrite."],
      ["ucloud-aliyun-migration", 2, "UCloud → Aliyun migration in flight for game infra", "ACK clusters on Aliyun account caikuai-cn; BlockCrush/BlockBlast/JDMJ/MJWD/Nebula workloads staged in waves."],
      ["starlink-ab-refactor", 1, "Starlink AB-test data model documented", "solutions → ways → tags → mini bundles; Starmap consumes the bundle snapshot, so tag edits re-snapshot before the ship window."],
      ["jumpserver-hardening", 1, "Quarterly JumpServer audits pruned stale accounts and enforced MFA", "jump.hs99.vip; export diffs before pruning; command-recording retention checked each quarter."],
      ["interview-loop-q3", 1, "DevOps interview loop refreshed for Q3", "k8s troubleshooting scenario + scored rubric + calibration notes; probation plan template linked from the same doc."],
      ["orbstack-standard", 1, "Dev Kubernetes standardized on OrbStack kind clusters", "One cluster per task, namespace = task slug, torn down at clock-out."],
      ["sentry-adoption", 1, "Sentry noise cut ~70% via fingerprint rules", "Retry storms fingerprinted; game-client noise routed to a low-priority project on sentry.hs99.vip."],
      ["cost-attribution-monthly", 1, "Aliyun monthly cost attribution is tag-based", "Joins the bill export with the tag inventory; untagged resources land in a 'needs-owner' bucket."],
      ["harbor-retention", 1, "Harbor retention: last 10 tagged per repo, untagged purged after 14 days", "Release tags exempt; dry-run first — registry disk usage halved."],
      ["pgpour-lag-incident", 1, "pgpour CDC slot lagged 40GB on starlink-dev-all-in-one", "Long-running transaction pinned the slot for 6h; check pg_replication_slots + pg_stat_activity xmin first. (段文彬 owns pgpour — Colin only operates it.)"],
      ["svn-freeze", 1, "SVN → git cutover used a 48h freeze on svn.youxi123.com", "Changelist inventory archived and authors mapping mirrored before the freeze."],
      ["blockblast-launch", 1, "BlockBlast match-service p99 spikes traced to GC pauses", "Pooled board objects and preallocated shuffle buffers flattened the spikes."],
      ["kong-tiered-limits", 1, "Kong gateway moved to Redis-backed per-consumer rate limits", "Game clients tight, internal services loose but metered; policy=redis for cross-node accuracy."],
      ["windmill-migration", 1, "Cron scripts migrated to Windmill with reviewed promotions", "Workspace per environment, deps pinned per script, schedules mirror the old crontab one-to-one."],
    ].map(([slug, version, abstract, body]) => memory("events", slug as string, version as number, abstract as string, body as string)),
    ...[
      ["prefers-make-targets", 2, "Always use make targets, never raw go/npm invocations", "make check/test/build/dev are the pipeline; bypassing them skips vet and the recall-eval gate."],
      ["merge-commits-only", 1, "Merge with merge commits — no squash/rebase of pushed branches", "Every agent's work must stay traceable in history (plan/parallel-work-and-versioning.md §1)."],
      ["branch-per-task", 1, "One task = one branch = one worktree", "Never work on the main checkout's main branch; worktrees at ../rmb-desktop-<slug>."],
      ["state-files-over-chat", 1, "Durable facts go to state files, not chat history", "PROGRESS.md / DECISIONS.md / feature_list.json are truth; an honest checkpoint beats a botched finish."],
      ["docs-in-same-commit", 1, "Docs move in the same commit as the code they describe", "No stale documentation; completed plans move to plan/done in the completing commit."],
      ["evidence-over-confidence", 1, "Definition of Done is runtime evidence, not agent confidence", "verify-feature layers must pass; never hand-set passing."],
      ["minimal-deps", 1, "Prefer hand-rolled minimal tooling over new dependencies", "The webui mocks stay flag-based, no MSW; new deps need a reason in the plan."],
      ["concise-reports", 1, "Status reports lead with the outcome", "TLDR first, then supporting detail; no arrow-chain shorthand in user-facing text."],
      ["proxy-for-github", 1, "Mainland network: GitHub goes through the 127.0.0.1:1081 proxy", "bash scripts/with-proxy.sh wraps gh/git fetch; Google Drive and pypi unreachable directly."],
      ["orbstack-over-docker-desktop", 1, "OrbStack over Docker Desktop on Apple silicon", "Faster cold start, native kind clusters, stable contexts."],
      ["cursor-daily-driver", 1, "Cursor + Claude Code as daily coding agents", "MCP configs in ~/.cursor/mcp.json and ~/.claude.json; zcode hooks feed rmb."],
      ["postgres-first", 1, "Reach for Postgres-native features before new infrastructure", "CDC via replication slots, FTS5/rmq patterns, large-scale migrations — deep Postgres skills are the house specialty."],
      ["chinese-english-mix", 1, "Comfortable with mixed 中文/English technical context", "Code and commits in English; conversation freely mixes both."],
      ["morning-deep-work", 1, "Deep work blocks in the Beijing morning", "Deploy windows and reviews cluster in the afternoon; incidents interrupt anytime."],
      ["clean-check-hygiene", 1, "Leave the worktree clean at clock-out", "make clean-check's five dimensions must pass; commit or stash everything, no drifting artifacts."],
      ["wip-one", 1, "WIP=1: a single active feature at a time", "feature_list.json enforces it; prod bug fixes may interleave."],
    ].map(([slug, version, abstract, body]) => memory("preferences", slug as string, version as number, abstract as string, body as string)),
    ...[
      ["rmbd", "The Go daemon at the core of rmb-desktop", "Serves /api/v1, embeds the webui, runs the distillation pipeline; default addr 127.0.0.1:19019."],
      ["rmb-cli", "rmb — the CLI for rmb", "hook-submit, recall, session trace events; lives at ~/.rmb/bin/rmb."],
      ["rmb-app", "Go menu-bar shell", "Tray states for daemon up/down/update; quick recall; opens the webui."],
      ["zcode", "The coding agent this session runs in", "Integration scope internal/hook; Stop-hook → hook-submit payload shape was the #61/#62 risk."],
      ["starlink", "HungryStudio's internal AB-testing platform", "Solutions/ways/tags/mini-bundles model; Starmap pipeline consumes bundle snapshots."],
      ["starorigin", "StarOrigin platform — sibling of Starlink", "Same super-admin (role 888) administration surface."],
      ["blockblast", "Flagship game at HungryStudio", "Match-service latency work (GC pooling) came from BlockBlast profiling."],
      ["hungrystudio", "北京彩块科技有限公司 — Colin's employer", "Product R&D center, architecture group; HR docs sometimes reference 北京迦游网络科技."],
      ["pgpour", "CDC/data-sync tool — owned solely by 段文彬 (duanwenbin)", "Colin operates pgpour-based stacks (starlink-dev-all-in-one, Kafka pipelines) but is not the maintainer."],
      ["orbstack", "Docker/K8s runtime on macOS", "Hosts dev kind clusters; per-task namespace convention."],
      ["jumpserver", "PAM/bastion at jump.hs99.vip", "Quarterly access audits; MFA on the audit group."],
      ["sentry-hs", "Error tracking at sentry.hs99.vip", "Fingerprint rules cut alert noise ~70%."],
      ["harbor-hs", "Container registry", "Retention: last 10 tagged per repo, untagged purged after 14 days."],
      ["windmill", "Workflow/script runtime", "Replaced the cron snowflake; dev→prod promotion with review."],
      ["airflow-hs", "Airflow for starlink ETLs", "DAG standards: idempotent tasks, SLAs on roots, no top-level IO."],
      ["kong", "API gateway in front of game services", "Redis-backed per-consumer rate limiting."],
      ["aliyun", "Target cloud for the migration (account caikuai-cn)", "RAM user liguanghui/effcy_dev, cn-beijing; monthly cost attribution runs here."],
      ["ucloud", "Origin cloud being migrated away from", "Legacy game-service infra; waves move to Aliyun ACK."],
      ["svn-server", "Legacy SVN at svn.youxi123.com", "Frozen 48h during the git cutover; changelist inventory archived."],
    ].map(([slug, abstract, body]) => memory("entities", slug as string, 1, abstract as string, body as string)),
  ];

  // ----- corrections --------------------------------------------------------
  const corrections: CorrectionRow[] = [
    correction(
      "Not 郭佳锋 — he is Starlink's product manager (third party); Colin is the DevOps/SRE lead.",
      ["rmb://memories/profile/colin-identity"],
    ),
    correction(
      "Proxy port is 1081 (not 7890) for GitHub access from the mainland network.",
      ["rmb://memories/preferences/proxy-for-github"],
    ),
    correction(
      "pgpour's owner is 段文彬 (duanwenbin) alone — Colin only operates the stacks, never maintains it.",
      ["rmb://memories/entities/pgpour"],
    ),
    correction(
      "The daemon's default port is 19019, not 19090.",
      ["rmb://memories/entities/rmbd"],
    ),
    correction(
      "modern-go merged as a merge commit (3ab60a5); the squash phrasing in older notes is wrong.",
      ["rmb://memories/events/merge-modern-go-pr69"],
    ),
    correction(
      "Workcode is 001232; the older 001229 id is stale.",
      ["rmb://memories/profile/contact-channels"],
    ),
    correction(
      "Harbor retention keeps the last 10 tagged per repo (not 5) and exempts release tags.",
      ["rmb://memories/events/harbor-retention"],
    ),
    correction(
      "OrbStack is also used for Docker, not just Kubernetes.",
      ["rmb://memories/preferences/orbstack-over-docker-desktop"],
    ),
  ];

  // ----- skills -------------------------------------------------------------
  const skills = SKILLS.map((s) => skillRow(s));
  const skillDetails = new Map<string, SkillDetail>(
    SKILLS.map((s) => [s.slug, skillDetail(s)]),
  );

  const config: ConfigView = {
    addr: "127.0.0.1:19019",
    db_path: "~/.rmb/rmb.db",
    config_path: "~/.rmb/config.yaml",
    distillation_enabled: true,
    launch_at_login: true,
    llm: {
      api_base: "https://api.deepseek.com/v1",
      api_key_set: true,
      api_key_suffix: "3f9a",
      model: "deepseek-chat",
      timeout: "30s",
    },
    embed: {
      api_base: "https://api.siliconflow.cn/v1",
      api_key_set: true,
      api_key_suffix: "8c21",
      model: "Qwen/Qwen3-Embedding-8B",
      dimensions: 1024,
    },
    pipeline: { ...DEFAULT_PIPELINE },
  };

  return {
    sessions,
    turns,
    atoms,
    scenes,
    pipelineStates,
    memories,
    skills,
    skillDetails,
    corrections,
    config,
    version: { version: "0.2.10-dev.1", commit: "af6b33b" },
  };
}

// ----- memory/skill/correction builders -------------------------------------

function memory(
  category: string,
  slug: string,
  version: number,
  abstract: string,
  body: string,
): MemoryRow {
  const uri = `rmb://memories/${category}/${slug}`;
  const created = ago(between(20, 55));
  const searched = rand() < 0.55;
  return {
    id: `m-${category}-${slug}`,
    uri,
    category,
    slug,
    version,
    abstract,
    body,
    source_scene_uris: [
      `rmb://scenes/sc-s-${String(between(1, 36)).padStart(3, "0")}-1`,
    ],
    source_correction_uris: [],
    created_at: iso(created),
    updated_at: iso(ago(between(0, 18))),
    recall_stats: searched
      ? {
          uri,
          search_count: between(1, 60),
          cat_count: between(0, 15),
          meta_count: between(0, 10),
          last_searched_at: iso(ago(between(0, 12))),
          last_cated_at: iso(ago(between(0, 20))),
          last_metaed_at: iso(ago(between(0, 25))),
          updated_at: iso(ago(between(0, 5))),
        }
      : null,
  };
}

let correctionSeq = 0;
function correction(statement: string, target_uris: string[]): CorrectionRow {
  correctionSeq += 1;
  return {
    uri: `rmb://corrections/c-${String(correctionSeq).padStart(3, "0")}`,
    statement,
    target_uris,
    created_at: iso(ago(between(1, 30))),
  };
}
/** Reset before each buildData() so repeated builds (HMR, selftest) stay deterministic. */
function resetCorrectionSeq(): void {
  correctionSeq = 0;
}

interface SkillSeed {
  slug: string;
  name: string;
  description: string;
  tags: string[];
  daysAgo: number;
  files: Record<string, string>;
}

const SKILL_MD = (name: string, description: string, steps: string): string =>
  `---
name: ${name.toLowerCase().replace(/\s+/g, "-")}
description: ${description}
---

# ${name}

${description}

## When to use

Reach for this when the task touches ${name.toLowerCase()} and the
step-by-step below matches the situation.

## Steps

${steps}
`;

const SKILLS: SkillSeed[] = [
  {
    slug: "recall-rule-authoring",
    name: "Recall rule authoring",
    description: "Write recall rules that agents actually obey: imperative, scoped, with a verify step.",
    tags: ["recall", "agents", "webui"],
    daysAgo: 2,
    files: {
      "SKILL.md": SKILL_MD(
        "Recall rule authoring",
        "Write recall rules that agents actually obey.",
        "1. State the trigger (when this rule applies).\n2. State the action in imperative mood.\n3. Add a verify step the agent can run.\n4. Keep it under 120 words — rules compete for context.",
      ),
      "references/examples.md":
        "# Examples\n\n- 'Before committing, run make check; report failures verbatim.'\n- 'When touching internal/db, read docs/audit/…/MASTER-REPORT.md first.'",
    },
  },
  {
    slug: "hook-submit-debugging",
    name: "hook-submit debugging",
    description: "Diagnose agent hook → rmbd turn submission: payload shape, ports, auth, daemon logs.",
    tags: ["hooks", "rmbd"],
    daysAgo: 5,
    files: {
      "SKILL.md": SKILL_MD(
        "hook-submit debugging",
        "Diagnose agent hook → rmbd turn submission.",
        "1. Confirm rmbd is listening: 127.0.0.1:19019/healthz.\n2. Run the hook command by hand with a sample payload.\n3. Diff the stored turn against the golden file.\n4. Check ~/.rmb/logs for the ingest errors.",
      ),
    },
  },
  {
    slug: "session-trace-analysis",
    name: "Session trace analysis",
    description: "Query .harness/traces/traces.jsonl for session-start/end events and sprint timelines.",
    tags: ["harness", "observability"],
    daysAgo: 8,
    files: {
      "SKILL.md": SKILL_MD(
        "Session trace analysis",
        "Query traces.jsonl for sprint timelines.",
        "1. jq -r 'select(.event==\"session-start\")' .harness/traces/traces.jsonl\n2. Pair start/end by session id.\n3. Cross-reference feature_list.json evidence.",
      ),
    },
  },
  {
    slug: "release-pipeline",
    name: "Release pipeline",
    description: "Cut a release: make release VERSION=x.y.z with credentials from ~/.rmb/release.env.",
    tags: ["release", "signing"],
    daysAgo: 12,
    files: {
      "SKILL.md": SKILL_MD(
        "Release pipeline",
        "Cut a signed release.",
        "1. Bump VERSION via the Makefile only.\n2. Source ~/.rmb/release.env (never commit credentials).\n3. make release VERSION=x.y.z\n4. Without SIGN_KEYCHAIN_PASS you get an ad-hoc local build.",
      ),
      "references/release.env.example": "SIGN_IDENTITY=...\nSIGN_KEYCHAIN_PASS=...\nUPLOAD_TARGET=...",
    },
  },
  {
    slug: "pgpour-cdc-ops",
    name: "pgpour CDC ops",
    description: "Operate pgpour CDC stacks: slot lag triage, Kafka lag, restart runbooks. (Owner: 段文彬.)",
    tags: ["postgres", "cdc", "kafka"],
    daysAgo: 15,
    files: {
      "SKILL.md": SKILL_MD(
        "pgpour CDC ops",
        "Operate pgpour CDC stacks safely.",
        "1. Check pg_replication_slots: active_pid, restart_lsn, lag bytes.\n2. Find the oldest xmin holder in pg_stat_activity.\n3. Escalate structural changes to 段文彬 — Colin operates, does not maintain.",
      ),
    },
  },
  {
    slug: "starlink-starmap",
    name: "Starlink Starmap pipeline",
    description: "Navigate the AB-test model: solutions → ways → tags → mini bundles → Starmap snapshots.",
    tags: ["starlink", "ab-testing"],
    daysAgo: 18,
    files: {
      "SKILL.md": SKILL_MD(
        "Starlink Starmap pipeline",
        "Navigate the AB-test data model.",
        "1. Solutions own ways; tags attach to ways.\n2. Mini bundles snapshot ways+tags.\n3. Starmap consumes snapshots — re-snapshot after tag edits, before the ship window.",
      ),
    },
  },
  {
    slug: "postgres-slot-hygiene",
    name: "Postgres slot hygiene",
    description: "Keep replication slots from pinning WAL: lag alerts, max_slot_wal_keep_size, vacuum windows.",
    tags: ["postgres", "replication"],
    daysAgo: 21,
    files: {
      "SKILL.md": SKILL_MD(
        "Postgres slot hygiene",
        "Keep replication slots from pinning WAL.",
        "1. Alert on slot lag > 5GB.\n2. Set max_slot_wal_keep_size so a dead slot cannot fill disk.\n3. Move vacuum-heavy batches off-peak; watch WAL burst rate.",
      ),
    },
  },
  {
    slug: "vite-mock-dev",
    name: "Vite mock dev loop",
    description: "Tweak the rmb webui against mock data: npm run dev:mock, MOCK badge toggle, RMB_API_TARGET for real data.",
    tags: ["webui", "vite", "dev"],
    daysAgo: 1,
    files: {
      "SKILL.md": SKILL_MD(
        "Vite mock dev loop",
        "Tweak the webui without the daemon.",
        "1. cd webui && npm run dev:mock → http://localhost:5173 (VITE_MOCK=true, in-memory dataset).\n2. The corner MOCK badge toggles mock/proxied-real (localStorage rmb.mock).\n3. Real data: npm run dev (proxy → RMB_API_TARGET, default 127.0.0.1:19019).\n4. HMR picks up component edits live; the dataset is seeded and stable.",
      ),
    },
  },
  {
    slug: "onboarding-walkthrough",
    name: "Onboarding walkthrough",
    description: "Reset and replay first-run onboarding: marker file, demo mode, connection tests.",
    tags: ["webui", "onboarding"],
    daysAgo: 25,
    files: {
      "SKILL.md": SKILL_MD(
        "Onboarding walkthrough",
        "Replay first-run onboarding.",
        "1. Delete ~/.rmb/onboarding.complete (or POST /api/v1/onboarding/reset).\n2. Reload the webui — the gate redirects to /onboarding.\n3. VITE_MOCK_ONBOARDING=true for the UI-only demo path.",
      ),
    },
  },
  {
    slug: "clean-check-dimensions",
    name: "Clean-check dimensions",
    description: "The five clean-state dimensions make clean-check verifies, and what each catches.",
    tags: ["harness", "hygiene"],
    daysAgo: 28,
    files: {
      "SKILL.md": SKILL_MD(
        "Clean-check dimensions",
        "The five clean-state dimensions.",
        "D1 worktree clean · D2 no stray processes · D3 machine-readable state valid · D4 docs/staleness sweep · D5 no secrets in tree. All five must pass at clock-out.",
      ),
    },
  },
  {
    slug: "aliyun-ack-basics",
    name: "Aliyun ACK basics",
    description: "Everyday ACK operations on caikuai-cn: contexts, node pools, log project naming.",
    tags: ["aliyun", "k8s"],
    daysAgo: 32,
    files: {
      "SKILL.md": SKILL_MD(
        "Aliyun ACK basics",
        "Everyday ACK operations.",
        "1. aliyun configure list — profile liguanghui-admin.\n2. Contexts named <cluster>-<env>; node pools per workload class.\n3. Log projects: starlink-<env>-logs.",
      ),
    },
  },
  {
    slug: "sentry-triage",
    name: "Sentry triage",
    description: "Triage sentry.hs99.vip: fingerprint noise, release health baselines, routing rules.",
    tags: ["sentry", "observability"],
    daysAgo: 35,
    files: {
      "SKILL.md": SKILL_MD(
        "Sentry triage",
        "Triage the noisy Sentry project.",
        "1. Group by fingerprint before writing rules.\n2. Baseline release health per release channel.\n3. Route client noise to the low-priority project.",
      ),
    },
  },
];

function skillRow(s: SkillSeed): SkillRow {
  const updated = ago(s.daysAgo);
  const searched = rand() < 0.5;
  return {
    slug: s.slug,
    name: s.name,
    description: s.description,
    tags: s.tags,
    uri: `rmb://skills/${s.slug}`,
    version: between(1, 4),
    updated_at: iso(updated),
    recall_stats: searched
      ? {
          uri: `rmb://skills/${s.slug}`,
          search_count: between(1, 30),
          cat_count: between(0, 10),
          meta_count: between(0, 6),
          last_searched_at: iso(ago(between(0, 14))),
          updated_at: iso(ago(between(0, 14))),
        }
      : null,
  };
}

function skillDetail(s: SkillSeed): SkillDetail {
  const row = skillRow(s);
  const children = Object.keys(s.files)
    .filter((p) => p !== "SKILL.md")
    .map((p) => ({
      name: p.replace("references/", ""),
      path: p,
      type: "file" as const,
    }));
  return {
    skill: {
      uri: row.uri,
      slug: s.slug,
      name: s.name,
      description: s.description,
      tags: s.tags,
      version: row.version,
      bundle_sha256: `mock-${s.slug}-${row.version}`,
      created_at: iso(ago(s.daysAgo + 10)),
      updated_at: row.updated_at,
    },
    tree: [
      { name: "SKILL.md", path: "SKILL.md", type: "file" },
      ...(children.length
        ? [{ name: "references", path: "references", type: "dir" as const, children }]
        : []),
    ],
    files: s.files,
  };
}

// ----- derived views ---------------------------------------------------------

export function mockOverview(data: MockData): Overview {
  const turns = [...data.turns.values()].reduce((n, ts) => n + ts.length, 0);
  const atoms = [...data.atoms.values()].reduce((n, as) => n + as.length, 0);
  const scenes = [...data.scenes.values()].reduce((n, ss) => n + ss.length, 0);
  const byCategory = (cat: string) =>
    data.memories.filter((m) => m.category === cat).length;
  return {
    counts: {
      sessions: data.sessions.length,
      turns,
      atoms,
      scenes,
      memories: data.memories.length,
      pipeline_states: data.pipelineStates.size,
      tasks: 6,
      corrections: data.corrections.length,
      skills: data.skills.length,
    },
    memory_by_category: {
      profile_version: byCategory("profile"),
      events: byCategory("events"),
      preferences: byCategory("preferences"),
      entities: byCategory("entities"),
    },
  };
}

export function mockPipelineHealth(data: MockData): PipelineHealth {
  const count = (stage: 0 | 1 | 2, status: string) =>
    data.sessions.filter(
      (s) =>
        [s.t1_status, s.t2_status, s.t3_status][stage] === status,
    ).length;
  const counts = (stage: 0 | 1 | 2) => ({
    pending: count(stage, "pending"),
    running: count(stage, "running"),
    failed: count(stage, "failed"),
    idle: count(stage, "idle") + count(stage, "done"),
    waiting: count(stage, "waiting"),
  });
  const problems = data.sessions
    .map((s) => {
      if (s.t1_status === "failed")
        return { s, stage: "t1", reason: "LLM request failed: 429 rate limited" };
      if (s.t2_status === "failed")
        return { s, stage: "t2", reason: "scene parse returned empty body" };
      if (s.t3_status === "pending")
        return { s, stage: "t3", reason: "waiting in L3 backlog" };
      return null;
    })
    .filter((p): p is { s: SessionRow; stage: string; reason: string } => p !== null)
    .slice(0, 6)
    .map(({ s, stage, reason }) => ({
      session_key: s.session_key,
      session_uri: s.uri,
      stage,
      status: stage === "t3" ? "pending" : "failed",
      updated_at: s.updated_at,
      reason,
    }));
  return {
    distillation_enabled: data.config.distillation_enabled,
    tracked_sessions: data.sessions.length,
    generated_at: new Date().toISOString(),
    stages: { t1: counts(0), t2: counts(1), t3: counts(2) },
    funnel: {
      sessions: data.sessions.length,
      t1_done: count(0, "done"),
      t2_done: count(1, "done"),
      t3_done: count(2, "done"),
    },
    problems,
  };
}

export function buildMockData(): MockData {
  return buildData();
}
