import type { ConfigView, ConfigUpdateRequest } from "./types";
import { isPipelineMocked } from "./pipelineMock";
import { mockPipelineHealth } from "./pipelineHealthMock";
import { mockOverview } from "./overviewMock";

const API = "/api/v1";

async function apiGet<T>(path: string): Promise<T> {
  const res = await fetch(`${API}${path}`, {
    headers: { Accept: "application/json" },
  });
  const body = (await res.json().catch(() => ({}))) as T | { error?: string };
  if (!res.ok) {
    const message =
      (body as { error?: string }).error ?? res.statusText ?? "request failed";
    throw new Error(message);
  }
  return body as T;
}

async function apiPut<T>(path: string, payload: unknown): Promise<T> {
  const res = await fetch(`${API}${path}`, {
    method: "PUT",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });
  const body = (await res.json().catch(() => ({}))) as T | { error?: string };
  if (!res.ok) {
    const message =
      (body as { error?: string }).error ?? res.statusText ?? "request failed";
    throw new Error(message);
  }
  return body as T;
}

async function apiSend<T>(method: string, path: string, payload?: unknown): Promise<T> {
  const res = await fetch(`${API}${path}`, {
    method,
    headers: {
      Accept: "application/json",
      ...(payload !== undefined ? { "Content-Type": "application/json" } : {}),
    },
    body: payload !== undefined ? JSON.stringify(payload) : undefined,
  });
  const body = (await res.json().catch(() => ({}))) as T | { error?: string };
  if (!res.ok) {
    const message =
      (body as { error?: string }).error ?? res.statusText ?? "request failed";
    throw new Error(message);
  }
  return body as T;
}

export function getOverview() {
  if (isPipelineMocked()) {
    return Promise.resolve(mockOverview());
  }
  return apiGet<import("./types").Overview>("/browse/overview");
}

export function getPipelineHealth() {
  if (isPipelineMocked()) {
    return Promise.resolve(mockPipelineHealth());
  }
  return apiGet<import("./types").PipelineHealth>("/browse/pipeline-health");
}

export function getConfig(): Promise<ConfigView> {
  return apiGet<ConfigView>("/config");
}

export function getVersion(): Promise<{ version: string; commit: string }> {
  return apiGet<{ version: string; commit: string }>("/version");
}

export function putConfig(
  update: ConfigUpdateRequest,
): Promise<{ ok: boolean; reembed_started: boolean; config: ConfigView }> {
  return apiPut("/config", update);
}

export function postRestart(): Promise<{ ok: boolean }> {
  return apiSend<{ ok: boolean }>("POST", "/system/restart");
}

export type RestartPhase = "stopping" | "waiting" | "done";

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function healthOk(): Promise<boolean> {
  try {
    const res = await fetch("/healthz", { cache: "no-store" });
    return res.ok;
  } catch {
    return false;
  }
}

async function waitForHealthDown(timeoutMs = 5000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (!(await healthOk())) return;
    await sleep(200);
  }
}

async function waitForHealthUp(timeoutMs = 20000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await healthOk()) return;
    await sleep(400);
  }
  throw new Error("restart timeout");
}

export async function restartService(onPhase?: (phase: RestartPhase) => void): Promise<void> {
  onPhase?.("stopping");
  await postRestart();
  await sleep(500);

  onPhase?.("waiting");
  await waitForHealthDown();
  await waitForHealthUp();

  onPhase?.("done");
  await sleep(1000);
}

export interface PageRequest {
  limit: number;
  offset: number;
  q?: string;
  category?: string;
  sort?: string;
  order?: "asc" | "desc";
}

export interface Page<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

async function listPage<T>(path: string, req: PageRequest): Promise<Page<T>> {
  const params = new URLSearchParams({
    limit: String(req.limit),
    offset: String(req.offset),
  });
  if (req.q) params.set("q", req.q);
  if (req.category) params.set("category", req.category);
  if (req.sort) {
    params.set("sort", req.sort);
    params.set("order", req.order ?? "desc");
  }
  const page = await apiGet<Page<T>>(`${path}?${params.toString()}`);
  return { ...page, items: page.items ?? [] };
}

export const pageSessions = (req: PageRequest) =>
  listPage<import("./types").SessionRow>("/browse/sessions", req);

export function getSession(sessionKey: string) {
  return apiGet<import("./types").SessionDetail>(
    `/browse/sessions/${encodeURIComponent(sessionKey)}`,
  );
}

export const pageMemories = (req: PageRequest) =>
  listPage<import("./types").MemoryRow>("/browse/memories", req);

export const pageSkills = (req: PageRequest) =>
  listPage<import("./types").SkillRow>("/browse/skills", req);

export function getSkill(slug: string) {
  return apiGet<import("./types").SkillDetail>(
    `/browse/skills/${encodeURIComponent(slug)}`,
  );
}

export function listCorrections(target: string) {
  const params = new URLSearchParams({ limit: "200", offset: "0", target });
  return apiGet<{ items: import("./types").CorrectionRow[] }>(
    `/corrections?${params.toString()}`,
  ).then((page) => page.items ?? []);
}

export function createCorrection(input: {
  statement: string;
  target_uris: string[];
}) {
  return apiSend<{ uri: string; target_uris: string[] }>("POST", "/corrections", {
    target_uris: input.target_uris,
    statement: input.statement,
  });
}

export function retractCorrection(uri: string) {
  return apiSend<{ uri: string; retracted: boolean }>(
    "DELETE",
    `/corrections?uri=${encodeURIComponent(uri)}`,
  );
}
