-- 0006 (SQLite) 资产模型增强：部件层级 + 二级位置 + 定位预留 + 扩展字段。
-- 新表 locations 两级位置树（parent_id NULL=一级区域，非空=二级点位），带静态坐标。
-- assets 新增（全部可空/带默认，存量零迁移成本）：
--   parent_tag_id 部件挂载（部件=子资产，一物一码独立扫码/履历）
--   location_id   结构化安装位置（locations 两级树）
--   qty           部件数量（同部件多批次采购=多条部件记录各带数量，默认 1）
--   brand / keeper_phone / warranty_until  扩展属性
--   lat / lng / located_at                 最近定位（动态定位功能预留）
CREATE TABLE locations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    parent_id  INTEGER REFERENCES locations(id),
    lat        REAL,
    lng        REAL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now'))
);
CREATE INDEX IF NOT EXISTS idx_locations_parent ON locations(parent_id);

ALTER TABLE assets ADD COLUMN parent_tag_id TEXT REFERENCES tags(tag_id);
ALTER TABLE assets ADD COLUMN location_id  INTEGER REFERENCES locations(id);
ALTER TABLE assets ADD COLUMN qty  INTEGER NOT NULL DEFAULT 1;
ALTER TABLE assets ADD COLUMN brand TEXT;
ALTER TABLE assets ADD COLUMN keeper_phone TEXT;
ALTER TABLE assets ADD COLUMN warranty_until TEXT;
ALTER TABLE assets ADD COLUMN lat REAL;
ALTER TABLE assets ADD COLUMN lng REAL;
ALTER TABLE assets ADD COLUMN located_at TEXT;

CREATE INDEX IF NOT EXISTS idx_assets_parent_tag ON assets(parent_tag_id);
CREATE INDEX IF NOT EXISTS idx_assets_location ON assets(location_id);
