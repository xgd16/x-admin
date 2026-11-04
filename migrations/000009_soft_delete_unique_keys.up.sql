-- Free unique keys held by soft-deleted rows.
-- Without this, re-creating a deleted username / role code / dictionary code /
-- parameter key passes the application-level existence check and then fails on
-- the database unique index.
--
-- The marker is appended at the end so a very long value is truncated from the
-- left instead of losing the unique suffix.

UPDATE `sys_users`
SET `username` = CONCAT(
  LEFT(`username`, 64 - CHAR_LENGTH(CONCAT('#deleted#', `id`))),
  '#deleted#', `id`
)
WHERE `deleted_at` IS NOT NULL
  AND `username` NOT LIKE '%#deleted#%';

UPDATE `sys_roles`
SET `code` = CONCAT(
  LEFT(`code`, 64 - CHAR_LENGTH(CONCAT('#deleted#', `id`))),
  '#deleted#', `id`
)
WHERE `deleted_at` IS NOT NULL
  AND `code` NOT LIKE '%#deleted#%';

UPDATE `sys_dict_types`
SET `code` = CONCAT(
  LEFT(`code`, 64 - CHAR_LENGTH(CONCAT('#deleted#', `id`))),
  '#deleted#', `id`
)
WHERE `deleted_at` IS NOT NULL
  AND `code` NOT LIKE '%#deleted#%';

UPDATE `sys_parameters`
SET `key` = CONCAT(
  LEFT(`key`, 128 - CHAR_LENGTH(CONCAT('#deleted#', `id`))),
  '#deleted#', `id`
)
WHERE `deleted_at` IS NOT NULL
  AND `key` NOT LIKE '%#deleted#%';
