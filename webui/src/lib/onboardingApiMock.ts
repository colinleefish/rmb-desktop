import type { ConfigTestPayload, ConfigTestResult } from "./onboardingApi";
import { saveModelsDraft } from "./onboardingState";

function delay(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function testSide(
  label: string,
  apiBase: string,
  apiKey: string,
  model: string,
): ConfigTestResult["llm"] {
  if (!apiBase.trim()) {
    return { ok: false, error: `${label}: API base URL is required` };
  }
  if (!apiKey.trim()) {
    return { ok: false, error: `${label}: API key is required` };
  }
  if (!model.trim()) {
    return { ok: false, error: `${label}: model is required` };
  }
  if (apiKey.trim().length < 8) {
    return { ok: false, error: `${label}: API key looks too short (demo check)` };
  }
  return { ok: true, latency_ms: 180 + Math.floor(Math.random() * 320) };
}

export async function testConfig(payload: ConfigTestPayload): Promise<ConfigTestResult> {
  await delay(600);
  return {
    llm: testSide("LLM", payload.llm.api_base, payload.llm.api_key, payload.llm.model),
    embed: testSide(
      "Embedding",
      payload.embed.api_base,
      payload.embed.api_key,
      payload.embed.model,
    ),
  };
}

export async function saveOnboardingModels(payload: ConfigTestPayload): Promise<void> {
  await delay(200);
  saveModelsDraft({
    llmBase: payload.llm.api_base,
    llmModel: payload.llm.model,
    llmKey: payload.llm.api_key,
    embedBase: payload.embed.api_base,
    embedModel: payload.embed.model,
    embedKey: payload.embed.api_key,
    embedDims: payload.embed.dimensions,
  });
}
