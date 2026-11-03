ALTER TABLE `sys_users`
  DROP COLUMN `totp_recovery_codes`,
  DROP COLUMN `totp_enabled`,
  DROP COLUMN `totp_secret`;
