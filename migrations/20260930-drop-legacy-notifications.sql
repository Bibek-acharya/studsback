-- P2.5 legacy drop (doc 14 §2.5). Data was backfilled in Phase 1; preferences
-- rows were migrated in Phase 2 core. No live readers/writers remain.
-- Column inventory verified 2026-09-09: provider_settings carried email_notifs
-- + sms_notifs (struct fields removed in the provider-stack cleanup — dead).
-- institution_settings.email_notifs is NOT dropped: it is still live
-- (institution Service Get/UpdateSettings + auth signup seed + DTO).
-- institution_settings never had sms_notifs.
-- All statements IF EXISTS: re-runnable, fresh-DB safe.
ALTER TABLE provider_settings DROP COLUMN IF EXISTS email_notifs;
ALTER TABLE provider_settings DROP COLUMN IF EXISTS sms_notifs;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS provider_notifications;
