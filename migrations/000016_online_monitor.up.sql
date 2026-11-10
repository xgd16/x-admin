-- Admin online session management and system monitor pages.

INSERT IGNORE INTO `sys_menus`
(`id`,`parent_id`,`name`,`type`,`path`,`component`,`permission`,`icon`,`sort`,`visible`,`status`,`created_at`,`updated_at`) VALUES
(210,2,'在线用户',2,'/system/online','system/online','system:online:list','login',9,1,1,NOW(3),NOW(3)),
(211,2,'系统监控',2,'/system/monitor','system/monitor','system:monitor:view','activity',10,1,1,NOW(3),NOW(3)),
(212,210,'强制下线',3,'','','system:online:kick','',1,1,1,NOW(3),NOW(3));

INSERT IGNORE INTO `sys_role_menus` (`role_id`,`menu_id`) VALUES
(1,210),(1,211),(1,212);
