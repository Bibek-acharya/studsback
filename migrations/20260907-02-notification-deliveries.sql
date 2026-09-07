-- Partial unique indexes for delivery idempotency (doc 04 §4a).
-- 'failed' rows may be superseded by retries (status filter).

CREATE UNIQUE INDEX IF NOT EXISTS uq_del_notification
    ON notification_deliveries (delivery_key)
    WHERE delivery_kind = 'notification' AND channel = 'email' AND status <> 'failed';

CREATE UNIQUE INDEX IF NOT EXISTS uq_del_digest
    ON notification_deliveries (delivery_key)
    WHERE delivery_kind = 'digest' AND channel = 'email' AND status <> 'failed';

CREATE UNIQUE INDEX IF NOT EXISTS uq_del_anonymous
    ON notification_deliveries (delivery_key)
    WHERE delivery_kind = 'anonymous' AND channel = 'email' AND status <> 'failed';

-- Subject invariant: kind/subject XOR enforced by DB (readiness review H2).
-- Idempotent: drop + re-add so repeated migration runs don't fail.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'chk_del_subject'
        AND conrelid = 'notification_deliveries'::regclass
    ) THEN
        ALTER TABLE notification_deliveries DROP CONSTRAINT chk_del_subject;
    END IF;
END $$;

ALTER TABLE notification_deliveries
    ADD CONSTRAINT chk_del_subject CHECK (
        (delivery_kind = 'notification' AND notification_id IS NOT NULL AND digest_batch_id IS NULL)
        OR (delivery_kind = 'digest' AND notification_id IS NULL AND digest_batch_id IS NOT NULL)
        OR (delivery_kind = 'anonymous' AND notification_id IS NULL AND digest_batch_id IS NULL)
    );
