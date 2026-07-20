DROP TABLE IF EXISTS campaign_decision_logs;
ALTER TABLE campaigns DROP CONSTRAINT IF EXISTS campaigns_active_rule_version_fk;
ALTER TABLE campaigns DROP COLUMN IF EXISTS active_rule_version;
DROP TABLE IF EXISTS campaign_rule_versions;
ALTER TABLE users DROP COLUMN IF EXISTS member_tags;
ALTER TABLE users DROP COLUMN IF EXISTS member_level;
