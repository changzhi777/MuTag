import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { App, Alert, Button, Card, Checkbox, Col, Radio, Row, Select, Space, Spin, Statistic, Tabs, Typography } from "antd";
import { DownloadOutlined, PrinterOutlined, ReloadOutlined } from "@ant-design/icons";
import QRCodeStyling from "qr-code-styling";
import JSZip from "jszip";
import { api } from "../api/client";
import { STATUS_TEXT, type Asset } from "../types";

// 码版面打印工场：批量活码生成 + 标签模板排版 + 美化全量 + PNG zip 导出
// 活码语义：码内容 = {origin}/t/{urlCode}，打印后目标可换，码图永不重印

const STATUS_OPTIONS = [
  { value: "", label: "全部" },
  ...Object.entries(STATUS_TEXT).map(([value, label]) => ({ value, label }))
];

// 标签模板规格：A4 210×297mm，页边距 10mm；每枚含 QR + 文本区
const TEMPLATES = [
  { key: "30x30", label: "30×30mm（6列×8行）", tw: 30, th: 30, cols: 6, rows: 8, qrmm: 22 },
  { key: "40x30", label: "40×30mm（4列×8行）", tw: 40, th: 30, cols: 4, rows: 8, qrmm: 24 },
  { key: "50x50", label: "50×50mm（3列×5行）", tw: 50, th: 50, cols: 3, rows: 5, qrmm: 38 },
  { key: "60x40", label: "60×40mm（3列×6行）", tw: 60, th: 40, cols: 3, rows: 6, qrmm: 30 }
];

// 码样式默认（持久化 localStorage）
type QrStyle = {
  dotsType: "square" | "dots" | "rounded" | "classy" | "extra-rounded";
  colorMode: "solid" | "gradient";
  color: string;
  color2: string;
  cornerSquareType: "square" | "dot" | "extra-rounded";
  cornerDotType: "square" | "dot";
  ecl: "L" | "M" | "Q" | "H";
  logoOn: boolean;
  logoDataUrl: string; // 空串 = 无 logo
  bgTransparent: boolean;
};

const DEFAULT_STYLE: QrStyle = {
  dotsType: "dots",
  colorMode: "solid",
  color: "#0D9488",
  color2: "#134e4a",
  cornerSquareType: "extra-rounded",
  cornerDotType: "dot",
  ecl: "H",
  logoOn: true,
  logoDataUrl: "/logo.png", // 青沐品牌 logo（public/logo.png），用户可上传覆盖或关闭
  bgTransparent: true
};

const loadStyle = (): QrStyle => {
  try {
    const raw = localStorage.getItem("mutag_qr_style");
    return raw ? { ...DEFAULT_STYLE, ...JSON.parse(raw) } : DEFAULT_STYLE;
  } catch {
    return DEFAULT_STYLE;
  }
};

const loadCfg = (): { tpl: string; fields: string[] } => {
  try {
    const raw = localStorage.getItem("mutag_print_cfg");
    return raw ? JSON.parse(raw) : { tpl: "40x30", fields: ["name", "tag_id", "keeper"] };
  } catch {
    return { tpl: "40x30", fields: ["name", "tag_id", "keeper"] };
  }
};

const FIELD_OPTIONS = [
  { value: "name", label: "资产名称" },
  { value: "tag_id", label: "码号" },
  { value: "keeper", label: "保管人" },
  { value: "location", label: "位置" },
  { value: "sn", label: "SN" }
];

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const styleToOptions = (s: QrStyle, data: string, px: number): any => ({
  width: px,
  height: px,
  type: "svg" as const,
  data,
  image: s.logoOn && s.logoDataUrl ? s.logoDataUrl : undefined,
  qrOptions: { errorCorrectionLevel: s.ecl },
  dotsOptions: {
    color: s.color,
    type: s.dotsType
  },
  cornersSquareOptions: { type: s.cornerSquareType, color: s.color },
  cornersDotOptions: { type: s.cornerDotType, color: s.color },
  backgroundOptions: { color: s.bgTransparent ? "transparent" : "#ffffff" },
  ...(s.colorMode === "gradient"
    ? {
        dotsOptions: {
          type: s.dotsType,
          gradient: { type: "linear" as const, rotation: 0, colorStops: [{ offset: 0, color: s.color }, { offset: 1, color: s.color2 }] }
        }
      }
    : {})
});

export default function PrintPage() {
  const { message } = App.useApp();
  const [assets, setAssets] = useState<Asset[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);
  const [tplKey, setTplKey] = useState(loadCfg().tpl);
  const [fields, setFields] = useState<string[]>(loadCfg().fields);
  const [style, setStyle] = useState<QrStyle>(loadStyle);
  const [ziping, setZiping] = useState(false);
  const [liveBase, setLiveBase] = useState(window.location.origin); // 服务端 BASE_URL 下发（fallback origin）
  const [statusFilter, setStatusFilter] = useState(""); // 客户端状态筛选（假控件修复：onChange 真正生效）
  const gridRef = useRef<HTMLDivElement>(null);
  const vpRef = useRef<HTMLDivElement>(null); // 预览视口（缩放包裹层）
  const [fit, setFit] = useState(1); // A4 预览缩放比（容器宽/794px，封顶 1）
  const naturalH = useRef(1123); // sheet 未缩放总高（px，实测刷新）

  const tpl = TEMPLATES.find((t) => t.key === tplKey) || TEMPLATES[1];

  useEffect(() => {
    document.title = "码版面打印 · MuTag 管理后台";
  }, []);
  useEffect(() => {
    // 活码对外域名从服务端取（BASE_URL 单一事实源，不随后台入口域名漂移）
    api<{ base_url: string }>("/config")
      .then((r) => { if (r.base_url) setLiveBase(r.base_url.replace(/\/$/, "")); })
      .catch(() => {});
  }, []);
  useEffect(() => localStorage.setItem("mutag_qr_style", JSON.stringify(style)), [style]);
  useEffect(() => localStorage.setItem("mutag_print_cfg", JSON.stringify({ tpl: tplKey, fields })), [tplKey, fields]);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const all: Asset[] = [];
      for (let page = 1; ; page++) {
        const res = await api<{ total: number; items: Asset[] }>(`/assets?page=${page}&size=100`);
        all.push(...res.items);
        if (all.length >= res.total) break;
      }
      setAssets(all);
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  const chosen = useMemo(() => assets.filter((a) => selected.has(a.tag_id)), [assets, selected]);
  const shown = useMemo(
    () => (statusFilter ? assets.filter((a) => a.status === statusFilter) : assets),
    [assets, statusFilter]
  );
  const pages = useMemo(() => {
    const per = tpl.cols * tpl.rows;
    const out: Asset[][] = [];
    for (let i = 0; i < chosen.length; i += per) out.push(chosen.slice(i, i + per));
    return out.length ? out : [[]];
  }, [chosen, tpl]);

  // 预览缩放：A4 物理宽 210mm≈794px，右列放不下 → transform scale 等比适配（封顶 1 不放大）
  useEffect(() => {
    const vp = vpRef.current;
    if (!vp) return;
    const measure = () => {
      const sheet = gridRef.current;
      if (sheet) naturalH.current = sheet.offsetHeight; // offsetHeight 不受 transform 影响
      setFit(Math.min(1, vp.clientWidth / 794));
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(vp);
    if (gridRef.current) ro.observe(gridRef.current);
    return () => ro.disconnect();
  }, [chosen.length, tplKey]);

  // 单枚标签容器：挂载 qr-code-styling SVG（样式/资产变化即重渲）
  const downloadOne = async (asset: Asset, fmt: "png" | "svg") => {
    try {
      const holder = document.createElement("div");
      holder.style.cssText = "position:fixed;left:-9999px;top:-9999px;";
      document.body.appendChild(holder);
      const qr = new QRCodeStyling({
        ...styleToOptions(style, `${liveBase}/t/${asset.tag_id}`, 800),
        width: 800,
        height: 800,
        type: fmt === "png" ? ("canvas" as const) : ("svg" as const)
      });
      qr.append(holder);
      const blob = (await qr.getRawData(fmt === "png" ? "png" : "svg")) as Blob;
      holder.remove();
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = `${asset.tag_id}_${asset.name.slice(0, 20)}.${fmt}`;
      a.click();
      URL.revokeObjectURL(a.href);
      message.success(`已下载 ${fmt.toUpperCase()}：${asset.tag_id}`);
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const TagCell = ({ asset, px }: { asset: Asset; px: number }) => {
    const boxRef = useRef<HTMLDivElement>(null);
    useEffect(() => {
      const box = boxRef.current;
      if (!box) return;
      box.innerHTML = "";
      const qr = new QRCodeStyling({
        ...styleToOptions(style, `${liveBase}/t/${asset.tag_id}`, px),
        width: px,
        height: px,
        type: "svg" as const
      });
      qr.append(box);
    }, [asset.tag_id, px, style]);
    return (
      <div className="mt-tag" style={{ width: `${tpl.tw}mm`, height: `${tpl.th}mm` }}>
        <div className="mt-tag-dl print-hide">
          <button title="下载 PNG" onClick={() => void downloadOne(asset, "png")}>PNG</button>
          <button title="下载 SVG（矢量）" onClick={() => void downloadOne(asset, "svg")}>SVG</button>
        </div>
        <div ref={boxRef} className="mt-tag-qr" style={{ width: `${tpl.qrmm}mm`, height: `${tpl.qrmm}mm` }} />
        <div className="mt-tag-text">
          {fields.includes("name") && <div className="mt-t-name">{asset.name}</div>}
          {fields.includes("tag_id") && <div className="mt-t-code">{asset.tag_id}</div>}
          {fields.includes("keeper") && asset.keeper && <div className="mt-t-line">保管人：{asset.keeper}</div>}
          {fields.includes("location") && asset.location && <div className="mt-t-line">位置：{asset.location}</div>}
          {fields.includes("sn") && asset.sn && <div className="mt-t-line">SN：{asset.sn}</div>}
        </div>
      </div>
    );
  };

  // 批量导出 PNG zip：离屏逐个渲染 getRawData
  const onZip = async () => {
    if (chosen.length === 0) return;
    setZiping(true);
    try {
      const holder = document.createElement("div");
      holder.style.cssText = "position:fixed;left:-9999px;top:-9999px;";
      document.body.appendChild(holder);
      const zip = new JSZip();
      for (const a of chosen) {
        const qr = new QRCodeStyling({
          ...styleToOptions(style, `${liveBase}/t/${a.tag_id}`, 600),
          width: 600,
          height: 600,
          type: "canvas" as const
        });
        qr.append(holder);
        const blob = (await qr.getRawData("png")) as Blob;
        zip.file(`${a.tag_id}_${a.name.slice(0, 20)}.png`, blob);
        holder.innerHTML = "";
      }
      holder.remove();
      const blob = await zip.generateAsync({ type: "blob" });
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = `MuTag活码_${chosen.length}枚.zip`;
      a.click();
      URL.revokeObjectURL(a.href);
      message.success(`已导出 ${chosen.length} 枚 PNG`);
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setZiping(false);
    }
  };

  const toggle = (tagID: string) => {
    const next = new Set(selected);
    next.has(tagID) ? next.delete(tagID) : next.add(tagID);
    setSelected(next);
  };

  const listCols: { title: string; dataIndex?: string; render?: (a: Asset) => import("react").ReactNode; width?: number }[] = [
    {
      title: "",
      width: 40,
      render: (a: Asset) => (
        <Checkbox checked={selected.has(a.tag_id)} onChange={() => toggle(a.tag_id)} />
      )
    },
    { title: "码号", dataIndex: "tag_id", width: 140 },
    { title: "名称", dataIndex: "name" },
    { title: "保管人", dataIndex: "keeper", width: 90, render: (a: Asset) => a.keeper || "-" },
    { title: "位置", dataIndex: "location", width: 110, render: (a: Asset) => a.location || "-" },
    {
      title: "状态",
      dataIndex: "status",
      width: 70,
      render: (a: Asset) => STATUS_TEXT[a.status]
    }
  ];

  return (
    <div>
      <style>{`
        @media print {
          body * { visibility: hidden !important; }
          #mt-print-sheet, #mt-print-sheet * { visibility: visible !important; }
          #mt-print-sheet { position: absolute; left: 0; top: 0; width: 210mm; }
          .mt-page { page-break-after: always; }
          .mt-page:last-child { page-break-after: auto; }
        }
        .mt-tag { display: flex; align-items: center; gap: 2mm; padding: 1.5mm; box-sizing: border-box; border: 0.2mm dashed #bbb; }
        .mt-tag-qr svg { width: 100%; height: 100%; }
        .mt-tag { position: relative; }
        .mt-tag-dl { position: absolute; top: 0; right: 0; display: none; gap: 2px; z-index: 2; }
        .mt-tag:hover .mt-tag-dl { display: flex; }
        .mt-tag-dl button { font-size: 8pt; padding: 0 4px; cursor: pointer; background: #0D9488; color: #fff; border: none; border-radius: 2px; }
        .mt-tag-text { flex: 1; min-width: 0; overflow: hidden; }
        .mt-t-name { font-weight: 600; font-size: 9pt; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
        .mt-t-code { font-family: monospace; font-size: 8pt; letter-spacing: 0.5px; }
        .mt-t-line { font-size: 7.5pt; color: #444; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
        .mt-page { display: grid; grid-template-columns: repeat(${tpl.cols}, ${tpl.tw}mm); grid-auto-rows: ${tpl.th}mm; gap: 2mm; padding: 10mm; }
        .mt-print-viewport { overflow-y: auto; }
        @media print {
          /* 缩放包裹层复位：sheet 的 absolute 定位重新锚回页面原点（与原打印语义逐字节等价） */
          .mt-print-viewport { transform: none !important; height: auto !important; max-height: none !important; overflow: visible !important; }
          #mt-print-sheet { transform: none !important; }
        }
      `}</style>

      <Space wrap style={{ marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>码版面打印</Typography.Title>
        <Statistic title="已选" value={chosen.length} valueStyle={{ color: chosen.length ? "#0D9488" : undefined }} />
        <Button icon={<ReloadOutlined />} onClick={() => void reload()}>重载资产</Button>
        <Button icon={<PrinterOutlined />} type="primary" disabled={chosen.length === 0} onClick={() => window.print()}>
          打印（A4 × {pages.length} 页）
        </Button>
        <Button icon={<DownloadOutlined />} loading={ziping} disabled={chosen.length === 0} onClick={() => void onZip()}>
          导出 PNG zip
        </Button>
      </Space>

      <Row gutter={16}>
        <Col xs={24} xl={9}>
          <Card size="small" title={`资产选择（${shown.length} 条）`} style={{ marginBottom: 16 }}>
            <Space style={{ marginBottom: 8 }} wrap>
              <Select style={{ width: 120 }} value={statusFilter} placeholder="状态筛选" options={STATUS_OPTIONS}
                onChange={(v) => setStatusFilter(v)} />
            </Space>
            <div style={{ maxHeight: 380, overflow: "auto" }}>
              {loading ? <Spin style={{ display: "block", margin: "24px auto" }} /> : (
                <table style={{ width: "100%", minWidth: 560, fontSize: 12, borderCollapse: "collapse" }}>
                  <tbody>
                    {shown.map((a) => (
                      <tr key={a.tag_id} style={{ borderBottom: "1px solid #f0f0f0" }}>
                        {listCols.map((c, i) => (
                          <td key={i} style={{ padding: "4px 6px", width: c.width }}>{c.render ? c.render(a) : (a as never)[c.dataIndex!]}</td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </Card>
        </Col>
        <Col xs={24} xl={15}>
          <Card size="small" title="模板与码样式" style={{ marginBottom: 16 }} className="print-hide">
            <Tabs
              items={[
                {
                  key: "tpl",
                  label: "标签模板",
                  children: (
                    <Space direction="vertical" style={{ width: "100%" }}>
                      <Select
                        style={{ width: 240 }}
                        value={tplKey}
                        options={TEMPLATES.map((t) => ({ value: t.key, label: t.label }))}
                        onChange={setTplKey}
                      />
                      <Checkbox.Group options={FIELD_OPTIONS} value={fields} onChange={(v) => setFields(v as string[])} />
                      <Typography.Text type="secondary">每页 {tpl.cols}×{tpl.rows} = {tpl.cols * tpl.rows} 枚 · A4 纵向 · 虚线为裁切线</Typography.Text>
                    </Space>
                  )
                },
                {
                  key: "style",
                  label: "码美化",
                  children: (
                    <Space direction="vertical" style={{ width: "100%" }} size="small">
                      <Space wrap>
                        <Typography.Text>点阵：</Typography.Text>
                        <Radio.Group value={style.dotsType} onChange={(e) => setStyle({ ...style, dotsType: e.target.value })}
                          options={[
                            { value: "square", label: "方块" }, { value: "dots", label: "圆点" },
                            { value: "rounded", label: "圆角" }, { value: "classy", label: "优雅" },
                            { value: "extra-rounded", label: "全圆" }
                          ]} optionType="button" size="small" />
                      </Space>
                      <Space wrap>
                        <Typography.Text>颜色：</Typography.Text>
                        <Radio.Group value={style.colorMode} onChange={(e) => setStyle({ ...style, colorMode: e.target.value })}
                          options={[{ value: "solid", label: "单色" }, { value: "gradient", label: "渐变" }]} optionType="button" size="small" />
                        <input type="color" value={style.color} onChange={(e) => setStyle({ ...style, color: e.target.value })} />
                        {style.colorMode === "gradient" && (
                          <input type="color" value={style.color2} onChange={(e) => setStyle({ ...style, color2: e.target.value })} />
                        )}
                      </Space>
                      <Space wrap>
                        <Typography.Text>角块：</Typography.Text>
                        <Select size="small" style={{ width: 120 }} value={style.cornerSquareType} options={[
                          { value: "square", label: "方角" }, { value: "dot", label: "圆点" }, { value: "extra-rounded", label: "圆角" }
                        ]} onChange={(v) => setStyle({ ...style, cornerSquareType: v })} />
                        <Typography.Text>容错：</Typography.Text>
                        <Radio.Group size="small" value={style.ecl} onChange={(e) => setStyle({ ...style, ecl: e.target.value })}
                          options={[{ value: "L", label: "L" }, { value: "M", label: "M" }, { value: "Q", label: "Q" }, { value: "H", label: "H（嵌 logo）" }]} optionType="button" />
                        <Checkbox checked={style.bgTransparent} onChange={(e) => setStyle({ ...style, bgTransparent: e.target.checked })}>透明背景</Checkbox>
                      </Space>
                      <Space wrap>
                        <Checkbox checked={style.logoOn} onChange={(e) => setStyle({ ...style, logoOn: e.target.checked })}>嵌入 Logo</Checkbox>
                        {style.logoOn && (
                          <input type="file" accept="image/*" onChange={(e) => {
                            const f = e.target.files?.[0];
                            if (!f) return;
                            const r = new FileReader();
                            r.onload = () => setStyle({ ...style, logoDataUrl: String(r.result) });
                            r.readAsDataURL(f);
                          }} />
                        )}
                      </Space>
                      <Alert type="info" showIcon message="容错等级 H 可承受 30% 遮挡，嵌 Logo 建议必选；样式自动保存在本机浏览器" />
                    </Space>
                  )
                }
              ]}
            />
          </Card>

          {chosen.length === 0 && (
            <Alert type="info" showIcon message="从左侧勾选需要贴码的资产，下方实时预览 A4 排版" />
          )}

          <Card
            size="small"
            id="mt-print-wrap"
            title={`打印预览（已选 ${chosen.length} 枚 · A4 纵向 × ${pages.length} 页）`}
            extra={chosen.length > 0 ? <Typography.Text type="secondary">{Math.round(fit * 100)}%</Typography.Text> : undefined}
          >
            {chosen.length === 0 ? (
              <Typography.Text type="secondary">勾选资产后预览</Typography.Text>
            ) : (
              <div ref={vpRef} className="mt-print-viewport" style={{ height: naturalH.current * fit, maxHeight: 560 }}>
                <div
                  id="mt-print-sheet"
                  ref={gridRef}
                  style={{ transform: `scale(${fit})`, transformOrigin: "top left" }}
                >
                  {pages.map((pageAssets, pi) => (
                    <div className="mt-page" key={pi}>
                      {pageAssets.map((a) => (
                        <TagCell key={a.tag_id} asset={a} px={Math.round(tpl.qrmm * 8)} />
                      ))}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
}
