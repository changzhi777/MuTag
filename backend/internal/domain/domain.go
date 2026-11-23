// Package domain 定义 MuTag 核心领域模型：码、资产、履历。
// 全部为纯数据结构 + 状态枚举，不依赖任何基础设施库。
package domain

import "time"

// TagStatus 码状态：active 可扫 / void 已作废（归档不复用）。
type TagStatus string

const (
	TagActive TagStatus = "active"
	TagVoid   TagStatus = "void"
)

// Tag 码表实体。tag_id 永不 recycle，报废后置 void 归档。
type Tag struct {
	TagID     string    `json:"tag_id"`
	QrScene   string    `json:"qr_scene"` // 小程序码 scene：t:{tag_id}
	UrlCode   string    `json:"url_code"` // 活码 URL 路径段
	Status    TagStatus `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// AssetStatus 资产状态：在用/闲置/维修/借出/报废。
type AssetStatus string

const (
	AssetInUse  AssetStatus = "in_use"
	AssetIdle   AssetStatus = "idle"
	AssetRepair AssetStatus = "repair"
	AssetBorrow AssetStatus = "borrowed"
	AssetScrap  AssetStatus = "scrapped"
)

// Asset 资产实体，一物一码（tag_id 唯一绑定）。
// 部件模型：parent_tag_id 非空 = 该资产是 parent 下的部件（部件也是一条资产，自带码可独立扫码/履历）；
// 多件部件按采购批次各建一条记录，qty 记录该批次数量。
type Asset struct {
	AssetID     int64       `json:"asset_id"`
	TagID       string      `json:"tag_id"`
	Name        string      `json:"name"`
	Category    *string     `json:"category"`
	Model       *string     `json:"model"`
	Sn          *string     `json:"sn"`
	BuyDate     *time.Time  `json:"buy_date"`
	Price       *float64    `json:"price"`
	DeptID      *string     `json:"dept_id"`
	Keeper      *string     `json:"keeper"`
	Location    *string     `json:"location"` // 旧自由文本位置（历史兼容，展示优先取位置树）
	Status      AssetStatus `json:"status"`
	Unit        *string     `json:"unit"`
	WarrantyEnd *time.Time  `json:"warranty_end"`
	Remark      *string     `json:"remark"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`

	// 0006 增强：部件层级 / 二级位置 / 定位预留 / 扩展属性
	ParentTagID  *string  `json:"parent_tag_id"`            // 所属设备 tag_id（NULL=独立资产/设备本体）
	LocationID   *int64   `json:"location_id"`              // 安装位置（locations 两级树叶子/区域）
	Qty          int      `json:"qty"`                      // 数量（部件按采购批次各一条，默认 1）
	Brand        *string  `json:"brand"`                    // 品牌
	KeeperPhone  *string  `json:"keeper_phone"`             // 保管人电话
	WarrantyUntil *time.Time `json:"warranty_until"`        // 维保到期日
	Lat          *float64 `json:"lat"`                      // 最近定位纬度（动态定位预留）
	Lng          *float64 `json:"lng"`                      // 最近定位经度
	LocatedAt    *string  `json:"located_at"`               // 最近定位时间
}

// Location 两级位置树节点：parent_id 为空 = 一级（区域），非空 = 二级（点位）。
// lat/lng 为该位置的静态安装坐标（定位功能预留）。
type Location struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	ParentID  *int64    `json:"parent_id"`
	Lat       *float64  `json:"lat"`
	Lng       *float64  `json:"lng"`
	CreatedAt time.Time `json:"created_at"`
}

// Admin 后台管理员（0007 多管理员；password_hash 格式 sha256$<salt-hex>$<hash-hex>）。
type Admin struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"display_name"`
	CreatedAt    time.Time `json:"created_at"`
}

// EventType 履历类型。
type EventType string

const (
	EventCreate   EventType = "create"
	EventReceive  EventType = "receive"
	EventReturn   EventType = "return"
	EventRepair   EventType = "repair"
	EventTransfer EventType = "transfer"
	EventScrap    EventType = "scrap"
	EventUpdate   EventType = "update"
)

// UserRole 用户角色（设计方案权限模型：admin 全部 / asset_mgr 资产管理 / staff 默认只读+上报）。
type UserRole string

const (
	RoleAdmin    UserRole = "admin"
	RoleAssetMgr UserRole = "asset_mgr"
	RoleStaff    UserRole = "staff"
)

// User 微信用户（M2 登录态）。
type User struct {
	OpenID    string    `json:"openid"`
	Nickname  string    `json:"nickname"`
	Role      UserRole  `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// AssetEvent 资产履历：字段级版本快照（旧值→新值），全量留痕无旁路。
type AssetEvent struct {
	EventID   int64          `json:"event_id"`
	AssetID   int64          `json:"asset_id"`
	Type      EventType      `json:"type"`
	Actor     string         `json:"actor"`
	OldValue  map[string]any `json:"old_value"`
	NewValue  map[string]any `json:"new_value"`
	Note      *string        `json:"note"`
	CreatedAt time.Time      `json:"created_at"`
}

// FinColumn 财务模板单列映射：模板列名 ↔ 系统字段。
type FinColumn struct {
	Name     string `json:"name"`               // 模板列名（如「原值」）
	Field    string `json:"field"`              // 系统字段（assets 列白名单）
	Readonly bool   `json:"readonly,omitempty"` // 只读字段（tag_id）导出可用、导入忽略
}

// FinMappingDef 财务模板映射定义（存 fin_mappings.mapping 的 JSON 结构）。
// 纪律 7：金额/日期/状态格式转换全部收敛在此映射层，系统内统一存储。
type FinMappingDef struct {
	Columns    []FinColumn       `json:"columns"`
	DateFormat string            `json:"date_format"`          // 导出日期格式，默认 YYYY-MM-DD
	StatusMap  map[string]string `json:"status_map"`           // 系统状态 → 模板中文（导入反向查）
	FixedRows  [][]string        `json:"fixed_rows,omitempty"` // 表头附加行（单位名称等）
}

// FinMapping 财务模板实体。
type FinMapping struct {
	MappingID string        `json:"mapping_id"`
	Name      string        `json:"name"`
	Kind      string        `json:"kind"` // yonyou_u8 / kingdee / guanjiapo / generic
	Def       FinMappingDef `json:"def"`
	Version   int64         `json:"version"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// ExportJob 导出审计任务（财务合规：记录操作人与过滤条件）。
type ExportJob struct {
	JobID     string         `json:"job_id"`
	MappingID string         `json:"mapping_id"`
	Filters   map[string]any `json:"filters"`
	Actor     string         `json:"actor"`
	RowCount  int64          `json:"row_count"`
	CreatedAt time.Time      `json:"created_at"`
}

// ImportBatchStatus 导入批次状态机：pending（预览落库）→ confirmed（入库）→ rolled_back（已回滚）。
type ImportBatchStatus string

const (
	BatchPending    ImportBatchStatus = "pending"
	BatchConfirmed  ImportBatchStatus = "confirmed"
	BatchRolledBack ImportBatchStatus = "rolled_back"
)

// ImportBatch 导入批次：preview→confirm 两段式，items 全量明细支撑按批次回滚。
type ImportBatch struct {
	BatchID      string            `json:"batch_id"`
	MappingID    string            `json:"mapping_id"`
	FileName     *string           `json:"file_name"`
	Summary      map[string]any    `json:"summary"`
	Items        map[string]any    `json:"items"`
	Status       ImportBatchStatus `json:"status"`
	Actor        string            `json:"actor"`
	CreatedAt    time.Time         `json:"created_at"`
	ConfirmedAt  *time.Time        `json:"confirmed_at"`
	RolledBackAt *time.Time        `json:"rolled_back_at"`
}
