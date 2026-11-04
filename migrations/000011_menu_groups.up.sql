-- Group built-in menus into the navigation sections used by the admin UI.
-- The frontend renders groups from this tree, so a flat "系统管理" directory
-- would collapse the rail into a single module.

INSERT IGNORE INTO `sys_menus`
(`id`,`parent_id`,`name`,`type`,`path`,`component`,`permission`,`icon`,`sort`,`visible`,`status`,`created_at`,`updated_at`) VALUES
(4,0,'工作台',1,'','','','grid',1,1,1,NOW(3),NOW(3)),
(5,0,'人员与权限',1,'','','','users',2,1,1,NOW(3),NOW(3)),
(6,0,'审计',1,'','','','file',4,1,1,NOW(3),NOW(3));

UPDATE `sys_menus` SET `name`='系统', `icon`='sliders', `sort`=3 WHERE `id`=2;
UPDATE `sys_menus` SET `parent_id`=4 WHERE `id`=1;
UPDATE `sys_menus` SET `parent_id`=5 WHERE `id` IN (10,11,12,13);
UPDATE `sys_menus` SET `parent_id`=6 WHERE `id` IN (18,19);

-- Keep existing role grants valid after reparenting: a role that can see a
-- child menu must also see its new directory node.
INSERT IGNORE INTO `sys_role_menus` (`role_id`,`menu_id`)
SELECT DISTINCT rm.`role_id`, 4 FROM `sys_role_menus` rm
JOIN `sys_menus` m ON m.`id` = rm.`menu_id`
WHERE m.`parent_id` = 4;

INSERT IGNORE INTO `sys_role_menus` (`role_id`,`menu_id`)
SELECT DISTINCT rm.`role_id`, 5 FROM `sys_role_menus` rm
JOIN `sys_menus` m ON m.`id` = rm.`menu_id`
WHERE m.`parent_id` = 5;

INSERT IGNORE INTO `sys_role_menus` (`role_id`,`menu_id`)
SELECT DISTINCT rm.`role_id`, 6 FROM `sys_role_menus` rm
JOIN `sys_menus` m ON m.`id` = rm.`menu_id`
WHERE m.`parent_id` = 6;
