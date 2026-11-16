import { useEffect, useState } from "react";
import { Card, Col, Progress, Row, Statistic, message } from "antd";
import { api } from "../api/client";
import { STATUS_TEXT, type AssetStatus } from "../types";

interface StatsData {
  total: number;
  by_status: { status: string; count: number }[];
}

export default function Stats() {
  const [data, setData] = useState<StatsData | null>(null);

  useEffect(() => {
    api<StatsData>("/stats")
      .then(setData)
      .catch((e) => message.error((e as Error).message));
  }, []);

  const statuses = (data?.by_status ?? []).filter(
    (s) => s.status in STATUS_TEXT
  ) as { status: AssetStatus; count: number }[];

  return (
    <div>
      <Row gutter={[16, 16]}>
        <Col span={6}>
          <Card>
            <Statistic title="资产总数" value={data?.total ?? "…"} />
          </Card>
        </Col>
        {statuses.map((s) => (
          <Col span={6} key={s.status}>
            <Card>
              <Statistic
                title={STATUS_TEXT[s.status]}
                value={s.count}
                valueStyle={{ color: s.status === "scrapped" ? "#999" : undefined }}
              />
              <Progress
                percent={data?.total ? Math.round((s.count / data.total) * 100) : 0}
                strokeColor={s.status === "scrapped" ? "#d9d9d9" : undefined}
                showInfo={false}
                size="small"
              />
            </Card>
          </Col>
        ))}
      </Row>
    </div>
  );
}
