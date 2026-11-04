-- Revocable file share links. Only token hashes are stored.
CREATE TABLE IF NOT EXISTS `sys_file_shares` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `file_id` BIGINT UNSIGNED NOT NULL,
  `token_hash` CHAR(64) NOT NULL,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `expires_at` DATETIME(3) NOT NULL,
  `revoked_at` DATETIME(3) NULL,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_file_shares_token` (`token_hash`),
  KEY `idx_file_shares_file` (`file_id`),
  KEY `idx_file_shares_expires` (`expires_at`),
  KEY `idx_file_shares_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
