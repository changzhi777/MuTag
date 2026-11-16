import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// dev 代理 /api → 本地 Go 服务；build 产物由 backend/internal/adminui embed
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5180,
    proxy: { "/api": "http://localhost:8080" }
  },
  base: "/admin/",
  build: { outDir: "dist" }
});
