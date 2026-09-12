-- Make institution_users.registration_number optional (nullable).
-- Postgres unique indexes allow multiple NULLs, so unsetting the NOT NULL
-- constraint lets many institutions register without a number while keeping
-- uniqueness enforced for non-NULL values.
UPDATE institution_users SET registration_number = NULL WHERE registration_number = '';
ALTER TABLE institution_users ALTER COLUMN registration_number DROP NOT NULL;
