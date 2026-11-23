import { useState } from "react";
import { App, Button, Card, Form, Input } from "antd";
import { useNavigate } from "react-router";
import { post, setToken } from "../api/client";

export default function Login() {
  const { message } = App.useApp();
  const nav = useNavigate();
  const [loading, setLoading] = useState(false);

  const onFinish = async ({ username, password }: { username?: string; password: string }) => {
    setLoading(true);
    try {
      // 双轨：填了账号走多管理员校验，留空走单管理密码（兼容形态）
      const res = await post<{ token: string; open?: boolean; display_name?: string }>("/login", {
        username: username?.trim() || "",
        password
      });
      setToken(res.token);
      if (res.display_name) sessionStorage.setItem("mutag_admin_name", res.display_name);
      if (res.open) sessionStorage.setItem("mutag_auth_open", "1");
      nav("/");
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center", background: "#F5F7FA" }}>
      <Card style={{ width: 360 }}>
        <div style={{ textAlign: "center", fontSize: 18, fontWeight: 600, marginBottom: 24 }}>
          MuTag 管理后台
        </div>
        <Form onFinish={onFinish}>
          <Form.Item name="username">
            <Input placeholder="账号（管理员账号登录用，单密码模式留空）" allowClear />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: "请输入密码" }]}>
            <Input.Password placeholder="密码" onPressEnter={() => undefined} autoFocus />
          </Form.Item>
          <Button type="primary" htmlType="submit" block loading={loading}>
            登录
          </Button>
        </Form>
        <div style={{ textAlign: "center", color: "#9ca3af", fontSize: 12, marginTop: 12 }}>
          管理员：填账号 + 密码登录 · 单密码模式：仅填密码
        </div>
      </Card>
    </div>
  );
}
