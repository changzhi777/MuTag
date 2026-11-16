import { useCallback, useEffect, useState } from "react";
import {
  Button, Drawer, Form, Input, InputNumber, Modal, Select, Space, Table, Tag, message
} from "antd";
import { PlusOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { api, post, put } from "../api/client";
import { STATUS_COLOR, STATUS_TEXT, type Asset, type AssetDraft, type AssetStatus } from "../types";

const STATUS_OPTIONS = Object.entries(STATUS_TEXT).map(([value, label]) => ({ value, label }));

function fmtDate(v: string | null): string {
  return v ? v.slice(0, 10) : "";
}

export default function Assets() {
  const [items, setItems] = useState<Asset[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<Asset | null>(null);
  const [creating, setCreating] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const qs = new URLSearchParams({
        page: String(page), size: String(size),
        keyword, status: statusFilter
      });
      const res = await api<{ total: number; items: Asset[] }>(`/assets?${qs}`);
      setItems(res.items);
      setTotal(res.total);
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [page, size, keyword, statusFilter]);

  useEffect(() => { void load(); }, [load]);

  const onCreate = async (draft: AssetDraft) => {
    const res = await post<{ tag_id: string }>("/assets", draft);
    message.success(`已创建 ${res.tag_id}`);
    setCreating(false);
    void load();
  };

  const onUpdate = async (tagID: string, draft: AssetDraft) => {
    await put(`/assets/${tagID}`, draft);
    message.success("已保存（变更已写履历）");
    setEditing(null);
    void load();
  };

  const columns: ColumnsType<Asset> = [
    { title: "码号", dataIndex: "tag_id", width: 150, render: (v: string) => <code>{v}</code> },
    { title: "名称", dataIndex: "name", ellipsis: true },
    { title: "保管人", dataIndex: "keeper", width: 100, render: (v) => v || "-" },
    { title: "采购日期", dataIndex: "buy_date", width: 110, render: fmtDate },
    { title: "单位", dataIndex: "unit", width: 70, render: (v) => v || "-" },
    {
      title: "状态", dataIndex: "status", width: 90,
      render: (s: AssetStatus) => <Tag color={STATUS_COLOR[s]}>{STATUS_TEXT[s]}</Tag>
    },
    {
      title: "操作", width: 90,
      render: (_, record) => (
        <a onClick={() => setEditing(record)}>编辑</a>
      )
    }
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16 }} wrap>
        <Input.Search
          placeholder="搜名称/码号/保管人/SN"
          allowClear
          onSearch={(v) => { setPage(1); setKeyword(v); }}
          style={{ width: 260 }}
        />
        <Select
          placeholder="状态筛选"
          allowClear
          options={STATUS_OPTIONS}
          onChange={(v) => { setPage(1); setStatusFilter(v || ""); }}
          style={{ width: 120 }}
        />
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreating(true)}>
          新建资产
        </Button>
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

      <AssetFormModal
        open={creating}
        onCancel={() => setCreating(false)}
        onSubmit={onCreate}
      />

      <Drawer
        title={`编辑 · ${editing?.tag_id ?? ""}`}
        open={editing !== null}
        width={480}
        onClose={() => setEditing(null)}
        destroyOnHidden
      >
        {editing && (
          <AssetForm
            initial={editing}
            onSubmit={(draft) => onUpdate(editing.tag_id, draft)}
          />
        )}
      </Drawer>
    </div>
  );
}

function AssetFormModal(props: {
  open: boolean;
  onCancel: () => void;
  onSubmit: (draft: AssetDraft) => Promise<void>;
}) {
  return (
    <Modal
      title="新建资产（自动配码）"
      open={props.open}
      onCancel={props.onCancel}
      destroyOnHidden
      footer={null}
    >
      <AssetForm onSubmit={props.onSubmit} />
    </Modal>
  );
}

function AssetForm(props: {
  initial?: Asset;
  onSubmit: (draft: AssetDraft) => Promise<void>;
}) {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const onFinish = async (values: AssetDraft & { price_tmp?: number }) => {
    setLoading(true);
    try {
      await props.onSubmit(values as AssetDraft);
      if (!props.initial) form.resetFields();
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const init = props.initial;
  return (
    <Form
      form={form}
      layout="vertical"
      onFinish={onFinish}
      initialValues={init ? {
        name: init.name, category: init.category || undefined,
        model: init.model || undefined, sn: init.sn || undefined,
        buy_date: fmtDate(init.buy_date) || undefined,
        price: init.price ?? undefined, dept_id: init.dept_id || undefined,
        keeper: init.keeper || undefined, location: init.location || undefined,
        status: init.status, unit: init.unit || undefined,
        warranty_end: fmtDate(init.warranty_end) || undefined,
        remark: init.remark || undefined
      } : { status: "in_use" }}
    >
      <Form.Item name="name" label="名称" rules={[{ required: true, message: "名称必填" }]}>
        <Input />
      </Form.Item>
      <Space size="middle" style={{ display: "flex" }}>
        <Form.Item name="keeper" label="保管人" style={{ width: 140 }}>
          <Input />
        </Form.Item>
        <Form.Item name="unit" label="单位" style={{ width: 90 }}>
          <Input />
        </Form.Item>
        <Form.Item name="buy_date" label="采购日期" style={{ width: 170 }}>
          <Input placeholder="YYYY-MM-DD" />
        </Form.Item>
      </Space>
      <Space size="middle" style={{ display: "flex" }}>
        <Form.Item name="model" label="型号" style={{ width: 140 }}>
          <Input />
        </Form.Item>
        <Form.Item name="sn" label="SN" style={{ width: 200 }}>
          <Input />
        </Form.Item>
      </Space>
      <Space size="middle" style={{ display: "flex" }}>
        <Form.Item name="price" label="价格" style={{ width: 140 }}>
          <InputNumber style={{ width: "100%" }} min={0} precision={2} />
        </Form.Item>
        <Form.Item name="status" label="状态" style={{ width: 140 }}>
          <Select options={STATUS_OPTIONS} />
        </Form.Item>
      </Space>
      <Form.Item name="location" label="位置">
        <Input />
      </Form.Item>
      <Form.Item name="remark" label="备注">
        <Input.TextArea rows={2} />
      </Form.Item>
      <Button type="primary" htmlType="submit" block loading={loading}>
        保存
      </Button>
    </Form>
  );
}
