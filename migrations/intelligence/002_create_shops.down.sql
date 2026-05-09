DROP TRIGGER IF EXISTS trg_shop_registered_notify ON shops;
DROP FUNCTION IF EXISTS notify_shop_registered();
DROP TRIGGER IF EXISTS trg_shops_updated_at ON shops;
DROP TABLE IF EXISTS shops;
