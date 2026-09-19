import { LockOutlined } from "@ant-design/icons";
import { Alert, Avatar, Button, Card, Flex, Form, Input, Typography, theme } from "antd";

interface LoginViewProps {
  loading: boolean;
  error: string;
  onSubmit: (token: string) => void;
}

export function LoginView({ loading, error, onSubmit }: LoginViewProps) {
  const { token } = theme.useToken();
  return (
    <Flex
      className="login-page"
      align="center"
      justify="center"
      style={{ background: token.colorBgLayout }}
    >
      <Card className="login-card" variant="borderless">
        <Flex vertical align="center" gap={8} className="login-heading">
          <Avatar shape="square" size={48} style={{ background: token.colorPrimary, fontSize: 24 }}>L</Avatar>
          <Typography.Title level={2}>LunaBox 更新控制台</Typography.Title>
          <Typography.Text type="secondary">查看发布状态、客户端更新进度与失败详情</Typography.Text>
        </Flex>
        <Form
          layout="vertical"
          size="large"
          onFinish={(values: { token: string }) => onSubmit(values.token.trim())}
          requiredMark={false}
        >
          <Form.Item
            label="管理令牌"
            name="token"
            rules={[{ required: true, message: "请输入管理令牌" }]}
          >
            <Input.Password
              autoComplete="current-password"
              prefix={<LockOutlined />}
              placeholder="ADMIN_TOKEN"
            />
          </Form.Item>
          {error ? <Alert className="login-alert" message={error} type="error" showIcon /> : null}
          <Button type="primary" htmlType="submit" loading={loading} block>
            登录
          </Button>
        </Form>
        <Typography.Paragraph className="login-note" type="secondary">
          管理令牌保存在当前浏览器标签页，关闭标签页后自动清除。
        </Typography.Paragraph>
      </Card>
    </Flex>
  );
}
