// 后端 API 封装：活码/资产查询
const app = getApp();

// 从扫码结果中提取 tagId：
//   活码 URL   https://mu.qingmu.cn/t/MT-2026-000001
//   scene 格式 t:MT-2026-000001
//   直接输入   MT-2026-000001
function extractTagID(raw) {
  if (!raw) return "";
  raw = String(raw).trim();
  const sceneMatch = raw.match(/t:([A-Za-z0-9-]+)/);
  if (sceneMatch) return sceneMatch[1];
  const pathMatch = raw.match(/\/t\/([A-Za-z0-9-]+)/);
  if (pathMatch) return pathMatch[1];
  if (/^MT-\d{4}-\d{6}$/.test(raw)) return raw;
  return "";
}

// GET /api/v1/assets/:tagID → { asset, events }
function fetchAsset(tagID) {
  return new Promise((resolve, reject) => {
    wx.request({
      url: `${app.globalData.apiBase}/api/v1/assets/${tagID}`,
      method: "GET",
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
