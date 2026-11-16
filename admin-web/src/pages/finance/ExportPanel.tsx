import { useCallback, useEffect, useState } from "react";
import { Button, Card, DatePicker, Input, Select, Space, Table, Tag, Typography, message } from "antd";
import { DownloadOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import type { Dayjs } from "dayjs";
import * as XLSX from "xlsx";
import { api, post } from "../../api/client";
import type { ExportJob, ExportResult, FinMapping } from "../../types";
import { STATUS_TEXT } from "../../types";

const STATUS_OPTIONS = Object.entries(STATUS_TEXT).map(([value, label]) => ({ value, label }));

// 财务导出面板：过滤条件 → 模板映射输出行 → SheetJS 生成 XLSX 下载；后端留审计
export default function ExportPanel({ mappings }: { mappings: FinMapping[] }) {
  const [mappingID, setMappingID] = useState("");
  const [status, setStatus] = useState("");
  const [dept, setDept] = useState("");
  const [keyword, setKeyword] = useState("");
  const [range, setRange] = useState<[Dayjs | null, Dayjs | null] | null>(null);
  const [exporting, setExporting] = useState(false);
  const [jobs, setJobs] = useState<ExportJob[]>([]);

  const reloadJobs = useCallback(async () => {
    try {
      const res = await api<{ items: ExportJob[] }>("/fin/export/jobs");
      setJobs(res.items);
    } catch (e) {
      message.error((e as Error).message);
    }
  }, []);

  useEffect(() => {
    if (mappings.length > 0 && !mappingID) setMappingID(mappings[0].mapping_id);
  }, [mappings, mappingID]);
  useEffect(() => {
    void reloadJobs();
  }, [reloadJobs]);

  const onExport = async () => {
    const mapping = mappings.find((m) => m.mapping_id === mappingID);
    if (!mapping) {
      message.error("请选择模板");
      return;
    }
    setExporting(true);
    try {
      const res = await post<ExportResult>("/fin/export", {
        mapping_id: mappingID,
        status,
        dept,
        keyword,
        buy_from: range?.[0] ? range[0].format("YYYY-MM-DD") : "",
        buy_to: range?.[1] ? range[1].format("YYYY-MM-DD") : ""
      });
      if (res.row_count === 0) {
        message.warning("过滤条件下无资产，未生成文件");
        void reloadJobs();
        return;
      }
      // 拼表：表头附加行 → 列名行 → 数据行
      const header = res.columns.map((c) => c.name);
      const aoa: string[][] = [...(res.fixed_rows || []), header, ...res.rows];
      const ws = XLSX.utils.aoa_to_sheet(aoa);
      const wb = XLSX.utils.book_new();
      XLSX.utils.book_append_sheet(wb, ws, "资产导出");
      const fname = `MuTag导出_${mapping.name}_${new Date().toISOString().slice(0, 10)}.xlsx`;
      XLSX.writeFile(wb, fname);
      message.success(`已导出 ${res.row_count} 行（审计单号 ${res.job_id}）`);
      void reloadJobs();
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setExporting(false);
    }
  };

  const jobCols: ColumnsType<ExportJob> = [
    { title: "审计单号", dataIndex: "job_id", width: 220, ellipsis: true },
    { title: "模板", dataIndex: "mapping_id", width: 110 },
    { title: "行数", dataIndex: "row_count", width: 70 },
    { title: "操作人", dataIndex: "actor", width: 90 },
    { title: "过滤条件", dataIndex: "filters", ellipsis: true, render: (f) => JSON.stringify(f) },
    {
      title: "时间",
      dataIndex: "created_at",
      width: 170,
      render: (t) => new Date(t).toLocaleString("zh-CN", { hour12: false })
    }
  ];

  return (
    <Space direction="vertical" style={{ width: "100%" }} size="middle">
      <Card size="small" title="导出条件（模板映射决定列与格式）">
        <Space wrap>
          <Select
            style={{ width: 220 }}
            value={mappingID || undefined}
            options={mappings.map((m) => ({ value: m.mapping_id, label: m.name }))}
            onChange={setMappingID}
            placeholder="选择模板"
          />
          <Select
            style={{ width: 110 }}
            allowClear
            placeholder="状态"
            value={status || undefined}
            options={STATUS_OPTIONS}
            onChange={(v) => setStatus(v || "")}
          />
          <Input style={{ width: 130 }} placeholder="部门编码" value={dept} onChange={(e) => setDept(e.target.value)} />
          <Input style={{ width: 150 }} placeholder="关键字（名称/码/保管人/SN）" value={keyword} onChange={(e) => setKeyword(e.target.value)} />
          <DatePicker.RangePicker
            value={range}
            onChange={(v) => setRange(v)}
            placeholder={["入账起", "入账止"]}
          />
          <Button type="primary" icon={<DownloadOutlined />} loading={exporting} onClick={onExport}>
            生成 XLSX
          </Button>
        </Space>
      </Card>

      <Card size="small" title="导出审计记录（财务合规留痕）">
        <Table
          rowKey="job_id"
          size="small"
          columns={jobCols}
          dataSource={jobs}
          pagination={{ pageSize: 5 }}
          locale={{ emptyText: <Typography.Text type="secondary">暂无导出记录</Typography.Text> }}
        />
      </Card>
      <Typography.Text type="secondary">
        <Tag>说明</Tag> XLSX 在浏览器端生成下载；每次导出在后端留存操作人与过滤条件审计单。
      </Typography.Text>
    </Space>
  );
}
