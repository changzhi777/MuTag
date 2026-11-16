// pages/mine/mine —— 我的上报：本人发起的操作记录（后端按 openid 过滤）
const auth = require("../../utils/auth");
const fmt = require("../../utils/format");

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
          id: e.event_id,
          typeText: fmt.EVENT_TEXT[e.type] || e.type,
          time: fmt.formatDateTime(e.created_at),
          note: e.note || ""
        }));
        this.setData({ events, loading: false });
      })
      .catch((e) => {
        this.setData({ loading: false, error: e.message });
      });
  }
});
