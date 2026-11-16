import { useCallback, useEffect, useState } from "react";
import { Popconfirm, Select, Space, Table, Tag, message } from "antd";
import type { ColumnsType } from "antd/es/table";
import { api, post } from "../api/client";
import type { Tag as TagEntity } from "../types";

function fmtTime(v: string): string {
  return v ? v.slice(0, 19).replace("T", " ") : "";
}

export default function Codes() {
  const [items, setItems] = useState<TagEntity[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [statusFilter, setStatusFilter] = useState<string>("");
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const qs = new URLSearchParams({
        page: String(page), size: String(size), status: statusFilter
      });
      const res = await api<{ total: number; items: TagEntity[] }>(`/tags?${qs}`);
      setItems(res.items);
      setTotal(res.total);
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [page, size, statusFilter]);

  useEffect(() => { void load(); }, [load]);

  const onVoid = async (tagID: string) => {
    try {
      await post(`/tags/${tagID}/void`, {});
      message.success(`${tagID} 已作废`);
      void load();
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const columns: ColumnsType<TagEntity> = [
    { title: "码号", dataIndex: "tag_id", width: 150, render: (v: string) => <code>{v}</code> },
    { title: "scene", dataIndex: "qr_scene", width: 180, render: (v) => v || "-" },
    { title: "活码路径", dataIndex: "url_code", width: 160 },
    {
      title: "状态", dataIndex: "status", width: 90,
      render: (s: string) =>
        s === "active" ? <Tag color="green">active</Tag> : <Tag>void</Tag>
    },
    { title: "创建时间", dataIndex: "created_at", width: 180, render: fmtTime },
    {
      title: "操作", width: 90,
      render: (_, record) =>
        record.status === "active" ? (
          <Popconfirm
            title={`作废 ${record.tag_id}？`}
            description="码作废后资产同步报废，码号永不复用。"
            onConfirm={() => void onVoid(record.tag_id)}
          >
            <a style={{ color: "#cf1322" }}>作废</a>
          </Popconfirm>
        ) : null
    }
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Select
          placeholder="状态筛选"
          allowClear
          style={{ width: 120 }}
          options={[
            { value: "active", label: "active" },
            { value: "void", label: "void" }
          ]}
          onChange={(v) => { setPage(1); setStatusFilter(v || ""); }}
        />
      </Space>
      <Table
        rowKey="tag_id"
        columns={columns}
        dataSource={items}
        loading={loading}
        size="middle"
        pagination={{
          current: page, pageSize: size, total, showSizeChanger: true,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (p, s) => { setPage(p); setSize(s); }
        }}
      />
    </div>
  );
}
