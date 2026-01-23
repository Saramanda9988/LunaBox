import { defineConfig, presetIcons, presetWind3 } from "unocss";

export default defineConfig({
  presets: [
    presetWind3({
      dark: "class",
    }),
    presetIcons(),
  ],

  // 自定义 variants - 支持 data-glass 属性
  variants: [
    // data-glass variant: 当元素或父元素有 data-glass="true" 时生效
    (matcher) => {
      if (!matcher.startsWith("data-glass:"))
        return matcher;

      return {
        matcher: matcher.slice(11), // 移除 'data-glass:' 前缀
        selector: s => `[data-glass="true"] ${s}, ${s}[data-glass="true"]`,
      };
    },
  ],

  shortcuts: [
    // 玻璃态效果基础类
    {
      "glass": "backdrop-filter backdrop-blur-20 backdrop-saturate-180",
      "glass-border": "border border-white/18 dark:border-white/10",
      "glass-text": "drop-shadow-[0_1px_2px_rgba(0,0,0,0.3)] drop-shadow-[0_0_8px_rgba(0,0,0,0.2)]",
    },

    // 玻璃态层级系统（从不透明到透明）

    // 1. glass-aside - 侧边栏（最不透明，需要清晰的导航）
    [/^glass-aside$/, () => "data-glass:bg-white/12 data-glass:dark:bg-black/15 data-glass:backdrop-blur-28 data-glass:backdrop-saturate-180 data-glass:border-r data-glass:border-white/20 data-glass:dark:border-white/12"],

    // 2. glass-btn - 按钮（保持可见，需要明确的交互反馈）
    [/^glass-btn-(.*)$/, ([, color]) => {
      const colorMap: Record<string, string> = {
        neutral: "data-glass:bg-neutral-600/65",
        error: "data-glass:bg-error-500/65",
        success: "data-glass:bg-success-600/65",
        primary: "data-glass:bg-primary-600/65",
      };
      return `data-glass:backdrop-blur-12 data-glass:border data-glass:border-white/25 data-glass:dark:border-white/15 ${colorMap[color] || "data-glass:bg-neutral-600/65"}`;
    }],

    // 3. glass-card - 卡片（统计卡、列表项等，中等透明）
    [/^glass-card$/, () => "data-glass:bg-white/8 data-glass:dark:bg-black/12 data-glass:backdrop-blur-20 data-glass:backdrop-saturate-180 data-glass:border data-glass:border-white/22 data-glass:dark:border-white/12"],

    // 4. glass-panel - 面板容器（较透明，轻量感）
    [/^glass-panel$/, () => "data-glass:bg-white/5 data-glass:dark:bg-black/8 data-glass:backdrop-blur-20 data-glass:backdrop-saturate-180 data-glass:border data-glass:border-white/18 data-glass:dark:border-white/10"],

    // 5. glass-input - 输入框（最透明，突出内容）
    [/^glass-input$/, () => "data-glass:bg-white/3 data-glass:dark:bg-black/5 data-glass:backdrop-blur-16 data-glass:backdrop-saturate-150 data-glass:border data-glass:border-white/25 data-glass:dark:border-white/18"],
  ],

  theme: {
    colors: {
      // 基础灰度色板
      brand: {
        50: "#fdfdfdff",
        100: "#f3f4f6",
        200: "#e5e7eb",
        300: "#d1d5db",
        400: "#9ca3af",
        500: "#6b7280",
        600: "#4b5563",
        700: "#44484eff",
        750: "#303235ff",
        800: "#1c1e1fff",
        900: "#121416ff",
      },
      // 主色调 (primary) - 月光紫
      primary: {
        50: "#F5F3FF",
        100: "#EDE9FE",
        200: "#DDD6FE",
        300: "#C4B5FD",
        400: "#A78BFA",
        500: "#7C6AEF",
        600: "#6D5DD3",
        700: "#5B4CB8",
        800: "#4A3D96",
        900: "#2E2660",
      },
      // 强调色 (Accent) - 星光蓝
      accent: {
        50: "#EFF6FF",
        100: "#DBEAFE",
        200: "#BFDBFE",
        300: "#93C5FD",
        400: "#60A5FA",
        500: "#3B82F6",
        600: "#2563EB",
        700: "#1D4ED8",
        800: "#1E40AF",
        900: "#1E3A8A",
      },
      // 中性色 (Neutral) - 月夜灰
      neutral: {
        50: "#F8FAFC",
        100: "#F1F5F9",
        200: "#E2E8F0",
        300: "#CBD5E1",
        400: "#94A3B8",
        500: "#64748B",
        600: "#475569",
        700: "#334155",
        800: "#1E293B",
        900: "#0F172A",
      },
      // 成功色 (Success) - 极光绿
      success: {
        50: "#ECFDF5",
        100: "#D1FAE5",
        200: "#A7F3D0",
        300: "#6EE7B7",
        400: "#34D399",
        500: "#10B981",
        600: "#059669",
        700: "#047857",
        800: "#065F46",
        900: "#064E3B",
      },
      // 警告色 (Warning) - 晨曦金
      warning: {
        50: "#FFFBEB",
        100: "#FEF3C7",
        200: "#FDE68A",
        300: "#FCD34D",
        400: "#FBBF24",
        500: "#F59E0B",
        600: "#D97706",
        700: "#B45309",
        800: "#92400E",
        900: "#78350F",
      },
      // 错误色 (Error) - 玫瑰红
      error: {
        50: "#FEF2F2",
        100: "#FEE2E2",
        200: "#FECACA",
        300: "#FCA5A5",
        400: "#F87171",
        500: "#EF4444",
        600: "#DC2626",
        700: "#B91C1C",
        800: "#991B1B",
        900: "#7F1D1D",
      },
      // 信息色 (Info) - 冰蓝
      info: {
        50: "#F0F9FF",
        100: "#E0F2FE",
        200: "#BAE6FD",
        300: "#7DD3FC",
        400: "#38BDF8",
        500: "#0EA5E9",
        600: "#0284C7",
        700: "#0369A1",
        800: "#075985",
        900: "#0C181D",
      },
    },
  },
});
