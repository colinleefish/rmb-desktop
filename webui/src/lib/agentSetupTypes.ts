export type ChangeType = "create" | "modify" | "append" | "unchanged";
export type ApplyMode = "write" | "copy_only";
export type ArtifactStatus = "pending" | "applied" | "unchanged";

export type SetupArtifact = {
  id: string;
  title: string;
  path: string;
  description: string;
  exists: boolean;
  current: string;
  proposed: string;
  changeType: ChangeType;
  applyMode: ApplyMode;
  warnings: string[];
  language: "json" | "markdown";
};

export type AgentSetupState = {
  id: string;
  name: string;
  description: string;
  detected: boolean;
  hookStatus: "configured" | "pending" | "none";
  recallStatus: "configured" | "pending" | "none";
  lastHookAt?: string;
  artifacts: SetupArtifact[];
};
