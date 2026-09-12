/**
 * F10 L2 runtime gate: exercises the mock API router under plain node
 * (bundled via esbuild — see the feature's layer cmd). Asserts route
 * coverage, paging/filtering and the mutable endpoints stay consistent
 * with the shapes in lib/types.ts. Exits non-zero on the first failure.
 */
import { handleMockApi, mockHealthOk } from "./server";
import { buildMockData } from "./data";

let passed = 0;

/** Uncaught throw ⇒ non-zero node exit; normal end ⇒ exit 0 (no node globals in src). */
function assert(cond: boolean, label: string): void {
  if (!cond) throw new Error(`selftest failed: ${label}`);
  passed += 1;
  console.log(`ok - ${label}`);
}

function get(path: string): { status: number; json: any } {
  const r = handleMockApi("GET", path);
  return { status: r.status, json: r.json };
}

/* eslint-disable-next-line @typescript-eslint/no-explicit-any */
function call(method: string, path: string, body?: unknown): { status: number; json: any } {
  const r = handleMockApi(method, path, body);
  return { status: r.status, json: r.json };
}

// -- overview -----------------------------------------------------------------
const seed = buildMockData();
const overview = get("/api/v1/browse/overview");
assert(overview.status === 200, "GET overview is 200");
assert(overview.json.counts.sessions === seed.sessions.length, "overview sessions count matches dataset");
assert(overview.json.counts.memories === seed.memories.length, "overview memories count matches dataset");
assert(overview.json.counts.skills === seed.skills.length, "overview skills count matches dataset");

// -- pipeline health ------------------------------------------------------------
const health = get("/api/v1/browse/pipeline-health");
assert(health.status === 200 && health.json.tracked_sessions === seed.sessions.length, "pipeline health tracks all sessions");
assert(health.json.funnel.t1_done > 0 && health.json.funnel.t1_done <= seed.sessions.length, "funnel t1_done in range");
assert(Array.isArray(health.json.problems) && health.json.problems.length > 0, "pipeline health lists problems");

// -- sessions paging + search ----------------------------------------------------
const page1 = get("/api/v1/browse/sessions?limit=10&offset=0&sort=updated&order=desc");
assert(page1.json.items.length === 10, "sessions paging returns limit items");
assert(page1.json.total === seed.sessions.length, "sessions total matches dataset");
const dates = page1.json.items.map((s: { updated_at: string }) => Date.parse(s.updated_at));
assert(dates.every((d: number, i: number) => i === 0 || d <= dates[i - 1]), "sessions sorted updated desc");

const searched = get("/api/v1/browse/sessions?limit=50&offset=0&q=postgres");
assert(
  searched.json.total > 0 &&
    searched.json.items.every((s: { abstract: string; session_key: string }) =>
      `${s.session_key} ${s.abstract}`.toLowerCase().includes("postgres"),
    ),
  "sessions q filters on key+abstract",
);

// -- session detail ---------------------------------------------------------------
const doneSession = seed.sessions.find((s) => s.t3_status === "done");
assert(!!doneSession, "dataset has a fully distilled session");
const detail = get(`/api/v1/browse/sessions/${encodeURIComponent(doneSession!.session_key)}`);
assert(detail.status === 200, "GET session detail is 200");
assert(detail.json.session.session_key === doneSession!.session_key, "detail echoes the session key");
assert(detail.json.turns.length === doneSession!.turn_count, "detail turns match turn_count");
assert(detail.json.atoms.length === doneSession!.atom_count, "detail atoms match atom_count");
assert(detail.json.scenes.length === doneSession!.scene_count, "detail scenes match scene_count");
assert(detail.json.pipeline_state !== null, "detail carries a pipeline state");
const parsedTurn = JSON.parse(detail.json.turns[0].messages_jsonl);
assert(typeof parsedTurn.role === "string" && typeof parsedTurn.content === "string", "turn messages_jsonl parses to {role, content}");
assert(get("/api/v1/browse/sessions/nope:missing").status === 404, "unknown session is 404");

// -- memories ---------------------------------------------------------------------
const memories = get("/api/v1/browse/memories?limit=200&offset=0&category=profile");
assert(memories.json.items.length > 0, "profile memories exist");
assert(memories.json.items.every((m: { category: string }) => m.category === "profile"), "category filter applies");
const memSearch = get("/api/v1/browse/memories?limit=200&offset=0&q=postgres");
assert(memSearch.json.items.length > 0, "memories q search hits");

// -- skills -------------------------------------------------------------------------
const skills = get("/api/v1/browse/skills?limit=5&offset=0&sort=updated&order=desc");
assert(skills.json.total === seed.skills.length, "skills total matches dataset");
assert(skills.json.items.length === 5, "skills paging limit applies");
const skillDetail = get("/api/v1/browse/skills/vite-mock-dev");
assert(skillDetail.status === 200, "GET skill detail is 200");
assert(
  Array.isArray(skillDetail.json.tree) &&
    skillDetail.json.tree.some((n: { path: string }) => n.path === "SKILL.md") &&
    typeof skillDetail.json.files["SKILL.md"] === "string",
  "skill detail carries tree + SKILL.md content",
);
assert(get("/api/v1/browse/skills/nope").status === 404, "unknown skill is 404");

// -- corrections (mutable) ------------------------------------------------------------
const target = seed.memories[0].uri;
const listed = get(`/api/v1/corrections?target=${encodeURIComponent(target)}`);
const before = listed.json.items.length;
assert(listed.status === 200 && Array.isArray(listed.json.items), "corrections list for target");
const created = call("POST", "/api/v1/corrections", {
  statement: "selftest correction",
  target_uris: [target],
});
assert(created.status === 200 && typeof created.json.uri === "string", "correction created");
const afterCreate = get(`/api/v1/corrections?target=${encodeURIComponent(target)}`);
assert(afterCreate.json.items.length === before + 1, "created correction appears in list");
const retracted = call("DELETE", `/api/v1/corrections?uri=${encodeURIComponent(created.json.uri)}`);
assert(retracted.json.retracted === true, "correction retracted");
const afterRetract = get(`/api/v1/corrections?target=${encodeURIComponent(target)}`);
assert(afterRetract.json.items.length === before, "retracted correction gone from list");

// -- config GET/PUT ---------------------------------------------------------------------
const config = get("/api/v1/config");
assert(config.status === 200 && config.json.embed.dimensions === 1024, "config GET returns seed config");
const put = call("PUT", "/api/v1/config", {
  llm: { model: "deepseek-reasoner" },
});
assert(put.json.ok === true && put.json.reembed_started === false, "config PUT llm model, no re-embed");
assert(get("/api/v1/config").json.llm.model === "deepseek-reasoner", "config PUT persisted");
const putDims = call("PUT", "/api/v1/config", { embed: { dimensions: 768 } });
assert(putDims.json.reembed_started === true, "embedding dimension change signals re-embed");
assert(get("/api/v1/config").json.embed.dimensions === 768, "embedding dimension persisted");

// -- version / restart / healthz ----------------------------------------------------------
const version = get("/api/v1/version");
assert(version.status === 200 && typeof version.json.version === "string", "GET version");
assert(mockHealthOk(), "healthz ok before restart");
assert(call("POST", "/api/v1/system/restart").json.ok === true, "restart accepted");
assert(!mockHealthOk(), "healthz down right after restart");

// -- onboarding -----------------------------------------------------------------------------
assert(get("/api/v1/onboarding/status").json.completed === true, "onboarding completed by default");
assert(call("POST", "/api/v1/onboarding/reset").status === 200, "onboarding reset");
assert(get("/api/v1/onboarding/status").json.completed === false, "onboarding status flipped off");
assert(call("POST", "/api/v1/onboarding/complete").status === 200, "onboarding completed");
assert(get("/api/v1/onboarding/status").json.completed === true, "onboarding status flipped on");

// -- config tests + unknown route -------------------------------------------------------------
const llmTest = call("POST", "/api/v1/config/test/llm", { api_base: "x" });
assert(llmTest.json.ok === true && typeof llmTest.json.latency_ms === "number", "llm connection test ok");
assert(call("GET", "/api/v1/does-not-exist").status === 404, "unknown route is 404");

console.log(`\n${passed} assertions passed`);
