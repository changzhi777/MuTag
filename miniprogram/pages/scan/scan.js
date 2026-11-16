// pages/scan/scan —— 扫码首页：小程序码 scene 直达 + wx.scanCode + 手动输入兜底
const api = require("../../utils/api");
const auth = require("../../utils/auth");

Page({
  data: {
    inputTag: "",
    error: ""
  },

  // 小程序码启动直达：scene=t:{tagId}（URL 编码），兼容 ?tag= 直传
  onLoad(query) {
    const raw = (query && (query.scene || query.tag)) || "";
    if (!raw) return;
    const tagID = api.extractTagID(raw);
    if (tagID) {
      this.goDetail(tagID);
    } else {
      this.setData({ error: "未识别出 MuTag 码：" + raw });
    }
  },

  onInputTag(e) {
    this.setData({ inputTag: e.detail.value, error: "" });
  },

  // 手动输入查询（模拟器/码面污损兜底）
  onManualQuery() {
    const tagID = api.extractTagID(this.data.inputTag);
    if (!tagID) {
      this.setData({ error: "编号格式不正确，示例：MT-2026-000001" });
      return;
    }
    this.goDetail(tagID);
  },

  // 拉起微信扫一扫（兼容普通二维码与小程序码：小程序码结果在 res.path）
  onScan() {
    wx.scanCode({
      onlyFromCamera: false,
      success: (res) => {
        const raw = res.result || res.path || "";
        const tagID = api.extractTagID(raw);
        if (!tagID) {
          this.setData({ error: "未识别出 MuTag 码：" + raw });
          return;
        }
        this.goDetail(tagID);
      },
      fail: (err) => {
        // 用户主动取消不提示；其余失败（相机权限拒绝等）给出反馈
        const msg = (err && err.errMsg) || "";
        if (msg.indexOf("cancel") > -1) return;
        this.setData({ error: "扫码未成功，请重试或手动输入编号" });
      }
    });
  },

  goDetail(tagID) {
    auth.recordHistory(tagID);
    wx.navigateTo({ url: `/pages/detail/detail?tag=${tagID}` });
  },

  goHistory() {
    wx.navigateTo({ url: "/pages/history/history" });
  },

  goMine() {
    wx.navigateTo({ url: "/pages/mine/mine" });
  }
});
