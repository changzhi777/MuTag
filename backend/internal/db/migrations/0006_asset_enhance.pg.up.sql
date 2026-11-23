-- 0006 (PG) 资产模型增强：部件层级 + 二级位置 + 定位预留 + 扩展字段。
-- 新表 locations 两级位置树（parent_id NULL=一级区域，非空=二级点位），带静态坐标。
-- assets 新增（全部可空/带默认，存量零迁移成本）：parent_tag_id / location_id / qty /
--   brand / keeper_phone / warranty_until / lat / lng / located_at。
CREATE TABLE locations (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    parent_id  BIGINT REFERENCES locations(id),
    lat        DOUBLE PRECISION,
    lng        DOUBLE PRECISION,
    created_at TEXT NOT NULL DEFAULT (now()::text)
);
CREATE INDEX idx_locations_parent ON locations(parent_id);

ALTER TABLE assets ADD COLUMN parent_tag_id TEXT REFERENCES tags(tag_id);
ALTER TABLE assets ADD COLUMN location_id  BIGINT REFERENCES locations(id);
ALTER TABLE assets ADD COLUMN qty  INTEGER NOT NULL DEFAULT 1;
ALTER TABLE assets ADD COLUMN brand TEXT;
ALTER TABLE assets ADD COLUMN keeper_phone TEXT;
ALTER TABLE assets ADD COLUMN warranty_until TEXT;
ALTER TABLE assets ADD COLUMN lat DOUBLE PRECISION;
ALTER TABLE assets ADD COLUMN lng DOUBLE PRECISION;
ALTER TABLE assets ADD COLUMN located_at TEXT;

CREATE INDEX idx_assets_parent_tag ON assets(parent_tag_id);
CREATE INDEX idx_assets_location ON assets(location_id);
