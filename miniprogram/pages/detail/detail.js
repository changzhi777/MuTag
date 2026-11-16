// pages/detail/detail —— 资产详情：基本信息卡 + 履历时间线（只读）
const api = require("../../utils/api");

const STATUS_TEXT = {
  in_use: "在用",
  idle: "闲置",
  repair: "维修中",
  borrowed: "借出",
  scrapped: "已报废"
};

const EVENT_TEXT = {
  create: "入库",
  receive: "领用",
  return: "归还",
  repair: "维修",
  transfer: "调拨",
  scrap: "报废",
  update: "信息变更"
};

function fmtDate(v) {
  if (!v) return "";
  return v.slice(0, 10);
}

// 字段卡渲染模型：空值整行隐藏
function buildFields(asset) {
  return [
    { label: "采购日期", value: fmtDate(asset.buy_date) },
    { label: "单位", value: asset.unit },
    { label: "保管人", value: asset.keeper },
    { label: "型号", value: asset.model },
    { label: "SN", value: asset.sn, mono: true },
    {
      label: "价格",
      value: asset.price
        ? "¥" + Number(asset.price).toLocaleString("zh-CN", { minimumFractionDigits: 2 })
        : ""
    },
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
    const tagID = query.tag || "";
    this.setData({ tagID });
    if (tagID) this.load(tagID);
  },

  load(tagID) {
    this.setData({ loading: true, error: "" });
    api
      .fetchAsset(tagID)
      .then((data) => {
        const events = (data.events || []).map((e) => ({
          typeText: EVENT_TEXT[e.type] || e.type,
          actor: e.actor,
          time: fmtDate(e.created_at),
          note: e.note || "",
          cls: "ev-" + e.type
        }));
        this.setData({
          loading: false,
          asset: data.asset,
          statusText: STATUS_TEXT[data.asset.status] || data.asset.status,
          fields: buildFields(data.asset),
          events
        });
      })
      .catch((err) => {
        this.setData({ loading: false, error: err.message });
      });
  },

  goActions() {
    wx.navigateTo({ url: `/pages/actions/actions?tag=${this.data.tagID}` });
  },

  onRetry() {
    if (this.data.tagID) this.load(this.data.tagID);
  }
});
