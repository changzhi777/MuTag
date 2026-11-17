import { useMemo, useState } from "react";
import {
  App, Alert, Button, Card, Input, Popconfirm, Result, Segmented,
  Space, Spin, Statistic, Steps, Table, Tag, Typography
} from "antd";
import { ClearOutlined, ReloadOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { api, post } from "../api/client";

// 重复清理三步流：扫描 → 预览 → 执行（快照兜底可回滚）。
// 规划：.claude/plan/db-import-clean-dedupe.md 任务 3.4（简化：all 模式 + earliest_1/latest_1，manual/align 留迭代）。

type Member = {
  tag_id: string; name: string; keeper: string; buy_date: string;
  created_at: string; status: string; events: number; scans: number;
  keep: boolean; action: "" | "delete" | "void";
};
type Group = {
  key: string; name: string; keeper: string; buy_date: string;
  members: Member[]; clusters: number; expected: number; remove_count: number;
};
type ScanResult = {
  groups: Group[];
  summary: { delete: number; void: number; keep: number };
};

export default function CleanupPage() {
  const { message, modal } = App.useApp();
  const [step, setStep] = useState(0);
  const [scanning, setScanning] = useState(false);
  const [strategy, setStrategy] = useState<string>("earliest_1");
  const [scan, setScan] = useState<ScanResult | null>(null);
  const [groups, setGroups] = useState<Group[]>([]);
  const [executing, setExecuting] = useState(false);
  const [confirmInput, setConfirmInput] = useState("");
  const [done, setDone] = useState<{ job_id: string; deleted: number; voided: number } | null>(null);

  const doScan = async (keepStrategy: string) => {
    setScanning(true);
    setDone(null);
    try {
      const r = await api<ScanResult>("/cleanup/scan", {
        method: "POST", body: JSON.stringify({ mode: "all", keep_strategy: keepStrategy })
      });
      setScan(r.summary.delete + r.summary.void > 0 ? r : { groups: [], summary: { delete: 0, void: 0, keep: 0 } });
      setGroups(r.groups);
      setStrategy(keepStrategy);
      setStep(1);
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setScanning(false);
    }
  };

  // 策略切换：重扫（服务端重算 keep/action——数据量小，代价可忽略）
  const onStrategyChange = (v: string | number) => {
    if (v === strategy) return;
    void doScan(String(v));
  };

  const plan = useMemoPlan(groups);
  const totalPlan = plan.delete.length + plan.void.length;

  const onExecute = () => {
    modal.confirm({
      title: "确认执行清理？",
      content: (
        <Space direction="vertical">
          <span>物理删除 <b style={{ color: "#cf1322" }}>{plan.delete.length}</b> 条 · 作废 <b style={{ color: "#d48806" }}>{plan.void.length}</b> 条（码置 void，号簿保留）</span>
          <span>执行前自动创建快照（可一键恢复）</span>
          <Input
            placeholder={`输入 ${plan.delete.length} 确认物理删除条数`}
            onChange={(e) => setConfirmInput(e.target.value)}
          />
        </Space>
      ),
      okText: "执行清理", okButtonProps: { danger: true, disabled: confirmInput !== String(plan.delete.length) },
      onOk: () => void doExecute()
    });
  };

  const doExecute = async () => {
    setExecuting(true);
    try {
      const r = await post<{ job_id: string; deleted: number; voided: number }>("/cleanup/execute", {
        mode: "all", keep_strategy: strategy,
        delete_tag_ids: plan.delete, void_tag_ids: plan.void,
        confirm_count: totalPlan
      });
      setDone(r);
      setStep(2);
      message.success(`清理完成：删除 ${r.deleted} · 作废 ${r.voided}`);
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setExecuting(false);
    }
  };

  const onRestore = async (jobID: string) => {
    setExecuting(true);
    try {
      const r = await post<{ restored: number }>(`/cleanup/jobs/${jobID}/restore`, {});
      message.success(`已从快照恢复 ${r.restored} 条资产`);
      setScan(null);
      setGroups([]);
      setDone(null);
      setStep(0);
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setExecuting(false);
    }
  };

  const memberCols: ColumnsType<Member> = [
    { title: "码号", dataIndex: "tag_id", width: 150 },
    { title: "保管人", dataIndex: "keeper", width: 90, render: (v) => v || "-" },
    { title: "采购日期", dataIndex: "buy_date", width: 110, render: (v) => v || "-" },
    { title: "创建时间", dataIndex: "created_at", width: 170, render: (v) => (v || "-").slice(0, 19) },
    { title: "履历", dataIndex: "events", width: 70 },
    { title: "扫码", dataIndex: "scans", width: 70 },
    {
      title: "处置", width: 110, render: (_, m) =>
        m.keep ? <Tag color="success">保留</Tag>
          : m.action === "delete" ? <Tag color="error">物理删除</Tag>
            : <Tag color="warning">作废</Tag>
    }
  ];

  const groupCols: ColumnsType<Group> = [
    { title: "键（名称|保管人|日期）", dataIndex: "key", ellipsis: true },
    { title: "成员数", width: 80, render: (_, g) => g.members.length },
    { title: "时间簇", width: 80, render: (_, g) => g.clusters === 1 ? <Tag>1（疑似合法多件）</Tag> : g.clusters },
    { title: "将清理", width: 90, render: (_, g) => g.remove_count > 0 ? <Typography.Text type="danger">{g.remove_count}</Typography.Text> : "-" }
  ];

  return (
    <Card>
      <Typography.Title level={5}>重复清理（存量去重）</Typography.Title>
      <Steps
        size="small"
        current={step}
        items={[{ title: "扫描" }, { title: "预览" }, { title: "执行" }]}
        style={{ marginBottom: 16 }}
      />

      {step < 2 && (
        <Space direction="vertical" style={{ width: "100%" }} size="middle">
          <Space wrap>
            <Button
              type="primary" icon={<ClearOutlined />} loading={scanning}
              onClick={() => void doScan(strategy)}
            >
              {scan ? "重新扫描" : "扫描库内重复"}
            </Button>
            {scan && (
              <Space wrap align="center">
                保留策略：
                <Segmented
                  value={strategy}
                  onChange={(v) => onStrategyChange(v as string)}
                  options={[
                    { value: "earliest_1", label: "每组保留最早 1 条" },
                    { value: "latest_1", label: "每组保留最新 1 条" }
                  ]}
                />
              </Space>
            )}
          </Space>

          {scanning && <Spin style={{ display: "block", margin: "24px auto" }} />}

          {scan && step === 1 && (
            <>
              {scan.summary.delete + scan.summary.void === 0 ? (
                <Alert type="success" showIcon message="扫描完成：库内无重复资产" />
              ) : (
                <>
                  <Alert
                    type="warning" showIcon
                    message={`发现 ${groups.length} 组重复：物理删除 ${scan.summary.delete} · 作废 ${scan.summary.void} · 保留 ${scan.summary.keep}`}
                    description="无履历且无扫码的重复预标「物理删除」（快照兜底可恢复）；有业务履历或扫码的预标「作废」（资产改报废保留，码作废号簿保留）。"
                  />
                  <Table
                    rowKey="key"
                    columns={groupCols}
                    dataSource={groups}
                    size="small"
                    pagination={{ pageSize: 10, showTotal: (t) => `共 ${t} 组` }}
                    expandable={{
                      expandedRowRender: (g) => (
                        <Table
                          rowKey="tag_id"
                          columns={memberCols}
                          dataSource={g.members}
                          size="small"
                          pagination={false}
                          rowClassName={(m) => (!m.keep && m.action === "delete" ? "cleanup-row-del" : !m.keep ? "cleanup-row-void" : "")}
                        />
                      )
                    }}
                  />
                  <Space style={{ position: "sticky", bottom: 8, width: "100%", justifyContent: "space-between", background: "#fff", padding: 8, border: "1px solid #f0f0f0", borderRadius: 8 }}>
                    <Space>
                      <Statistic title="物理删除" value={plan.delete.length} valueStyle={{ color: "#cf1322" }} />
                      <Statistic title="作废" value={plan.void.length} valueStyle={{ color: "#d48806" }} />
                      <Statistic title="保留" value={scan.summary.keep} valueStyle={{ color: "#0D9488" }} />
                    </Space>
                    <Button danger type="primary" icon={<ClearOutlined />} disabled={totalPlan === 0} loading={executing} onClick={onExecute}>
                      执行清理（删 {plan.delete.length} · 废 {plan.void.length}）
                    </Button>
                  </Space>
                </>
              )}
            </>
          )}
        </Space>
      )}

      {step === 2 && done && (
        <Result
          status="success"
          title="清理完成"
          subTitle={`job ${done.job_id} · 物理删除 ${done.deleted} · 作废 ${done.voided}（快照已存，可恢复）`}
          extra={[
            <Button key="rescan" type="primary" icon={<ReloadOutlined />} onClick={() => { setDone(null); setScan(null); setStep(0); }}>
              再扫一轮
            </Button>,
            <Popconfirm
              key="restore"
              title="从快照恢复本次清理删除的资产？"
              onConfirm={() => void onRestore(done.job_id)}
            >
              <Button>从快照恢复</Button>
            </Popconfirm>
          ]}
        />
      )}

      <style>{`
        .cleanup-row-del { background: #fff1f0; }
        .cleanup-row-void { background: #fffbe6; }
      `}</style>
    </Card>
  );
}

// useMemoPlan 依赖 groups 的 keep/action 汇总出执行清单
function useMemoPlan(groups: Group[]): { delete: string[]; void: string[] } {
  return useMemo(() => {
    const del: string[] = [];
    const void_: string[] = [];
    for (const g of groups) {
      for (const m of g.members) {
        if (m.keep) continue;
        if (m.action === "delete") del.push(m.tag_id);
        else if (m.action === "void") void_.push(m.tag_id);
      }
    }
    return { delete: del, void: void_ };
  }, [groups]);
}
