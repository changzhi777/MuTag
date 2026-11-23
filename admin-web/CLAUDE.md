[根目录](../CLAUDE.md) > **admin-web**

# admin-web · MuTag 管理后台 — 模块文档

> 更新：2026-11-18（init-architect V1.16 首建）｜ React 19 + AntD 6 + Vite + TS
> 根上下文：[../CLAUDE.md](../CLAUDE.md) · 唯一事实来源：[../MuTag-项目设计方案.md](../MuTag-项目设计方案.md)

---

## 变更记录 (Changelog)

| 日期 | 版本 | 变更内容 |
|------|------|----------|
| 2026-11-16 | — | V1.2 M3 提前落地（登录/台账 CRUD/码管理/批量导入/统计，React+AntD 选型钉死，产物 embed 进 Go 二进制）→ V1.3 码版面打印 → V1.4 财务对接三 Tab（模板/导出审计/导入两段式）→ V1.7 表格编辑页（react-data-grid 选型深挖） |
| 2026-11-17 | — | V1.11 打印工场重构（qr-code-styling 全量美化 + 4 标签规格 + 批量 zip）→ V1.12 BASE_URL 收敛（服务端 config 下发）→ V1.15 **P0** 静态 message 根治（43 处换 App.useApp() + build.sh 防回归闸门）+ **P1** ImportPage 预检三态 UI + date.ts 序列号 UTC 数学 + **P2** CleanupPage 三步流 |
| 2026-11-18 | V1.0 | 🆕 本模块 CLAUDE.md 首建（init-architect V1.16）：页面/路由/依赖/闸门全景入档；qr-logo（PrintPage 默认 logoOn+/logo.png）登记为待办未实施 |

---

## 模块职责

资产管理员的 Web 工作台：台账 CRUD、码管理（作废/扫码统计）、码版面打印工场（批量出码图）、批量导入（xlsx + 去重预检）、表格编辑（全量网格行内改）、财务对接（四模板导入导出/回滚）、重复清洗三步流、统计。构建产物写入 `backend/internal/adminui/dist` 由 Go embed——**没有独立前端部署**，随 backend 单二进制走。

## 入口与构建

| 入口 | 说明 |
|------|------|
| `src/main.tsx` | 应用入口：**四层包裹 AntdApp**（P0 根治——静态 message 在 antd 6 × React 19 下全静默） |
| `src/App.tsx` | 路由 + Sider 布局 + RequireAuth（token 空且非 open 形态 → /login） |
| `build.sh` | 一键：**静态 message 防回归闸门** → pnpm build → 产物覆盖 `../backend/internal/adminui/dist` → go build 编译后端 |
| `vite.config.ts` | Vite 6 构建 |

```bash
bash build.sh                # 开发后必跑（闸门 + embed + 后端编译）
pnpm dev                     # 本地联调（Vite dev server）
```

## 页面与路由（src/App.tsx）

| 路由 | 文件 | 职责 |
|---|---|---|
| /login | `pages/Login.tsx` | 管理端登录（ADMIN_PASSWORD；open 形态直接放行） |
| / | `pages/Assets.tsx` | 资产台账 CRUD + AssetFilter（dept/入账区间/状态）+ **ScanCamera 组件**（`pages/ScanCamera.tsx`，摄像头扫码定位资产） |
| /codes | `pages/Codes.tsx` | 码管理：列表/作废/扫码统计列/单枚 hover 下载 PNG·SVG |
| /print | `pages/PrintPage.tsx` | **码版面打印工场**：4 标签规格（30×30/40×30/50×50/60×40 自动行列+切刀线）+ 字段勾选 + 码美化全量（点阵 5 形状/渐变/角块/容错 L-TH/logo 嵌入/透明背景，localStorage 持久化）+ 台账多选批量（A4 页数自动 + PNG zip 导出，jszip）+ 活码内容用服务端 BASE_URL（fallback origin） |
| /import | `pages/ImportPage.tsx` | 批量导入：SheetJS 解析 xlsx → **precheck 三态 UI（新建/跳过/疑似 Popover，P1）** → 确认入库（dedup 默认 true，force 逃生口） |
| /grid | `pages/GridEdit.tsx` | react-data-grid 全量网格行内编辑（改动列高亮、资产码只读——纪律 1）→ `_tag_id` 直匹配 preview → 确认入库 |
| /finance | `pages/Finance.tsx` | 三 Tab：`finance/ImportPanel.tsx`（两段式预览/确认/回滚）· `finance/ExportPanel.tsx`（模板映射导出）· `finance/MappingEditor.tsx`（列映射配置，version 递增） |
| /cleanup | `pages/CleanupPage.tsx` | 重复清洗三步流（Steps/Segmented 策略/行级色标/danger confirm/Result 快照恢复，P2） |
| /stats | `pages/Stats.tsx` | 统计（echarts 类图表 + 分类/部门/状态分布） |

支撑文件：`src/api/client.ts`（fetch 封装 + token 管理）· `src/types.ts` · `src/utils/date.ts`（**xlsx 序列号 UTC 数学**——cellDates 本地时区补偿会差一天，序列号 46135=2026-04-23 为权威）· `src/print.css` / `src/grid.css`

## 关键依赖（package.json）

| 依赖 | 版本 | 用途 |
|---|---|---|
| react / react-dom | ^19 | （antd 6 × React 19 是 P0 事故根源，见 FAQ） |
| antd / @ant-design/icons | ^6 | UI 组件库 |
| react-router | ^7 | 路由 |
| react-data-grid | 7.0.0-beta.64 | 表格编辑（Comcast MIT，选型排除 Univer/Handsontable 等） |
| xlsx (SheetJS) | ^0.18.5 | xlsx 解析/生成（导入前端解析、导出前端生成——后端零新依赖裁决） |
| qr-code-styling | ^1.9.2 | 码美化（V1.11 选型） |
| qrcode | ^1.5.4 | 基础码图生成 |
| jszip | ^3.10.2 | 批量 PNG 打包下载 |
| dayjs | 1.11.23 | 日期 |

## 数据模型（前端视角）

消费后端 `/api/admin/*` JSON；核心实体 = Asset（tag_id 外键关联）、Tag（active/void）、FinMapping（column_map JSON + fixed_rows + version）、ImportBatch（pending→confirm/rollback）、CleanupJob（scan/execute/restore 三步）。详见 [../CLAUDE.md](../CLAUDE.md) 数据模型速览与 [../backend/CLAUDE.md](../backend/CLAUDE.md)。

## 测试与质量

- **无前端单测**：逻辑回归由 backend 进程内 e2e 承接（TestE2EGridEdit / TestE2EImportDedupe / TestE2ECleanupChain 等覆盖页面背后的 API 链路）
- **build.sh 静态 message 闸门是硬门**：双形态检测（单行 import 子串匹配 / 多行独立 message 行），用 `/usr/bin/grep` 绝对路径（本机 grep 可能是 ugrep，`\b` 语义与 exit code 不同），`|| true` 防 set -e 误杀；检出即 exit 1
- 66 条重复入库事故（2026-11-17）根因即静态 message 静默 + 无防重点击——P0 闸门 + P1 dedup 后已根治

## 常见问题 (FAQ)

1. **提示不弹？** 组件体内 `const { message } = App.useApp();`——静态 `import { message } from "antd"` 在 antd 6 × React 19 静默不渲染；build.sh 会拦截。
2. **xlsx 日期差一天？** 勿用 `cellDates: true` 的本地时区换算，用 `src/utils/date.ts` 的序列号 UTC 数学。
3. **构建产物去哪了？** `backend/internal/adminui/dist`（Go embed）——本目录 dist 仅中间产物；`tsconfig.tsbuildinfo` 已 gitignore。
4. **扫码笔/摄像头？** Assets 页内嵌 ScanCamera 组件（非独立路由）。
5. **待办（已批准未实施）**：PrintPage 默认 `logoOn: true, logoDataUrl: "/logo.png"`（新用户默认出品牌码，可关）；logo 原图 256×256 RGBA 已定位（`/Volumes/user/新素材/微信图片_20260903153544_17_1821.png`）。规划见 `../.zcf/plan/current/qr-logo.md`。**勿当作已完成。**

## 相关文件清单

- `build.sh`（先读——闸门语义）· `package.json` · `vite.config.ts` · `tsconfig.json`
- `src/main.tsx` · `src/App.tsx`（路由全景）· `src/api/client.ts`
- `src/pages/`（10 文件：Login/Assets/ScanCamera/Codes/PrintPage/ImportPage/GridEdit/Finance/CleanupPage/Stats + finance/ 三面板）
- `src/utils/date.ts`（时区数学）· `src/print.css` · `src/grid.css`
