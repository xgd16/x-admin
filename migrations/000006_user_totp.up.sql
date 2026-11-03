ALTER TABLE `sys_users`
  ADD COLUMN `totp_secret` VARCHAR(64) NOT NULL DEFAULT '' AFTER `remark`,
  ADD COLUMN `totp_enabled` TINYINT(1) NOT NULL DEFAULT 0 AFTER `totp_secret`,
  ADD COLUMN `totp_recovery_codes` VARCHAR(1024) NOT NULL DEFAULT '' AFTER `totp_enabled`;
