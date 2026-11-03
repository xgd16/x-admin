-- The built-in account should keep a neutral display name, not a person's name.
UPDATE `sys_users`
SET `nickname` = '管理员', `updated_at` = NOW(3)
WHERE `username` = 'admin' AND `nickname` = '李经理';
