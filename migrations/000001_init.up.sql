-- 基线迁移：完整表结构 + 内置种子数据（由 v1.3.0 之前的 1~16 号迁移压缩而来）。
--
-- 约定：
--   1. 首次安装只需执行本文件即可得到完整可用数据库；
--   2. 本文件发布后视为冻结，不要再修改，后续变更请新增 000002_xxx.up.sql；
--   3. 表结构使用 IF NOT EXISTS、种子数据使用 INSERT IGNORE，重复执行不会报错。

-- ---------------------------------------------------------------------------
-- 组织与账号
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `sys_departments` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `parent_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `name` VARCHAR(64) NOT NULL,
  `leader` VARCHAR(64) NOT NULL DEFAULT '',
  `phone` VARCHAR(32) NOT NULL DEFAULT '',
  `email` VARCHAR(128) NOT NULL DEFAULT '',
  `sort` INT NOT NULL DEFAULT 0,
  `status` TINYINT NOT NULL DEFAULT 1,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_departments_parent` (`parent_id`),
  KEY `idx_departments_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `sys_users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username` VARCHAR(64) NOT NULL,
  `password` VARCHAR(100) NOT NULL,
  `nickname` VARCHAR(64) NOT NULL DEFAULT '',
  `email` VARCHAR(128) NOT NULL DEFAULT '',
  `phone` VARCHAR(32) NOT NULL DEFAULT '',
  `avatar` VARCHAR(255) NOT NULL DEFAULT '',
  `dept_id` BIGINT UNSIGNED NULL,
  `status` TINYINT NOT NULL DEFAULT 1,
  `last_login_at` DATETIME(3) NULL,
  `last_login_ip` VARCHAR(64) NOT NULL DEFAULT '',
  `remark` VARCHAR(255) NOT NULL DEFAULT '',
  `totp_secret` VARCHAR(64) NOT NULL DEFAULT '',
  `totp_enabled` TINYINT(1) NOT NULL DEFAULT 0,
  `totp_recovery_codes` VARCHAR(1024) NOT NULL DEFAULT '',
  `session_version` BIGINT NOT NULL DEFAULT 0,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_users_username` (`username`),
  KEY `idx_users_dept` (`dept_id`),
  KEY `idx_users_email` (`email`),
  KEY `idx_users_phone` (`phone`),
  KEY `idx_users_status` (`status`),
  KEY `idx_users_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `sys_roles` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(64) NOT NULL,
  `code` VARCHAR(64) NOT NULL,
  `status` TINYINT NOT NULL DEFAULT 1,
  `sort` INT NOT NULL DEFAULT 0,
  `remark` VARCHAR(255) NOT NULL DEFAULT '',
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_roles_code` (`code`),
  KEY `idx_roles_status` (`status`),
  KEY `idx_roles_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- ---------------------------------------------------------------------------
-- 菜单与权限
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `sys_menus` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `parent_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `name` VARCHAR(64) NOT NULL,
  `type` TINYINT NOT NULL DEFAULT 2,
  `path` VARCHAR(200) NOT NULL DEFAULT '',
  `component` VARCHAR(200) NOT NULL DEFAULT '',
  `permission` VARCHAR(100) NOT NULL DEFAULT '',
  `icon` VARCHAR(64) NOT NULL DEFAULT '',
  `sort` INT NOT NULL DEFAULT 0,
  `visible` TINYINT(1) NOT NULL DEFAULT 1,
  `status` TINYINT NOT NULL DEFAULT 1,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_menus_parent` (`parent_id`),
  KEY `idx_menus_permission` (`permission`),
  KEY `idx_menus_type` (`type`),
  KEY `idx_menus_status` (`status`),
  KEY `idx_menus_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `sys_user_roles` (
  `user_id` BIGINT UNSIGNED NOT NULL,
  `role_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`user_id`, `role_id`),
  KEY `idx_user_roles_role` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `sys_role_menus` (
  `role_id` BIGINT UNSIGNED NOT NULL,
  `menu_id` BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (`role_id`, `menu_id`),
  KEY `idx_role_menus_menu` (`menu_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `sys_passkeys` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `credential_id` VARCHAR(512) NOT NULL,
  `name` VARCHAR(64) NOT NULL DEFAULT '',
  `credential` TEXT NOT NULL,
  `last_used_at` DATETIME(3) NULL,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_passkeys_credential` (`credential_id`),
  KEY `idx_passkeys_user` (`user_id`),
  KEY `idx_passkeys_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- ---------------------------------------------------------------------------
-- 字典与参数
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `sys_dict_types` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(64) NOT NULL,
  `code` VARCHAR(64) NOT NULL,
  `status` TINYINT NOT NULL DEFAULT 1,
  `remark` VARCHAR(255) NOT NULL DEFAULT '',
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_dict_types_code` (`code`),
  KEY `idx_dict_types_status` (`status`),
  KEY `idx_dict_types_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `sys_dict_items` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `dict_type_id` BIGINT UNSIGNED NOT NULL,
  `label` VARCHAR(64) NOT NULL,
  `value` VARCHAR(64) NOT NULL,
  `sort` INT NOT NULL DEFAULT 0,
  `status` TINYINT NOT NULL DEFAULT 1,
  `tag_color` VARCHAR(32) NOT NULL DEFAULT '',
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_dict_items_type` (`dict_type_id`),
  KEY `idx_dict_items_status` (`status`),
  KEY `idx_dict_items_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `sys_parameters` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `key` VARCHAR(128) NOT NULL,
  `value` TEXT NULL,
  `name` VARCHAR(64) NOT NULL,
  `public` TINYINT(1) NOT NULL DEFAULT 0,
  `remark` VARCHAR(255) NOT NULL DEFAULT '',
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_parameters_key` (`key`),
  KEY `idx_parameters_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- ---------------------------------------------------------------------------
-- 文件与导出
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `sys_files` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(255) NOT NULL,
  `key` VARCHAR(255) NOT NULL,
  `url` VARCHAR(255) NOT NULL,
  `mime` VARCHAR(128) NOT NULL DEFAULT '',
  `ext` VARCHAR(32) NOT NULL DEFAULT '',
  `size` BIGINT NOT NULL DEFAULT 0,
  `uploader_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_files_key` (`key`),
  KEY `idx_files_uploader` (`uploader_id`),
  KEY `idx_files_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 分享链接只保存 token 哈希，不保存明文。
CREATE TABLE IF NOT EXISTS `sys_file_shares` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `file_id` BIGINT UNSIGNED NOT NULL,
  `token_hash` CHAR(64) NOT NULL,
  `created_by` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `expires_at` DATETIME(3) NOT NULL,
  `revoked_at` DATETIME(3) NULL,
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_file_shares_token` (`token_hash`),
  KEY `idx_file_shares_file` (`file_id`),
  KEY `idx_file_shares_expires` (`expires_at`),
  KEY `idx_file_shares_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- 异步导出任务：一行对应一次导出，包含进度与产物文件。
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

-- ---------------------------------------------------------------------------
-- 定时任务
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `sys_jobs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(64) NOT NULL,
  `cron` VARCHAR(64) NOT NULL,
  `handler` VARCHAR(64) NOT NULL,
  `params` TEXT NULL,
  `status` TINYINT NOT NULL DEFAULT 1,
  `last_run_at` DATETIME(3) NULL,
  `last_result` VARCHAR(255) NOT NULL DEFAULT '',
  `remark` VARCHAR(255) NOT NULL DEFAULT '',
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_jobs_status` (`status`),
  KEY `idx_jobs_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

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

-- ---------------------------------------------------------------------------
-- 日志与通知
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS `sys_operation_logs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `username` VARCHAR(64) NOT NULL DEFAULT '',
  `method` VARCHAR(16) NOT NULL DEFAULT '',
  `path` VARCHAR(255) NOT NULL DEFAULT '',
  `ip` VARCHAR(64) NOT NULL DEFAULT '',
  `user_agent` VARCHAR(255) NOT NULL DEFAULT '',
  `status` INT NOT NULL DEFAULT 0,
  `latency_ms` BIGINT NOT NULL DEFAULT 0,
  `request_id` VARCHAR(64) NOT NULL DEFAULT '',
  `body` TEXT NULL,
  `error` VARCHAR(500) NOT NULL DEFAULT '',
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_operation_logs_user` (`user_id`),
  KEY `idx_operation_logs_username` (`username`),
  KEY `idx_operation_logs_request` (`request_id`),
  KEY `idx_operation_logs_deleted` (`deleted_at`),
  KEY `idx_operation_logs_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `sys_login_logs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL DEFAULT 0,
  `username` VARCHAR(64) NOT NULL DEFAULT '',
  `ip` VARCHAR(64) NOT NULL DEFAULT '',
  `user_agent` VARCHAR(255) NOT NULL DEFAULT '',
  `status` INT NOT NULL DEFAULT 0,
  `message` VARCHAR(255) NOT NULL DEFAULT '',
  `created_at` DATETIME(3) NULL,
  `updated_at` DATETIME(3) NULL,
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_login_logs_user` (`user_id`),
  KEY `idx_login_logs_username` (`username`),
  KEY `idx_login_logs_deleted` (`deleted_at`),
  KEY `idx_login_logs_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

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

-- ---------------------------------------------------------------------------
-- 种子数据
-- ---------------------------------------------------------------------------

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
(1,'admin','$2a$10$jQpu.NXktwX0g2pclC2k9.ph6cI6S71WO/NzZ7v.VW/8SVZnFEaTe','管理员','admin@x-admin.cn','13800000000',1,1,'系统内置账号',NOW(3),NOW(3));

INSERT IGNORE INTO `sys_user_roles` (`user_id`,`role_id`) VALUES (1,1);

-- 导航分三组：工作台、人员与权限、审计；其余系统模块直接挂在「系统」下。
INSERT IGNORE INTO `sys_menus`
(`id`,`parent_id`,`name`,`type`,`path`,`component`,`permission`,`icon`,`sort`,`visible`,`status`,`created_at`,`updated_at`) VALUES
(1,4,'总览',2,'/dashboard','dashboard','dashboard:view','grid',1,1,1,NOW(3),NOW(3)),
(2,0,'系统',1,'/system','','','sliders',3,1,1,NOW(3),NOW(3)),
(3,0,'个人中心',2,'/profile','profile','','user',90,0,1,NOW(3),NOW(3)),
(4,0,'工作台',1,'','','','grid',1,1,1,NOW(3),NOW(3)),
(5,0,'人员与权限',1,'','','','users',2,1,1,NOW(3),NOW(3)),
(6,0,'审计',1,'','','','file',4,1,1,NOW(3),NOW(3)),

(10,5,'用户管理',2,'/system/users','system/users','system:user:list','users',1,1,1,NOW(3),NOW(3)),
(11,5,'角色管理',2,'/system/roles','system/roles','system:role:list','shield',2,1,1,NOW(3),NOW(3)),
(12,5,'菜单管理',2,'/system/menus','system/menus','system:menu:list','tree',3,1,1,NOW(3),NOW(3)),
(13,5,'部门管理',2,'/system/departments','system/departments','system:dept:list','building',4,1,1,NOW(3),NOW(3)),
(14,2,'字典管理',2,'/system/dicts','system/dicts','system:dict:list','book',5,1,1,NOW(3),NOW(3)),
(15,2,'参数配置',2,'/system/parameters','system/parameters','system:param:list','sliders',6,1,1,NOW(3),NOW(3)),
(16,2,'文件管理',2,'/system/files','system/files','system:file:list','folder',7,1,1,NOW(3),NOW(3)),
(17,2,'定时任务',2,'/system/jobs','system/jobs','system:job:list','clock',8,1,1,NOW(3),NOW(3)),
(18,6,'操作日志',2,'/system/logs/operations','system/logs/operations','system:log:operation','file',9,1,1,NOW(3),NOW(3)),
(19,6,'登录日志',2,'/system/logs/logins','system/logs/logins','system:log:login','login',10,1,1,NOW(3),NOW(3)),

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
(93,17,'执行任务',3,'','','system:job:run','',4,1,1,NOW(3),NOW(3)),

(200,6,'导出记录',2,'/system/exports','system/exports','system:export:list','download',11,1,1,NOW(3),NOW(3)),
(201,200,'提交导出',3,'','','system:export:submit','',1,1,1,NOW(3),NOW(3)),
(202,200,'取消导出',3,'','','system:export:cancel','',2,1,1,NOW(3),NOW(3)),
(203,200,'删除记录',3,'','','system:export:delete','',3,1,1,NOW(3),NOW(3)),
(204,200,'下载导出',3,'','','system:export:download','',4,1,1,NOW(3),NOW(3)),
(205,200,'文件名配置',3,'','','system:export:config','',5,1,1,NOW(3),NOW(3)),

(210,2,'在线用户',2,'/system/online','system/online','system:online:list','login',9,1,1,NOW(3),NOW(3)),
(211,2,'系统监控',2,'/system/monitor','system/monitor','system:monitor:view','activity',10,1,1,NOW(3),NOW(3)),
(212,210,'强制下线',3,'','','system:online:kick','',1,1,1,NOW(3),NOW(3));

-- 超级管理员拥有全部菜单；运营角色只读看板与审计日志。
INSERT IGNORE INTO `sys_role_menus` (`role_id`,`menu_id`) SELECT 1, `id` FROM `sys_menus`;
INSERT IGNORE INTO `sys_role_menus` (`role_id`,`menu_id`) VALUES (2,1),(2,2),(2,4),(2,6),(2,18),(2,19);

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
(3,'upload.max_size','20','上传大小上限（MB）',0,'',NOW(3),NOW(3)),
(4,'export.filename_template','{"segments":[{"type":"module"},{"type":"time","format":"20060102150405"}]}','导出文件名模板',0,'在“导出记录”页可视化配置；值为 JSON 片段数组，请勿在此直接编辑',NOW(3),NOW(3));

INSERT IGNORE INTO `sys_jobs`
(`id`,`name`,`cron`,`handler`,`params`,`status`,`remark`,`created_at`,`updated_at`) VALUES
(1,'清理登录日志','0 3 * * *','clean_logs','{"days":90}',1,'每天 03:00 清理 90 天前登录日志',NOW(3),NOW(3)),
(2,'清理站内通知','30 3 * * *','clean_notifications','{"days":90}',1,'每天 03:30 清理 90 天前站内通知',NOW(3),NOW(3)),
(3,'清理导出文件','0 4 * * *','clean_export_tasks','{"days":7}',1,'每天 04:00 清理 7 天前的导出任务与文件',NOW(3),NOW(3));
