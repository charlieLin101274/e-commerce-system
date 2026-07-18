ALTER TABLE products
    ADD COLUMN category VARCHAR(100) NOT NULL DEFAULT '';

ALTER TABLE products
    ADD CONSTRAINT products_category_canonical_check
    CHECK (category = LOWER(BTRIM(category)));

CREATE INDEX products_category_status_idx ON products (category, status);

CREATE TYPE campaign_status AS ENUM ('draft', 'scheduled', 'running', 'paused', 'ended', 'archived');
CREATE TYPE campaign_benefit_type AS ENUM ('fixed_amount', 'percentage');

CREATE TABLE campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(200) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status campaign_status NOT NULL DEFAULT 'draft',
    priority INTEGER NOT NULL DEFAULT 0,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    promotion_title VARCHAR(200) NOT NULL,
    promotion_description TEXT NOT NULL DEFAULT '',
    benefit_type campaign_benefit_type NOT NULL,
    benefit_value BIGINT NOT NULL CHECK (benefit_value > 0),
    maximum_discount_amount BIGINT CHECK (maximum_discount_amount > 0),
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    CHECK (starts_at < ends_at),
    CHECK (benefit_type = 'percentage' OR maximum_discount_amount IS NULL),
    CHECK (benefit_type <> 'percentage' OR benefit_value <= 100)
);

CREATE TABLE campaign_products (
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id),
    PRIMARY KEY (campaign_id, product_id)
);

CREATE TABLE campaign_categories (
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    category VARCHAR(100) NOT NULL CHECK (category <> '' AND category = LOWER(BTRIM(category))),
    PRIMARY KEY (campaign_id, category)
);

CREATE INDEX campaigns_public_lookup_idx
    ON campaigns (status, starts_at, ends_at, priority DESC, id);
CREATE INDEX campaign_products_product_id_idx ON campaign_products (product_id, campaign_id);
CREATE INDEX campaign_categories_category_idx ON campaign_categories (category, campaign_id);
