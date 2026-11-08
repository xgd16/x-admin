-- Async export tasks: one row per export job with progress and the produced file.

CREATE TABLE IF NOT EXISTS `sys_export_tasks` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `module` VARCHAR(64) NOT NULL,
  `title` VARCHAR(128) NOT NULL,
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '0待处理 1进行中 2成功 3失败 4已取消',
  `total` BIGINT NOT NULL DEFAULT 0,
  `processed` BIGINT NOT NULL DEFAULT 0,
  `file_key` VARCHAR(255) NOT NULL DEFAULT '',
  `file_name` VARCHAR(255) NOT NULL DEFAULT '',
  `file_size` BIGINT NOT NULL DEFAULT 0,
  `error` VARCHAR(512) NOT NULL DEFAULT '',
  `canceled` TINYINT(1) NOT NULL DEFAULT 0,
  `finished_at` DATETIME(3) NULL,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_export_tasks_user` (`user_id`, `id`),
  KEY `idx_export_tasks_status` (`status`),
  KEY `idx_export_tasks_finished` (`finished_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Menu + button permissions under the audit group (parent 6, see 000011).
INSERT IGNORE INTO `sys_menus`
(`id`,`parent_id`,`name`,`type`,`path`,`component`,`permission`,`icon`,`sort`,`visible`,`status`,`created_at`,`updated_at`) VALUES
(200,6,'导出记录',2,'/system/exports','system/exports','system:export:list','download',11,1,1,NOW(3),NOW(3)),
(201,200,'提交导出',3,'','','system:export:submit','',1,1,1,NOW(3),NOW(3)),
(202,200,'取消导出',3,'','','system:export:cancel','',2,1,1,NOW(3),NOW(3)),
(203,200,'删除记录',3,'','','system:export:delete','',3,1,1,NOW(3),NOW(3)),
(204,200,'下载导出',3,'','','system:export:download','',4,1,1,NOW(3),NOW(3));

INSERT IGNORE INTO `sys_role_menus` (`role_id`,`menu_id`) VALUES
(1,200),(1,201),(1,202),(1,203),(1,204);

-- Daily cleanup job: removes tasks (and their files) older than keep_days.
INSERT INTO `sys_jobs` (`name`,`cron`,`handler`,`params`,`status`,`remark`,`created_at`,`updated_at`)
SELECT '清理导出文件','0 4 * * *','clean_export_tasks','{"days":7}',1,'每天 04:00 清理 7 天前的导出任务与文件',NOW(3),NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM `sys_jobs` WHERE `handler` = 'clean_export_tasks');
