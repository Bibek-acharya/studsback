-- Messaging system tables
-- Replaces old messages + institution_messages tables

BEGIN;

-- Conversations: one per student-institution pair
CREATE TABLE IF NOT EXISTS conversations (
    id                 BIGSERIAL PRIMARY KEY,
    created_at         TIMESTAMPTZ DEFAULT NOW(),
    updated_at         TIMESTAMPTZ DEFAULT NOW(),
    student_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    institution_id     BIGINT NOT NULL REFERENCES institution_users(id) ON DELETE CASCADE,
    last_message_id    BIGINT,
    last_message_at    TIMESTAMPTZ,
    last_message_preview VARCHAR(255),
    UNIQUE(student_id, institution_id)
);

CREATE INDEX IF NOT EXISTS idx_conversations_student ON conversations(student_id);
CREATE INDEX IF NOT EXISTS idx_conversations_institution ON conversations(institution_id);
CREATE INDEX IF NOT EXISTS idx_conversations_last_message ON conversations(last_message_at DESC);

-- Participants: per-user read state, supports multi-staff
CREATE TABLE IF NOT EXISTS conversation_participants (
    id                     BIGSERIAL PRIMARY KEY,
    conversation_id        BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    participant_type       VARCHAR(20) NOT NULL CHECK (participant_type IN ('student', 'institution')),
    participant_id         BIGINT NOT NULL,
    last_read_message_id   BIGINT,
    last_read_at           TIMESTAMPTZ,
    unread_count           INT DEFAULT 0,
    is_typing              BOOLEAN DEFAULT FALSE,
    typing_at              TIMESTAMPTZ,
    UNIQUE(conversation_id, participant_type, participant_id)
);

CREATE INDEX IF NOT EXISTS idx_participants_conversation ON conversation_participants(conversation_id);
CREATE INDEX IF NOT EXISTS idx_participants_user ON conversation_participants(participant_type, participant_id);

-- Messages: with idempotency and edit support
CREATE TABLE IF NOT EXISTS messages (
    id                 BIGSERIAL PRIMARY KEY,
    created_at         TIMESTAMPTZ DEFAULT NOW(),
    conversation_id    BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_type        VARCHAR(20) NOT NULL CHECK (sender_type IN ('student', 'institution')),
    sender_id          BIGINT NOT NULL,
    client_message_id  VARCHAR(36) NOT NULL,
    content            TEXT,
    edited_at          TIMESTAMPTZ,
    deleted_at         TIMESTAMPTZ,
    UNIQUE(conversation_id, client_message_id)
);

CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender_type, sender_id);

-- Attachments: file attachments on messages
CREATE TABLE IF NOT EXISTS message_attachments (
    id            BIGSERIAL PRIMARY KEY,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    message_id    BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    uploader_type VARCHAR(20) NOT NULL,
    uploader_id   BIGINT NOT NULL,
    file_name     VARCHAR(255) NOT NULL,
    file_size     BIGINT NOT NULL,
    file_type     VARCHAR(50) NOT NULL,
    storage_key   VARCHAR(500) NOT NULL,
    thumbnail_key VARCHAR(500)
);

CREATE INDEX IF NOT EXISTS idx_attachments_message ON message_attachments(message_id);

-- Pending uploads: staged before message send
CREATE TABLE IF NOT EXISTS pending_uploads (
    id            BIGSERIAL PRIMARY KEY,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    uploader_type VARCHAR(20) NOT NULL,
    uploader_id   BIGINT NOT NULL,
    file_name     VARCHAR(255) NOT NULL,
    file_size     BIGINT NOT NULL,
    file_type     VARCHAR(50) NOT NULL,
    storage_key   VARCHAR(500) NOT NULL,
    thumbnail_key VARCHAR(500),
    message_id    BIGINT,
    expires_at    TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pending_uploads_user ON pending_uploads(uploader_type, uploader_id);
CREATE INDEX IF NOT EXISTS idx_pending_uploads_expiry ON pending_uploads(expires_at) WHERE message_id IS NULL;

-- Outbox: reliable event publication
CREATE TABLE IF NOT EXISTS outbox_events (
    id             BIGSERIAL PRIMARY KEY,
    created_at     TIMESTAMPTZ DEFAULT NOW(),
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id   BIGINT NOT NULL,
    event_type     VARCHAR(100) NOT NULL,
    payload        JSONB NOT NULL,
    published      BOOLEAN DEFAULT FALSE,
    published_at   TIMESTAMPTZ,
    retry_count    INT DEFAULT 0,
    max_retries    INT DEFAULT 5
);

CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON outbox_events(published, created_at) WHERE NOT published;

COMMIT;