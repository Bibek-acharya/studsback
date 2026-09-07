-- migrations/20260903-notification-tables.sql
CREATE TABLE IF NOT EXISTS account_notifications (
  id bigserial PRIMARY KEY,
  created_at timestamptz, updated_at timestamptz, deleted_at timestamptz,
  account_type varchar(20) NOT NULL, account_id bigint NOT NULL,
  event_key varchar(100) NOT NULL, category varchar(40) NOT NULL,
  priority varchar(10) NOT NULL DEFAULT 'normal',
  title varchar(200) NOT NULL, body text, link varchar(500),
  data jsonb, read_at timestamptz, archived_at timestamptz,
  occurrence_key varchar(200), broadcast_id bigint,
  actor_type varchar(20), actor_id bigint, correlation_id varchar(64),
  legacy_source varchar(20), legacy_id bigint
);
CREATE TABLE IF NOT EXISTS notification_outbox (
  id bigserial PRIMARY KEY, created_at timestamptz, available_at timestamptz,
  claimed_at timestamptz, claim_token varchar(64), lease_expires_at timestamptz,
  kind varchar(20) NOT NULL, payload jsonb NOT NULL, occurrence_key varchar(200),
  attempts int DEFAULT 0, last_error text, done boolean DEFAULT false
);
CREATE TABLE IF NOT EXISTS notification_broadcasts (
  id bigserial PRIMARY KEY, created_at timestamptz, updated_at timestamptz,
  title varchar(200) NOT NULL, body text NOT NULL, link varchar(500),
  priority varchar(10) NOT NULL DEFAULT 'critical',
  audience jsonb NOT NULL, audience_criteria jsonb,
  status varchar(20) NOT NULL DEFAULT 'sending',
  total_count int DEFAULT 0, sent_count int DEFAULT 0, failed_count int DEFAULT 0,
  created_by bigint NOT NULL, idempotency_key varchar(100)
);
CREATE TABLE IF NOT EXISTS notification_dedupe_leases (
  id bigserial PRIMARY KEY, created_at timestamptz,
  account_type varchar(20) NOT NULL, account_id bigint NOT NULL,
  dedupe_key varchar(200) NOT NULL, notification_id bigint,
  expires_at timestamptz, superseded_count int DEFAULT 0
);
