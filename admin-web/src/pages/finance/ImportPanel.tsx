import { useCallback, useEffect, useState } from "react";
import {
  Alert, Button, Card, Popconfirm, Select, Space, Statistic, Table, Tag, Typography, Upload, message
} from "antd";
import { CheckCircleOutlined, FileExcelOutlined, RollbackOutlined, UploadOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import * as XLSX from "xlsx";
import { api, post } from "../../api/client";
import type { ImportBatch, PreviewResult } from "../../types";

// Excel 日期序列 → 本地 YYYY-MM-DD（toISOString 是 UTC 会偏一天，手拼）
const fmtLocalDate = (d: Date): string =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

// SheetJS 读出的行列 JSON：键=模板列名；Date 值统一转本地日期字符串
function readRows(file: ArrayBuffer): Record<string, unknown>[] {
  const wb = XLSX.read(file, { cellDates: true });
  const sheet = wb.Sheets[wb.SheetNames[0]];
  const raw = XLSX.utils.sheet_to_json<Record<string, unknown>>(sheet, { defval: "" });
  return raw.map((row) => {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(row)) {
      out[String(k).trim()] = v instanceof Date ? fmtLocalDate(v) : v;
    }
    return out;
  });
}

const BATCH_TAG: Record<string, { color: string; text: string }> = {
  pending: { color: "orange", text: "待确认" },
  confirmed: { color: "green", text: "已入库" },
  rolled_back: { color: "default", text: "已回滚" }
};

// 财务导入面板：上传 → 按模板反向解析出差异（新增/更新/错误）→ 人工确认入库 → 支持按批次回滚
export default function ImportPanel({ mappings }: { mappings: { mapping_id: string; name: string }[] }) {
  const [mappingID, setMappingID] = useState("");
  const [fileName, setFileName] = useState("");
  const [preview, setPreview] = useState<PreviewResult | null>(null);
  const [batches, setBatches] = useState<ImportBatch[]>([]);
  const [confirming, setConfirming] = useState(false);

  const reloadBatches = useCallback(async () => {
    try {
      const res = await api<{ items: ImportBatch[] }>("/fin/import/batches");
      setBatches(res.items);
    } catch (e) {
      message.error((e as Error).message);
    }
  }, []);

  useEffect(() => {
    if (mappings.length > 0 && !mappingID) setMappingID(mappings[0].mapping_id);
  }, [mappings, mappingID]);
  useEffect(() => {
    void reloadBatches();
  }, [reloadBatches]);

  const beforeUpload = async (file: File): Promise<false> => {
    if (!mappingID) {
      message.error("请先选择模板");
      return false;
    }
    try {
      const rows = readRows(await file.arrayBuffer());
      if (rows.length === 0) {
        message.error("未解析到数据行");
        return false;
      }
      const res = await post<PreviewResult>("/fin/import/preview", {
        mapping_id: mappingID,
        file_name: file.name,
        rows
      });
      setPreview(res);
      setFileName(file.name);
      message.success(`解析完成：新增 ${res.summary.creates} / 更新 ${res.summary.updates} / 错误 ${res.summary.errors}`);
    } catch (e) {
      message.error((e as Error).message);
    }
    return false;
  };

  const onConfirm = async () => {
    if (!preview) return;
    setConfirming(true);
    try {
      const res = await post<{ created: number; updated: number }>(
        `/fin/import/batches/${preview.batch_id}/confirm`, {}
      );
      message.success(`已入库：新增 ${res.created} / 更新 ${res.updated}`);
      setPreview(null);
      setFileName("");
      void reloadBatches();
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setConfirming(false);
    }
  };

  const onRollback = async (batchID: string) => {
    try {
      await post(`/fin/import/batches/${batchID}/rollback`, {});
      message.success("批次已回滚，账面已恢复");
      void reloadBatches();
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const errCols: ColumnsType<{ row: number; reason: string }> = [
    { title: "文件行号", dataIndex: "row", width: 90 },
    { title: "错误原因", dataIndex: "reason", render: (r) => <Typography.Text type="danger">{r}</Typography.Text> }
  ];
  const createCols: ColumnsType<{ row: number; fields: Record<string, string> }> = [
    { title: "文件行号", dataIndex: "row", width: 90 },
    { title: "名称", dataIndex: ["fields", "name"] },
    { title: "金额", dataIndex: ["fields", "price"], width: 100 },
    { title: "日期", dataIndex: ["fields", "buy_date"], width: 110 },
    { title: "SN", dataIndex: ["fields", "sn"], width: 130 }
  ];
  const updateCols: ColumnsType<PreviewResult["updates"][number]> = [
    { title: "文件行号", dataIndex: "row", width: 90 },
    { title: "资产码", dataIndex: "tag_id", width: 150 },
    {
      title: "变化明细",
      dataIndex: "new",
      render: (_, rec) => (
        <Space direction="vertical" size={0}>
          {Object.keys(rec.new).map((f) => (
            <Typography.Text key={f} style={{ fontSize: 12 }}>
              {f}：{rec.old[f] || "（空）"} → <b>{rec.new[f]}</b>
            </Typography.Text>
          ))}
        </Space>
      )
    }
  ];
  const batchCols: ColumnsType<ImportBatch> = [
    { title: "批次", dataIndex: "batch_id", width: 200, ellipsis: true },
    { title: "文件", dataIndex: "file_name", ellipsis: true },
    { title: "模板", dataIndex: "mapping_id", width: 110 },
    {
      title: "结果",
      key: "summary",
      width: 200,
      render: (_, b) => (
        <span>
          <Tag color="green">增{b.summary.creates}</Tag>
          <Tag color="blue">改{b.summary.updates}</Tag>
          <Tag color="red">错{b.summary.errors}</Tag>
        </span>
      )
    },
    {
      title: "状态",
      dataIndex: "status",
      width: 100,
      render: (s) => <Tag color={BATCH_TAG[s]?.color}>{BATCH_TAG[s]?.text || s}</Tag>
    },
    {
      title: "时间",
      dataIndex: "created_at",
      width: 160,
      render: (t) => new Date(t).toLocaleString("zh-CN", { hour12: false })
    },
    {
      title: "操作",
      key: "act",
      width: 100,
      render: (_, b) =>
        b.status === "confirmed" ? (
          <Popconfirm title="回滚将删除本批新增并恢复本批更新，确认？" onConfirm={() => onRollback(b.batch_id)}>
            <Button size="small" danger icon={<RollbackOutlined />}>回滚</Button>
          </Popconfirm>
        ) : null
    }
  ];

  return (
    <Space direction="vertical" style={{ width: "100%" }} size="middle">
      <Card size="small" title="第一步 · 上传并预览差异">
        <Space wrap>
          <Select
            style={{ width: 220 }}
            value={mappingID || undefined}
            options={mappings.map((m) => ({ value: m.mapping_id, label: m.name }))}
            onChange={setMappingID}
            placeholder="选择模板"
          />
          <Upload accept=".xlsx,.csv" showUploadList={false} beforeUpload={beforeUpload}>
            <Button icon={<UploadOutlined />}>选择 XLSX/CSV</Button>
          </Upload>
          {fileName && <Typography.Text><FileExcelOutlined /> {fileName}</Typography.Text>}
        </Space>
        <Typography.Paragraph type="secondary" style={{ marginTop: 12, marginBottom: 0 }}>
          按所选模板列名反向解析；SN 匹配库内资产判定新增/更新；tag_id 列只读忽略；错误行不入库。
        </Typography.Paragraph>
      </Card>

      {preview && (
        <Card
          size="small"
          title="第二步 · 确认入库（差异预览）"
          extra={
            <Popconfirm title="确认按此差异入库？" onConfirm={onConfirm}>
              <Button type="primary" icon={<CheckCircleOutlined />} loading={confirming}>
                确认入库
              </Button>
            </Popconfirm>
          }
        >
          <Space size="large" style={{ marginBottom: 16 }}>
            <Statistic title="新增" value={preview.summary.creates} valueStyle={{ color: "#3f8600" }} />
            <Statistic title="更新" value={preview.summary.updates} valueStyle={{ color: "#1677ff" }} />
            <Statistic title="错误行" value={preview.summary.errors} valueStyle={{ color: "#cf1322" }} />
          </Space>
          {preview.summary.errors > 0 && (
            <Alert
              type="warning"
              style={{ marginBottom: 16 }}
              message={`有 ${preview.summary.errors} 行错误将被跳过，其余行照常入库`}
            />
          )}
          {preview.updates.length > 0 && (
            <Table rowKey="row" size="small" title={() => "更新明细"} columns={updateCols} dataSource={preview.updates} pagination={false} />
          )}
          {preview.creates.length > 0 && (
            <Table rowKey="row" size="small" title={() => "新增明细"} columns={createCols} dataSource={preview.creates} pagination={false} style={{ marginTop: 12 }} />
          )}
          {preview.errors.length > 0 && (
            <Table rowKey="row" size="small" title={() => "错误行"} columns={errCols} dataSource={preview.errors} pagination={false} style={{ marginTop: 12 }} />
          )}
        </Card>
      )}

      <Card size="small" title="导入批次记录（支持按批次回滚）">
        <Table
          rowKey="batch_id"
          size="small"
          columns={batchCols}
          dataSource={batches}
          pagination={{ pageSize: 5 }}
          locale={{ emptyText: <Typography.Text type="secondary">暂无导入批次</Typography.Text> }}
        />
      </Card>
    </Space>
  );
}
