import React from "react";
import ReactDOM from "react-dom/client";
import { HashRouter } from "react-router";
import { App as AntdApp, ConfigProvider } from "antd";
import zhCN from "antd/locale/zh_CN";
import App from "./App";
import "./print.css";
import "./grid.css";

// 四层包裹：ConfigProvider(主题/zhCN) → AntdApp(useApp 实例) → HashRouter → App
// antd 6 × React 19 静态 message 静默，全部提示必须经 App.useApp()（build.sh 有防回归闸门）
ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ConfigProvider locale={zhCN} theme={{ token: { colorPrimary: "#0D9488" } }}>
      <AntdApp>
        <HashRouter>
          <App />
        </HashRouter>
      </AntdApp>
    </ConfigProvider>
  </React.StrictMode>
);
