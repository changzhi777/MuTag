// MuTag 管理后台 · 领域类型（与后端 domain 一致）
export type AssetStatus = "in_use" | "idle" | "repair" | "borrowed" | "scrapped";
export type TagStatus = "active" | "void";

export interface Asset {
  asset_id: number;
  tag_id: string;
  name: string;
  category: string | null;
  model: string | null;
  sn: string | null;
  buy_date: string | null;
  price: number | null;
  dept_id: string | null;
  keeper: string | null;
  location: string | null;
  status: AssetStatus;
  unit: string | null;
  warranty_end: string | null;
  remark: string | null;
  created_at: string;
  updated_at: string;
  // 0006 增强：部件层级 / 二级位置 / 定位预留 / 扩展属性
  parent_tag_id: string | null;
  location_id: number | null;
  qty: number;
  brand: string | null;
  keeper_phone: string | null;
  warranty_until: string | null;
  lat: number | null;
  lng: number | null;
  located_at: string | null;
}

// 两级位置树节点（parent_id null=一级区域，非空=二级点位）
export interface Location {
  id: number;
  name: string;
  parent_id: number | null;
  lat: number | null;
  lng: number | null;
  created_at: string;
}

export interface Tag {
  tag_id: string;
  qr_scene: string | null;
  url_code: string;
  status: TagStatus;
  created_at: string;
}

export interface AssetDraft {
  name: string;
  category?: string | null;
  model?: string | null;
  sn?: string | null;
  buy_date?: string | null;
  price?: number | null;
  dept_id?: string | null;
  keeper?: string | null;
  location?: string | null;
  status?: string;
  unit?: string | null;
  warranty_end?: string | null;
  remark?: string | null;
  // 0006 增强
  parent_tag_id?: string | null;
  location_id?: number | null;
  qty?: number;
  brand?: string | null;
  keeper_phone?: string | null;
  warranty_until?: string | null;
  lat?: number | null;
  lng?: number | null;
}

// 批量导入条目（SheetJS 解析后的标准化结构）
export interface ImportItem {
  seq: string;
  name: string;
  buy_date: string | null;
  qty: number;
  unit: string | null;
  keeper: string | null;
}

// ---------------------------------------------------------------------------
// M4 财务对接类型（与后端 domain 一致）
// ---------------------------------------------------------------------------

// 财务模板单列映射：模板列名 ↔ 系统字段
export interface FinColumn {
  name: string;
  field: string;
  readonly?: boolean;
}

// 财务模板映射定义（格式转换收敛在映射层，系统内统一存储）
export interface FinMappingDef {
  columns: FinColumn[];
  date_format: string;
  status_map: Record<string, string>;
  fixed_rows?: string[][];
}

// 财务模板实体
export interface FinMapping {
  mapping_id: string;
  name: string;
  kind: string;
  def: FinMappingDef;
  version: number;
  updated_at: string;
}

// 导出审计任务（财务合规：谁/何时/什么条件/导出几行）
export interface ExportJob {
  job_id: string;
  mapping_id: string;
  filters: Record<string, string>;
  actor: string;
  row_count: number;
  created_at: string;
}

// 导出响应：按模板输出行 + 列定义（前端 SheetJS 拼 XLSX）
export interface ExportResult {
  job_id: string;
  row_count: number;
  columns: FinColumn[];
  fixed_rows?: string[][];
  rows: string[][];
}

// 导入批次状态机：pending（预览落库）→ confirmed（入库）→ rolled_back（已回滚）
export type BatchStatus = "pending" | "confirmed" | "rolled_back";

// 导入批次
export interface ImportBatch {
  batch_id: string;
  mapping_id: string;
  file_name: string | null;
  summary: { creates: number; updates: number; errors: number };
  status: BatchStatus;
  actor: string;
  created_at: string;
  confirmed_at: string | null;
  rolled_back_at: string | null;
}

// 预览响应：差异明细
export interface PreviewResult {
  batch_id: string;
  creates: { row: number; fields: Record<string, string> }[];
  updates: { row: number; asset_id: number; tag_id: string; new: Record<string, string>; old: Record<string, string> }[];
  errors: { row: number; reason: string }[];
  summary: { creates: number; updates: number; errors: number };
}

export const STATUS_TEXT: Record<AssetStatus, string> = {
  in_use: "在用",
  idle: "闲置",
  repair: "维修中",
  borrowed: "借出",
  scrapped: "已报废"
};

export const STATUS_COLOR: Record<AssetStatus, string> = {
  in_use: "green",
  idle: "blue",
  repair: "orange",
  borrowed: "purple",
  scrapped: "default"
};
