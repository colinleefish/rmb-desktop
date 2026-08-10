/**
 * rmb memory capture for Pi
 *
 * On agent_settled, pipes a hook payload to `rmb hook-submit --source=pi`.
 * Install: written by rmb-desktop setup to ~/.pi/agent/extensions/rmb-hook.ts
 *
 * Env:
 *   RMB_HOOK_BIN  optional override for rmb binary path
 *   RMB_URL       target API (read by rmb hook-submit)
 */

import type { ExtensionAPI, ExtensionContext } from "@earendil-works/pi-coding-agent";
import type { AssistantMessage } from "@earendil-works/pi-ai";

const DEFAULT_RMB_HOOK_BIN = __RMB_HOOK_BIN_JSON__;

function rmbBinPath(): string {
	return process.env.RMB_HOOK_BIN?.trim() || DEFAULT_RMB_HOOK_BIN;
}

function extractAssistantText(message: AssistantMessage): string {
	const parts: string[] = [];
	for (const block of message.content) {
		if (block.type === "text" && block.text?.trim()) {
			parts.push(block.text.trim());
		}
	}
	return parts.join("\n");
}

function findLastAssistantText(ctx: ExtensionContext): string {
	for (const entry of [...ctx.sessionManager.getBranch()].reverse()) {
		if (entry.type !== "message") continue;
		if (entry.message.role !== "assistant") continue;
		const text = extractAssistantText(entry.message as AssistantMessage);
		if (text) return text;
	}
	return "";
}

async function submitHook(pi: ExtensionAPI, payload: Record<string, unknown>): Promise<void> {
	const rmbBin = rmbBinPath();
	const json = JSON.stringify(payload);
	const script = [
		"const {spawnSync}=require('child_process');",
		`const input=${JSON.stringify(json)};`,
		`const r=spawnSync(${JSON.stringify(rmbBin)}, ['hook-submit','--source=pi'], {input, encoding:'utf8', timeout:5000});`,
		"if (r.error) process.exit(1);",
		"process.exit(r.status ?? 1);",
	].join("");
	await pi.exec("node", ["-e", script], { timeout: 6000 });
}

export default function (pi: ExtensionAPI) {
	pi.on("agent_settled", async (_event, ctx) => {
		const sessionID = ctx.sessionManager.getSessionId();
		const sessionFile = ctx.sessionManager.getSessionFile();
		if (!sessionID || !sessionFile) return;

		const assistant = findLastAssistantText(ctx);
		if (!assistant) return;

		const payload = {
			agent: "pi",
			session_id: sessionID,
			session_file: sessionFile,
			transcript_path: sessionFile,
			cwd: ctx.sessionManager.getCwd(),
			last_assistant_message: assistant,
			hook_event_name: "agent_settled",
		};

		try {
			await submitHook(pi, payload);
		} catch {
			// Best-effort capture; never block Pi on memory upload.
		}
	});
}
