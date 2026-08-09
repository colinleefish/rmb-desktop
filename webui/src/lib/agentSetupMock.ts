import type { AgentSetupState } from "./agentSetupTypes";

const RMB_BIN = "/Users/you/.rmb/bin/rmb";

const CURSOR_RECALL_RULE = `---
description: Use of rmb command line 
alwaysApply: true
---
ALWAYS RUN \`rmb\` cli at the start of each conversation.
Search before asking the user. Follow rmb://agent for recall rules.
`;

const RECALL_MARKDOWN_BLOCK = `# RMB memory

ALWAYS RUN \`rmb\` cli at the start of each conversation.
Search before asking the user. Follow rmb://agent for recall rules.
`;

const RMB_RECALL_BLOCK = RECALL_MARKDOWN_BLOCK;

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

const CURSOR_RULES_PROPOSED = CURSOR_RECALL_RULE;

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

const CC_CLAUDE_PROPOSED = `${CC_CLAUDE_CURRENT}

${RMB_RECALL_BLOCK}`;

const CODEX_HOOKS_CURRENT = `{
  "description": "Optional lifecycle hooks for this workspace.",
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/bin/python3 \\"$(git rev-parse --show-toplevel)/.codex/hooks/pre_tool_use_policy.py\\"",
            "statusMessage": "Checking Bash command"
          }
        ]
      }
    ]
  }
}`;

const CODEX_HOOKS_PROPOSED = `{
  "description": "Optional lifecycle hooks for this workspace.",
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/bin/python3 \\"$(git rev-parse --show-toplevel)/.codex/hooks/pre_tool_use_policy.py\\"",
            "statusMessage": "Checking Bash command"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "${RMB_BIN} hook-submit --source=codex",
            "timeout": 15
          }
        ]
      }
    ]
  }
}`;

const CODEX_AGENTS_CURRENT = `# Project notes

Follow team coding standards.`;

const CODEX_AGENTS_PROPOSED = `${CODEX_AGENTS_CURRENT}

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
        description: "",
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
        path: "Customize → Rules",
        description: "",
        exists: false,
        current: "",
        proposed: CURSOR_RULES_PROPOSED,
        changeType: "create",
        applyMode: "copy_only",
        warnings: [],
        language: "markdown",
      },
    ],
  },
  {
    id: "claude-code",
    name: "Claude Code",
    description: "Stop hook in settings.json plus recall block in CLAUDE.md.",
    detected: true,
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
    description: "Stop hook in hooks.json plus recall block in AGENTS.md.",
    detected: true,
    hookStatus: "none",
    recallStatus: "none",
    artifacts: [
      {
        id: "hooks",
        title: "Conversation capture",
        path: "~/.codex/hooks.json",
        description: "Adds a Stop hook that submits conversation turns to rmbd.",
        exists: true,
        current: CODEX_HOOKS_CURRENT,
        proposed: CODEX_HOOKS_PROPOSED,
        changeType: "modify",
        applyMode: "write",
        warnings: [],
        language: "json",
      },
      {
        id: "agents_md",
        title: "Recall instructions",
        path: "~/.codex/AGENTS.md",
        description: "User-level instructions read by Codex at session start.",
        exists: true,
        current: CODEX_AGENTS_CURRENT,
        proposed: CODEX_AGENTS_PROPOSED,
        changeType: "append",
        applyMode: "write",
        warnings: [],
        language: "markdown",
      },
    ],
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
