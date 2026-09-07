CREATE UNIQUE INDEX IF NOT EXISTS uq_pref
    ON notification_preferences (account_type, account_id, pref_key);
