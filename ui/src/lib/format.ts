export interface ChatMessage {
  role?: string;
  content?: unknown;
}

export function truncate(text: string, max: number): string {
  if (text.length <= max) return text;
  return `${text.slice(0, max - 1)}…`;
}

/** Format an ISO timestamp in the browser's local timezone. */
export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

/** Normalize message content to displayable text. */
export function messageContent(content: unknown): string {
  if (content == null) return "";
  if (typeof content === "string") return content;
  if (Array.isArray(content)) {
    return content
      .map((part) => {
        if (typeof part === "string") return part;
        if (part && typeof part === "object") {
          const record = part as Record<string, unknown>;
          if (typeof record.text === "string") return record.text;
          if (typeof record.content === "string") return record.content;
        }
        return JSON.stringify(part);
      })
      .filter(Boolean)
      .join("\n");
  }
  if (typeof content === "object") {
    return JSON.stringify(content, null, 2);
  }
  return String(content);
}

/** Parse stored turn messages — JSON array (upload) or JSONL (legacy). */
export function parseTurnMessages(raw: string | null | undefined): ChatMessage[] {
  if (!raw?.trim()) return [];
  const trimmed = raw.trim();

  if (trimmed.startsWith("[")) {
    try {
      const parsed = JSON.parse(trimmed) as unknown;
      if (Array.isArray(parsed)) {
        return parsed.map((item) => {
          if (item && typeof item === "object") {
            const record = item as Record<string, unknown>;
            return {
              role: typeof record.role === "string" ? record.role : undefined,
              content: record.content,
            };
          }
          return { role: "message", content: item };
        });
      }
    } catch {
      /* fall through */
    }
  }

  const messages: ChatMessage[] = [];
  for (const line of trimmed.split("\n")) {
    const lineTrimmed = line.trim();
    if (!lineTrimmed) continue;
    try {
      messages.push(JSON.parse(lineTrimmed) as ChatMessage);
    } catch {
      messages.push({ role: "message", content: lineTrimmed });
    }
  }
  return messages;
}

export function stripUserQueryTags(text: string): string {
  return text
    .replace(/^\s*<user_query>\s*/i, "")
    .replace(/\s*<\/user_query>\s*$/i, "")
    .trim();
}

export function formatTurnMessage(
  role: string | undefined,
  content: unknown,
): { aside: string | null; body: string } {
  const normalizedRole = (role ?? "").toLowerCase();
  let body = stripUserQueryTags(messageContent(content)).trim();
  body = body.replace(/\n*\[REDACTED\]\s*$/gi, "").trim();
  if (!body) return { aside: null, body: "" };

  if (normalizedRole === "assistant") {
    const match = body.match(/^(You're saying[^\n]+)\n\n([\s\S]+)$/);
    if (match) {
      return { aside: match[1].trim(), body: match[2].trim() };
    }
  }
  return { aside: null, body };
}

export function turnMessagePreview(messages: ChatMessage[]): string {
  const user = messages.find((m) => (m.role ?? "").toLowerCase() === "user");
  const text = formatTurnMessage(user?.role, user?.content).body;
  if (text) return truncate(text.replace(/\s+/g, " "), 120);

  const last = [...messages].reverse().find((m) => messageContent(m.content).trim());
  if (!last) return "—";
  const role = last.role ? `${last.role}: ` : "";
  return truncate(`${role}${messageContent(last.content).replace(/\s+/g, " ").trim()}`, 120);
}

export function shortKey(key: string, len = 8): string {
  if (key.length <= len) return key;
  return `${key.slice(0, len)}…`;
}

export function sessionSourceLabel(source: string | null | undefined): string {
  switch ((source ?? "").toLowerCase()) {
    case "cursor":
      return "Cursor";
    case "cc":
    case "claude":
      return "Claude Code";
    case "codex":
      return "Codex";
    case "pi":
      return "Pi";
    case "opencode":
      return "OpenCode";
    case "workbuddy":
      return "WorkBuddy";
    default:
      return source?.trim() || "—";
  }
}

export function sessionSourceShortLabel(source: string | null | undefined): string {
  switch ((source ?? "").toLowerCase()) {
    case "cursor":
      return "Cursor";
    case "cc":
    case "claude":
      return "CC";
    case "codex":
      return "Codex";
    case "pi":
      return "Pi";
    case "opencode":
      return "Open";
    case "workbuddy":
      return "WB";
    default:
      return sessionSourceLabel(source);
  }
}

export function turnRoleLabel(role: string | undefined): string {
  switch ((role ?? "").toLowerCase()) {
    case "user":
      return "You";
    case "assistant":
      return "Assistant";
    case "system":
      return "System";
    case "tool":
      return "Tool";
    default:
      return role?.trim() || "Message";
  }
}

/** @deprecated use parseTurnMessages */
export const parseJSONL = parseTurnMessages;
