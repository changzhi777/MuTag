// 日期归一化 util：Excel 日期解析的唯一入口（修跨日漂移的源头，规划 P1 任务 2.1）。
// 背景：cellDates:true 构造的 Date 受本地时区补偿影响（实测 04-23 被读成 04-22），
// 而 Excel 序列号语义 = 1899-12-30 起算的天数（时区中立）——序列号走 UTC 数学即权威值。

// excelSerialToDateText 任意 xlsx 单元格值 → YYYY-MM-DD 文本（null = 非法/空）。
export function excelSerialToDateText(v: unknown): string | null {
  // 主路径：序列号（XLSX.read 默认 cellDates:false，日期单元格为 number）
  if (typeof v === "number" && isFinite(v) && v > 0) {
    return new Date(Date.UTC(1899, 11, 30) + v * 86400000).toISOString().slice(0, 10);
  }
  // 兜底：Date 对象（cellDates:true 历史——UTC 分量取值）
  if (v instanceof Date && !isNaN(v.getTime())) {
    return v.toISOString().slice(0, 10);
  }
  // 字符串：严格 YYYY-MM-DD
  if (typeof v === "string") {
    const s = v.trim();
    return /^\d{4}-\d{2}-\d{2}$/.test(s) ? s : null;
  }
  return null;
}
