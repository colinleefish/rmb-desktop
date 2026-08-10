/**
 * rmb memory capture for OpenCode
 *
 * On session idle, fetches the latest user/assistant turn and pipes a JSON
 * payload to `rmb hook-submit --source=opencode`.
 *
 * Install: written by rmb-desktop setup to ~/.config/opencode/plugin/rmb-hook.ts
 *
 * Env:
 *   RMB_HOOK_BIN  optional override for rmb binary path
 *   RMB_URL       target API (read by rmb hook-submit)
 */

import type { Plugin } from "@opencode-ai/plugin";

const DEFAULT_RMB_HOOK_BIN = __RMB_HOOK_BIN_JSON__;

type SessionMessage = {
	info: { id?: string; role: "user" | "assistant" };
	parts: Array<{ type: string; text?: string }>;
};

const lastUploadedAssistantID = new Map<string, string>();

function rmbBinPath(): string {
	return process.env.RMB_HOOK_BIN?.trim() || DEFAULT_RMB_HOOK_BIN;
}

function extractTextParts(parts: SessionMessage["parts"]): string {
	return parts
		.filter((part) => part.type === "text" && part.text?.trim())
		.map((part) => part.text!.trim())
		.join("\n");
}

function findLastTurn(messages: SessionMessage[]): {
	user: string;
	assistant: string;
	assistantMessageID: string;
} {
	let user = "";
	let assistant = "";
	let assistantMessageID = "";
	for (const message of messages) {
		const text = extractTextParts(message.parts);
		if (!text) continue;
		if (message.info.role === "user") user = text;
		if (message.info.role === "assistant") {
			assistant = text;
			assistantMessageID = message.info.id?.trim() ?? "";
		}
	}
	return { user, assistant, assistantMessageID };
}

function sessionIDFromEvent(event: { type: string; properties?: Record<string, unknown> }): string | null {
	const props = event.properties;
	if (!props) return null;
	const id = props.sessionID ?? props.session_id;
	return typeof id === "string" && id.trim() ? id.trim() : null;
}

function isIdleEvent(event: { type: string; properties?: Record<string, unknown> }): boolean {
	if (event.type !== "session.status") return false;
	const status = event.properties?.status as { type?: string } | undefined;
	return status?.type === "idle";
}

async function fetchSessionMessages(
	client: { session: { messages: (opts: Record<string, unknown>) => Promise<{ data?: SessionMessage[] }> } },
	sessionID: string,
	directory: string,
): Promise<SessionMessage[]> {
	for (const path of [{ sessionID }, { id: sessionID }]) {
		try {
			const response = await client.session.messages({
				path,
				query: { directory },
			});
			if (Array.isArray(response.data)) return response.data;
		} catch {
			// try the other path key shape (SDK v1 vs v2)
		}
	}
	return [];
}

async function submitHook(payload: Record<string, unknown>): Promise<void> {
	const rmbBin = rmbBinPath();
	const json = JSON.stringify(payload);
	const proc = Bun.spawn([rmbBin, "hook-submit", "--source=opencode"], {
		stdin: new Blob([json]),
		stdout: "ignore",
		stderr: "pipe",
		env: { ...process.env },
	});
	const exitCode = await proc.exited;
	if (exitCode !== 0) {
		const errText = await new Response(proc.stderr).text();
		throw new Error(errText.trim() || `rmb hook-submit exited ${exitCode}`);
	}
}

const server: Plugin = async ({ client, directory }) => {
	return {
		event: async ({ event }) => {
			if (!isIdleEvent(event)) return;

			const sessionID = sessionIDFromEvent(event);
			if (!sessionID) return;

			try {
				const messages = await fetchSessionMessages(client, sessionID, directory);
				const { user, assistant, assistantMessageID } = findLastTurn(messages);
				if (!assistant) return;

				const dedupeKey = assistantMessageID || `${user}\0${assistant}`;
				if (lastUploadedAssistantID.get(sessionID) === dedupeKey) return;
				lastUploadedAssistantID.set(sessionID, dedupeKey);

				const home = process.env.HOME?.trim();
				const sessionDBPath = home ? `${home}/.local/share/opencode/opencode.db` : undefined;

				await submitHook({
					agent: "opencode",
					session_id: sessionID,
					last_user_message: user,
					last_assistant_message: assistant,
					session_db_path: sessionDBPath,
					cwd: directory,
					hook_event_name: event.type,
				});
			} catch (error) {
				const message = error instanceof Error ? error.message : String(error);
				try {
					await client.app.log({
						service: "rmb-hook",
						level: "error",
						message: "hook-submit failed",
						extra: { sessionID, error: message },
					});
				} catch {
					console.error(`[rmb-hook] upload failed for ${sessionID}: ${message}`);
				}
			}
		},
	};
};

export const RmbHook = server;

export default {
	id: "rmb-hook",
	server,
};
