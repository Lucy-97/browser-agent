-- Browser Agent migration: tenant and resource ownership.
-- Changelog:
-- - 2026-07-19: Add tenant boundary columns and indexes for Worker/Automation resources.

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

DROP PROCEDURE IF EXISTS add_column_if_missing;
DELIMITER $$
CREATE PROCEDURE add_column_if_missing(
  IN target_table VARCHAR(64),
  IN target_column VARCHAR(64),
  IN ddl_statement TEXT
)
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = target_table
      AND COLUMN_NAME = target_column
  ) THEN
    SET @ddl_statement = ddl_statement;
    PREPARE migration_stmt FROM @ddl_statement;
    EXECUTE migration_stmt;
    DEALLOCATE PREPARE migration_stmt;
  END IF;
END$$
DELIMITER ;

DROP PROCEDURE IF EXISTS add_index_if_missing;
DELIMITER $$
CREATE PROCEDURE add_index_if_missing(
  IN target_table VARCHAR(64),
  IN target_index VARCHAR(64),
  IN ddl_statement TEXT
)
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = target_table
      AND INDEX_NAME = target_index
  ) THEN
    SET @ddl_statement = ddl_statement;
    PREPARE migration_stmt FROM @ddl_statement;
    EXECUTE migration_stmt;
    DEALLOCATE PREPARE migration_stmt;
  END IF;
END$$
DELIMITER ;

CALL add_column_if_missing(
  'worker_device',
  'tenant_id',
  'ALTER TABLE worker_device ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT ''tenant_local'' AFTER id'
);
CALL add_column_if_missing(
  'worker_pairing',
  'tenant_id',
  'ALTER TABLE worker_pairing ADD COLUMN tenant_id VARCHAR(64) NULL AFTER id'
);
CALL add_column_if_missing(
  'automation_job',
  'tenant_id',
  'ALTER TABLE automation_job ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT ''tenant_local'' AFTER id'
);
CALL add_column_if_missing(
  'automation_run',
  'tenant_id',
  'ALTER TABLE automation_run ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT ''tenant_local'' AFTER job_id'
);
CALL add_column_if_missing(
  'automation_checkpoint',
  'tenant_id',
  'ALTER TABLE automation_checkpoint ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT ''tenant_local'' AFTER run_id'
);
CALL add_column_if_missing(
  'automation_artifact',
  'tenant_id',
  'ALTER TABLE automation_artifact ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT ''tenant_local'' AFTER run_id'
);
CALL add_column_if_missing(
  'automation_manual_action',
  'tenant_id',
  'ALTER TABLE automation_manual_action ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT ''tenant_local'' AFTER run_id'
);
CALL add_column_if_missing(
  'automation_audit_event',
  'tenant_id',
  'ALTER TABLE automation_audit_event ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT ''tenant_local'' AFTER id'
);

CALL add_index_if_missing(
  'worker_device',
  'idx_worker_device_tenant_status',
  'ALTER TABLE worker_device ADD KEY idx_worker_device_tenant_status (tenant_id, status)'
);
CALL add_index_if_missing(
  'worker_pairing',
  'idx_worker_pairing_tenant_status',
  'ALTER TABLE worker_pairing ADD KEY idx_worker_pairing_tenant_status (tenant_id, status)'
);
CALL add_index_if_missing(
  'automation_job',
  'idx_automation_job_tenant_status',
  'ALTER TABLE automation_job ADD KEY idx_automation_job_tenant_status (tenant_id, status)'
);
CALL add_index_if_missing(
  'automation_run',
  'idx_automation_run_tenant_status',
  'ALTER TABLE automation_run ADD KEY idx_automation_run_tenant_status (tenant_id, status)'
);
CALL add_index_if_missing(
  'automation_checkpoint',
  'idx_automation_checkpoint_tenant_run',
  'ALTER TABLE automation_checkpoint ADD KEY idx_automation_checkpoint_tenant_run (tenant_id, run_id)'
);
CALL add_index_if_missing(
  'automation_artifact',
  'idx_automation_artifact_tenant_run',
  'ALTER TABLE automation_artifact ADD KEY idx_automation_artifact_tenant_run (tenant_id, run_id)'
);
CALL add_index_if_missing(
  'automation_manual_action',
  'idx_automation_manual_action_tenant_status',
  'ALTER TABLE automation_manual_action ADD KEY idx_automation_manual_action_tenant_status (tenant_id, status)'
);
CALL add_index_if_missing(
  'automation_audit_event',
  'idx_automation_audit_tenant_created',
  'ALTER TABLE automation_audit_event ADD KEY idx_automation_audit_tenant_created (tenant_id, created_at)'
);

DROP PROCEDURE IF EXISTS add_index_if_missing;
DROP PROCEDURE IF EXISTS add_column_if_missing;
