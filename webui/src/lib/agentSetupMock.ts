import type { AgentSetupState } from "./agentSetupTypes";

const RMB_BIN = "/Users/you/.rmb/bin/rmb";

const RMB_RECALL_BLOCK = `<!-- rmb:recall:start -->
ALWAYS RUN \`rmb\` cli at the start of each conversation.
Search before asking the user. Follow rmb://agent for recall rules.
<!-- rmb:recall:end -->`;

const CURSOR_HOOKS_CURRENT = `{
  "version": 1,
  "hooks": {
    "afterFileEdit": [
      {
        "command": "./hooks/format.sh"
      }
    ]
  }
}`;

const CURSOR_HOOKS_PROPOSED = `{
  "version": 1,
  "hooks": {
    "afterFileEdit": [
      {
        "command": "./hooks/format.sh"
      }
    ],
    "stop": [
      {
        "command": "${RMB_BIN} hook-submit --source=cursor",
        "timeout": 15
      }
    ]
  }
}`;

const CURSOR_RULES_CURRENT = `Follow ALL user, tool, system, and skill instructions precisely.

When communicating with the user:
- Use code citation blocks to reference existing code
- Prefer simple, accessible language`;

const CURSOR_RULES_PROPOSED = `${CURSOR_RULES_CURRENT}

${RMB_RECALL_BLOCK}`;

const CC_SETTINGS_CURRENT = `{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:15721"
  },
  "model": "sonnet"
}`;

const CC_SETTINGS_PROPOSED = `{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:15721"
  },
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "command": "${RMB_BIN} hook-submit --source=cc",
            "timeout": 15,
            "type": "command"
          }
        ]
      }
    ]
  },
  "model": "sonnet"
}`;

const CC_CLAUDE_CURRENT = `# Project notes

Use conventional commits for this repo.`;

const CC_CLAUDE_PROPOSED = `# Project notes

Use conventional commits for this repo.

# rmb — recall guide for AI agents

At the start of each session, run \`rmb\` to load profile, memory, and skills before you ask questions or take action.

${RMB_RECALL_BLOCK}`;

export const MOCK_AGENTS: AgentSetupState[] = [
  {
    id: "cursor",
    name: "Cursor",
    description: "Capture conversations via stop hook and teach the agent to recall with rmb.",
    detected: true,
    hookStatus: "pending",
    recallStatus: "none",
    artifacts: [
      {
        id: "hooks",
        title: "Conversation capture",
        path: "~/.cursor/hooks.json",
        description: "Runs rmb hook-submit when an agent conversation ends.",
        exists: true,
        current: CURSOR_HOOKS_CURRENT,
        proposed: CURSOR_HOOKS_PROPOSED,
        changeType: "modify",
        applyMode: "write",
        warnings: [],
        language: "json",
      },
      {
        id: "user_rules",
        title: "Recall instructions",
        path: "Cursor Settings → Rules",
        description: "Teaches the agent to run rmb search and cat at session start.",
        exists: true,
        current: CURSOR_RULES_CURRENT,
        proposed: CURSOR_RULES_PROPOSED,
        changeType: "append",
        applyMode: "copy_only",
        warnings: [
          "Cursor stores user rules internally — preview only, copy to paste into Settings → Rules.",
        ],
        language: "markdown",
      },
    ],
  },
  {
    id: "claude-code",
    name: "Claude Code",
    description: "Stop hook in settings.json plus recall block in CLAUDE.md.",
    detected: false,
    hookStatus: "none",
    recallStatus: "none",
    artifacts: [
      {
        id: "settings",
        title: "Conversation capture",
        path: "~/.claude/settings.json",
        description: "Adds a Stop hook that submits conversation turns to rmbd.",
        exists: true,
        current: CC_SETTINGS_CURRENT,
        proposed: CC_SETTINGS_PROPOSED,
        changeType: "modify",
        applyMode: "write",
        warnings: [
          "env values are redacted in preview. Only the Stop hook entry will change.",
        ],
        language: "json",
      },
      {
        id: "claude_md",
        title: "Recall instructions",
        path: "~/.claude/CLAUDE.md",
        description: "User-level instructions read by Claude Code at session start.",
        exists: true,
        current: CC_CLAUDE_CURRENT,
        proposed: CC_CLAUDE_PROPOSED,
        changeType: "append",
        applyMode: "write",
        warnings: [],
        language: "markdown",
      },
    ],
  },
  {
    id: "codex",
    name: "Codex",
    description: "Stop hook in Codex settings plus recall instructions.",
    detected: false,
    hookStatus: "none",
    recallStatus: "none",
    artifacts: [],
  },
  {
    id: "opencode",
    name: "OpenCode",
    description: "Stop hook in OpenCode config plus recall instructions.",
    detected: false,
    hookStatus: "none",
    recallStatus: "none",
    artifacts: [],
  },
  {
    id: "pi",
    name: "Pi",
    description: "Stop hook in Pi config plus recall instructions.",
    detected: false,
    hookStatus: "none",
    recallStatus: "none",
    artifacts: [],
  },
];

export function getMockAgent(id: string): AgentSetupState | undefined {
  return MOCK_AGENTS.find((a) => a.id === id);
}

export function listMockAgents(): AgentSetupState[] {
  return MOCK_AGENTS;
}
