export interface OverviewCounts {
  sessions: number;
  turns: number;
  atoms: number;
  scenes: number;
  memories: number;
  pipeline_states: number;
  tasks: number;
  corrections: number;
  skills: number;
}

export interface MemoryCategoryOverview {
  profile_version: number;
  events: number;
  preferences: number;
  entities: number;
}

export interface Overview {
  counts: OverviewCounts;
  memory_by_category: MemoryCategoryOverview;
}

export interface SessionRow {
  id: string;
  session_key: string;
  source?: string | null;
  status: string;
  abstract: string | null;
  turn_count: number;
  atom_count: number;
  scene_count: number;
  t1_status?: string;
  t2_status?: string;
  t3_status?: string;
  uri: string;
  created_at: string;
  updated_at: string;
  last_turn_at: string | null;
}

export interface TurnRow {
  id: string;
  turn_index: number;
  uri: string;
  messages_jsonl: string;
  created_at: string;
  updated_at: string;
}

export interface PipelineState {
  session_id: string;
  t1_status: string;
  t2_status: string;
  t3_status: string;
  t1_advanced_at?: string | null;
  t2_advanced_at?: string | null;
  t3_advanced_at?: string | null;
  t1_turns_since_advanced: number;
  warmup_threshold: number;
  updated_at: string;
}

export interface AtomRow {
  id: string;
  session_id: string;
  category: string;
  priority: number;
  scene_name?: string | null;
  slug?: string | null;
  content: string;
  source_turn_ids: string[];
  created_at: string;
  updated_at: string;
  uri?: string;
}

export interface RecallStats {
  uri?: string;
  search_count: number;
  cat_count: number;
  meta_count: number;
  last_searched_at?: string | null;
  last_cated_at?: string | null;
  last_metaed_at?: string | null;
  updated_at?: string;
}

export interface SceneRow {
  id: string;
  session_id: string;
  display_name?: string | null;
  abstract?: string | null;
  body?: string | null;
  source_atoms: string[];
  created_at: string;
  updated_at: string;
  uri?: string;
  recall_stats?: RecallStats | null;
}

export interface MemoryRow {
  id: string;
  uri: string;
  category: string;
  slug?: string | null;
  version: number;
  abstract?: string | null;
  body?: string | null;
  source_scene_uris: string[];
  source_correction_uris: string[];
  created_at: string;
  updated_at: string;
  recall_stats?: RecallStats | null;
}

export interface CorrectionRow {
  uri: string;
  statement: string;
  created_at: string;
  target_uris?: string[];
}

export interface PipelineStateRow extends PipelineState {
  session_key: string;
  session_uri: string;
}

export interface SessionDetail {
  session: SessionRow;
  turns: TurnRow[];
  pipeline_state: PipelineState | null;
  atoms: AtomRow[];
  scenes: SceneRow[];
}

export interface SkillRow {
  slug: string;
  name: string;
  description: string;
  tags?: string[];
  uri: string;
  version: number;
  updated_at: string;
  recall_stats?: RecallStats | null;
}

export interface SkillFileNode {
  name: string;
  path: string;
  type: "file" | "dir";
  children?: SkillFileNode[];
}

export interface SkillDetail {
  skill: {
    uri: string;
    slug: string;
    name: string;
    description: string;
    tags?: string[];
    version: number;
    bundle_sha256: string;
    created_at: string;
    updated_at: string;
  };
  tree: SkillFileNode[];
  files: Record<string, string>;
}

export interface TaskRow {
  id?: string;
  kind?: string;
  status?: string;
  session_id?: string;
  progress?: number;
  error?: string;
  result_uri?: string;
  created_at?: string;
  updated_at?: string;
}

export interface Page<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface ConfigView {
  addr: string;
  db_path: string;
  config_path: string;
  distillation_enabled: boolean;
  launch_at_login: boolean;
  llm: {
    api_base: string;
    api_key_set: boolean;
    model: string;
  };
  embed: {
    api_base: string;
    api_key_set: boolean;
    model: string;
    dimensions: number;
  };
  pipeline: {
    l1_poll_interval: string;
    l2_poll_interval: string;
    l3_poll_interval: string;
    embed_poll_interval: string;
    l1_every_n: number;
    l1_idle_seconds: string;
    l1_warmup: boolean;
    l2_delay_after_l1: string;
    l1_max_turns_per_batch: number;
    l1_max_chars_per_batch: number;
    l2_max_atoms_per_batch: number;
    l3_max_atoms_per_batch: number;
    embed_batch_size: number;
  };
}

export interface ConfigUpdateRequest {
  addr?: string;
  launch_at_login?: boolean;
  llm?: {
    api_base?: string;
    api_key?: string;
    model?: string;
  };
  embed?: {
    api_base?: string;
    api_key?: string;
    model?: string;
    dimensions?: number;
  };
  pipeline?: Partial<ConfigView["pipeline"]>;
}
