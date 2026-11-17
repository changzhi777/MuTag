import { useCallback, useEffect, useState } from "react";
import { App, Tabs, Typography } from "antd";
import { api } from "../api/client";
import type { FinMapping } from "../types";
import MappingEditor from "./finance/MappingEditor";
import ExportPanel from "./finance/ExportPanel";
import ImportPanel from "./finance/ImportPanel";

// 财务对接页：模板配置 / 导出（审计留痕）/ 导入（差异预览·确认·回滚）
export default function Finance() {
  const { message } = App.useApp();
  const [mappings, setMappings] = useState<FinMapping[]>([]);

  const reload = useCallback(async () => {
    try {
      const res = await api<{ items: FinMapping[] }>("/fin/mappings");
      setMappings(res.items);
    } catch (e) {
      message.error((e as Error).message);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  return (
    <div>
      <Typography.Title level={4}>财务对接</Typography.Title>
      <Tabs
        defaultActiveKey="import"
        items={[
          {
            key: "import",
            label: "财务导入",
            children: <ImportPanel mappings={mappings} />
          },
          {
            key: "export",
            label: "财务导出",
            children: <ExportPanel mappings={mappings} />
          },
          {
            key: "mapping",
            label: "模板配置",
            children: <MappingEditor mappings={mappings} onReload={reload} />
          }
        ]}
      />
    </div>
  );
}
