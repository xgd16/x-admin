UPDATE `sys_users`
SET `nickname` = '李经理', `updated_at` = NOW(3)
WHERE `username` = 'admin' AND `nickname` = '管理员';
