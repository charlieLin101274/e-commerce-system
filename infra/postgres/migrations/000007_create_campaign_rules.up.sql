ALTER TABLE users
    ADD COLUMN member_level VARCHAR(50) NOT NULL DEFAULT '',
    ADD COLUMN member_tags TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[];

CREATE TABLE campaign_rule_versions (
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    version INTEGER NOT NULL CHECK (version > 0),
    context_type VARCHAR(50) NOT NULL CHECK (context_type IN ('campaign_discovery', 'cart_recall')),
    eligibility_rule JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (campaign_id, version)
);

ALTER TABLE campaigns
    ADD COLUMN active_rule_version INTEGER,
    ADD CONSTRAINT campaigns_active_rule_version_fk
    FOREIGN KEY (id, active_rule_version)
    REFERENCES campaign_rule_versions(campaign_id, version)
    DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE campaign_decision_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id),
    rule_version INTEGER NOT NULL,
    context_type VARCHAR(50) NOT NULL,
    eligible BOOLEAN NOT NULL,
    reason_code VARCHAR(100) NOT NULL,
    facts_snapshot JSONB NOT NULL,
    matched_condition_ids TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    failed_condition_id TEXT,
    missing_facts TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    evaluation_duration_microseconds BIGINT NOT NULL CHECK (evaluation_duration_microseconds >= 0),
    evaluated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX campaign_decision_logs_campaign_evaluated_idx
    ON campaign_decision_logs (campaign_id, evaluated_at DESC);
