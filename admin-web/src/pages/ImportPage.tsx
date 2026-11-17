import { useEffect, useMemo, useState } from "react";
import { App, Alert, Button, Card, Checkbox, Popover, Space, Table, Typography, Upload } from "antd";
import { UploadOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import * as XLSX from "xlsx";
import { post } from "../api/client";
import { excelSerialToDateText } from "../utils/date";
import type { ImportItem } from "../types";

// xlsx 列头 → 标准字段（与 seed 链路同构：序号/名称/采购日期/数量/单位/领用人签字）
// 列头行自动定位：兼容模板首行为「固定资产需打标签」大标题（sheet_to_json 默认取首行当列头会全空）
// 日期解析走序列号 UTC 数学（utils/date）——cellDates 本地时区补偿曾致 04-23 读成 04-22
function parseWorkbook(file: ArrayBuffer): ImportItem[] {
  const wb = XLSX.read(file); // cellDates 默认 false：日期单元格为序列号 number，走 excelSerialToDateText 主路径
  const sheet = wb.Sheets[wb.SheetNames[0]];
  const aoa = XLSX.utils.sheet_to_json<unknown[]>(sheet, { header: 1, defval: "" });
  const headIdx = aoa.findIndex((r) => r.some((c) => String(c).trim() === "名称"));
  if (headIdx === -1) return []; // 找不到「名称」列头 → 视为无效模板
  const head: string[] = (aoa[headIdx] as unknown[]).map((c) => String(c).trim());
  const val = (r: unknown[], name: string): unknown => r[head.indexOf(name)];

  return aoa
    .slice(headIdx + 1)
    .filter((r) => String(val(r, "名称") ?? "").trim() !== "")
    .map((r) => ({
      seq: String(val(r, "序号") ?? "").trim(),
      name: String(val(r, "名称")).trim(),
      buy_date: excelSerialToDateText(val(r, "采购日期")),
      qty: Math.max(parseInt(String(val(r, "数量") ?? "1"), 10) || 1, 1),
      unit: String(val(r, "单位") ?? "").trim() || null,
      keeper: String(val(r, "领用人签字") ?? "").trim() || null
    }));
}

// precheck 响应类型（后端 /import/precheck，规划 P1 任务 2.3）
type PrecheckGroup = {
  key: string; name: string; keeper: string; buy_date: string;
  incoming: number; existing: number; verdict: "exact" | "partial" | "new";
};
type PrecheckSummary = { create: number; skip_exact: number; suspect: number };

export default function ImportPage() {
  const { message, modal } = App.useApp();
  const [items, setItems] = useState<ImportItem[]>([]);
  const [fileName, setFileName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [prechecking, setPrechecking] = useState(false);
  const [summary, setSummary] = useState<PrecheckSummary | null>(null);
  const [dupGroups, setDupGroups] = useState<PrecheckGroup[]>([]);
  const [force, setForce] = useState(false);

  // 解析完成自动预检（只读，失败降级为现状全量导入不阻塞）
  useEffect(() => {
    if (items.length === 0) { setSummary(null); setDupGroups([]); setForce(false); return; }
    setPrechecking(true);
    setSummary(null);
    post<{ groups: PrecheckGroup[]; summary: PrecheckSummary }>("/import/precheck", { items })
      .then((r) => { setSummary(r.summary); setDupGroups(r.groups.filter((g) => g.existing > 0)); })
      .catch(() => setSummary(null))
      .finally(() => setPrechecking(false));
  }, [items]);

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
      const res = await post<{ created: number; skipped: number; last_tag: string }>("/import", {
        items, dedup: !force
      });
      const skippedNote = res.skipped > 0 ? `，跳过重复 ${res.skipped} 条` : "";
      message.success(`导入 ${res.created} 条${skippedNote}，末码 ${res.last_tag}`);
      setItems([]);
      setFileName("");
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  const onForceToggle = (checked: boolean) => {
    if (!checked) { setForce(false); return; }
    modal.confirm({
      title: "确认强制全部导入？",
      content: `将与库内重复的 ${summary?.skip_exact ?? 0} 条也全部入库（生成新码）。去重键：名称+保管人+采购日期。`,
      okText: "强制导入", okButtonProps: { danger: true },
      onOk: () => setForce(true),
      onCancel: () => setForce(false)
    });
  };

  const totalQty = items.reduce((s, it) => s + it.qty, 0);
  // 精确重复（verdict=exact 的 incoming 全部跳过）；partial 键跳过 existing 条、其余照导
  const skipPlan = useMemo(() => {
    if (!summary) return 0;
    return dupGroups.reduce((s, g) => s + Math.min(g.incoming, g.existing), 0);
  }, [summary, dupGroups]);
  const importPlan = totalQty - skipPlan;

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
        message="上传《固定资产需打标签》同结构的 xlsx：序号 / 名称 / 采购日期 / 数量 / 单位 / 领用人签字。数量>1 的行自动拆分为多条资产、各配一码。导入前自动与库内比对去重（名称+保管人+采购日期）。"
      />
      <Space direction="vertical" style={{ width: "100%" }} size="middle">
        <Upload accept=".xlsx,.xls" maxCount={1} beforeUpload={beforeUpload} showUploadList={false}>
          <Button icon={<UploadOutlined />}>选择 xlsx 文件</Button>
        </Upload>
        {fileName && (
          <Alert type="success" showIcon message={`已解析：${fileName}（${items.length} 行 / 拆分后 ${totalQty} 条资产）`} />
        )}
        {items.length > 0 && prechecking && (
          <Alert type="info" showIcon message="正在与库内资产比对重复…" />
        )}
        {items.length > 0 && !prechecking && summary && (
          <Alert
            type={summary.skip_exact > 0 ? "warning" : "success"}
            showIcon
            message={
              summary.skip_exact > 0
                ? `比对完成：将新建 ${summary.create} 条，跳过精确重复 ${summary.skip_exact} 条`
                : "比对完成：库内无精确重复，全部可导入"
            }
            description={
              <Space wrap size="small">
                <span>将新建 <b style={{ color: "#0D9488" }}>{summary.create}</b></span>
                <span>跳过精确重复 <b style={{ color: "#8c8c8c" }}>{summary.skip_exact}</b></span>
                {summary.suspect > 0 && (
                  <Popover content="与库内资产名称+保管人相同但采购日期不同（或库方缺失），已按新资产处理，请人工核对">
                    <span>疑似重复 <b style={{ color: "#d48806" }}>{summary.suspect}</b>（悬停说明）</span>
                  </Popover>
                )}
                <Checkbox checked={force} onChange={(e) => onForceToggle(e.target.checked)}>
                  强制全部导入（不去重）
                </Checkbox>
              </Space>
            }
          />
        )}
        {items.length > 0 && !prechecking && !summary && (
          <Alert type="warning" showIcon message="无法预检（接口异常），将按全量导入执行" />
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
            <Button type="primary" loading={submitting || prechecking} onClick={() => void onSubmit()}>
              {force
                ? `全部导入 ${totalQty} 条（含重复 ${skipPlan} 条）`
                : skipPlan > 0
                  ? `跳过 ${skipPlan} 条重复，导入 ${importPlan} 条`
                  : `确认导入 ${totalQty} 条资产`}
            </Button>
          </>
        )}
      </Space>
    </Card>
  );
}
