import { readFileSync } from "node:fs";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// 构建时注入仓库根 VERSION（发布规程 V1.0.x 产出）→ 前端 __APP_VERSION__ 显示
const APP_VERSION = readFileSync(new URL("../../VERSION", import.meta.url), "utf8").trim();

// dev 代理 /api → 本地 Go 服务；build 产物由 backend/internal/adminui embed
export default defineConfig({
  plugins: [react()],
  define: {
    __APP_VERSION__: JSON.stringify(APP_VERSION)
  },
  server: {
    port: 5180,
    proxy: { "/api": "http://localhost:8080" }
  },
  base: "/admin/",
  build: { outDir: "dist" }
});
