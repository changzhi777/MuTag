// pages/history/history —— 扫码历史：本地缓存最近 100 条，点击直达详情
const auth = require("../../utils/auth");
const fmt = require("../../utils/format");

Page({
  data: {
    items: []
  },

  onShow() {
    const items = auth.getHistory().map((x) => ({
      tag: x.tag,
      timeText: fmt.formatShortTime(x.time)
    }));
    this.setData({ items });
  },

  onTap(e) {
    const tag = e.currentTarget.dataset.tag;
    wx.navigateTo({ url: `/pages/detail/detail?tag=${tag}` });
  }
});
