import type {
  AgentSetupState,
  ApplyMode,
  ChangeType,
  SetupArtifact,
} from "./agentSetupTypes";
import * as mock from "./setupApiMock";
import { isSetupMocked } from "./setupMock";

export { isSetupMocked } from "./setupMock";

const API = "/api/v1";

type ApiArtifact = {
  id: string;
  title: string;
  path: string;
  description: string;
  exists: boolean;
  current: string;
  proposed: string;
  change_type: ChangeType;
  apply_mode: ApplyMode;
  warnings: string[];
  language: "json" | "markdown";
};

type ApiAgent = {
  id: string;
  name: string;
  description: string;
  detected: boolean;
  hook_status: "configured" | "pending" | "none";
  recall_status: "configured" | "pending" | "none";
  artifacts: ApiArtifact[];
};

type ApiStatusResponse = {
  agents: Array<{
    id: string;
    name: string;
    detected: boolean;
    hook_status: "configured" | "pending" | "none";
    recall_status: "configured" | "pending" | "none";
  }>;
};

function mapArtifact(a: ApiArtifact): SetupArtifact {
  return {
    id: a.id,
    title: a.title,
    path: a.path,
    description: a.description,
    exists: a.exists,
    current: a.current,
    proposed: a.proposed,
    changeType: a.change_type,
    applyMode: a.apply_mode,
    warnings: a.warnings ?? [],
    language: a.language,
  };
}

function mapAgent(a: ApiAgent): AgentSetupState {
  return {
    id: a.id,
    name: a.name,
    description: a.description,
    detected: a.detected,
    hookStatus: a.hook_status,
    recallStatus: a.recall_status,
    artifacts: (a.artifacts ?? []).map(mapArtifact),
  };
}

async function setupGet<T>(path: string): Promise<T> {
  const res = await fetch(`${API}${path}`, { headers: { Accept: "application/json" } });
  const body = (await res.json().catch(() => ({}))) as T | { error?: string };
  if (!res.ok) {
    throw new Error((body as { error?: string }).error ?? res.statusText);
  }
  return body as T;
}

async function setupPost<T>(path: string, payload: unknown): Promise<T> {
  const res = await fetch(`${API}${path}`, {
    method: "POST",
    headers: { Accept: "application/json", "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const body = (await res.json().catch(() => ({}))) as T | { error?: string };
  if (!res.ok) {
    throw new Error((body as { error?: string }).error ?? res.statusText);
  }
  return body as T;
}

export async function fetchSetupStatus(): Promise<AgentSetupState[]> {
  if (isSetupMocked()) return mock.fetchSetupStatus();
  const status = await setupGet<ApiStatusResponse>("/setup/status");
  const agents = await Promise.all(
    status.agents.map(async (summary) => {
      const preview = await setupGet<{ agent: ApiAgent }>(
        `/setup/${encodeURIComponent(summary.id)}/preview`,
      );
      return mapAgent(preview.agent);
    }),
  );
  return agents;
}

export async function fetchAgentPreview(agentId: string): Promise<AgentSetupState> {
  if (isSetupMocked()) return mock.fetchAgentPreview(agentId);
  const preview = await setupGet<{ agent: ApiAgent }>(
    `/setup/${encodeURIComponent(agentId)}/preview`,
  );
  return mapAgent(preview.agent);
}

export async function applySetupArtifact(
  agentId: string,
  artifactId: string,
): Promise<AgentSetupState> {
  if (isSetupMocked()) return mock.applySetupArtifact(agentId, artifactId);
  const res = await setupPost<{ agent: ApiAgent }>(
    `/setup/${encodeURIComponent(agentId)}/apply`,
    { artifacts: [artifactId] },
  );
  return mapAgent(res.agent);
}
