CREATE TABLE IF NOT EXISTS `sys_passkeys` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `credential_id` VARCHAR(512) NOT NULL,
  `name` VARCHAR(64) NOT NULL DEFAULT '',
  `credential` TEXT NOT NULL,
  `last_used_at` DATETIME(3) NULL,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_passkeys_credential` (`credential_id`),
  KEY `idx_passkeys_user` (`user_id`),
  KEY `idx_passkeys_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
