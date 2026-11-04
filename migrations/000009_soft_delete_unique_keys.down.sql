-- Intentionally irreversible: restoring old unique values could collide with
-- names that were legitimately reused after the upgrade. Keep the tombstones.
SELECT 1;
