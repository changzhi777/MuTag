// 管理员管理页（0007）：账号列表 / 新增 / 改密码与显示名 / 删除（不能删自己）。
import { useCallback, useEffect, useState } from "react";
import { App, Button, Form, Input, Modal, Space, Table } from "antd";
import { PlusOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { api, del, post, put } from "../api/client";

interface AdminItem {
  id: number;
  username: string;
  display_name: string;
  created_at: string;
}

export default function Admins() {
  const { message } = App.useApp();
  const [items, setItems] = useState<AdminItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<AdminItem | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api<{ items: AdminItem[] }>("/admins");
      setItems(res.items);
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const remove = async (id: number, username: string) => {
    Modal.confirm({
      title: `删除管理员 ${username}？`,
      content: "删除后该账号无法登录（可重新创建）",
      okText: "删除",
      okType: "danger",
      onOk: async () => {
        try {
          await del(`/admins/${id}`);
          message.success("已删除");
          void load();
        } catch (e) {
          message.error((e as Error).message);
        }
      }
    });
  };

  const columns: ColumnsType<AdminItem> = [
    { title: "账号", dataIndex: "username", width: 140, render: (v: string) => <code>{v}</code> },
    { title: "姓名/显示名", dataIndex: "display_name" },
    { title: "创建时间", dataIndex: "created_at", width: 180, render: (v: string) => v?.slice(0, 19).replace("T", " ") || "-" },
    {
      title: "操作", width: 160,
      render: (_, record) => (
        <Space>
          <a onClick={() => setEditing(record)}>改密码/显示名</a>
          <a style={{ color: "#ff4d4f" }} onClick={() => remove(record.id, record.username)}>删除</a>
        </Space>
      )
    }
  ];

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreating(true)}>
          新增管理员
        </Button>
      </Space>
      <Table
        rowKey="id"
        columns={columns}
        dataSource={items}
        loading={loading}
        size="middle"
        pagination={false}
      />

      <Modal
        title="新增管理员"
        open={creating}
        onCancel={() => setCreating(false)}
        destroyOnHidden
        footer={null}
      >
        <AdminForm
          onSubmit={async (draft) => {
            await post("/admins", draft);
            message.success("管理员已创建");
            setCreating(false);
            void load();
          }}
        />
      </Modal>

      <Modal
        title={`改密码/显示名 · ${editing?.username ?? ""}`}
        open={editing !== null}
        onCancel={() => setEditing(null)}
        destroyOnHidden
        footer={null}
      >
        {editing && (
          <AdminForm
            initial={editing}
            submitText="保存"
            onSubmit={async (draft) => {
              await put(`/admins/${editing.id}`, draft);
              message.success("已保存");
              setEditing(null);
              void load();
            }}
          />
        )}
      </Modal>
    </div>
  );
}

function AdminForm(props: {
  initial?: AdminItem;
  submitText?: string;
  onSubmit: (draft: { username: string; password: string; display_name: string }) => Promise<void>;
}) {
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const isEdit = props.initial != null;

  const onFinish = async (values: { username: string; password: string; display_name: string }) => {
    setLoading(true);
    try {
      await props.onSubmit(values);
      form.resetFields();
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Form
      form={form}
      layout="vertical"
      onFinish={onFinish}
      initialValues={props.initial ? { username: props.initial.username, display_name: props.initial.display_name } : undefined}
    >
      <Form.Item name="username" label="账号">
        <Input disabled={isEdit} placeholder="登录账号（如 zc）" />
      </Form.Item>
      <Form.Item
        name="password"
        label={isEdit ? "新密码（留空=不改）" : "密码"}
        rules={isEdit ? [] : [{ required: true, message: "请输入密码" }]}
      >
        <Input.Password placeholder="至少 6 位" />
      </Form.Item>
      <Form.Item name="display_name" label="姓名/显示名">
        <Input placeholder="如 张晨（履历入库显示用）" />
      </Form.Item>
      <Button type="primary" htmlType="submit" block loading={loading}>
        {props.submitText || "创建"}
      </Button>
    </Form>
  );
}
