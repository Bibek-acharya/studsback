-- Self-bootstrapping DDL: cmd/migrate (migrate-then-deploy) runs before the
-- first server boot, when GORM AutoMigrate has not created the table yet.
-- Column names/shapes match the NotificationPreference model.
CREATE TABLE IF NOT EXISTS notification_preferences (
  id bigserial PRIMARY KEY,
  created_at timestamptz, updated_at timestamptz,
  account_type varchar(20) NOT NULL, account_id bigint NOT NULL,
  pref_key varchar(100) NOT NULL,
  in_app boolean, email boolean, realtime boolean
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_pref
    ON notification_preferences (account_type, account_id, pref_key);
