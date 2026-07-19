-- QIYUAN database baseline.
-- Changelog:
-- - 2026-07-19: Add customer account schema for tenant login and membership validation.
-- - 2026-07-19: Add tenant and resource ownership schema for customer isolation.
-- - 2026-06-18: Add Local Automation Worker platform schema baseline.
-- - 2026-06-22: Add extraction_result table for LLM template-driven extraction.

CREATE TABLE IF NOT EXISTS users (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  uuid VARCHAR(64) NOT NULL,
  email VARCHAR(320) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  nickname VARCHAR(64) NOT NULL,
  member_level VARCHAR(32) NOT NULL DEFAULT 'FREE',
  platform_role VARCHAR(32) NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  last_login_at DATETIME(3) NULL,
  password_changed_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uniq_users_uuid (uuid),
  UNIQUE KEY uniq_users_email (email),
  KEY idx_users_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tenant (
  id VARCHAR(64) NOT NULL,
  name VARCHAR(255) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_tenant_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tenant_membership (
  tenant_id VARCHAR(64) NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  role VARCHAR(32) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (tenant_id, user_id),
  KEY idx_tenant_membership_user_status (user_id, status),
  CONSTRAINT fk_tenant_membership_tenant FOREIGN KEY (tenant_id) REFERENCES tenant (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS worker_device (
  id VARCHAR(64) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL DEFAULT 'tenant_local',
  user_id VARCHAR(64) NULL,
  name VARCHAR(255) NOT NULL,
  platform VARCHAR(64) NOT NULL,
  worker_version VARCHAR(64) NOT NULL,
  hostname_hash VARCHAR(128) NULL,
  device_token_hash VARCHAR(128) NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  capabilities_json JSON NULL,
  last_seen_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  revoked_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_worker_device_token_hash (device_token_hash),
  KEY idx_worker_device_tenant_status (tenant_id, status),
  KEY idx_worker_device_user_status (user_id, status),
  KEY idx_worker_device_last_seen (last_seen_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS worker_pairing (
  id VARCHAR(64) NOT NULL,
  tenant_id VARCHAR(64) NULL,
  pairing_code_hash VARCHAR(128) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  device_id VARCHAR(64) NULL,
  requested_platform VARCHAR(64) NULL,
  requested_worker_version VARCHAR(64) NULL,
  requested_hostname_hash VARCHAR(128) NULL,
  requested_capabilities_json JSON NULL,
  approved_by_user_id VARCHAR(64) NULL,
  expires_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  approved_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_worker_pairing_code_hash (pairing_code_hash),
  KEY idx_worker_pairing_status_expires (status, expires_at),
  KEY idx_worker_pairing_tenant_status (tenant_id, status),
  KEY idx_worker_pairing_device (device_id),
  CONSTRAINT fk_worker_pairing_device FOREIGN KEY (device_id) REFERENCES worker_device (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS automation_job (
  id VARCHAR(64) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL DEFAULT 'tenant_local',
  user_id VARCHAR(64) NULL,
  job_type VARCHAR(128) NOT NULL,
  adapter VARCHAR(128) NOT NULL,
  title VARCHAR(255) NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'queued',
  priority INT NOT NULL DEFAULT 0,
  assigned_device_id VARCHAR(64) NULL,
  required_capabilities_json JSON NULL,
  target_json JSON NOT NULL,
  input_json JSON NOT NULL,
  policy_json JSON NOT NULL,
  last_cursor_json JSON NULL,
  last_error_code VARCHAR(128) NULL,
  last_error_message TEXT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  assigned_at DATETIME(3) NULL,
  started_at DATETIME(3) NULL,
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_automation_job_user_status (user_id, status),
  KEY idx_automation_job_tenant_status (tenant_id, status),
  KEY idx_automation_job_type_status (job_type, status),
  KEY idx_automation_job_assigned_device (assigned_device_id, status),
  KEY idx_automation_job_priority (status, priority, created_at),
  CONSTRAINT fk_automation_job_assigned_device FOREIGN KEY (assigned_device_id) REFERENCES worker_device (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS automation_run (
  id VARCHAR(64) NOT NULL,
  job_id VARCHAR(64) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL DEFAULT 'tenant_local',
  user_id VARCHAR(64) NULL,
  device_id VARCHAR(64) NOT NULL,
  adapter VARCHAR(128) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'running',
  worker_version VARCHAR(64) NULL,
  capabilities_json JSON NULL,
  started_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  last_heartbeat_at DATETIME(3) NULL,
  ended_at DATETIME(3) NULL,
  summary_json JSON NULL,
  error_code VARCHAR(128) NULL,
  error_message TEXT NULL,
  PRIMARY KEY (id),
  KEY idx_automation_run_job_status (job_id, status),
  KEY idx_automation_run_tenant_status (tenant_id, status),
  KEY idx_automation_run_device_status (device_id, status),
  KEY idx_automation_run_last_heartbeat (last_heartbeat_at),
  CONSTRAINT fk_automation_run_job FOREIGN KEY (job_id) REFERENCES automation_job (id),
  CONSTRAINT fk_automation_run_device FOREIGN KEY (device_id) REFERENCES worker_device (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS automation_checkpoint (
  id VARCHAR(64) NOT NULL,
  job_id VARCHAR(64) NOT NULL,
  run_id VARCHAR(64) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL DEFAULT 'tenant_local',
  sequence INT NOT NULL,
  stage VARCHAR(128) NOT NULL,
  cursor_json JSON NULL,
  progress_json JSON NULL,
  result_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uniq_automation_checkpoint_run_sequence (run_id, sequence),
  KEY idx_automation_checkpoint_job (job_id, created_at),
  KEY idx_automation_checkpoint_tenant_run (tenant_id, run_id),
  CONSTRAINT fk_automation_checkpoint_job FOREIGN KEY (job_id) REFERENCES automation_job (id),
  CONSTRAINT fk_automation_checkpoint_run FOREIGN KEY (run_id) REFERENCES automation_run (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS automation_artifact (
  id VARCHAR(64) NOT NULL,
  job_id VARCHAR(64) NOT NULL,
  run_id VARCHAR(64) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL DEFAULT 'tenant_local',
  result_id VARCHAR(64) NULL,
  artifact_type VARCHAR(64) NOT NULL,
  storage_key VARCHAR(512) NULL,
  filename VARCHAR(255) NULL,
  content_type VARCHAR(128) NULL,
  size_bytes BIGINT NULL,
  sha256 VARCHAR(64) NULL,
  metadata_json JSON NULL,
  redaction_status VARCHAR(32) NOT NULL DEFAULT 'not_required',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uniq_automation_artifact_idempotency (job_id, run_id, artifact_type, sha256),
  KEY idx_automation_artifact_run (run_id, artifact_type),
  KEY idx_automation_artifact_tenant_run (tenant_id, run_id),
  KEY idx_automation_artifact_result (result_id),
  CONSTRAINT fk_automation_artifact_job FOREIGN KEY (job_id) REFERENCES automation_job (id),
  CONSTRAINT fk_automation_artifact_run FOREIGN KEY (run_id) REFERENCES automation_run (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS automation_manual_action (
  id VARCHAR(64) NOT NULL,
  job_id VARCHAR(64) NOT NULL,
  run_id VARCHAR(64) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL DEFAULT 'tenant_local',
  type VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  prompt TEXT NOT NULL,
  details_json JSON NULL,
  expires_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  resolved_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_automation_manual_action_run_status (run_id, status),
  KEY idx_automation_manual_action_tenant_status (tenant_id, status),
  KEY idx_automation_manual_action_job_status (job_id, status),
  KEY idx_automation_manual_action_expires (status, expires_at),
  CONSTRAINT fk_automation_manual_action_job FOREIGN KEY (job_id) REFERENCES automation_job (id),
  CONSTRAINT fk_automation_manual_action_run FOREIGN KEY (run_id) REFERENCES automation_run (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS automation_audit_event (
  id VARCHAR(64) NOT NULL,
  tenant_id VARCHAR(64) NOT NULL DEFAULT 'tenant_local',
  job_id VARCHAR(64) NULL,
  run_id VARCHAR(64) NULL,
  device_id VARCHAR(64) NULL,
  event_type VARCHAR(128) NOT NULL,
  risk_level VARCHAR(32) NOT NULL DEFAULT 'low',
  summary VARCHAR(512) NOT NULL,
  payload_json JSON NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_automation_audit_job (job_id, created_at),
  KEY idx_automation_audit_tenant_created (tenant_id, created_at),
  KEY idx_automation_audit_run (run_id, created_at),
  KEY idx_automation_audit_device (device_id, created_at),
  KEY idx_automation_audit_event_type (event_type, created_at),
  CONSTRAINT fk_automation_audit_job FOREIGN KEY (job_id) REFERENCES automation_job (id),
  CONSTRAINT fk_automation_audit_run FOREIGN KEY (run_id) REFERENCES automation_run (id),
  CONSTRAINT fk_automation_audit_device FOREIGN KEY (device_id) REFERENCES worker_device (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS extraction_result (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  result_id VARCHAR(64) NOT NULL COMMENT '关联 qiyuan_literature_result.id',
  template_name VARCHAR(100) NOT NULL COMMENT '抽取模板名称',
  template_version INT NOT NULL DEFAULT 1 COMMENT '模板版本号',
  extractions JSON NOT NULL COMMENT 'LLM 抽取的结构化数组',
  llm_model VARCHAR(100) DEFAULT NULL COMMENT '使用的 LLM 模型',
  prompt_tokens INT DEFAULT NULL COMMENT 'LLM prompt token 数',
  completion_tokens INT DEFAULT NULL COMMENT 'LLM completion token 数',
  extraction_status VARCHAR(20) NOT NULL DEFAULT 'pending' COMMENT 'pending|success|failed',
  error_code VARCHAR(50) DEFAULT NULL,
  error_message TEXT DEFAULT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_extraction_result_id (result_id),
  INDEX idx_extraction_template (template_name),
  INDEX idx_extraction_status (extraction_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
