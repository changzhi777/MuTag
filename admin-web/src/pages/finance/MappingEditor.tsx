import { useCallback, useEffect, useState } from "react";
import { App, Button, Input, Select, Space, Table, Typography } from "antd";
import { DeleteOutlined, PlusOutlined, SaveOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { put } from "../../api/client";
import type { FinColumn, FinMapping } from "../../types";

// 系统字段白名单（与后端 importableFields + tag_id 一致）
const FIELD_OPTIONS = [
  { value: "tag_id", label: "资产编号 tag_id（只读）" },
  { value: "name", label: "名称 name" },
  { value: "category", label: "类别 category" },
  { value: "model", label: "规格型号 model" },
  { value: "sn", label: "序列号 sn" },
  { value: "unit", label: "单位 unit" },
  { value: "buy_date", label: "购置日期 buy_date" },
  { value: "price", label: "金额 price" },
  { value: "dept_id", label: "部门编码 dept_id" },
  { value: "keeper", label: "保管人 keeper" },
  { value: "location", label: "存放地点 location" },
  { value: "warranty_end", label: "保修到期 warranty_end" },
  { value: "status", label: "状态 status" },
  { value: "remark", label: "备注 remark" }
];

// fixed_rows 文本 ⇄ 二维数组（每行单元格用 | 分隔）
const fixedRowsToText = (rows?: string[][]): string =>
  (rows || []).map((r) => r.join("|")).join("\n");
const textToFixedRows = (text: string): string[][] =>
  text
    .split("\n")
    .filter((line) => line.trim() !== "")
    .map((line) => line.split("|").map((c) => c.trim()));

// 财务模板列映射编辑器：列名 ⇄ 系统字段 + 日期格式 + 表头附加行，保存即 version+1
export default function MappingEditor({ mappings, onReload }: { mappings: FinMapping[]; onReload: () => void }) {
  const { message } = App.useApp();
  const [currentID, setCurrentID] = useState<string>("");
  const [columns, setColumns] = useState<FinColumn[]>([]);
  const [dateFormat, setDateFormat] = useState("YYYY-MM-DD");
  const [fixedText, setFixedText] = useState("");
  const [saving, setSaving] = useState(false);

  const current = mappings.find((m) => m.mapping_id === currentID);

  const load = useCallback((m: FinMapping | undefined) => {
    if (!m) return;
    setColumns(m.def.columns.map((c) => ({ ...c })));
    setDateFormat(m.def.date_format || "YYYY-MM-DD");
    setFixedText(fixedRowsToText(m.def.fixed_rows));
  }, []);

  useEffect(() => {
    if (!currentID && mappings.length > 0) {
      setCurrentID(mappings[0].mapping_id);
      load(mappings[0]);
    }
  }, [mappings, currentID, load]);

  const switchTo = (id: string) => {
    setCurrentID(id);
    load(mappings.find((m) => m.mapping_id === id));
  };

  const save = async () => {
    if (!current) return;
    if (columns.some((c) => !c.name.trim() || !c.field)) {
      message.error("列名与系统字段均必填");
      return;
    }
    setSaving(true);
    try {
      await put(`/fin/mappings/${current.mapping_id}`, {
        def: {
          columns: columns.map((c) => ({ ...c, name: c.name.trim() })),
          date_format: dateFormat,
          status_map: current.def.status_map,
          fixed_rows: textToFixedRows(fixedText)
        }
      });
      message.success(`模板已保存（v${current.version + 1}）`);
      onReload();
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const colTable: ColumnsType<FinColumn> = [
    {
      title: "模板列名",
      dataIndex: "name",
      width: 200,
      render: (_, rec) =>
        rec.readonly ? (
          <Input value={rec.name} disabled />
        ) : (
          <Input value={rec.name} onChange={(e) => setColumns(columns.map((c) => (c === rec ? { ...c, name: e.target.value } : c)))} />
        )
    },
    {
      title: "系统字段",
      dataIndex: "field",
      width: 240,
      render: (_, rec) =>
        rec.readonly ? (
          <Typography.Text type="secondary">{rec.field}（导出只读，导入忽略）</Typography.Text>
        ) : (
          <Select
            style={{ width: "100%" }}
            value={rec.field}
            options={FIELD_OPTIONS}
            onChange={(v) => setColumns(columns.map((c) => (c === rec ? { ...c, field: v } : c)))}
          />
        )
    },
    {
      title: "操作",
      width: 70,
      render: (_, rec) =>
        rec.readonly ? null : (
          <Button
            type="text"
            danger
            icon={<DeleteOutlined />}
            onClick={() => setColumns(columns.filter((c) => c !== rec))}
          />
        )
    }
  ];

  return (
    <Space direction="vertical" style={{ width: "100%" }} size="middle">
      <Space wrap>
        <Select
          style={{ width: 260 }}
          value={currentID || undefined}
          options={mappings.map((m) => ({ value: m.mapping_id, label: `${m.name}（v${m.version}）` }))}
          onChange={switchTo}
        />
        <Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={save}>
          保存映射
        </Button>
        <Typography.Text type="secondary">
          金额/日期/状态格式转换全部由映射层负责；保存后版本号自动递增
        </Typography.Text>
      </Space>

      {current && (
        <>
          <Table
            rowKey={(r) => `${r.field}-${r.name}`}
            size="small"
            columns={colTable}
            dataSource={columns}
            pagination={false}
            footer={() => (
              <Button
                icon={<PlusOutlined />}
                onClick={() => setColumns([...columns, { name: "", field: "name" }])}
              >
                加列
              </Button>
            )}
          />
          <Space wrap size="large">
            <Space>
              <Typography.Text>日期导出格式：</Typography.Text>
              <Select
                style={{ width: 140 }}
                value={dateFormat}
                options={[
                  { value: "YYYY-MM-DD", label: "2026-01-02" },
                  { value: "YYYY/MM/DD", label: "2026/01/02" }
                ]}
                onChange={setDateFormat}
              />
            </Space>
            <Space align="start">
              <Typography.Text>表头附加行（每行一格一线，| 分隔）：</Typography.Text>
              <Input.TextArea
                rows={2}
                style={{ width: 320 }}
                value={fixedText}
                placeholder="单位名称：| （填公司名）"
                onChange={(e) => setFixedText(e.target.value)}
              />
            </Space>
          </Space>
        </>
      )}
    </Space>
  );
}
