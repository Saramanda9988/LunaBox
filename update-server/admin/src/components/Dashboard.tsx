import {
  CheckCircleOutlined,
  CloudDownloadOutlined,
  DesktopOutlined,
  ExclamationCircleOutlined,
  EyeOutlined,
} from "@ant-design/icons";
import {
  Avatar,
  Badge,
  Button,
  Card,
  Col,
  Empty,
  Flex,
  Input,
  List,
  Progress,
  Row,
  Segmented,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
  theme,
  type TableColumnsType,
} from "antd";
import { useMemo, useState, type ReactNode } from "react";
import { formatBytes, formatDateTime, formatNumber, successRate } from "../format";
import type { DashboardData, VersionStatus, VersionSummary } from "../types";
import { TrendChart } from "./TrendChart";

interface DashboardProps {
  data: DashboardData;
  onOpenVersion: (version: string) => void;
}

export function Dashboard({ data, onOpenVersion }: DashboardProps) {
  const { token } = theme.useToken();
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState<VersionStatus>("all");
  const versions = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return data.versions.filter(row => {
      if (normalized && !row.version.toLowerCase().includes(normalized))
        return false;
      if (status === "active")
        return row.version === data.active_version;
      if (status === "failed")
        return row.install_failed > 0;
      if (status === "healthy")
        return row.install_success > 0 && row.install_failed === 0;
      return true;
    });
  }, [data.active_version, data.versions, query, status]);

  const columns: TableColumnsType<VersionSummary> = [
    {
      title: "目标版本",
      dataIndex: "version",
      width: 180,
      fixed: "left",
      render: (version: string) => (
        <Space size={8}>
          <Button type="link" className="table-link" onClick={() => onOpenVersion(version)}>{version}</Button>
          {version === data.active_version ? <Tag color="blue">当前发布</Tag> : null}
        </Space>
      ),
    },
    {
      title: "设备",
      dataIndex: "devices",
      width: 96,
      align: "right",
      sorter: (left, right) => left.devices - right.devices,
      render: formatNumber,
    },
    {
      title: "成功率",
      key: "success_rate",
      width: 180,
      sorter: (left, right) => successRate(left.install_success, left.install_failed) - successRate(right.install_success, right.install_failed),
      render: (_, row) => {
        const rate = successRate(row.install_success, row.install_failed);
        return <Progress percent={rate} size="small" status={row.install_failed > 0 ? "exception" : "normal"} />;
      },
    },
    {
      title: "成功",
      dataIndex: "install_success",
      width: 96,
      align: "right",
      sorter: (left, right) => left.install_success - right.install_success,
      render: (value: number) => <Typography.Text type="success">{formatNumber(value)}</Typography.Text>,
    },
    {
      title: "失败",
      dataIndex: "install_failed",
      width: 96,
      align: "right",
      sorter: (left, right) => left.install_failed - right.install_failed,
      render: (value: number) => <Typography.Text type={value ? "danger" : "secondary"}>{formatNumber(value)}</Typography.Text>,
    },
    {
      title: "资源请求",
      dataIndex: "download_requests",
      width: 112,
      align: "right",
      render: formatNumber,
    },
    {
      title: "传输量",
      dataIndex: "requested_bytes",
      width: 112,
      align: "right",
      render: formatBytes,
    },
    {
      title: "最近事件",
      dataIndex: "last_event_at",
      width: 184,
      render: formatDateTime,
    },
    {
      title: "操作",
      key: "action",
      fixed: "right",
      width: 96,
      render: (_, row) => (
        <Button type="link" icon={<EyeOutlined />} onClick={() => onOpenVersion(row.version)}>查看</Button>
      ),
    },
  ];

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

      <Card
        title="版本更新统计"
        extra={<Typography.Text type="secondary">共 {formatNumber(data.versions.length)} 个版本</Typography.Text>}
        styles={{ body: { padding: 0 } }}
      >
        <Flex className="table-toolbar" align="center" justify="space-between" gap={16} wrap>
          <Input.Search
            allowClear
            placeholder="输入版本号"
            value={query}
            onChange={event => setQuery(event.target.value)}
            className="version-search"
          />
          <Segmented
            value={status}
            onChange={value => setStatus(value as VersionStatus)}
            options={[
              { label: "全部", value: "all" },
              { label: "当前发布", value: "active" },
              { label: "含失败", value: "failed" },
              { label: "全部成功", value: "healthy" },
            ]}
          />
        </Flex>
        <Table
          columns={columns}
          dataSource={versions}
          rowKey="version"
          scroll={{ x: 1150 }}
          pagination={{ pageSize: 10, showSizeChanger: false, hideOnSinglePage: true }}
          locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无匹配版本" /> }}
          onRow={row => ({ onDoubleClick: () => onOpenVersion(row.version) })}
        />
      </Card>

      {data.invalid_manifests.length ? (
        <Card size="small">
          <Badge status="warning" text={`${data.invalid_manifests.length} 个发布目录缺少有效清单：${data.invalid_manifests.join("、")}`} />
        </Card>
      ) : null}
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
