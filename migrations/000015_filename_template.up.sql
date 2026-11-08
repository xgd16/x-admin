-- Configurable export filename template, stored in sys_parameters and edited
-- from the export records page (drag & drop segments). Default keeps the
-- legacy behaviour: module identifier + compact timestamp.

INSERT INTO `sys_parameters` (`key`,`value`,`name`,`public`,`remark`,`created_at`,`updated_at`)
SELECT 'export.filename_template', '{"segments":[{"type":"module"},{"type":"time","format":"20060102150405"}]}',
       '导出文件名模板', 0, '在“导出记录”页可视化配置；值为 JSON 片段数组，请勿在此直接编辑', NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM `sys_parameters` WHERE `key` = 'export.filename_template');

-- Button permission for editing the template from the export records page.
INSERT IGNORE INTO `sys_menus`
(`id`,`parent_id`,`name`,`type`,`path`,`component`,`permission`,`icon`,`sort`,`visible`,`status`,`created_at`,`updated_at`) VALUES
(205,200,'文件名配置',3,'','','system:export:config','',5,1,1,NOW(3),NOW(3));

INSERT IGNORE INTO `sys_role_menus` (`role_id`,`menu_id`) VALUES (1,205);
