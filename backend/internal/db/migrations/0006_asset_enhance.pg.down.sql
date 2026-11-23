-- 0006 (PG down) 还原资产模型（丢弃 locations 表与 assets 扩展列）。
DROP INDEX IF EXISTS idx_assets_location;
DROP INDEX IF EXISTS idx_assets_parent_tag;
DROP TABLE IF EXISTS locations;
ALTER TABLE assets DROP COLUMN located_at;
ALTER TABLE assets DROP COLUMN lng;
ALTER TABLE assets DROP COLUMN lat;
ALTER TABLE assets DROP COLUMN warranty_until;
ALTER TABLE assets DROP COLUMN keeper_phone;
ALTER TABLE assets DROP COLUMN brand;
ALTER TABLE assets DROP COLUMN qty;
ALTER TABLE assets DROP COLUMN location_id;
ALTER TABLE assets DROP COLUMN parent_tag_id;
