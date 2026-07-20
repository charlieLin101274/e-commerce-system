DROP TABLE IF EXISTS notification_delivery_receipts;
DROP TABLE IF EXISTS notification_tasks;
DROP TABLE IF EXISTS notification_templates;
DROP TYPE IF EXISTS notification_task_status;
DROP TYPE IF EXISTS notification_channel;
ALTER TABLE users DROP COLUMN IF EXISTS notification_channels;
ALTER TABLE users DROP COLUMN IF EXISTS marketing_consent;
