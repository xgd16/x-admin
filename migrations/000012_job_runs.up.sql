-- Execution history for scheduled/manual jobs.
CREATE TABLE IF NOT EXISTS `sys_job_runs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `job_id` BIGINT UNSIGNED NOT NULL,
  `job_name` VARCHAR(64) NOT NULL DEFAULT '',
  `trigger` VARCHAR(16) NOT NULL DEFAULT '',
  `status` TINYINT NOT NULL DEFAULT 0,
  `result` VARCHAR(255) NOT NULL DEFAULT '',
  `started_at` DATETIME(3) NOT NULL,
  `finished_at` DATETIME(3) NOT NULL,
  `duration_ms` BIGINT NOT NULL DEFAULT 0,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_job_runs_job` (`job_id`),
  KEY `idx_job_runs_started` (`started_at`),
  KEY `idx_job_runs_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
