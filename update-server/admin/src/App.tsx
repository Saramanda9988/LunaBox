import {
  App as AntApp,
  Avatar,
  Button,
  ConfigProvider,
  Flex,
  Layout,
  Menu,
  Skeleton,
  Space,
  Tag,
  Typography,
  theme,
} from "antd";
import {
  BarChartOutlined,
  DashboardOutlined,
  LogoutOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import zhCN from "antd/locale/zh_CN";
import { lazy, Suspense, useCallback, useEffect, useState } from "react";
import { clearToken, getDashboard, readToken, saveToken, UnauthorizedError } from "./api";
import { LoginView } from "./components/LoginView";
import { formatDateTime } from "./format";
import type { DashboardData } from "./types";

const Dashboard = lazy(async () => {
  const module = await import("./components/Dashboard");
  return { default: module.Dashboard };
});

const VersionStatistics = lazy(async () => {
  const module = await import("./components/VersionStatistics");
  return { default: module.VersionStatistics };
});

const ReleaseDetail = lazy(async () => {
  const module = await import("./components/ReleaseDetail");
  return { default: module.ReleaseDetail };
});

type AdminRoute =
  | { page: "dashboard" }
  | { page: "releases" }
  | { page: "release-detail"; version: string };

function routeFromURL(): AdminRoute {
  const pathname = window.location.pathname.replace(/\/+$/, "") || "/";
  if (pathname === "/admin/releases")
    return { page: "releases" };

  const detailMatch = pathname.match(/^\/admin\/releases\/([^/]+)$/);
  if (detailMatch) {
    try {
      return { page: "release-detail", version: decodeURIComponent(detailMatch[1]) };
    }
    catch {
      return { page: "releases" };
    }
  }

  return { page: "dashboard" };
}

export default function App() {
  return (
    <ConfigProvider
      locale={zhCN}
      theme={{
        algorithm: theme.defaultAlgorithm,
        cssVar: {},
        token: {
          colorPrimary: "#1677ff",
          colorSuccess: "#52c41a",
          colorWarning: "#faad14",
          colorError: "#ff4d4f",
          borderRadius: 6,
          fontSize: 14,
        },
      }}
    >
      <AntApp>
        <AppContent />
      </AntApp>
    </ConfigProvider>
  );
}

function AppContent() {
  const { message } = AntApp.useApp();
  const { token: designToken } = theme.useToken();
  const [token, setToken] = useState(readToken);
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(Boolean(token));
  const [error, setError] = useState("");
  const [route, setRoute] = useState<AdminRoute>(routeFromURL);
  const [collapsed, setCollapsed] = useState(false);

  const signOut = useCallback(() => {
    clearToken();
    setToken("");
    setData(null);
    setError("");
    setRoute({ page: "dashboard" });
    history.replaceState(null, "", "/admin/");
  }, []);

  const load = useCallback(async (credential: string, quiet = false) => {
    if (!quiet)
      setLoading(true);
    setError("");
    try {
      const dashboard = await getDashboard(credential);
      setData(dashboard);
      if (quiet)
        message.success("数据已更新");
    }
    catch (reason) {
      if (reason instanceof UnauthorizedError) {
        clearToken();
        setToken("");
        setData(null);
        setRoute({ page: "dashboard" });
        history.replaceState(null, "", "/admin/");
      }
      setError(reason instanceof Error ? reason.message : "控制台数据读取失败");
    }
    finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    if (token)
      void load(token);
  }, [load, token]);

  useEffect(() => {
    const onPopState = () => setRoute(routeFromURL());
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  const authenticate = useCallback((credential: string) => {
    saveToken(credential);
    setToken(credential);
    setLoading(true);
  }, []);

  const navigate = useCallback((nextRoute: AdminRoute) => {
    const pathname = nextRoute.page === "dashboard"
      ? "/admin/"
      : nextRoute.page === "releases"
        ? "/admin/releases"
        : `/admin/releases/${encodeURIComponent(nextRoute.version)}`;
    history.pushState(null, "", pathname);
    setRoute(nextRoute);
    window.scrollTo({ top: 0, behavior: "smooth" });
  }, []);

  const openVersion = useCallback((version: string) => {
    navigate({ page: "release-detail", version });
  }, [navigate]);

  const closeVersion = useCallback(() => navigate({ page: "releases" }), [navigate]);

  if (!token || !data)
    return <LoginView loading={loading} error={error} onSubmit={authenticate} />;

  const selectedMenuKey = route.page === "dashboard" ? "dashboard" : "releases";
  return (
    <Layout className="app-layout">
      <Layout.Sider
        theme="light"
        width={224}
        breakpoint="lg"
        collapsedWidth={64}
        collapsed={collapsed}
        onCollapse={setCollapsed}
      >
        <div className="app-logo">
          <Avatar shape="square" size={36} style={{ background: designToken.colorPrimary }}>L</Avatar>
          {collapsed ? null : (
            <div>
              <Typography.Text strong>LunaBox</Typography.Text>
              <Typography.Text type="secondary">更新控制台</Typography.Text>
            </div>
          )}
        </div>
        <Menu
          mode="inline"
          selectedKeys={[selectedMenuKey]}
          items={[
            { key: "dashboard", icon: <DashboardOutlined />, label: "首页大盘" },
            { key: "releases", icon: <BarChartOutlined />, label: "版本更新统计" },
          ]}
          onClick={({ key }) => {
            if (key === "dashboard")
              navigate({ page: "dashboard" });
            else
              navigate({ page: "releases" });
          }}
        />
      </Layout.Sider>

      <Layout>
        <Layout.Header className="app-header" style={{ background: designToken.colorBgContainer }}>
          <Typography.Text type="secondary">
            数据更新时间：{formatDateTime(data.generated_at)}
          </Typography.Text>
          <Space>
            <Tag color="blue">当前发布 {data.active_version ?? "暂无"}</Tag>
            <Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load(token, true)}>刷新</Button>
            <Button icon={<LogoutOutlined />} onClick={signOut}>退出</Button>
          </Space>
        </Layout.Header>
        <Layout.Content className="app-content">
          <Suspense fallback={<Skeleton active paragraph={{ rows: 10 }} />}>
            {route.page === "release-detail"
              ? <ReleaseDetail token={token} version={route.version} onBack={closeVersion} onUnauthorized={signOut} />
              : route.page === "releases"
                ? <VersionStatistics data={data} onOpenVersion={openVersion} />
                : <Dashboard data={data} />}
          </Suspense>
        </Layout.Content>
      </Layout>
    </Layout>
  );
}
