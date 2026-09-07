-- Migrate legacy EmailNotifs=false flags into notification_preferences.
-- Only inserts rows for accounts that had email OFF (sparse storage).
-- Accounts with EmailNotifs=true get no row (registry defaults apply).
-- Idempotent: ON CONFLICT DO NOTHING.

-- Providers with EmailNotifs=false
INSERT INTO notification_preferences (created_at, updated_at, account_type, account_id, pref_key, email)
SELECT now(), now(), 'provider', ps.provider_id, '*', false
FROM provider_settings ps
WHERE ps.email_notifs = false
ON CONFLICT (account_type, account_id, pref_key) DO NOTHING;

-- Institutions with EmailNotifs=false
INSERT INTO notification_preferences (created_at, updated_at, account_type, account_id, pref_key, email)
SELECT now(), now(), 'institution', ist.institution_id, '*', false
FROM institution_settings ist
WHERE ist.email_notifs = false
ON CONFLICT (account_type, account_id, pref_key) DO NOTHING;
