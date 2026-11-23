// MuTag 扫码 App 全局常量。
// BASE_URL 唯一网络出口：活码分流 /t、H5 详情 /h5/t 均挂此域名（生产直连，已 TLS）。
export const BASE_URL = 'https://mutag.qingmulife.cn';

// 手动输入编号格式：MT-YYYY-NNNNNN（年份不限定，随 tagId 递增演进）。
export const TAG_ID_RE = /^MT-\d{4}-\d{6}$/;

// 归一化手动输入：trim + 大写 + 全角横线转半角。
export function normalizeTagInput(raw: string): string {
  return raw.trim().replace(/－/g, '-').toUpperCase();
}
