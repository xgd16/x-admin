UPDATE `sys_menus` SET `parent_id`=2 WHERE `id` IN (10,11,12,13,18,19);
UPDATE `sys_menus` SET `parent_id`=0 WHERE `id`=1;
DELETE FROM `sys_role_menus` WHERE `menu_id` IN (4,5,6);
DELETE FROM `sys_menus` WHERE `id` IN (4,5,6);
UPDATE `sys_menus` SET `name`='系统管理', `icon`='settings', `sort`=2 WHERE `id`=2;
