// pages/mine/mine —— 我的上报：本人发起的操作记录（后端按 openid 过滤）
const auth = require("../../utils/auth");

const EVENT_TEXT = {
  receive: "领用",
  return: "归还",
  repair: "报修",
  transfer: "位置变更",
  create: "入库",
  scrap: "报废",
  update: "信息变更"
};

function fmtTime(v) {
  return v ? v.slice(0, 16).replace("T", " ") : "";
}

Page({
  data: {
    events: [],
    loading: true,
    error: ""
  },

  onShow() {
    this.load();
  },

  load() {
    this.setData({ loading: true, error: "" });
    auth
      .ensureLogin()
      .then(() => auth.request("GET", "/api/v1/auth/my-events"))
      .then((data) => {
        const events = (data.events || []).map((e) => ({
          typeText: EVENT_TEXT[e.type] || e.type,
          time: fmtTime(e.created_at),
          note: e.note || ""
        }));
        this.setData({ events, loading: false });
      })
      .catch((e) => {
        this.setData({ loading: false, error: e.message });
      });
  }
});
