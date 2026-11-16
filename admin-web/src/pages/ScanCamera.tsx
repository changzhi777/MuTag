import { useEffect, useRef, useState } from "react";
import { Alert, Button, Modal, Spin, Typography, message } from "antd";
import { ScanOutlined } from "@ant-design/icons";

// 网页摄像头扫码：BarcodeDetector API（Chrome/Edge/Safari 16.4+ 原生，零依赖）。
// 识别活码 URL（…/t/{tagID}）或裸码号（MT-YYYY-NNNNNN）→ 回调 tagID。
type BarcodeDetectorLike = { detect: (src: HTMLVideoElement) => Promise<{ rawValue: string }[]> };

const extractTag = (raw: string): string | null => {
  const m = raw.match(/\/t\/([A-Za-z0-9-]+)/);
  if (m) return m[1];
  const code = raw.match(/MT-\d{4}-\d{6}/);
  return code ? code[0] : null;
};

export default function ScanCamera({
  open,
  onClose,
  onDetected
}: {
  open: boolean;
  onClose: () => void;
  onDetected: (tagID: string) => void;
}) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [err, setErr] = useState("");
  const [ok, setOk] = useState(false);
  const [hits, setHits] = useState(0);

  useEffect(() => {
    if (!open) return;
    let stream: MediaStream | null = null;
    let raf = 0;
    let stopped = false;
    const Detector = (window as unknown as { BarcodeDetector?: new (o: { formats: string[] }) => BarcodeDetectorLike }).BarcodeDetector;

    (async () => {
      try {
        if (!Detector) {
          setErr("当前浏览器不支持原生扫码（BarcodeDetector），请使用 Chrome / Edge / Safari 16.4+，或用微信扫活码");
          return;
        }
        stream = await navigator.mediaDevices.getUserMedia({
          video: { facingMode: "environment" }
        });
        if (videoRef.current) {
          videoRef.current.srcObject = stream;
          await videoRef.current.play();
        }
        const detector = new Detector({ formats: ["qr_code"] });
        setOk(true);
        const tick = async () => {
          if (stopped || !videoRef.current) return;
          try {
            const found = await detector.detect(videoRef.current);
            for (const f of found) {
              const tag = extractTag(f.rawValue);
              if (tag) {
                stopped = true;
                setHits((h) => h + 1);
                message.success(`识别成功：${tag}`);
                onDetected(tag);
                onClose();
                return;
              }
            }
          } catch {
            /* 单帧失败忽略 */
          }
          if (!stopped) raf = requestAnimationFrame(tick);
        };
        void tick();
      } catch (e) {
        setErr((e as Error).message || "摄像头打开失败");
      }
    })();

    return () => {
      stopped = true;
      cancelAnimationFrame(raf);
      stream?.getTracks().forEach((t) => t.stop());
    };
  }, [open, onClose, onDetected]);

  return (
    <Modal
      open={open}
      onCancel={() => {
        onClose();
        setErr("");
        setOk(false);
      }}
      footer={null}
      title={
        <span>
          <ScanOutlined /> 扫码识别
        </span>
      }
      width={520}
      destroyOnClose
    >
      {err ? (
        <Alert type="warning" showIcon message={err} />
      ) : !ok ? (
        <div style={{ textAlign: "center", padding: 24 }}>
          <Spin />
          <Typography.Text type="secondary" style={{ display: "block", marginTop: 12 }}>
            正在打开摄像头…
          </Typography.Text>
        </div>
      ) : (
        <>
          <video ref={videoRef} style={{ width: "100%", borderRadius: 8, background: "#000" }} playsInline muted />
          <Typography.Text type="secondary" style={{ display: "block", marginTop: 8 }}>
            对准资产上的二维码，识别活码 URL 或裸码号（MT-YYYY-NNNNNN）后自动定位
          </Typography.Text>
          {hits > 0 && <Typography.Text type="success">已识别</Typography.Text>}
        </>
      )}
      {err && (
        <Button block style={{ marginTop: 12 }} onClick={onClose}>
          关闭
        </Button>
      )}
    </Modal>
  );
}
