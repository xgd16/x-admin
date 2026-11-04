-- Dashboard trend and retention jobs filter/order by created_at.
CREATE INDEX `idx_operation_logs_created` ON `sys_operation_logs` (`created_at`);
CREATE INDEX `idx_login_logs_created` ON `sys_login_logs` (`created_at`);
