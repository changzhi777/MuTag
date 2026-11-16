// pages/actions/actions —— 快捷操作：领用/归还/报修/位置变更（≤3 字段，提交即写履历）
const auth = require("../../utils/auth");

const ACTIONS = [
  { key: "receive", label: "领用", hint: "资产将记到你名下" },
  { key: "return", label: "归还", hint: "解除你与资产的保管关系" },
  { key: "repair", label: "报修", hint: "资产状态将转为维修中" },
  { key: "transfer", label: "位置变更", hint: "更新资产存放位置", needLocation: true }
];

Page({
  data: {
    tagID: "",
    actions: ACTIONS,
    current: "receive",
    location: "",
    note: "",
    submitting: false,
    done: false
  },

  onLoad(query) {
    this.setData({ tagID: query.tag || "" });
  },

  onPick(e) {
    this.setData({ current: e.currentTarget.dataset.key });
  },

  onLocation(e) {
    this.setData({ location: e.detail.value });
  },

  onNote(e) {
    this.setData({ note: e.detail.value });
  },

  onSubmit() {
    if (this.data.submitting) return; // 并发守卫：防连点写重复履历
    const { tagID, current, location, note } = this.data;
    if (!tagID) {
      message("缺码号");
      return;
    }
    if (current === "transfer" && !location) {
      message("请填写新位置");
      return;
    }
    this.setData({ submitting: true });
    auth
      .ensureLogin()
      .then(() =>
        auth.request("POST", `/api/v1/assets/${tagID}/actions`, {
          action: current,
          location,
          note
        })
      )
      .then(() => {
        this.setData({ done: true });
      })
      .catch((e) => message(e.message))
      .finally(() => this.setData({ submitting: false }));
  },

  backToDetail() {
    if (getCurrentPages().length > 1) {
      wx.navigateBack();
    } else {
      // 无页面栈（直接进入本页）时兜底跳转详情
      wx.redirectTo({ url: `/pages/detail/detail?tag=${this.data.tagID}` });
    }
  }
});

function message(text) {
  wx.showToast({ title: text, icon: "none" });
}
