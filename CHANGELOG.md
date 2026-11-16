## V1.0.4 (2026-11-17 02:10 CST)

- 🔳 QR 贴码印刷套件（对标草料 B 档）：标签模板 4 规格 + 字段勾选 + 批量多选 + A4 切刀线打印 + PNG zip 导出
- 🎨 码美化全量（qr-code-styling MIT）：点阵形状/颜色渐变/角块/容错/logo 嵌入/透明背景，样式持久化
- 📈 扫码统计：/tags/scan-stats 聚合 API（+e2e）+ 码管理页扫码次数/最近扫码列
- ☁️ Supabase 生产实证补完：应用/备份切直连（pooler 3F000 免疫）+ pg_dump 17 客户端 + 每日备份 cron（14 份轮换）
- 🔐 管理后台登录修复（ADMIN_PASSWORD 重设）+ 生产首次更新流程实战（git pull → build → restart）
- 锚定 8d7f982；验证：e2e 全绿（SQLite+Supabase 双形态）+ 生产 HTTPS 全链路 + CI 双 job

## V1.0.3 (2026-11-17 00:52 CST)

- 📊 台账表格编辑：react-data-grid（MIT 深挖选型）行内编辑 → 差异预览 → 确认入库（_tag_id 直匹配通道）
- ☁️ Supabase 托管 PG17 上线形态就绪并实证：全套 e2e（8 组含财务/网格/限流）在真 PG 全绿
- 🛠️ PG 适配三修：占位符 rebind 驱动包装（?→$n）、simple protocol（免疫 pooler 42P05）、RETURNING 替代 LastInsertId
- 🛡️ 扫码限流（IP 令牌桶，纪律 4）+ cmd/migrate 独立迁移器 + GitHub Actions CI 双 job 首跑绿
- 锚定 b3085ea；验证：SQLite 回归全绿 + Supabase e2e 全绿 + verify.sh 15 连检

## V1.0.2 (2026-11-16 23:25 CST)

- 发版：见 git log a432893..HEAD 与 CLAUDE.md changelog

# MuTag 版本变更记录

> 规程：V1.0.x 第 3 位递增（`sh scripts/next_version.sh apply patch` 一条命令全自动）

## V1.0.1 (2026-11-16 23:30 CST)

- 🏷️ 首个发布版本，锚定 `e7954df`：M1-M4 全量落地
- 活码双链路（302 分流 + IP 令牌桶限流 + H5 SSR）
- 管理后台（React 19 + AntD 6）：台账/码管理/打印版面/批量导入/财务对接/统计
- 小程序员工端（扫码/详情/快捷操作四动作/上报/历史）
- 财务对接：四内置模板 + 导出审计 + 导入两段式（预览/确认/按批次回滚）
- 验证：e2e 五包全绿 + verify.sh 15/15 PASS
