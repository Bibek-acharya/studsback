-- migrations/20260903-03-notification-backfill.sql
-- Applies AFTER tables (…-01) and indexes (…-02): its ON CONFLICT target
-- needs uq_an_occurrence from the indexes file.
-- Students: legacy `notifications` → account_notifications (user,…)
-- Note: the ON CONFLICT predicate must imply uq_an_occurrence's predicate,
-- which is `occurrence_key <> '' AND deleted_at IS NULL`.
INSERT INTO account_notifications (
  created_at, updated_at, account_type, account_id, event_key, category, priority,
  title, body, link, read_at, legacy_source, legacy_id, occurrence_key
)
SELECT n.created_at, n.updated_at, 'user', n.user_id,
  CASE n.type
    WHEN 'application' THEN 'application.status_changed'
    WHEN 'counselling' THEN 'counselling.booking_confirmed'
    ELSE 'legacy.' || n.type END,
  CASE n.type
    WHEN 'application' THEN 'application'
    WHEN 'counselling' THEN 'counselling'
    ELSE 'system' END,
  'normal', n.title, n.message, n.link,
  CASE WHEN n.read THEN n.updated_at ELSE NULL END,
  'student', n.id,
  'legacy:student:' || n.id
FROM notifications n
WHERE n.deleted_at IS NULL
ON CONFLICT (account_type, account_id, occurrence_key) WHERE occurrence_key <> '' AND deleted_at IS NULL DO NOTHING;

-- Providers: legacy `provider_notifications` → account_notifications (provider,…)
INSERT INTO account_notifications (
  created_at, updated_at, account_type, account_id, event_key, category, priority,
  title, body, link, read_at, legacy_source, legacy_id, occurrence_key
)
SELECT p.created_at, p.updated_at, 'provider', p.provider_id,
  CASE p.type
    WHEN 'application' THEN 'application.status_changed'
    WHEN 'interview'   THEN 'application.interview_scheduled'
    WHEN 'system'      THEN CASE p.title
                            WHEN 'New Login' THEN 'account.new_login'
                            WHEN 'Profile Incomplete' THEN 'account.profile_incomplete'
                            ELSE 'legacy.' || p.type END
    ELSE 'content.created_own' END,
  CASE p.type
    WHEN 'application' THEN 'application'
    WHEN 'interview'   THEN 'scholarship'
    WHEN 'system'      THEN 'account'
    ELSE 'content' END,
  'normal', p.title, p.message, p.link,
  CASE WHEN p.read THEN p.updated_at ELSE NULL END,
  'provider', p.id,
  'legacy:provider:' || p.id
FROM provider_notifications p
WHERE p.deleted_at IS NULL
ON CONFLICT (account_type, account_id, occurrence_key) WHERE occurrence_key <> '' AND deleted_at IS NULL DO NOTHING;
