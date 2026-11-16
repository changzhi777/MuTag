import { useCallback, useEffect, useState } from "react";
import { Alert, Button, Card, InputNumber, Radio, Select, Space, Spin, message } from "antd";
import { PrinterOutlined } from "@ant-design/icons";
import QRCode from "qrcode";
import { api } from "../api/client";
import { STATUS_TEXT, type Asset } from "../types";

// 码版面打印：活码 URL 二维码 + 码号 + 资产名，A4 网格排版，浏览器原生打印。
// 码内容 = {base}/t/{urlCode}（活码语义：打印后目标可换，码图永不重印）。

const STATUS_OPTIONS = [
  { value: "", label: "全部" },
  ...Object.entries(STATUS_TEXT).map(([value, label]) => ({ value, label }))
];

export default function PrintPage() {
  const [status, setStatus] = useState<string>("");
  const [cols, setCols] = useState(4);
  const [base, setBase] = useState<string>(() => window.location.origin);
  const [qrs, setQrs] = useState<{ asset: Asset; dataUrl: string }[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    document.title = "码版面 · MuTag 管理后台";
  }, []);

  const generate = useCallback(async () => {
    setLoading(true);
    try {
      // 分页拉全匹配资产
      const all: Asset[] = [];
      for (let page = 1; ; page++) {
        const qs = new URLSearchParams({
          page: String(page), size: "100",
          keyword: "", status
        });
        const res = await api<{ total: number; items: Asset[] }>(`/assets?${qs}`);
        all.push(...res.items);
        if (all.length >= res.total) break;
      }

      const cells: { asset: Asset; dataUrl: string }[] = [];
      for (const a of all) {
        const dataUrl = await QRCode.toDataURL(`${base}/t/${a.tag_id}`, {
          margin: 1, width: 240, errorCorrectionLevel: "M"
        });
        cells.push({ asset: a, dataUrl });
      }
      setQrs(cells);
      message.success(`已生成 ${cells.length} 个码`);
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [status, base]);

  return (
    <div>
      <Card style={{ marginBottom: 16 }} className="print-hide">
        <Space wrap size="middle">
          <span>范围：</span>
          <Select
            style={{ width: 120 }}
            value={status}
            options={STATUS_OPTIONS}
            onChange={setStatus}
          />
          <span>每行列数：</span>
          <InputNumber min={2} max={6} value={cols} onChange={(v) => setCols(v || 4)} />
          <span>码域名：</span>
          <Radio.Group value={base === window.location.origin ? "local" : "prod"} buttonStyle="solid">
            <Radio.Button
              value="local"
              onClick={() => setBase(window.location.origin)}
            >
              本机
            </Radio.Button>
            <Radio.Button value="prod" onClick={() => setBase("https://mu.qingmu.cn")}>
              mu.qingmu.cn
            </Radio.Button>
          </Radio.Group>
          <Button type="primary" loading={loading} onClick={() => void generate()}>
            生成版面
          </Button>
          {qrs.length > 0 && (
            <Button
              type="default"
              icon={<PrinterOutlined />}
              onClick={() => window.print()}
            >
              打印
            </Button>
          )}
        </Space>
        <Alert
          style={{ marginTop: 12 }}
          type="info"
          showIcon
          message={`码内容 = ${base}/t/{urlCode}（活码：打印后目标可换，码图永不重印）。打印选 A4 纵向、边距默认、关闭页眉页脚。`}
        />
      </Card>

      {loading && <Spin style={{ display: "block", margin: "40px auto" }} />}

      <div className="print-sheet" style={{ display: "grid", gridTemplateColumns: `repeat(${cols}, 1fr)`, gap: 8 }}>
        {qrs.map(({ asset, dataUrl }) => (
          <div
            key={asset.tag_id}
            style={{
              border: "1px solid #D0D5DD",
              borderRadius: 8,
              padding: 8,
              textAlign: "center",
              breakInside: "avoid"
            }}
          >
            <img src={dataUrl} alt={asset.tag_id} style={{ width: "100%", maxWidth: 160 }} />
            <div style={{ fontFamily: "ui-monospace, Menlo, monospace", fontWeight: 600, fontSize: 12, letterSpacing: 1 }}>
              {asset.tag_id}
            </div>
            <div style={{ fontSize: 11, color: "#344054", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {asset.name}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
