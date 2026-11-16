// 小程序登录态与请求封装：wx.login → code → 后端换 token（HMAC，7 天）
const app = getApp();

const TOKEN_KEY = "mutag_token";
const USER_KEY = "mutag_user";
const HISTORY_KEY = "mutag_scan_history";

function getToken() {
  return wx.getStorageSync(TOKEN_KEY) || "";
}

function setSession(token, user) {
  wx.setStorageSync(TOKEN_KEY, token);
  wx.setStorageSync(USER_KEY, user);
}

function getUser() {
  return wx.getStorageSync(USER_KEY) || null;
}

// 登录（幂等：已登录直接返回）
function ensureLogin() {
  return new Promise((resolve, reject) => {
    if (getToken()) {
      resolve(getUser());
      return;
    }
    wx.login({
      success: (res) => {
        wx.request({
          url: `${app.globalData.apiBase}/api/v1/auth/login`,
          method: "POST",
          data: { code: res.code },
          success(r) {
            if (r.statusCode === 200) {
              setSession(r.data.token, r.data.user);
              resolve(r.data.user);
            } else {
              reject(new Error(r.data.error || "登录失败"));
            }
          },
          fail: () => reject(new Error("网络异常"))
        });
      },
      fail: () => reject(new Error("wx.login 失败"))
    });
  });
}

// 带 token 的 GET/POST
function request(method, path, data) {
  return new Promise((resolve, reject) => {
    wx.request({
      url: `${app.globalData.apiBase}${path}`,
      method,
      data,
      header: { Authorization: `Bearer ${getToken()}` },
      success(r) {
        if (r.statusCode === 200) resolve(r.data);
        else reject(new Error((r.data && r.data.error) || `请求失败（${r.statusCode}）`));
      },
      fail: () => reject(new Error("网络异常"))
    });
  });
}

// 扫码历史：离线缓存最近 100 条（tagID + 时间）
function recordHistory(tagID) {
  const list = wx.getStorageSync(HISTORY_KEY) || [];
  const next = [{ tag: tagID, time: Date.now() }, ...list.filter((x) => x.tag !== tagID)];
  wx.setStorageSync(HISTORY_KEY, next.slice(0, 100));
}

function getHistory() {
  return wx.getStorageSync(HISTORY_KEY) || [];
}

module.exports = { ensureLogin, getToken, getUser, request, recordHistory, getHistory };
