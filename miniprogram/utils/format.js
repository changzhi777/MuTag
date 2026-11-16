// 公共格式化与文案映射：集中 EVENT_TEXT / STATUS_TEXT 与日期/金额格式化，
// 消除 detail / mine / history 多页重复定义（DRY）。

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
  repair: "报修",
  transfer: "位置变更",
  scrap: "报废",
  update: "信息变更"
};

// ISO 字符串 → "YYYY-MM-DD"（空值返回 ""）
function formatDate(v) {
  return v ? String(v).slice(0, 10) : "";
}

// ISO 字符串 → "YYYY-MM-DD HH:mm"
function formatDateTime(v) {
  return v ? String(v).slice(0, 16).replace("T", " ") : "";
}

// 时间戳 → "MM-DD HH:mm"（非法值返回 ""）
function formatShortTime(ts) {
  const d = new Date(ts);
  if (isNaN(d.getTime())) return "";
  const pad = (n) => String(n).padStart(2, "0");
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// 金额 → "¥1,234.00"：不依赖 Intl，规避部分 Android 运行时 toLocaleString options 失效
function formatMoney(v) {
  if (v === null || v === undefined || v === "") return "";
  const n = Number(v);
  if (!isFinite(n)) return "";
  const parts = Math.abs(n).toFixed(2).split(".");
  parts[0] = parts[0].replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  return (n < 0 ? "-" : "") + "¥" + parts.join(".");
}

module.exports = {
  STATUS_TEXT,
  EVENT_TEXT,
  formatDate,
  formatDateTime,
  formatShortTime,
  formatMoney
};
