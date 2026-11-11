-- 回滚基线：按依赖顺序删除全部业务表。
-- 注意：该操作会清空所有数据，仅用于开发/测试环境重置。

DROP TABLE IF EXISTS `sys_export_tasks`;
DROP TABLE IF EXISTS `sys_file_shares`;
DROP TABLE IF EXISTS `sys_job_runs`;
DROP TABLE IF EXISTS `sys_passkeys`;
DROP TABLE IF EXISTS `sys_notifications`;
DROP TABLE IF EXISTS `sys_login_logs`;
DROP TABLE IF EXISTS `sys_operation_logs`;
DROP TABLE IF EXISTS `sys_jobs`;
DROP TABLE IF EXISTS `sys_files`;
DROP TABLE IF EXISTS `sys_parameters`;
DROP TABLE IF EXISTS `sys_dict_items`;
DROP TABLE IF EXISTS `sys_dict_types`;
DROP TABLE IF EXISTS `sys_role_menus`;
DROP TABLE IF EXISTS `sys_user_roles`;
DROP TABLE IF EXISTS `sys_menus`;
DROP TABLE IF EXISTS `sys_roles`;
DROP TABLE IF EXISTS `sys_users`;
DROP TABLE IF EXISTS `sys_departments`;
