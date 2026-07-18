DROP TABLE IF EXISTS campaign_categories;
DROP TABLE IF EXISTS campaign_products;
DROP TABLE IF EXISTS campaigns;
DROP TYPE IF EXISTS campaign_benefit_type;
DROP TYPE IF EXISTS campaign_status;
DROP INDEX IF EXISTS products_category_status_idx;
ALTER TABLE products DROP COLUMN IF EXISTS category;
