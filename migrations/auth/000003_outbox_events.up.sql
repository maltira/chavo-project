-- transactional outbox для гарантированной доставки событий auth-events (account_deleted, session.revoked)
CREATE TABLE IF NOT EXISTS outbox_events (
    id           BIGSERIAL PRIMARY KEY,
    topic        VARCHAR(100) NOT NULL,
    event_key    TEXT NOT NULL,
    payload      JSONB NOT NULL,
    attempts     INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    failed_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON outbox_events (id)
WHERE published_at IS NULL AND failed_at IS NULL;
