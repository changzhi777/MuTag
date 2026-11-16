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
	Location    *string     `json:"location"`
	Status      AssetStatus `json:"status"`
	Unit        *string     `json:"unit"`
	WarrantyEnd *time.Time  `json:"warranty_end"`
	Remark      *string     `json:"remark"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
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
