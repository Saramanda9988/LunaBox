import type { GameRuntimeInfo } from "../src/store";
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { StrictMode, useState } from "react";
import { createRoot } from "react-dom/client";
import previewCover from "../src/assets/branding/luna1.webp";
import { PlayingIsland } from "../src/components/bar/PlayingIsland";
import { GameCard } from "../src/components/card/GameCard";
import { BetterDataTable } from "../src/components/ui/better/BetterDataTable";
import { BetterInput } from "../src/components/ui/better/BetterInput";
import { BetterSelect } from "../src/components/ui/better/BetterSelect";
import { CollapsibleSection } from "../src/components/ui/CollapsibleSection";
import { useAppStore } from "../src/store";
import "../src/i18n";
import "@unocss/reset/tailwind.css";
import "../src/style.css";
import "virtual:uno.css";

const params = new URLSearchParams(window.location.search);
const glass = !params.has("solid");
const dark = params.has("dark");
const previewGame = {
  id: "preview",
  name: "玻璃材质测试游戏",
  company: "LunaBox Studio",
  cover_url: previewCover,
} as NonNullable<GameRuntimeInfo["game"]>;
document.documentElement.classList.toggle("dark", dark);
document.documentElement.style.fontSize = `${Number(params.get("font")) || 16}px`;
useAppStore.setState({
  gameRuntimes: {
    test: {
      game: { id: "test", name: "LunaBox 测试游戏" } as GameRuntimeInfo["game"],
      gameId: "test",
      sessionId: "test",
      startTime: null,
      state: "launching",
    },
  },
  activeGameRuntimeId: "test",
});

export function Fixture() {
  const [value, setValue] = useState("system");
  return (
    <div
      className="relative h-screen w-full overflow-hidden"
      data-glass={glass ? "true" : "false"}
      data-native-webkit={params.has("webkit") ? "true" : "false"}
    >
      {glass && (
        <div
          className="absolute inset-0"
          style={{
            background:
              "repeating-linear-gradient(120deg, #6d356d 0 100px, #315b7b 100px 200px, #dab882 200px 300px, #99b6a1 300px 400px)",
            filter: "blur(10px)",
            transform: "scale(1.1)",
          }}
        />
      )}
      <div className="relative flex h-full w-full flex-col text-brand-900 dark:text-brand-100">
        <header className="relative z-50 h-[28px] shrink-0 bg-brand-50 text-center dark:bg-brand-800">
          LunaBox
        </header>
        {!params.has("no-island") && (
          <div data-testid="island-host" className="contents">
            <PlayingIsland />
          </div>
        )}
        <div className="relative flex-1 overflow-hidden">
          <div className="absolute left-0 top-0 h-full w-full shrink-0">
            <div className="flex h-full w-full overflow-hidden">
              <aside className="glass-aside w-14 shrink-0 bg-brand-50 dark:bg-brand-800" />
              <main
                data-testid="content"
                className="app-content-viewport @container flex-1 overflow-auto"
                style={{
                  containerType: "inline-size",
                  backgroundColor: glass
                    ? "rgba(var(--main-bg-rgb), 0.45)"
                    : "rgb(var(--main-bg-rgb))",
                }}
              >
                <div className="mx-auto max-w-5xl space-y-6 p-8">
                  <h1 className="text-4xl font-bold">设置</h1>
                  <CollapsibleSection title="基本设置" icon="i-mdi-cog-outline">
                    <div className="grid grid-cols-2 gap-4">
                      {["Bangumi", "Hikarinagi"].map(name => (
                        <div
                          key={name}
                          data-testid="account-card"
                          className="glass-panel relative isolate h-40 min-w-0 overflow-hidden rounded-2xl border border-brand-200/80 bg-white/55 p-4 dark:border-brand-700/80 dark:bg-brand-900/25"
                        >
                          <strong>{name}</strong>
                          <p className="text-xs text-brand-500 dark:text-brand-400">
                            账号设置
                          </p>
                          <span className="absolute bottom-4 right-4 text-4xl font-bold text-primary-500/30">
                            {name}
                          </span>
                        </div>
                      ))}
                    </div>
                    <label className="block space-y-2 text-sm font-medium text-brand-700 dark:text-brand-300">
                      <span>VNDB Access Token</span>
                      <BetterInput placeholder="输入访问令牌" />
                    </label>
                    {["主题", "语言", "界面缩放", "时区"].map(label => (
                      <div key={label} className="space-y-2">
                        <span className="block text-sm font-medium text-brand-700 dark:text-brand-300">
                          {label}
                        </span>
                        <BetterSelect
                          value={value}
                          onChange={setValue}
                          options={[
                            { value: "system", label: "跟随系统" },
                            { value: "custom", label: "自定义" },
                          ]}
                        />
                        <p className="text-xs text-brand-500 dark:text-brand-400">
                          设置会立即生效，并保存到应用配置。
                        </p>
                      </div>
                    ))}
                  </CollapsibleSection>
                  <CollapsibleSection
                    title="元数据设置"
                    icon="i-mdi-database-search"
                  >
                    <div className="glass-card rounded-lg bg-white p-6 dark:bg-brand-800">
                      <h2 className="font-semibold">游戏信息</h2>
                      <p className="text-sm text-brand-600 dark:text-brand-300">
                        使用统一玻璃卡片的内容区域。
                      </p>
                    </div>
                    <BetterDataTable
                      rows={[
                        { id: "one", name: "Bangumi" },
                        { id: "two", name: "Hikarinagi" },
                      ]}
                      rowKey={row => row.id}
                      columns={[
                        {
                          key: "name",
                          header: "元数据来源",
                          render: row => row.name,
                        },
                      ]}
                    />
                  </CollapsibleSection>
                  <div data-testid="game-card" className="w-48">
                    <GameCard game={previewGame} />
                  </div>
                  <div className="h-screen" />
                </div>
              </main>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

const router = createRouter({
  routeTree: createRootRoute({ component: Fixture }),
  history: createMemoryHistory({ initialEntries: ["/"] }),
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
);
