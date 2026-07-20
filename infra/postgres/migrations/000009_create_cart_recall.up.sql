CREATE TYPE cart_recall_status AS ENUM (
    'scheduled', 'evaluating', 'notification_pending', 'sent',
    'converted', 'skipped', 'cancelled'
);

CREATE TABLE domain_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(100) NOT NULL,
    aggregate_id UUID NOT NULL,
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    processing_started_at TIMESTAMPTZ
);
CREATE INDEX domain_outbox_pending_idx ON domain_outbox (occurred_at, id)
    WHERE published_at IS NULL;

CREATE TABLE event_inbox (
    consumer VARCHAR(100) NOT NULL,
    event_id UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (consumer, event_id)
);

CREATE TABLE cart_recall_journeys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    cart_id UUID NOT NULL REFERENCES carts(id),
    source_event_id UUID NOT NULL UNIQUE,
    status cart_recall_status NOT NULL DEFAULT 'scheduled',
    evaluate_at TIMESTAMPTZ NOT NULL,
    campaign_id UUID REFERENCES campaigns(id),
    rule_version INTEGER NOT NULL DEFAULT 0,
    matched_product_ids UUID[] NOT NULL DEFAULT ARRAY[]::UUID[],
    matched_products_snapshot JSONB,
    notification_task_id UUID REFERENCES notification_tasks(id),
    converted_order_id UUID REFERENCES orders(id),
    cancel_reason VARCHAR(100),
    processing_started_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((status <> 'converted') OR converted_order_id IS NOT NULL)
);
CREATE INDEX cart_recall_due_idx ON cart_recall_journeys (evaluate_at, id)
    WHERE status IN ('scheduled', 'evaluating');
CREATE INDEX cart_recall_user_status_idx ON cart_recall_journeys (user_id, status);

-- Only one still-actionable journey is retained for each cart. A later cart
-- mutation reschedules it instead of generating notification fan-out.
CREATE UNIQUE INDEX cart_recall_active_cart_idx ON cart_recall_journeys (cart_id)
    WHERE status IN ('scheduled', 'evaluating', 'notification_pending');

