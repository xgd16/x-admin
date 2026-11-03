-- Retention for personal notifications; the handler is also selectable in the
-- Jobs UI for custom schedules.
INSERT INTO `sys_jobs` (`name`,`cron`,`handler`,`params`,`status`,`remark`,`created_at`,`updated_at`)
SELECT '清理站内通知','30 3 * * *','clean_notifications','{"days":90}',1,'每天 03:30 清理 90 天前站内通知',NOW(3),NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM `sys_jobs` WHERE `handler` = 'clean_notifications');
