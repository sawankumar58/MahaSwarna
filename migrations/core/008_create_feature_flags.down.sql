DROP TRIGGER IF EXISTS trg_flag_notify ON feature_flags;
DROP FUNCTION IF EXISTS notify_flag_change();
DROP TRIGGER IF EXISTS trg_feature_flags_updated_at ON feature_flags;
DROP TABLE IF EXISTS feature_flags;
