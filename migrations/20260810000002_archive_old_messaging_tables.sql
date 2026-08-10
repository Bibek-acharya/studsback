BEGIN;

-- Backup old tables
CREATE TABLE IF NOT EXISTS messages_archived AS SELECT * FROM messages;
CREATE TABLE IF NOT EXISTS institution_messages_archived AS SELECT * FROM institution_messages;

-- Drop old tables
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS institution_messages;

COMMIT;
