INSERT IGNORE INTO `sys_departments`
(`id`,`parent_id`,`name`,`leader`,`phone`,`sort`,`status`,`created_at`,`updated_at`) VALUES
(1,0,'总公司','李经理','13800000000',1,1,NOW(3),NOW(3)),
(2,1,'技术部','陈默','13800000001',1,1,NOW(3),NOW(3)),
(3,1,'运营部','林小满','13800000002',2,1,NOW(3),NOW(3)),
(4,1,'财务部','周予安','13800000003',3,1,NOW(3),NOW(3));

INSERT IGNORE INTO `sys_roles`
(`id`,`name`,`code`,`status`,`sort`,`remark`,`created_at`,`updated_at`) VALUES
(1,'超级管理员','super_admin',1,1,'拥有全部权限',NOW(3),NOW(3)),
(2,'运营','operator',1,2,'查看看板与审计日志',NOW(3),NOW(3));

-- 密码：Admin@123456
INSERT IGNORE INTO `sys_users`
(`id`,`username`,`password`,`nickname`,`email`,`phone`,`dept_id`,`status`,`remark`,`created_at`,`updated_at`) VALUES
(1,'admin','$2a$10$jQpu.NXktwX0g2pclC2k9.ph6cI6S71WO/NzZ7v.VW/8SVZnFEaTe','李经理','admin@x-admin.cn','13800000000',1,1,'系统内置账号',NOW(3),NOW(3));

INSERT IGNORE INTO `sys_user_roles` (`user_id`,`role_id`) VALUES (1,1);

INSERT IGNORE INTO `sys_menus`
(`id`,`parent_id`,`name`,`type`,`path`,`component`,`permission`,`icon`,`sort`,`visible`,`status`,`created_at`,`updated_at`) VALUES
(1,0,'总览',2,'/dashboard','dashboard','dashboard:view','grid',1,1,1,NOW(3),NOW(3)),
(2,0,'系统管理',1,'/system','','','settings',2,1,1,NOW(3),NOW(3)),
(3,0,'个人中心',2,'/profile','profile','','user',90,0,1,NOW(3),NOW(3)),

(10,2,'用户管理',2,'/system/users','system/users','system:user:list','users',1,1,1,NOW(3),NOW(3)),
(11,2,'角色管理',2,'/system/roles','system/roles','system:role:list','shield',2,1,1,NOW(3),NOW(3)),
(12,2,'菜单管理',2,'/system/menus','system/menus','system:menu:list','tree',3,1,1,NOW(3),NOW(3)),
(13,2,'部门管理',2,'/system/departments','system/departments','system:dept:list','building',4,1,1,NOW(3),NOW(3)),
(14,2,'字典管理',2,'/system/dicts','system/dicts','system:dict:list','book',5,1,1,NOW(3),NOW(3)),
(15,2,'参数配置',2,'/system/parameters','system/parameters','system:param:list','sliders',6,1,1,NOW(3),NOW(3)),
(16,2,'文件管理',2,'/system/files','system/files','system:file:list','folder',7,1,1,NOW(3),NOW(3)),
(17,2,'定时任务',2,'/system/jobs','system/jobs','system:job:list','clock',8,1,1,NOW(3),NOW(3)),
(18,2,'操作日志',2,'/system/logs/operations','system/logs/operations','system:log:operation','file',9,1,1,NOW(3),NOW(3)),
(19,2,'登录日志',2,'/system/logs/logins','system/logs/logins','system:log:login','login',10,1,1,NOW(3),NOW(3)),

(20,10,'新建用户',3,'','','system:user:create','',1,1,1,NOW(3),NOW(3)),
(21,10,'编辑用户',3,'','','system:user:update','',2,1,1,NOW(3),NOW(3)),
(22,10,'删除用户',3,'','','system:user:delete','',3,1,1,NOW(3),NOW(3)),
(23,10,'重置密码',3,'','','system:user:reset','',4,1,1,NOW(3),NOW(3)),
(24,10,'导入用户',3,'','','system:user:import','',5,1,1,NOW(3),NOW(3)),
(25,10,'导出用户',3,'','','system:user:export','',6,1,1,NOW(3),NOW(3)),

(30,11,'新建角色',3,'','','system:role:create','',1,1,1,NOW(3),NOW(3)),
(31,11,'编辑角色',3,'','','system:role:update','',2,1,1,NOW(3),NOW(3)),
(32,11,'删除角色',3,'','','system:role:delete','',3,1,1,NOW(3),NOW(3)),
(33,11,'分配权限',3,'','','system:role:assign','',4,1,1,NOW(3),NOW(3)),

(40,12,'新建菜单',3,'','','system:menu:create','',1,1,1,NOW(3),NOW(3)),
(41,12,'编辑菜单',3,'','','system:menu:update','',2,1,1,NOW(3),NOW(3)),
(42,12,'删除菜单',3,'','','system:menu:delete','',3,1,1,NOW(3),NOW(3)),

(50,13,'新建部门',3,'','','system:dept:create','',1,1,1,NOW(3),NOW(3)),
(51,13,'编辑部门',3,'','','system:dept:update','',2,1,1,NOW(3),NOW(3)),
(52,13,'删除部门',3,'','','system:dept:delete','',3,1,1,NOW(3),NOW(3)),

(60,14,'新建字典',3,'','','system:dict:create','',1,1,1,NOW(3),NOW(3)),
(61,14,'编辑字典',3,'','','system:dict:update','',2,1,1,NOW(3),NOW(3)),
(62,14,'删除字典',3,'','','system:dict:delete','',3,1,1,NOW(3),NOW(3)),

(70,15,'新建参数',3,'','','system:param:create','',1,1,1,NOW(3),NOW(3)),
(71,15,'编辑参数',3,'','','system:param:update','',2,1,1,NOW(3),NOW(3)),
(72,15,'删除参数',3,'','','system:param:delete','',3,1,1,NOW(3),NOW(3)),

(80,16,'上传文件',3,'','','system:file:upload','',1,1,1,NOW(3),NOW(3)),
(81,16,'删除文件',3,'','','system:file:delete','',2,1,1,NOW(3),NOW(3)),
(82,16,'下载文件',3,'','','system:file:download','',3,1,1,NOW(3),NOW(3)),

(90,17,'新建任务',3,'','','system:job:create','',1,1,1,NOW(3),NOW(3)),
(91,17,'编辑任务',3,'','','system:job:update','',2,1,1,NOW(3),NOW(3)),
(92,17,'删除任务',3,'','','system:job:delete','',3,1,1,NOW(3),NOW(3)),
(93,17,'执行任务',3,'','','system:job:run','',4,1,1,NOW(3),NOW(3));

-- 超级管理员拥有全部菜单；运营角色只读看板与审计日志。
INSERT IGNORE INTO `sys_role_menus` (`role_id`,`menu_id`) SELECT 1, `id` FROM `sys_menus`;
INSERT IGNORE INTO `sys_role_menus` (`role_id`,`menu_id`) VALUES (2,1),(2,2),(2,18),(2,19);

INSERT IGNORE INTO `sys_dict_types`
(`id`,`name`,`code`,`status`,`remark`,`created_at`,`updated_at`) VALUES
(1,'用户状态','user_status',1,'',NOW(3),NOW(3)),
(2,'任务状态','job_status',1,'',NOW(3),NOW(3)),
(3,'菜单类型','menu_type',1,'',NOW(3),NOW(3));

INSERT IGNORE INTO `sys_dict_items`
(`id`,`dict_type_id`,`label`,`value`,`sort`,`status`,`tag_color`,`created_at`,`updated_at`) VALUES
(1,1,'正常','1',1,1,'green',NOW(3),NOW(3)),
(2,1,'停用','0',2,1,'default',NOW(3),NOW(3)),
(3,2,'启用','1',1,1,'green',NOW(3),NOW(3)),
(4,2,'停用','0',2,1,'default',NOW(3),NOW(3)),
(5,3,'目录','1',1,1,'blue',NOW(3),NOW(3)),
(6,3,'菜单','2',2,1,'green',NOW(3),NOW(3)),
(7,3,'按钮','3',3,1,'orange',NOW(3),NOW(3));

INSERT IGNORE INTO `sys_parameters`
(`id`,`key`,`value`,`name`,`public`,`remark`,`created_at`,`updated_at`) VALUES
(1,'site.name','x-admin','站点名称',1,'',NOW(3),NOW(3)),
(2,'site.logo','','站点 Logo',1,'',NOW(3),NOW(3)),
(3,'upload.max_size','20','上传大小上限（MB）',0,'',NOW(3),NOW(3));

INSERT IGNORE INTO `sys_jobs`
(`id`,`name`,`cron`,`handler`,`params`,`status`,`remark`,`created_at`,`updated_at`) VALUES
(1,'清理登录日志','0 3 * * *','clean_logs','{"days":90}',1,'每天 03:00 清理 90 天前登录日志',NOW(3),NOW(3));
