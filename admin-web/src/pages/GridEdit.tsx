import { useCallback, useEffect, useMemo, useState } from "react";
import { App, Alert, Button, Card, Popconfirm, Space, Spin, Statistic, Table, Tag, Typography } from "antd";
import { CheckCircleOutlined, ReloadOutlined } from "@ant-design/icons";
import { DataGrid, renderTextEditor, type Column } from "react-data-grid";
import "react-data-grid/lib/styles.css";
import { api, post } from "../api/client";
import type { Asset, PreviewResult } from "../types";
import { STATUS_TEXT } from "../types";

// 网格行：_tag_id 走后端网格匹配通道（fin.go diffRows）；其余字段与系统字段同名
interface GridRow extends Record<string, string> {
  _tag_id: string;
  tag_id: string;
  name: string;
  category: string;
  model: string;
  sn: string;
  buy_date: string;
  price: string;
  dept_id: string;
  keeper: string;
  location: string;
  status: string;
  unit: string;
  remark: string;
  brand: string;
  keeper_phone: string;
  warranty_until: string;
  qty: string;
}

const toGridRow = (a: Asset): GridRow => ({
  _tag_id: a.tag_id,
  tag_id: a.tag_id,
  name: a.name,
  category: a.category || "",
  model: a.model || "",
  sn: a.sn || "",
  buy_date: a.buy_date || "",
  price: a.price == null ? "" : a.price.toFixed(2),
  dept_id: a.dept_id || "",
  keeper: a.keeper || "",
  location: a.location || "",
  status: a.status,
  unit: a.unit || "",
  remark: a.remark || "",
  brand: a.brand || "",
  keeper_phone: a.keeper_phone || "",
  warranty_until: a.warranty_until || "",
  qty: String(a.qty ?? 1)
});

// 可编辑列（tag_id 只读——纪律 1）
const EDITOR_COLS: { key: string; name: string; width?: number }[] = [
  { key: "name", name: "名称", width: 180 },
  { key: "category", name: "类别", width: 100 },
  { key: "model", name: "规格型号", width: 130 },
  { key: "sn", name: "序列号", width: 130 },
  { key: "buy_date", name: "购置日期", width: 110 },
  { key: "price", name: "金额", width: 90 },
  { key: "dept_id", name: "部门", width: 90 },
  { key: "keeper", name: "保管人", width: 100 },
  { key: "location", name: "存放地点", width: 120 },
  { key: "status", name: "状态", width: 90 },
  { key: "unit", name: "单位", width: 70 },
  { key: "remark", name: "备注", width: 160 },
  { key: "brand", name: "品牌", width: 110 },
  { key: "keeper_phone", name: "保管人电话", width: 130 },
  { key: "warranty_until", name: "维保到期", width: 110 },
  { key: "qty", name: "数量", width: 70 }
];

const SYSTEM_STATUS = new Set(Object.keys(STATUS_TEXT));
void SYSTEM_STATUS; // 预留：状态下拉编辑器（当前文本编辑，后端值域兜底）

// 表格编辑页：台账全量网格行内编辑（类 Excel）→ 改动集走差异预览 → 人工确认入库（审计/回滚链路全复用）
export default function GridEdit() {
  const { message } = App.useApp();
  const [rows, setRows] = useState<GridRow[]>([]);
  const [snapshot, setSnapshot] = useState<GridRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [preview, setPreview] = useState<PreviewResult | null>(null);
  const [confirming, setConfirming] = useState(false);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api<{ items: Asset[] }>("/assets?size=0");
      const grid = res.items.map(toGridRow);
      setRows(grid);
      setSnapshot(grid);
      setPreview(null);
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  // 改动行：与快照逐字段 diff（tag_id/name 之外的空白编辑忽略）
  const dirty = useMemo(
    () =>
      rows.filter((r) => {
        const orig = snapshot.find((s) => s._tag_id === r._tag_id);
        if (!orig) return false;
        return Object.keys(r).some((k) => k !== "_tag_id" && k !== "tag_id" && r[k] !== orig[k]);
      }),
    [rows, snapshot]
  );

  const columns: Column<GridRow>[] = useMemo(
    () => [
      {
        key: "tag_id",
        name: "资产码",
        width: 130,
        editable: false,
        renderCell: ({ row }) => <Typography.Text code style={{ fontSize: 12 }}>{row.tag_id}</Typography.Text>
      },
      ...EDITOR_COLS.map(
        (c): Column<GridRow> => ({
          key: c.key,
          name: c.name,
          width: c.width,
          editable: true,
          renderEditCell: renderTextEditor, // rdg beta.64 必须显式给编辑器，仅 editable:true 无编辑器=不能编辑
          renderSummaryCell: undefined,
          cellClass: (row: GridRow) => {
            const orig = snapshot.find((s) => s._tag_id === row._tag_id);
            return orig && row[c.key] !== orig[c.key] ? "rdg-dirty" : "";
          }
        })
      )
    ],
    [snapshot]
  );

  const onPreview = async () => {
    if (dirty.length === 0) {
      message.info("没有改动");
      return;
    }
    try {
      const payloadRows = dirty.map((r) => {
        const orig = snapshot.find((s) => s._tag_id === r._tag_id)!;
        const fields: Record<string, string> = { _tag_id: r._tag_id };
        for (const c of EDITOR_COLS) {
          if (r[c.key] !== orig[c.key]) {
            fields[c.key] = r[c.key]; // 只发改动字段；空值=后端忽略
          }
        }
        return fields;
      });
      const res = await post<PreviewResult>("/fin/import/preview", {
        mapping_id: "generic",
        file_name: "表格编辑",
        rows: payloadRows
      });
      setPreview(res);
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const onConfirm = async () => {
    if (!preview) return;
    setConfirming(true);
    try {
      const res = await post<{ created: number; updated: number }>(
        `/fin/import/batches/${preview.batch_id}/confirm`, {}
      );
      message.success(`已入库：更新 ${res.updated} 条（履历已留痕）`);
      void reload();
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setConfirming(false);
    }
  };

  const errCols: ColumnsTypeOf<{ row: number; reason: string }> = [
    { title: "行", dataIndex: "row", width: 60 },
    { title: "原因", dataIndex: "reason", render: (r: string) => <Typography.Text type="danger">{r}</Typography.Text> }
  ];

  return (
    <div>
      <Space wrap style={{ marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>表格编辑</Typography.Title>
        <Statistic title="改动行" value={dirty.length} valueStyle={{ color: dirty.length > 0 ? "#fa8c16" : undefined }} />
        <Statistic title="资产总数" value={rows.length} />
        <Button icon={<ReloadOutlined />} onClick={() => void reload()}>重载</Button>
        <Popconfirm
          title="改动集将生成差异预览，确认？"
          disabled={dirty.length === 0}
          onConfirm={() => void onPreview()}
        >
          <Button type="primary" disabled={dirty.length === 0} icon={<CheckCircleOutlined />}>
            生成预览
          </Button>
        </Popconfirm>
        {preview && (
          <Popconfirm title={`按预览入库（更新 ${preview.summary.updates} / 错误 ${preview.summary.errors}），确认？`} onConfirm={() => void onConfirm()}>
            <Button type="primary" danger loading={confirming}>确认入库</Button>
          </Popconfirm>
        )}
      </Space>

      {preview && (
        <Alert
          style={{ marginBottom: 16 }}
          type={preview.summary.errors > 0 ? "warning" : "success"}
          message={`差异预览：更新 ${preview.summary.updates} 行${preview.summary.errors > 0 ? `，错误 ${preview.summary.errors} 行（将被跳过）` : ""}`}
          description={
            preview.errors.length > 0 ? (
              <Table rowKey="row" size="small" columns={errCols} dataSource={preview.errors} pagination={false} style={{ marginTop: 8 }} />
            ) : undefined
          }
        />
      )}

      <Card size="small">
        <Typography.Text type="secondary" style={{ display: "block", marginBottom: 8 }}>
          <Tag>说明</Tag> 双击单元格编辑；改动列高亮；资产码只读（tagId 永不 recycle）；提交走差异预览→确认链路，全部留审计可回滚
        </Typography.Text>
        {loading ? (
          <div style={{ textAlign: "center", padding: 40 }}><Spin /></div>
        ) : (
          <DataGrid
            columns={columns}
            rows={rows}
            onRowsChange={setRows}
            getRowKey={(r: GridRow) => r._tag_id}
            rowHeight={34}
            className="fill-grid"
            style={{ height: "calc(100vh - 260px)" }}
          />
        )}
      </Card>
    </div>
  );
}

// antd Table 列类型别称（局部使用）
type ColumnsTypeOf<T> = import("antd/es/table").ColumnsType<T>;
