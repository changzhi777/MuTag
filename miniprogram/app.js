// MuTag 员工端 · 应用入口
// apiBase 依小程序运行版本切换：release 走生产 HTTPS 域名，其余走本地联调。
// ⚠️ 上线前置：需在微信公众平台「开发设置 → 服务器域名」把生产域名加入 request 合法域名。
function resolveApiBase() {
  let envVersion = "develop";
  try {
    const info = wx.getAccountInfoSync();
    envVersion = (info && info.miniProgram && info.miniProgram.envVersion) || "develop";
  } catch (e) {
    // 低版本基础库无 getAccountInfoSync：按开发环境处理
  }
  if (envVersion === "release" || envVersion === "trial") return "https://mutag.qingmulife.com";
  return "http://localhost:8080";
}

App({
  globalData: {
    apiBase: ""
  },
  onLaunch() {
    this.globalData.apiBase = resolveApiBase();
  }
});
