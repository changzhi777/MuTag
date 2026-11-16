#!/usr/bin/env python3
"""一次性数据转换：固定资产 xlsx → seed JSON（供 Go seed 脚本入库）。

用法: python3 scripts/xlsx_to_json.py <xlsx路径> [输出路径]
输出: [{seq, name, buy_date, qty, unit, keeper}, ...]（buy_date 为 YYYY-MM-DD 或 null）
"""
import json
import sys
from datetime import datetime

import openpyxl


def fmt_date(v):
    if isinstance(v, datetime):
        return v.strftime("%Y-%m-%d")
    if v is None:
        return None
    s = str(v).strip()
    return s if s else None


def main():
    if len(sys.argv) < 2:
        print(__doc__, file=sys.stderr)
        sys.exit(1)
    xlsx_path = sys.argv[1]
    out_path = sys.argv[2] if len(sys.argv) > 2 else "seed/data.json"

    wb = openpyxl.load_workbook(xlsx_path, data_only=True)
    ws = wb["Sheet1"]

    records = []
    for row in ws.iter_rows(min_row=3, values_only=True):
        if row[0] is None:
            continue
        qty = int(row[3]) if row[3] else 1
        records.append({
            "seq": str(row[0]),
            "name": str(row[1]).strip(),
            "buy_date": fmt_date(row[2]),
            "qty": max(qty, 1),
            "unit": str(row[4]).strip() if row[4] else None,
            "keeper": str(row[5]).strip() if len(row) > 5 and row[5] else None,
        })

    with open(out_path, "w", encoding="utf-8") as f:
        json.dump(records, f, ensure_ascii=False, indent=1)
    print(f"转换完成：{len(records)} 行 → {out_path}")


if __name__ == "__main__":
    main()
