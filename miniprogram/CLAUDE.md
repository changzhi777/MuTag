[根目录](../CLAUDE.md) > **miniprogram**

# miniprogram · MuTag 员工端小程序 — 模块文档

> 更新：2026-11-18（init-architect V1.16 首建）｜ 原生微信小程序（测试号）
> 根上下文：[../CLAUDE.md](../CLAUDE.md) · 唯一事实来源：[../MuTag-项目设计方案.md](../MuTag-项目设计方案.md)

---

## 变更记录 (Changelog)

| 日期 | 版本 | 变更内容 |
|------|------|----------|
| 2026-11-16 | — | V1.1 M1 提前交付（scan/detail 两页）→ V1.3 M2 主体落地：users 登录（dev mock 模式）+ 快捷操作四动作 + 我的上报 + 扫码历史（本地缓存 100 条） |
| 2026-11-18 | V1.0 | 🆕 本模块 CLAUDE.md 首建（init-architect V1.16）：五页面/工具层/apiBase 解析全景入档；真机联调仍为 M2 余项 |

---

## 模块职责

员工侧的唯一入口：微信扫码（小程序码 scene / 活码 URL 双形态）看资产详情全生命周期，快捷操作四动作（领用/归还/报修/位置变更，提交即写 asset_events 履历——纪律 2/10），我的上报记录与扫码历史。纯原生小程序，无第三方框架/依赖。

## 入口与页面（app.json 注册 5 页）

| 页面 | 职责 |
|---|---|
| `pages/scan/` | 首页：`wx.scanCode` 扫码 + 手动输入编号（真机体验版主路径）；`utils/api.js` extractTagID 兼容四种输入（活码 URL `/t/{id}` / scene `t:{id}`（含 URL 编码）/ 小程序码 path / 裸编号 `MT-YYYY-NNNNNN`） |
| `pages/detail/` | 资产详情：字段 + asset_events 完整时间线（履历快照渲染） |
| `pages/actions/` | 快捷操作四动作表单（≤3 字段纪律）：领用/归还/报修/位置变更 → POST `/api/v1/assets/:tagID/actions` |
| `pages/mine/` | 我的上报（本人 event 列表，GET `/api/v1/auth/my-events`）+ 登录态 |
| `pages/history/` | 扫码历史（本地缓存 100 条，wx.storage） |

入口文件：`app.js`（globalData.apiBase 解析）· `app.json`（window 深色导航栏 #1F2937，lazyCodeLoading）· `project.config.json`（测试号配置）

## 对外接口（消费 backend）

| 调用 | 端点 | 鉴权 |
|---|---|---|
| 登录 | POST `/api/v1/auth/login`（code → token） | 公开；**WECHAT_APPID 未配 = dev mock openid**，配凭证即切真 code2session |
| 资产详情 | GET `/api/v1/assets/:tagID` → `{asset, events}` | 公开 |
| 快捷操作 | POST `/api/v1/assets/:tagID/actions` | 员工 HMAC token（`utils/auth.js`） |

## 关键机制

- **apiBase 解析**（`app.js` resolveApiBase）：`wx.getAccountInfoSync().miniProgram.envVersion`——`release`/`trial` → `https://mutag.qingmulife.com`，其余 → `http://localhost:8080`；低版本基础库无该 API 按开发环境兜底。⚠️ 上线前置：微信公众平台「开发设置 → 服务器域名」需加生产域名
- **`utils/api.js`**：fetchAsset / 提交动作封装；getApp() 一律函数内调用（模块顶层求值早于 App 实例化的坑）
- **`utils/auth.js`**：token 存取与请求头注入；**`utils/format.js`**：日期/状态中文格式化

## 数据模型（消费端视角）

只读消费 Asset + Events；写侧仅 actions（后端强制写 asset_events，actor = 登录用户）。无本地数据库——仅扫码历史 storage 缓存。

## 测试与质量

- **无小程序侧自动化测试**：员工链路回归由 backend e2e 承接（TestE2EEmployeeChain：dev 登录 → 四动作 → 我的上报全绿）
- 真机联调 = M2 余项 📋：体验版真机受「业务域名」校验限制（生产域名未配置合法域名时手动输入编号可用，扫码跳转受限——V1.13 实测已知）；微信内活码被拦（未备案），**可立即用路径 = 手机系统浏览器扫活码**

## 常见问题 (FAQ)

1. **扫码后 detail 空白？** 先看 extractTagID 是否匹配（scene 可能 URL 编码 `t%3A`）；后端 void 码返回「已作废」态。
2. **登录 401？** dev 模式依赖后端 WECHAT_APPID 为空；若后端配了凭证则必须真 environment。
3. **trial 版打不开接口？** apiBase 已切生产域名，检查微信公众平台 request 合法域名白名单。
4. **改了页面不生效？** lazyCodeLoading=requiredComponents 下需重新编译；project.config.json 为测试号 appid。

## 相关文件清单

- `app.js` · `app.json` · `app.wxss` · `project.config.json` · `sitemap.json`
- `utils/api.js`（extractTagID 核心函数）· `utils/auth.js` · `utils/format.js`
- `pages/scan/` · `pages/detail/` · `pages/actions/` · `pages/mine/` · `pages/history/`（各含 js/wxml/wxss/json）
