import { EyeOutlined } from "@ant-design/icons";
import {
  Badge,
  Button,
  Card,
  Empty,
  Flex,
  Input,
  Progress,
  Segmented,
  Space,
  Table,
  Tag,
  Typography,
  type TableColumnsType,
} from "antd";
import { useMemo, useState } from "react";
import { formatBytes, formatDateTime, formatNumber, successRate } from "../format";
import type { DashboardData, VersionStatus, VersionSummary } from "../types";

interface VersionStatisticsProps {
  data: DashboardData;
  onOpenVersion: (version: string) => void;
}

export function VersionStatistics({ data, onOpenVersion }: VersionStatisticsProps) {
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
          <Typography.Text code>{version}</Typography.Text>
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
      width: 108,
      render: (_, row) => (
        <Button type="link" icon={<EyeOutlined />} onClick={() => onOpenVersion(row.version)}>查看详情</Button>
      ),
    },
  ];

  return (
    <Flex vertical gap={24}>
      <Flex className="page-heading" align="flex-start" justify="space-between" gap={16} wrap>
        <div>
          <Typography.Title level={2}>版本更新统计</Typography.Title>
          <Typography.Paragraph type="secondary">按目标版本查看设备安装结果、成功率与资源传输情况</Typography.Paragraph>
        </div>
        <Tag color="processing">共 {formatNumber(data.versions.length)} 个版本</Tag>
      </Flex>

      <Card styles={{ body: { padding: 0 } }}>
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
