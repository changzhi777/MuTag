// pages/history/history —— 扫码历史：本地缓存最近 100 条，点击直达详情
const auth = require("../../utils/auth");

function fmtTime(ts) {
  const d = new Date(ts);
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getMonth() + 1}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

Page({
  data: {
    items: []
  },

  onShow() {
    const items = auth.getHistory().map((x) => ({
      tag: x.tag,
      timeText: fmtTime(x.time)
    }));
    this.setData({ items });
  },

  onTap(e) {
    const tag = e.currentTarget.dataset.tag;
    wx.navigateTo({ url: `/pages/detail/detail?tag=${tag}` });
  }
});
