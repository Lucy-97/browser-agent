export type Job = {
  job_id: string;
  job_type: string;
  adapter: string;
  status: string;
  priority: number;
  created_at: string;
  updated_at: string;
  input?: Record<string, unknown>;
};

export type Run = {
  run_id: string;
  job_id: string;
  device_id: string;
  status: string;
  current_step?: string;
  started_at: string;
  completed_at?: string;
  last_heartbeat_at?: string;
  error?: Record<string, unknown>;
};

export type Device = {
  id: string;
  name: string;
  platform: string;
  worker_version: string;
  status: string;
  capabilities?: string[];
  last_heartbeat?: string;
  revoked?: boolean;
};

export type ManualAction = {
  manual_action_id: string;
  run_id: string;
  action_type: string;
  message: string;
  status: string;
  created_at: string;
  resolved_at?: string;
  payload?: Record<string, unknown>;
};

export type Artifact = {
  artifact_id: string;
  run_id: string;
  artifact_type: string;
  local_path?: string;
  size_bytes?: number;
  created_at: string;
  metadata?: Record<string, unknown>;
};

export type Checkpoint = {
  checkpoint_id: string;
  run_id: string;
  job_id: string;
  status: string;
  cursor?: Record<string, unknown>;
  summary?: Record<string, unknown>;
  created_at: string;
};

export type TraceStep = {
  step: string;
  action?: string;
  index?: number;
  error?: string;
  result?: unknown;
  params?: Record<string, unknown>;
  [key: string]: unknown;
};

