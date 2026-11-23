import { useCallback, useEffect, useState } from "react";
import { App, Button, Cascader, Drawer, Form, Input, InputNumber, Modal, Select, Space, Table, Tag, Tree } from "antd";
import { PlusOutlined, ScanOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { api, post, put, del } from "../api/client";
import { STATUS_COLOR, STATUS_TEXT, type Asset, type AssetDraft, type AssetStatus, type Location } from "../types";
import ScanCamera from "./ScanCamera";

const STATUS_OPTIONS = Object.entries(STATUS_TEXT).map(([value, label]) => ({ value, label }));

function fmtDate(v: string | null): string {
  return v ? v.slice(0, 10) : "";
}

export default function Assets() {
  const { message } = App.useApp();
  const [items, setItems] = useState<Asset[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [scanOpen, setScanOpen] = useState(false);
  const [editing, setEditing] = useState<Asset | null>(null);
  const [creating, setCreating] = useState(false);
  const [locations, setLocations] = useState<Location[]>([]);
  const [locMgrOpen, setLocMgrOpen] = useState(false);

  const loadLocations = useCallback(async () => {
    try {
      const res = await api<{ items: Location[] }>("/locations");
      setLocations(res.items);
    } catch { /* 位置加载失败不阻断台账 */ }
  }, []);

  useEffect(() => { void loadLocations(); }, [loadLocations]);

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
        <Button icon={<ScanOutlined style={{ fontSize: 15 }} />} onClick={() => setScanOpen(true)}>扫码识别</Button>
        <Button onClick={() => setLocMgrOpen(true)}>位置管理</Button>
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
        locations={locations}
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
            locations={locations}
            onSubmit={(draft) => onUpdate(editing.tag_id, draft)}
          />
        )}
      </Drawer>

      <LocationManager
        open={locMgrOpen}
        locations={locations}
        onChanged={loadLocations}
        onClose={() => setLocMgrOpen(false)}
      />

      <ScanCamera
        open={scanOpen}
        onClose={() => setScanOpen(false)}
        onDetected={(tagID) => {
          setKeyword(tagID);
          setPage(1);
          void load();
        }}
      />
    </div>
  );
}

function AssetFormModal(props: {
  open: boolean;
  onCancel: () => void;
  onSubmit: (draft: AssetDraft) => Promise<void>;
  locations: Location[];
}) {
  return (
    <Modal
      title="新建资产（自动配码）"
      open={props.open}
      onCancel={props.onCancel}
      destroyOnHidden
      footer={null}
    >
      <AssetForm onSubmit={props.onSubmit} locations={props.locations} />
    </Modal>
  );
}

function AssetForm(props: {
  initial?: Asset;
  locations: Location[];
  onSubmit: (draft: AssetDraft) => Promise<void>;
}) {
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  // 位置级联选项：一级区域 → 二级点位（location_id 取叶子/区域 id）
  const locOptions = props.locations
    .filter((l) => l.parent_id === null)
    .map((zone) => ({
      value: zone.id,
      label: zone.name,
      children: props.locations
        .filter((s) => s.parent_id === zone.id)
        .map((s) => ({ value: s.id, label: s.name }))
    }));

  const onFinish = async (values: AssetDraft & { price_tmp?: number; locPath?: number[] }) => {
    setLoading(true);
    try {
      const { locPath, ...draft } = values as AssetDraft & { locPath?: number[] };
      const payload: AssetDraft = {
        ...draft,
        location_id: locPath?.length ? locPath[locPath.length - 1] : null
      };
      await props.onSubmit(payload);
      if (!props.initial) form.resetFields();
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const init = props.initial;
  // 编辑态反推位置级联路径（location_id → [区域id, 点位id]）
  let initLocPath: number[] | undefined;
  if (init?.location_id != null) {
    const leaf = props.locations.find((l) => l.id === init.location_id);
    if (leaf?.parent_id != null) initLocPath = [leaf.parent_id, leaf.id];
    else initLocPath = [init.location_id];
  }
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
        remark: init.remark || undefined,
        parent_tag_id: init.parent_tag_id || undefined,
        qty: init.qty ?? 1,
        brand: init.brand || undefined,
        keeper_phone: init.keeper_phone || undefined,
        warranty_until: fmtDate(init.warranty_until) || undefined,
        locPath: initLocPath
      } : { status: "in_use", qty: 1 }}
    >
      <Form.Item name="name" label="名称" rules={[{ required: true, message: "名称必填" }]}>
        <Input />
      </Form.Item>
      <Space size="middle" style={{ display: "flex" }}>
        <Form.Item name="keeper" label="保管人" style={{ width: 140 }}>
          <Input />
        </Form.Item>
        <Form.Item name="keeper_phone" label="保管人电话" style={{ width: 160 }}>
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
        <Form.Item name="brand" label="品牌" style={{ width: 140 }}>
          <Input />
        </Form.Item>
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
        <Form.Item name="qty" label="数量" style={{ width: 100 }}>
          <InputNumber style={{ width: "100%" }} min={1} precision={0} />
        </Form.Item>
        <Form.Item name="status" label="状态" style={{ width: 140 }}>
          <Select options={STATUS_OPTIONS} />
        </Form.Item>
      </Space>
      <Space size="middle" style={{ display: "flex" }}>
        <Form.Item name="locPath" label="安装位置（区域 → 点位）">
          <Cascader options={locOptions} placeholder="选择位置" changeOnSelect />
        </Form.Item>
        <Form.Item name="location" label="位置（旧文本）">
          <Input placeholder="历史自由文本" />
        </Form.Item>
      </Space>
      <Space size="middle" style={{ display: "flex" }}>
        <Form.Item name="parent_tag_id" label="所属设备码号（部件挂载）" style={{ width: 240 }}>
          <Input placeholder="如 MT-2026-000026，空=独立资产" />
        </Form.Item>
        <Form.Item name="warranty_until" label="维保到期" style={{ width: 170 }}>
          <Input placeholder="YYYY-MM-DD" />
        </Form.Item>
      </Space>
      <Form.Item name="remark" label="备注">
        <Input.TextArea rows={2} />
      </Form.Item>
      <Button type="primary" htmlType="submit" block loading={loading}>
        保存
      </Button>
    </Form>
  );
}

// LocationManager 位置两级树管理：添加一级区域 / 区域下添加二级点位 / 删除（有引用时后端拒绝并提示）。
function LocationManager(props: {
  open: boolean;
  locations: Location[];
  onChanged: () => void;
  onClose: () => void;
}) {
  const { message } = App.useApp();
  const [newZone, setNewZone] = useState("");
  const [newSpot, setNewSpot] = useState<Record<number, string>>({});

  const addZone = async () => {
    if (!newZone.trim()) return;
    try {
      await post("/locations", { name: newZone.trim() });
      setNewZone("");
      props.onChanged();
      message.success("区域已添加");
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const addSpot = async (parentId: number) => {
    const name = (newSpot[parentId] || "").trim();
    if (!name) return;
    try {
      await post("/locations", { name, parent_id: parentId });
      setNewSpot({ ...newSpot, [parentId]: "" });
      props.onChanged();
      message.success("点位已添加");
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const remove = async (id: number) => {
    try {
      await del(`/locations/${id}`);
      props.onChanged();
      message.success("已删除");
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const zones = props.locations.filter((l) => l.parent_id === null);
  return (
    <Modal
      title="安装位置管理（区域 → 点位）"
      open={props.open}
      onCancel={props.onClose}
      footer={null}
      width={520}
    >
      <Space.Compact style={{ width: "100%", marginBottom: 16 }}>
        <Input
          placeholder="新增一级区域，如：研发机房"
          value={newZone}
          onChange={(e) => setNewZone(e.target.value)}
          onPressEnter={addZone}
        />
        <Button type="primary" icon={<PlusOutlined />} onClick={addZone}>添加区域</Button>
      </Space.Compact>
      <Tree
        blockNode
        treeData={zones.map((z) => ({
          key: `z${z.id}`,
          title: (
            <span>
              {z.name}
              <a style={{ marginLeft: 8, color: "#ff4d4f" }} onClick={() => remove(z.id)}>删</a>
            </span>
          ),
          children: [
            ...props.locations
              .filter((s) => s.parent_id === z.id)
              .map((s) => ({
                key: `s${s.id}`,
                title: (
                  <span>
                    {s.name}
                    <a style={{ marginLeft: 8, color: "#ff4d4f" }} onClick={() => remove(s.id)}>删</a>
                  </span>
                )
              })),
            {
              key: `n${z.id}`,
              title: (
                <Space.Compact style={{ width: 260 }}>
                  <Input
                    size="small"
                    placeholder="新增二级点位"
                    value={newSpot[z.id] || ""}
                    onChange={(e) => setNewSpot({ ...newSpot, [z.id]: e.target.value })}
                    onPressEnter={() => addSpot(z.id)}
                  />
                  <Button size="small" icon={<PlusOutlined />} onClick={() => addSpot(z.id)} />
                </Space.Compact>
              )
            }
          ]
        }))}
        defaultExpandAll
      />
    </Modal>
  );
}
