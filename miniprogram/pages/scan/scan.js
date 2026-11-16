// pages/scan/scan —— 扫码首页：wx.scanCode + 手动输入兜底（模拟器无摄像头）
const api = require("../../utils/api");
const auth = require("../../utils/auth");

Page({
  data: {
    inputTag: "",
    error: "",
    loading: false
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

  // 拉起微信扫一扫
  onScan() {
    wx.scanCode({
      onlyFromCamera: false,
      success: (res) => {
        const tagID = api.extractTagID(res.result);
        if (!tagID) {
          this.setData({ error: "未识别出 MuTag 码：" + res.result });
          return;
        }
        this.goDetail(tagID);
      },
      fail: () => {
        // 用户取消扫码，不提示错误
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
