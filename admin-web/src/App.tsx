import { Navigate, Outlet, Route, Routes, useLocation, useNavigate } from "react-router";
import { Layout, Menu } from "antd";
import { HomeOutlined, LogoutOutlined, MoneyCollectOutlined, PrinterOutlined, QrcodeOutlined, StarOutlined, UploadOutlined } from "@ant-design/icons";
import { useEffect } from "react";
import { clearToken, getToken } from "./api/client";
import Login from "./pages/Login";
import Assets from "./pages/Assets";
import Codes from "./pages/Codes";
import Finance from "./pages/Finance";
import ImportPage from "./pages/ImportPage";
import PrintPage from "./pages/PrintPage";
import Stats from "./pages/Stats";

const { Sider, Content, Header } = Layout;

function Shell() {
  const nav = useNavigate();
  const loc = useLocation();
  const selected = loc.pathname === "/" ? ["assets"] : [loc.pathname.slice(1)];

  return (
    <Layout style={{ minHeight: "100vh" }}>
      <Sider theme="dark" width={200}>
        <div style={{ color: "#fff", padding: "20px 24px", fontWeight: 600, fontSize: 16 }}>
          MuTag 管理后台
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={selected}
          onClick={({ key }) => nav(key === "assets" ? "/" : `/${key}`)}
          items={[
            { key: "assets", icon: <HomeOutlined />, label: "资产台账" },
            { key: "codes", icon: <QrcodeOutlined />, label: "码管理" },
            { key: "print", icon: <PrinterOutlined />, label: "码版面打印" },
            { key: "import", icon: <UploadOutlined />, label: "批量导入" },
            { key: "finance", icon: <MoneyCollectOutlined />, label: "财务对接" },
            { key: "stats", icon: <StarOutlined />, label: "统计" }
          ]}
        />
      </Sider>
      <Layout>
        <Header style={{ background: "#fff", display: "flex", justifyContent: "flex-end", alignItems: "center", paddingInline: 24 }}>
          <a
            onClick={() => {
              clearToken();
              nav("/login");
            }}
          >
            <LogoutOutlined /> 退出
          </a>
        </Header>
        <Content style={{ padding: 24, background: "#F5F7FA" }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}

function RequireAuth() {
  // 后端未启用鉴权（open 形态）时 token 为空也放行；已启用则必须有 token
  const open = sessionStorage.getItem("mutag_auth_open") === "1";
  if (!getToken() && !open) return <Navigate to="/login" replace />;
  return <Shell />;
}

export default function App() {
  useEffect(() => {
    document.title = "MuTag 管理后台";
  }, []);
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<RequireAuth />}>
        <Route path="/" element={<Assets />} />
        <Route path="/codes" element={<Codes />} />
        <Route path="/print" element={<PrintPage />} />
        <Route path="/import" element={<ImportPage />} />
        <Route path="/finance" element={<Finance />} />
        <Route path="/stats" element={<Stats />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
