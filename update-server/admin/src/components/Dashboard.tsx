import {
  CheckCircleOutlined,
  CloudDownloadOutlined,
  DesktopOutlined,
  ExclamationCircleOutlined,
} from "@ant-design/icons";
import {
  Avatar,
  Badge,
  Card,
  Col,
  Empty,
  Flex,
  List,
  Row,
  Statistic,
  Tag,
  Typography,
  theme,
} from "antd";
import type { ReactNode } from "react";
import { formatBytes, formatNumber } from "../format";
import type { DashboardData } from "../types";
import { TrendChart } from "./TrendChart";

interface DashboardProps {
  data: DashboardData;
}

export function Dashboard({ data }: DashboardProps) {
  const { token } = theme.useToken();

  return (
    <Flex vertical gap={24}>
      <Flex className="page-heading" align="flex-start" justify="space-between" gap={16} wrap>
        <div>
          <Typography.Title level={2}>更新观测台</Typography.Title>
          <Typography.Paragraph type="secondary">汇总发布版本、客户端安装结果与资源传输情况</Typography.Paragraph>
        </div>
        <Tag color="processing">当前发布：{data.active_version ?? "暂无"}</Tag>
      </Flex>

      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} xl={6}>
          <SummaryCard
            title="成功更新"
            value={data.totals.successful_updates}
            icon={<CheckCircleOutlined />}
            iconColor={token.colorSuccess}
            iconBackground={token.colorSuccessBg}
            note="install_success 事件"
          />
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <SummaryCard
            title="设备数"
            value={data.totals.updated_installations}
            icon={<DesktopOutlined />}
            iconColor={token.colorPrimary}
            iconBackground={token.colorPrimaryBg}
            note="匿名安装标识去重"
          />
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <SummaryCard
            title="失败次数"
            value={data.totals.failed_updates}
            icon={<ExclamationCircleOutlined />}
            iconColor={token.colorError}
            iconBackground={token.colorErrorBg}
            note="install_failed 事件"
          />
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <SummaryCard
            title="资源请求"
            value={data.totals.download_requests}
            icon={<CloudDownloadOutlined />}
            iconColor={token.colorInfo}
            iconBackground={token.colorInfoBg}
            note={`累计传输 ${formatBytes(data.totals.requested_bytes)}`}
          />
        </Col>
      </Row>

      <Row gutter={[16, 16]} align="stretch">
        <Col xs={24} xl={16}>
          <Card title="近 30 天成功更新" extra={<Typography.Text type="secondary">按服务端接收日期统计</Typography.Text>} className="fill-card">
            <TrendChart rows={data.daily_updates} />
          </Card>
        </Col>
        <Col xs={24} xl={8}>
          <Card title="主要失败原因" extra={<Typography.Text type="secondary">累计排名</Typography.Text>} className="fill-card">
            {data.failures.length ? (
              <List
                dataSource={data.failures.slice(0, 7)}
                renderItem={failure => (
                  <List.Item extra={<Tag color="error">{formatNumber(failure.count)}</Tag>}>
                    <List.Item.Meta
                      avatar={<Badge status="error" />}
                      title={<Typography.Text code>{failure.code}</Typography.Text>}
                      description={failure.reason || "未分类"}
                    />
                  </List.Item>
                )}
              />
            ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无失败记录" />}
          </Card>
        </Col>
      </Row>

    </Flex>
  );
}

function SummaryCard({ title, value, icon, iconColor, iconBackground, note }: {
  title: string;
  value: number;
  icon: ReactNode;
  iconColor: string;
  iconBackground: string;
  note: string;
}) {
  return (
    <Card className="summary-card">
      <Flex align="flex-start" justify="space-between" gap={16}>
        <Statistic title={title} value={value} />
        <Avatar size={40} icon={icon} style={{ color: iconColor, background: iconBackground }} />
      </Flex>
      <Typography.Text type="secondary">{note}</Typography.Text>
    </Card>
  );
}
