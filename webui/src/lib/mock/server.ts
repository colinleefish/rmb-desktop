/**
 * Dev-only in-memory mock API for the full-app mock mode (lib/mockMode.ts).
 * One router serves every endpoint the SPA calls, so pages render with no
 * daemon. Mutations (config PUT, corrections, onboarding, restart) keep
 * in-memory state for the session. handleMockApi is a pure method+path→
 * {status,json} mapping so the node selftest can pin the contract; the
 * api helpers wrap it via mockApiFetch.
 */
import type { Page, PageRequest, SessionDetail } from "../types";import {
  buildMockData,
  mockOverview,
  mockPipelineHealth,
  type MockData,
} from "./data";

export interface MockResult {
  status: number;
  json: unknown;
}

const data: MockData = buildMockData();

/** Simulated restart window: /healthz answers 503 for this long after POST /system/restart. */
const RESTART_DOWN_MS = 1200;
let restartedAt = 0;

/** True unless a mock restart is still "in flight" (drives SettingsPage phases). */
export function mockHealthOk(): boolean {
  return restartedAt === 0 || Date.now() - restartedAt > RESTART_DOWN_MS;
}

export function handleMockApi(method: string, rawPath: string, body?: unknown): MockResult {
  const url = new URL(rawPath, "http://mock.local");
  const path = url.pathname.replace(/\/+$/, "") || "/";
  const params = url.searchParams;
  const ok = (json: unknown): MockResult => ({ status: 200, json });
  const notFound = (): MockResult => ({
    status: 404,
    json: { error: `mock: no route for ${method} ${rawPath}` },
  });

  // ----- system -------------------------------------------------------------
  if (path === "/healthz") return { status: mockHealthOk() ? 200 : 503, json: { ok: mockHealthOk() } };
  if (path === "/api/v1/version" && method === "GET") return ok(data.version);
  if (path === "/api/v1/system/restart" && method === "POST") {
    restartedAt = Date.now();
    return ok({ ok: true });
  }

  // ----- browse -------------------------------------------------------------
  if (path === "/api/v1/browse/overview") return ok(mockOverview(data));
  if (path === "/api/v1/browse/pipeline-health") return ok(mockPipelineHealth(data));

  if (path === "/api/v1/browse/sessions" && method === "GET") {
    const req = pageRequest(params);
    let items = [...data.sessions];
    if (req.q) {
      const q = req.q.toLowerCase();
      items = items.filter(
        (s) =>
          s.session_key.toLowerCase().includes(q) ||
          (s.abstract ?? "").toLowerCase().includes(q),
      );
    }
    items = sortByDate(items, req);
    return ok(toPage(items, req));
  }

  const sessionMatch = path.match(/^\/api\/v1\/browse\/sessions\/(.+)$/);
  if (sessionMatch && method === "GET") {
    const key = decodeURIComponent(sessionMatch[1]);
    const session = data.sessions.find((s) => s.session_key === key);
    if (!session) return { status: 404, json: { error: `mock: unknown session ${key}` } };
    const detail: SessionDetail = {
      session,
      turns: [...(data.turns.get(key) ?? [])].sort((a, b) => a.turn_index - b.turn_index),
      pipeline_state: data.pipelineStates.get(key) ?? null,
      atoms: data.atoms.get(key) ?? [],
      scenes: data.scenes.get(key) ?? [],
    };
    return ok(detail);
  }

  if (path === "/api/v1/browse/memories" && method === "GET") {
    const req = pageRequest(params);
    let items = [...data.memories];
    const category = params.get("category");
    if (category) items = items.filter((m) => m.category === category);
    if (req.q) {
      const q = req.q.toLowerCase();
      items = items.filter(
        (m) =>
          (m.abstract ?? "").toLowerCase().includes(q) ||
          m.uri.toLowerCase().includes(q) ||
          (m.body ?? "").toLowerCase().includes(q),
      );
    }
    items = sortByDate(items, req);
    return ok(toPage(items, req));
  }

  if (path === "/api/v1/browse/skills" && method === "GET") {
    const req = pageRequest(params);
    let items = [...data.skills];
    if (req.q) {
      const q = req.q.toLowerCase();
      items = items.filter(
        (s) =>
          s.name.toLowerCase().includes(q) ||
          s.slug.toLowerCase().includes(q) ||
          s.description.toLowerCase().includes(q) ||
          (s.tags ?? []).some((t) => t.toLowerCase().includes(q)),
      );
    }
    items = sortByDate(items, req);
    return ok(toPage(items, req));
  }

  const skillMatch = path.match(/^\/api\/v1\/browse\/skills\/(.+)$/);
  if (skillMatch && method === "GET") {
    const slug = decodeURIComponent(skillMatch[1]);
    const detail = data.skillDetails.get(slug);
    if (!detail) return { status: 404, json: { error: `mock: unknown skill ${slug}` } };
    return ok(detail);
  }

  // ----- corrections (mutable) ----------------------------------------------
  if (path === "/api/v1/corrections" && method === "GET") {
    const target = params.get("target");
    const items = target
      ? data.corrections.filter((c) => (c.target_uris ?? []).includes(target))
      : [...data.corrections];
    return ok({ items });
  }
  if (path === "/api/v1/corrections" && method === "POST") {
    const payload = body as { statement?: string; target_uris?: string[] };
    const uri = `rmb://corrections/c-${String(data.corrections.length + 1).padStart(3, "0")}-${Date.now()}`;
    data.corrections.push({
      uri,
      statement: payload.statement ?? "",
      target_uris: payload.target_uris ?? [],
      created_at: new Date().toISOString(),
    });
    return ok({ uri, target_uris: payload.target_uris ?? [] });
  }
  if (path === "/api/v1/corrections" && method === "DELETE") {
    const uri = params.get("uri");
    const before = data.corrections.length;
    data.corrections = data.corrections.filter((c) => c.uri !== uri);
    return ok({ uri, retracted: data.corrections.length < before });
  }

  // ----- config -------------------------------------------------------------
  if (path === "/api/v1/config" && method === "GET") return ok(data.config);
  if (path === "/api/v1/config" && method === "PUT") {
    const req = body as Parameters<typeof applyConfigUpdate>[0];
    const reembed = applyConfigUpdate(req);
    return ok({ ok: true, reembed_started: reembed, config: data.config });
  }

  // ----- config connection tests ---------------------------------------------
  if (path === "/api/v1/config/test/llm" && method === "POST") {
    void body;
    return ok({
      ok: true,
      latency_ms: 380 + Math.floor(Math.random() * 200),
      models_count: 12,
      models: ["deepseek-chat", "deepseek-reasoner"],
    });
  }
  if (path === "/api/v1/config/test/embed" && method === "POST") {
    void body;
    return ok({ ok: true, latency_ms: 240 + Math.floor(Math.random() * 120) });
  }

  // ----- onboarding -----------------------------------------------------------
  if (path === "/api/v1/onboarding/status" && method === "GET") {
    return ok({
      completed: onboardingComplete,
      marker_path: "~/.rmb/onboarding.complete",
      completed_at: onboardingComplete ? new Date(NOW_ANCHOR - 3 * 86400000).toISOString() : undefined,
    });
  }
  if (path === "/api/v1/onboarding/complete" && method === "POST") {
    onboardingComplete = true;
    return ok({ ok: true });
  }
  if (path === "/api/v1/onboarding/reset" && method === "POST") {
    onboardingComplete = false;
    return ok({ ok: true });
  }

  return notFound();
}

const NOW_ANCHOR = Date.now();
let onboardingComplete = true;

/** Mutates the in-memory config; returns whether a re-embed would be triggered. */
function applyConfigUpdate(update: {
  addr?: string;
  launch_at_login?: boolean;
  llm?: { api_base?: string; api_key?: string; model?: string; timeout?: string };
  embed?: { api_base?: string; api_key?: string; model?: string; dimensions?: number };
  pipeline?: Partial<MockData["config"]["pipeline"]>;
}): boolean {
  let reembed = false;
  if (update.addr !== undefined) data.config.addr = update.addr;
  if (update.launch_at_login !== undefined) data.config.launch_at_login = update.launch_at_login;
  if (update.llm) Object.assign(data.config.llm, update.llm);
  if (update.llm?.api_key) {
    data.config.llm.api_key_set = true;
    data.config.llm.api_key_suffix = update.llm.api_key.slice(-4);
  }
  if (update.embed) {
    if (
      update.embed.dimensions !== undefined &&
      update.embed.dimensions !== data.config.embed.dimensions
    ) {
      reembed = true;
    }
    if (update.embed.model !== undefined && update.embed.model !== data.config.embed.model) {
      reembed = true;
    }
    Object.assign(data.config.embed, update.embed);
  }
  if (update.embed?.api_key) {
    data.config.embed.api_key_set = true;
    data.config.embed.api_key_suffix = update.embed.api_key.slice(-4);
  }
  if (update.pipeline) Object.assign(data.config.pipeline, update.pipeline);
  return reembed;
}

// ----- paging helpers ---------------------------------------------------------

function pageRequest(params: URLSearchParams): PageRequest {
  return {
    limit: Number(params.get("limit") ?? 50),
    offset: Number(params.get("offset") ?? 0),
    q: params.get("q") ?? undefined,
    category: params.get("category") ?? undefined,
    sort: params.get("sort") ?? undefined,
    order: (params.get("order") as "asc" | "desc" | null) ?? "desc",
  };
}

function sortByDate<T extends { updated_at: string }>(items: T[], req: PageRequest): T[] {
  const dir = req.order === "asc" ? 1 : -1;
  const ts = (x: T) => {
    const created = (x as { created_at?: string }).created_at;
    return Date.parse(req.sort === "created" && created ? created : x.updated_at);
  };
  return items.sort((a, b) => dir * (ts(a) - ts(b)));
}

function toPage<T>(items: T[], req: PageRequest): Page<T> {
  return {
    items: items.slice(req.offset, req.offset + req.limit),
    total: items.length,
    limit: req.limit,
    offset: req.offset,
  };
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

/** fetch()-shaped wrapper: Response with the mocked status + JSON body. */
export async function mockApiFetch(path: string, init?: RequestInit): Promise<Response> {
  await sleep(60 + Math.floor(Math.random() * 120));
  const method = (init?.method ?? "GET").toUpperCase();
  let body: unknown;
  if (typeof init?.body === "string") {
    try {
      body = JSON.parse(init.body);
    } catch {
      body = init.body;
    }
  }
  const result = handleMockApi(method, path, body);
  return new Response(JSON.stringify(result.json), {
    status: result.status,
    headers: { "Content-Type": "application/json" },
  });
}
