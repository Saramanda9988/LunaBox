import {
  ArrowLeftOutlined,
  CheckCircleOutlined,
  CloudDownloadOutlined,
  DesktopOutlined,
  ExclamationCircleOutlined,
  FilterOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import {
  Alert,
  Avatar,
  Button,
  Card,
  Col,
  Descriptions,
  Empty,
  Flex,
  Input,
  List,
  Progress,
  Row,
  Select,
  Skeleton,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
  theme,
  type TableColumnsType,
} from "antd";
import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { getReleaseDetails, UnauthorizedError } from "../api";
import { eventLabels, formatBytes, formatDateTime, formatNumber, shortID, successRate } from "../format";
import type { EventType, ReleaseDetailData, ReleaseEvent, ReleaseFilters } from "../types";

interface ReleaseDetailProps {
  token: string;
  version: string;
  onBack: () => void;
  onUnauthorized: () => void;
}

const eventOptions = Object.entries(eventLabels).map(([value, label]) => ({ value, label }));

export function ReleaseDetail({ token, version, onBack, onUnauthorized }: ReleaseDetailProps) {
  const { token: designToken } = theme.useToken();
  const [draft, setDraft] = useState<ReleaseFilters>({ page: 1, page_size: 20 });
  const [filters, setFilters] = useState<ReleaseFilters>({ page: 1, page_size: 20 });
  const [data, setData] = useState<ReleaseDetailData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [requestKey, setRequestKey] = useState(0);

  useEffect(() => {
    setDraft({ page: 1, page_size: 20 });
    setFilters({ page: 1, page_size: 20 });
  }, [version]);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    getReleaseDetails(token, version, filters, controller.signal)
      .then(setData)
      .catch((reason: unknown) => {
        if (controller.signal.aborted)
          return;
        if (reason instanceof UnauthorizedError) {
          onUnauthorized();
          return;
        }
        setError(reason instanceof Error ? reason.message : "版本数据读取失败");
      })
      .finally(() => {
        if (!controller.signal.aborted)
          setLoading(false);
      });
    return () => controller.abort();
  }, [filters, onUnauthorized, requestKey, token, version]);

  const applyFilters = useCallback(() => {
    setFilters({ ...draft, page: 1, page_size: filters.page_size ?? 20 });
  }, [draft, filters.page_size]);

  const resetFilters = useCallback(() => {
    const initial = { page: 1, page_size: filters.page_size ?? 20 };
    setDraft(initial);
    setFilters(initial);
  }, [filters.page_size]);

  const columns = useMemo<TableColumnsType<ReleaseEvent>>(() => [
    {
      title: "状态",
      dataIndex: "event_type",
      width: 116,
      fixed: "left",
      render: (value: EventType) => <EventTag type={value} />,
    },
    {
      title: "接收时间",
      dataIndex: "created_at",
      width: 184,
      sorter: (left, right) => left.created_at.localeCompare(right.created_at),
      render: formatDateTime,
    },
    {
      title: "来源版本",
      dataIndex: "current_version",
      width: 120,
      render: (value: string | null) => value ?? "—",
    },
    { title: "渠道", dataIndex: "channel", width: 200, ellipsis: true },
    { title: "架构", dataIndex: "architecture", width: 88 },
    { title: "类型", dataIndex: "build_mode", width: 96 },
    {
      title: "安装标识",
      dataIndex: "installation_id",
      width: 112,
      render: (value: string | null, row) => <Typography.Text code>{shortID(value ?? row.transaction_id)}</Typography.Text>,
    },
    {
      title: "传输量",
      dataIndex: "transferred_bytes",
      width: 104,
      align: "right",
      render: formatBytes,
    },
    {
      title: "失败代码",
      dataIndex: "failure_code",
      width: 152,
      ellipsis: true,
      render: (value: string | null) => value ? <Typography.Text type="danger">{value}</Typography.Text> : "—",
    },
    {
      title: "失败原因",
      dataIndex: "failure_reason",
      width: 192,
      ellipsis: true,
      render: (value: string | null) => value ?? "—",
    },
  ], []);

  if (loading && !data)
    return <Card><Skeleton active paragraph={{ rows: 12 }} /></Card>;

  if (!data) {
    return (
      <Flex vertical gap={16}>
        <Button className="back-button" icon={<ArrowLeftOutlined />} onClick={onBack}>返回版本列表</Button>
        <Alert type="error" message={error || "版本数据读取失败"} showIcon />
      </Flex>
    );
  }

  const finished = data.summary.install_success + data.summary.install_failed;
  const rate = successRate(data.summary.install_success, data.summary.install_failed);
  const stageItems = [
    { label: "发现更新", value: data.summary.update_available, color: designToken.colorInfo },
    { label: "开始下载", value: data.summary.download_started, color: designToken.colorPrimary },
    { label: "校验完成", value: data.summary.download_verified, color: designToken.colorWarning },
    { label: "安装成功", value: data.summary.install_success, color: designToken.colorSuccess },
    { label: "安装失败", value: data.summary.install_failed, color: designToken.colorError },
  ];

  return (
    <Flex vertical gap={24}>
      <Flex className="page-heading" align="flex-start" justify="space-between" gap={16} wrap>
        <Space align="start" size={16}>
          <Button icon={<ArrowLeftOutlined />} onClick={onBack}>返回</Button>
          <div>
            <Space size={8} wrap>
              <Typography.Title level={2}>{version}</Typography.Title>
              <Tag color="blue">版本详情</Tag>
            </Space>
            <Typography.Paragraph type="secondary">
              {data.summary.first_event_at
                ? `${formatDateTime(data.summary.first_event_at)} 至 ${formatDateTime(data.summary.last_event_at)}`
                : "尚无事件时间范围"}
            </Typography.Paragraph>
          </div>
        </Space>
        <Button icon={<ReloadOutlined />} loading={loading} onClick={() => setRequestKey(value => value + 1)}>刷新数据</Button>
      </Flex>

      {error ? <Alert type="error" message={error} showIcon /> : null}

      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} xl={6}>
          <SummaryCard title="识别设备" value={data.summary.devices} note="按匿名安装标识去重" icon={<DesktopOutlined />} color={designToken.colorPrimary} background={designToken.colorPrimaryBg} />
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <SummaryCard title="安装成功" value={data.summary.install_success} note={`成功率 ${rate}%`} icon={<CheckCircleOutlined />} color={designToken.colorSuccess} background={designToken.colorSuccessBg} />
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <SummaryCard title="安装失败" value={data.summary.install_failed} note={`${formatNumber(data.failure_groups.length)} 类失败组合`} icon={<ExclamationCircleOutlined />} color={designToken.colorError} background={designToken.colorErrorBg} />
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <SummaryCard title="事件传输量" value={data.summary.transferred_bytes} formatter={formatBytes} note={`${formatNumber(data.summary.events)} 条筛选结果`} icon={<CloudDownloadOutlined />} color={designToken.colorInfo} background={designToken.colorInfoBg} />
        </Col>
      </Row>

      <Card title="更新统计" extra={<Typography.Text type="secondary">完成事件 {formatNumber(finished)}</Typography.Text>}>
        <Row gutter={[24, 24]} align="middle">
          <Col xs={24} lg={6}>
            <Flex vertical align="center" gap={8}>
              <Progress type="dashboard" percent={rate} strokeColor={designToken.colorSuccess} />
              <Typography.Text type="secondary">安装成功率</Typography.Text>
            </Flex>
          </Col>
          <Col xs={24} lg={18}>
            <Row gutter={[16, 16]}>
              {stageItems.map(item => (
                <Col xs={12} md={8} xl={4} key={item.label}>
                  <Statistic title={item.label} value={item.value} valueStyle={{ color: item.color }} />
                </Col>
              ))}
            </Row>
          </Col>
        </Row>
      </Card>

      <Row gutter={[16, 16]} align="stretch">
        <Col xs={24} xl={14}>
          <Card title="失败分类" extra={<Typography.Text type="secondary">点击记录进行筛选</Typography.Text>} className="fill-card">
            {data.failure_groups.length ? (
              <Table
                size="small"
                rowKey={row => `${row.code}-${row.reason}`}
                pagination={false}
                dataSource={data.failure_groups}
                columns={[
                  { title: "失败代码", dataIndex: "code", render: value => <Typography.Text code>{value}</Typography.Text> },
                  { title: "失败原因", dataIndex: "reason", ellipsis: true },
                  { title: "次数", dataIndex: "count", align: "right", width: 72, render: value => <Tag color="error">{formatNumber(value)}</Tag> },
                  { title: "最近发生", dataIndex: "last_seen_at", width: 176, render: formatDateTime },
                ]}
                onRow={group => ({
                  onClick: () => {
                    const next = { ...draft, event_type: "install_failed" as const, failure_code: group.code, failure_reason: group.reason };
                    setDraft(next);
                    setFilters({ ...next, page: 1, page_size: filters.page_size ?? 20 });
                  },
                })}
              />
            ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无失败记录" />}
          </Card>
        </Col>
        <Col xs={24} xl={10}>
          <Card title="发布资源" extra={<Typography.Text type="secondary">R2 发布清单</Typography.Text>} className="fill-card">
            {data.release ? (
              <Flex vertical gap={16}>
                <Descriptions size="small" column={1}>
                  <Descriptions.Item label="上传时间">{formatDateTime(data.release.uploaded_at)}</Descriptions.Item>
                  <Descriptions.Item label="补丁来源">{formatNumber(data.release.patches.length)} 个版本</Descriptions.Item>
                </Descriptions>
                {data.release.patches.length ? (
                  <List
                    size="small"
                    dataSource={data.release.patches.flatMap(relation => relation.channels.map(channel => ({ ...channel, sourceVersion: relation.source_version })))}
                    renderItem={item => (
                      <List.Item>
                        <List.Item.Meta
                          title={<Space><Tag>{item.sourceVersion}</Tag><Typography.Text>→</Typography.Text><Tag color="blue">{version}</Tag></Space>}
                          description={`${item.name} · ${item.asset}`}
                        />
                        <Progress type="circle" size={48} percent={item.saving_percent} />
                      </List.Item>
                    )}
                  />
                ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="此版本未包含增量补丁" />}
              </Flex>
            ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="未读取到有效发布清单" />}
          </Card>
        </Col>
      </Row>

      <Card
        title="事件明细"
        extra={<Typography.Text type="secondary">支持组合条件查询</Typography.Text>}
        styles={{ body: { padding: 0 } }}
      >
        <div className="event-filters">
          <Row gutter={[12, 12]}>
            <Col xs={24} sm={12} lg={8} xl={4}>
              <Select allowClear placeholder="事件状态" options={eventOptions} value={draft.event_type} onChange={value => setDraft(current => ({ ...current, event_type: value }))} />
            </Col>
            <Col xs={24} sm={12} lg={8} xl={4}>
              <Select allowClear showSearch placeholder="渠道" options={data.dimensions.channels.map(item => ({ value: item.value, label: `${item.value} · ${item.count}` }))} value={draft.channel} onChange={value => setDraft(current => ({ ...current, channel: value }))} />
            </Col>
            <Col xs={24} sm={12} lg={8} xl={4}>
              <Select allowClear placeholder="架构" options={data.dimensions.architectures.map(item => ({ value: item.value, label: `${item.value} · ${item.count}` }))} value={draft.architecture} onChange={value => setDraft(current => ({ ...current, architecture: value }))} />
            </Col>
            <Col xs={24} sm={12} lg={8} xl={4}>
              <Select allowClear placeholder="构建类型" options={data.dimensions.build_modes.map(item => ({ value: item.value, label: `${item.value} · ${item.count}` }))} value={draft.build_mode} onChange={value => setDraft(current => ({ ...current, build_mode: value }))} />
            </Col>
            <Col xs={24} sm={12} lg={8} xl={4}>
              <Select allowClear showSearch placeholder="失败代码" options={data.dimensions.failure_codes.map(item => ({ value: item.value, label: `${item.value} · ${item.count}` }))} value={draft.failure_code} onChange={value => setDraft(current => ({ ...current, failure_code: value }))} />
            </Col>
            <Col xs={24} sm={12} lg={8} xl={4}>
              <Input allowClear placeholder="失败原因" value={draft.failure_reason} onChange={event => setDraft(current => ({ ...current, failure_reason: event.target.value || undefined }))} onPressEnter={applyFilters} />
            </Col>
          </Row>
          <Flex justify="flex-end" gap={8} className="filter-actions">
            <Button onClick={resetFilters}>重置</Button>
            <Button type="primary" icon={<FilterOutlined />} onClick={applyFilters}>查询</Button>
          </Flex>
        </div>
        <Table
          columns={columns}
          dataSource={data.events}
          rowKey="event_id"
          loading={loading}
          scroll={{ x: 1360 }}
          pagination={{
            current: data.pagination.page,
            pageSize: data.pagination.page_size,
            total: data.pagination.total,
            showSizeChanger: true,
            showTotal: total => `共 ${formatNumber(total)} 条`,
            onChange: (page, pageSize) => {
              const next = { ...filters, page, page_size: pageSize };
              setDraft(current => ({ ...current, page, page_size: pageSize }));
              setFilters(next);
            },
          }}
          locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无匹配事件" /> }}
          expandable={{
            expandedRowRender: event => (
              <Descriptions size="small" column={{ xs: 1, sm: 2, lg: 3 }}>
                <Descriptions.Item label="事件 ID"><Typography.Text code>{event.event_id}</Typography.Text></Descriptions.Item>
                <Descriptions.Item label="事务 ID"><Typography.Text code>{event.transaction_id ?? "—"}</Typography.Text></Descriptions.Item>
                <Descriptions.Item label="安装标识"><Typography.Text code>{event.installation_id ?? "历史数据未上报"}</Typography.Text></Descriptions.Item>
                <Descriptions.Item label="客户端时间">{formatDateTime(event.client_time)}</Descriptions.Item>
                <Descriptions.Item label="资源">{event.artifact ?? "—"}</Descriptions.Item>
                <Descriptions.Item label="目标版本">{event.target_version}</Descriptions.Item>
              </Descriptions>
            ),
          }}
        />
      </Card>
    </Flex>
  );
}

function EventTag({ type }: { type: EventType }) {
  const color = type === "install_success" ? "success" : type === "install_failed" ? "error" : type === "download_verified" ? "cyan" : "blue";
  return <Tag color={color}>{eventLabels[type]}</Tag>;
}

function SummaryCard({ title, value, formatter, note, icon, color, background }: {
  title: string;
  value: number;
  formatter?: (value: number) => string;
  note: string;
  icon: ReactNode;
  color: string;
  background: string;
}) {
  return (
    <Card className="summary-card">
      <Flex align="flex-start" justify="space-between" gap={16}>
        <Statistic title={title} value={formatter ? formatter(value) : value} />
        <Avatar size={40} icon={icon} style={{ color, background }} />
      </Flex>
      <Typography.Text type="secondary">{note}</Typography.Text>
    </Card>
  );
}
