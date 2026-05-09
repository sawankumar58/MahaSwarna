DROP TRIGGER IF EXISTS trg_subscription_notify ON subscriptions;
DROP FUNCTION IF EXISTS notify_subscription_change();
DROP TRIGGER IF EXISTS trg_subscriptions_updated_at ON subscriptions;
DROP TABLE IF EXISTS subscriptions;
