-- QIYUAN migration: worker device token hash.
-- Changelog:
-- - 2026-06-18: Add worker_device.device_token_hash for persistent Worker token authentication.

SET @add_device_token_hash := (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE worker_device ADD COLUMN device_token_hash VARCHAR(128) NULL AFTER hostname_hash',
    'SELECT 1'
  )
  FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'worker_device'
    AND COLUMN_NAME = 'device_token_hash'
);
PREPARE stmt FROM @add_device_token_hash;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @add_device_token_hash_index := (
  SELECT IF(
    COUNT(*) = 0,
    'ALTER TABLE worker_device ADD UNIQUE KEY uniq_worker_device_token_hash (device_token_hash)',
    'SELECT 1'
  )
  FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'worker_device'
    AND INDEX_NAME = 'uniq_worker_device_token_hash'
);
PREPARE stmt FROM @add_device_token_hash_index;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
