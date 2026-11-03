CREATE TABLE IF NOT EXISTS `sys_notifications` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `title` VARCHAR(128) NOT NULL,
  `content` VARCHAR(500) NOT NULL DEFAULT '',
  `type` VARCHAR(16) NOT NULL DEFAULT 'info',
  `link` VARCHAR(255) NOT NULL DEFAULT '',
  `actor_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `read_at` DATETIME(3) NULL,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_notifications_user_read` (`user_id`, `read_at`),
  KEY `idx_notifications_created` (`created_at`),
  KEY `idx_notifications_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- One real announcement per existing account so the new center is discoverable
-- right after the upgrade.
INSERT INTO `sys_notifications` (`user_id`,`title`,`content`,`type`,`link`,`actor_id`,`created_at`,`updated_at`)
SELECT `id`, '站内通知已启用', '登录提醒、密码与角色变更等消息会出现在这里，点击铃铛即可查看。', 'info', '', 0, NOW(3), NOW(3)
FROM `sys_users`
WHERE `deleted_at` IS NULL;
