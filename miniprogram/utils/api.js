// 后端 API 封装：活码/资产查询。
// 说明：getApp() 必须在函数内调用——模块顶层求值可能早于 App 实例化。

// 从扫码结果中提取 tagId：
//   活码 URL     https://mu.qingmu.cn/t/MT-2026-000001
//   scene 格式   t:MT-2026-000001（小程序码 scene，可能被 URL 编码为 t%3AMT-...）
//   小程序码 path pages/scan/scan?scene=t%3AMT-2026-000001（wx.scanCode 结果）
//   直接输入     MT-2026-000001
function extractTagID(raw) {
  if (!raw) return "";
  let s = String(raw).trim();
  try {
    s = decodeURIComponent(s);
  } catch (e) {
    // 非法百分号转义：保留原文继续匹配
  }
  const sceneMatch = s.match(/t:([A-Za-z0-9-]+)/);
  if (sceneMatch) return sceneMatch[1];
  const pathMatch = s.match(/\/t\/([A-Za-z0-9-]+)/);
  if (pathMatch) return pathMatch[1];
  if (/^MT-\d{4}-\d{6}$/.test(s)) return s;
  return "";
}

function apiBase() {
  const app = getApp();
  return (app && app.globalData && app.globalData.apiBase) || "";
}

// GET /api/v1/assets/:tagID → { asset, events }
function fetchAsset(tagID) {
  return new Promise((resolve, reject) => {
    wx.request({
      url: `${apiBase()}/api/v1/assets/${tagID}`,
      method: "GET",
      timeout: 10000,
      success(res) {
        if (res.statusCode === 200) {
          resolve(res.data);
        } else {
          reject(new Error(res.statusCode === 404 ? "码不存在或已作废" : "服务暂不可用"));
        }
      },
      fail() {
        reject(new Error("网络异常，请检查后端服务"));
      }
    });
  });
}

module.exports = { extractTagID, fetchAsset };
