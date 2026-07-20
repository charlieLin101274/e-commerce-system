ALTER TABLE users
    ADD COLUMN marketing_consent BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN notification_channels TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[];

CREATE TYPE notification_channel AS ENUM ('in_app', 'push');
CREATE TYPE notification_task_status AS ENUM (
    'pending', 'processing', 'sent', 'delivered', 'opened',
    'retry_scheduled', 'failed', 'cancelled'
);

CREATE TABLE notification_templates (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    channel notification_channel NOT NULL,
    title_template TEXT NOT NULL,
    body_template TEXT NOT NULL,
    deep_link_template TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL CHECK (version > 0),
    status VARCHAR(20) NOT NULL CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, version)
);

CREATE TABLE notification_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    campaign_id UUID REFERENCES campaigns(id),
    journey_type VARCHAR(50) NOT NULL,
    journey_id UUID NOT NULL,
    template_id UUID NOT NULL,
    template_version INTEGER NOT NULL CHECK (template_version > 0),
    channel notification_channel NOT NULL,
    status notification_task_status NOT NULL DEFAULT 'pending',
    idempotency_key VARCHAR(300) NOT NULL UNIQUE,
    scheduled_at TIMESTAMPTZ NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL,
    processing_started_at TIMESTAMPTZ,
    sent_at TIMESTAMPTZ,
    opened_at TIMESTAMPTZ,
    failure_code VARCHAR(100),
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (template_id, template_version)
        REFERENCES notification_templates(id, version)
);

CREATE INDEX notification_tasks_delivery_idx
    ON notification_tasks (status, next_attempt_at, scheduled_at);
CREATE INDEX notification_tasks_user_created_idx
    ON notification_tasks (user_id, created_at DESC);
CREATE INDEX notification_tasks_campaign_user_sent_idx
    ON notification_tasks (campaign_id, user_id, sent_at DESC)
    WHERE sent_at IS NOT NULL;
CREATE UNIQUE INDEX notification_tasks_active_journey_idx
    ON notification_tasks (journey_type, journey_id)
    WHERE status IN ('pending', 'processing', 'retry_scheduled');

CREATE TABLE notification_delivery_receipts (
    task_id UUID PRIMARY KEY REFERENCES notification_tasks(id) ON DELETE CASCADE,
    idempotency_key VARCHAR(300) NOT NULL UNIQUE,
    delivered_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO notification_templates (
    id, channel, title_template, body_template, deep_link_template, version, status
) VALUES
    ('10000000-0000-0000-0000-000000000001', 'in_app', '{{.title}}', '{{.body}}', 'ecommerce://products/{{.product_id}}', 1, 'active'),
    ('10000000-0000-0000-0000-000000000002', 'push', '{{.title}}', '{{.body}}', 'ecommerce://products/{{.product_id}}', 1, 'active');
