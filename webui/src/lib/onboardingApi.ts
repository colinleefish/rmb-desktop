import { putConfig } from "./api";
import * as mock from "./onboardingApiMock";
import { isOnboardingDemo } from "./onboardingMock";

export type ConfigTestPayload = {
  llm: {
    api_base: string;
    api_key: string;
    model: string;
  };
  embed: {
    api_base: string;
    api_key: string;
    model: string;
    dimensions: number;
  };
};

export type ConfigTestSide = {
  ok: boolean;
  latency_ms?: number;
  error?: string;
  requested_model?: string;
  model_found?: boolean;
  models_count?: number;
  models?: string[];
};

export type ConfigTestResult = {
  llm: ConfigTestSide;
  embed: ConfigTestSide;
};

function connectionTestError(res: Response, body: { error?: string }): Error | null {
  if (res.status === 405) {
    return new Error(
      "Connection test API not available — rebuild and restart rmbd (make build, then restart the desktop app or make run-rmbd).",
    );
  }
  if (res.status === 502 || res.status === 503) {
    return new Error(
      "Cannot reach rmbd — start it with make run-rmbd or launch the desktop app (expected at http://127.0.0.1:19019).",
    );
  }
  if (!res.ok) {
    return new Error(body.error ?? res.statusText);
  }
  return null;
}

async function postConfigTestSide(path: string, body: unknown): Promise<ConfigTestSide> {
  const res = await fetch(path, {
    method: "POST",
    headers: { Accept: "application/json", "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  const parsed = (await res.json().catch(() => ({}))) as ConfigTestSide | { error?: string };
  const err = connectionTestError(res, parsed as { error?: string });
  if (err) {
    throw err;
  }
  return parsed as ConfigTestSide;
}

export async function testLLMConfig(llm: ConfigTestPayload["llm"]): Promise<ConfigTestSide> {
  return postConfigTestSide("/api/v1/config/test/llm", llm);
}

export async function testEmbedConfig(embed: ConfigTestPayload["embed"]): Promise<ConfigTestSide> {
  return postConfigTestSide("/api/v1/config/test/embed", embed);
}

export async function testConfig(payload: ConfigTestPayload): Promise<ConfigTestResult> {
  const [llm, embed] = await Promise.all([
    testLLMConfig(payload.llm),
    testEmbedConfig(payload.embed),
  ]);
  return { llm, embed };
}

export async function saveOnboardingModels(payload: ConfigTestPayload): Promise<void> {
  if (isOnboardingDemo()) {
    await mock.saveOnboardingModels(payload);
    return;
  }
  await putConfig({
    llm: {
      api_base: payload.llm.api_base,
      api_key: payload.llm.api_key,
      model: payload.llm.model,
    },
    embed: {
      api_base: payload.embed.api_base,
      api_key: payload.embed.api_key,
      model: payload.embed.model,
      dimensions: payload.embed.dimensions,
    },
  });
}
