// pages/detail/detail —— 资产详情：基本信息卡 + 履历时间线（只读）
const api = require("../../utils/api");
const fmt = require("../../utils/format");

// 字段卡渲染模型：空值整行隐藏
function buildFields(asset) {
  return [
    { label: "采购日期", value: fmt.formatDate(asset.buy_date) },
    { label: "单位", value: asset.unit },
    { label: "保管人", value: asset.keeper },
    { label: "型号", value: asset.model },
    { label: "SN", value: asset.sn, mono: true },
    { label: "价格", value: fmt.formatMoney(asset.price) },
    { label: "位置", value: asset.location },
    { label: "备注", value: asset.remark }
  ].filter((f) => f.value);
}

Page({
  data: {
    tagID: "",
    asset: null,
    statusText: "",
    fields: [],
    events: [],
    error: "",
    loading: true
  },

  onLoad(query) {
    const tagID = (query && query.tag) || "";
    this.setData({ tagID });
    // 边界：无码号时终止加载态并给出提示
    if (!tagID) this.setData({ loading: false, error: "缺少资产编号" });
  },

  // onShow 触发（首次进入 + 从快捷操作返回都会刷新），保证状态/履历实时
  onShow() {
    if (this.data.tagID) this.load(this.data.tagID);
  },

  load(tagID) {
    // 已有数据时静默刷新：不整屏闪现"加载中"
    this.setData({ loading: !this.data.asset, error: "" });
    api
      .fetchAsset(tagID)
      .then((data) => {
        const asset = data.asset || {};
        const events = (data.events || []).map((e) => ({
          id: e.event_id,
          typeText: fmt.EVENT_TEXT[e.type] || e.type,
          actor: e.actor,
          time: fmt.formatDate(e.created_at),
          note: e.note || "",
          cls: "ev-" + e.type
        }));
        this.setData({
          loading: false,
          asset,
          statusText: fmt.STATUS_TEXT[asset.status] || asset.status || "",
          fields: buildFields(asset),
          events
        });
      })
      .catch((err) => {
        if (this.data.asset) {
          // 静默刷新失败：保留已有数据，仅轻提示
          wx.showToast({ title: err.message, icon: "none" });
          this.setData({ loading: false });
        } else {
          this.setData({ loading: false, error: err.message });
        }
      });
  },

  goActions() {
    wx.navigateTo({ url: `/pages/actions/actions?tag=${this.data.tagID}` });
  },

  onRetry() {
    if (this.data.tagID) this.load(this.data.tagID);
  }
});
