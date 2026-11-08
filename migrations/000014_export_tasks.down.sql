DELETE FROM `sys_role_menus` WHERE `menu_id` IN (200,201,202,203,204);
DELETE FROM `sys_jobs` WHERE `handler` = 'clean_export_tasks';
DELETE FROM `sys_menus` WHERE `id` IN (200,201,202,203,204);
DROP TABLE IF EXISTS `sys_export_tasks`;
