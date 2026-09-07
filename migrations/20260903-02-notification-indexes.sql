-- migrations/20260903-notification-indexes.sql (authoritative for non-trivial indexes)
CREATE INDEX IF NOT EXISTS idx_an_inbox
  ON account_notifications (account_type, account_id, read_at NULLS FIRST, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_an_cat
  ON account_notifications (account_type, account_id, category, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_an_occurrence
  ON account_notifications (account_type, account_id, occurrence_key)
  WHERE occurrence_key <> '' AND deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_an_legacy
  ON account_notifications (legacy_source, legacy_id) WHERE legacy_source <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uq_broadcast_idem
  ON notification_broadcasts (created_by, idempotency_key) WHERE idempotency_key <> '';
CREATE UNIQUE INDEX IF NOT EXISTS uq_lease
  ON notification_dedupe_leases (account_type, account_id, dedupe_key);
