import { useState } from "react";
import { Alert, Button, Card, Space, Table, Typography, Upload, message } from "antd";
import { UploadOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import * as XLSX from "xlsx";
import { post } from "../api/client";
import type { ImportItem } from "../types";

// xlsx 列头 → 标准字段（与 seed 链路同构：序号/名称/采购日期/数量/单位/领用人签字）
function parseWorkbook(file: ArrayBuffer): ImportItem[] {
  const wb = XLSX.read(file, { cellDates: true });
  const sheet = wb.Sheets[wb.SheetNames[0]];
  const rows = XLSX.utils.sheet_to_json<Record<string, unknown>>(sheet, { defval: "" });

  const fmtDate = (v: unknown): string | null => {
    if (v instanceof Date) {
      const m = String(v.getMonth() + 1).padStart(2, "0");
      const d = String(v.getDate()).padStart(2, "0");
      return `${v.getFullYear()}-${m}-${d}`;
    }
    const s = String(v).trim();
    return s || null;
  };

  return rows
    .filter((r) => String(r["名称"] ?? "").trim() !== "")
    .map((r) => ({
      seq: String(r["序号"] ?? "").trim(),
      name: String(r["名称"]).trim(),
      buy_date: fmtDate(r["采购日期"]),
      qty: Math.max(parseInt(String(r["数量"] ?? "1"), 10) || 1, 1),
      unit: String(r["单位"] ?? "").trim() || null,
      keeper: String(r["领用人签字"] ?? "").trim() || null
    }));
}

export default function ImportPage() {
  const [items, setItems] = useState<ImportItem[]>([]);
  const [fileName, setFileName] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const beforeUpload = async (file: File): Promise<false> => {
    try {
      const buf = await file.arrayBuffer();
      const parsed = parseWorkbook(buf);
      if (parsed.length === 0) {
        message.error("未解析到有效行（需含「名称」列）");
        return false;
      }
      setItems(parsed);
      setFileName(file.name);
      message.success(`解析 ${parsed.length} 行`);
    } catch (e) {
      message.error(`解析失败：${(e as Error).message}`);
    }
    return false; // 阻止 antd 自动上传，全走手动提交
  };

  const onSubmit = async () => {
    setSubmitting(true);
    try {
      const res = await post<{ created: number; last_tag: string }>("/import", { items });
      message.success(`导入 ${res.created} 条，末码 ${res.last_tag}`);
      setItems([]);
      setFileName("");
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  const totalQty = items.reduce((s, it) => s + it.qty, 0);

  const columns: ColumnsType<ImportItem> = [
    { title: "来源序号", dataIndex: "seq", width: 90 },
    { title: "名称", dataIndex: "name", ellipsis: true },
    { title: "采购日期", dataIndex: "buy_date", width: 110 },
    { title: "数量", dataIndex: "qty", width: 70 },
    { title: "单位", dataIndex: "unit", width: 70 },
    { title: "领用人", dataIndex: "keeper", width: 110 },
    { title: "配码数", width: 90, render: (_, r) => `MT 序号 +${r.qty}` }
  ];

  return (
    <Card>
      <Typography.Title level={5}>批量导入（一物一码拆分）</Typography.Title>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="上传《固定资产需打标签》同结构的 xlsx：序号 / 名称 / 采购日期 / 数量 / 单位 / 领用人签字。数量>1 的行自动拆分为多条资产、各配一码。"
      />
      <Space direction="vertical" style={{ width: "100%" }} size="middle">
        <Upload accept=".xlsx,.xls" maxCount={1} beforeUpload={beforeUpload} showUploadList={false}>
          <Button icon={<UploadOutlined />}>选择 xlsx 文件</Button>
        </Upload>
        {fileName && (
          <Alert type="success" showIcon message={`已解析：${fileName}（${items.length} 行 / 拆分后 ${totalQty} 条资产）`} />
        )}
        {items.length > 0 && (
          <>
            <Table
              rowKey={(r) => `${r.seq}-${r.name}`}
              columns={columns}
              dataSource={items}
              size="small"
              pagination={{ pageSize: 10, showTotal: (t) => `共 ${t} 行` }}
            />
            <Button type="primary" loading={submitting} onClick={() => void onSubmit()}>
              确认导入 {totalQty} 条资产
            </Button>
          </>
        )}
      </Space>
    </Card>
  );
}
