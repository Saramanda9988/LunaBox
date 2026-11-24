// Hina UI（https://hinaui.dev）的设计令牌，取自其发布的 CSS 变量（--color-* 与 --hn-*）。
// 半透明的描边、柔和填充等在宿主中需要不透明色，这里按 Hina 的混合规则预先合成到对应主题的 surface 上，
// 注释里给出原始写法，便于与 Hina 更新对照。

/** Hina 品牌青色色阶（--color-brand-*） */
export const brand = {
  50: '#eefdfb',
  100: '#d5faf5',
  200: '#a9f3eb',
  300: '#72e8dc',
  400: '#45d4c9',
  500: '#39c5bb',
  600: '#2ba79e',
  700: '#1f827b',
  800: '#1a6862',
  900: '#185450',
  950: '#0a302e',
} as const;

/** Hina 中性灰色阶（--color-neutral-*），比宿主默认的冷灰更“纯” */
export const neutral = {
  0: '#ffffff',
  50: '#fafafa',
  100: '#f5f5f5',
  150: '#ededed',
  200: '#e5e5e5',
  300: '#d4d4d4',
  400: '#a3a3a3',
  500: '#737373',
  600: '#525252',
  700: '#404040',
  800: '#262626',
  850: '#1c1c1c',
  900: '#171717',
  950: '#0a0a0a',
  1000: '#000000',
} as const;

/** 一套主题下 Hina 的语义角色 */
export interface HinaRoles {
  canvas: string; // --hn-bg-canvas：页面底
  surface: string; // --hn-surface：承载内容的面（浅色纯白、深色纯黑）
  subtle: string; // --hn-bg-subtle：轻微区分
  inset: string; // --hn-bg-inset：凹陷区域
  border: string; // --hn-border
  borderStrong: string; // --hn-border-strong
  fgMax: string; // --hn-fg-max
  fg: string; // --hn-fg-default
  fgMuted: string; // --hn-fg-muted
  fgSubtle: string; // --hn-fg-subtle
  fgDisabled: string; // --hn-fg-disabled
  hover: string; // 中性元素悬停（--hn-state-hover-opacity 叠加前景色）
  accent: string; // --hn-accent：主操作、选中、当前状态
  accentHover: string; // accent 上叠加 accent-on 的悬停层
  accentOn: string; // --hn-accent-on：实心 accent 上的文字（Hina 规定用深色而不是白色）
  accentText: string; // --hn-accent-text：浅底上的 accent 文字
  accentSoft: string; // --hn-accent-soft
  accentBorder: string; // --hn-accent-border
  focusRing: string; // --hn-focus-ring
  neutralSolid: string; // --hn-neutral-solid（提示气泡等反色块）
  neutralSolidOn: string; // --hn-neutral-solid-on
}

export const light: HinaRoles = {
  canvas: neutral[50],
  surface: neutral[0],
  subtle: neutral[100],
  inset: neutral[150],
  border: '#ebebeb', // #00000014 合成到白色
  borderStrong: '#d9d9d9', // #00000026 合成到白色
  fgMax: neutral[1000],
  fg: neutral[900],
  fgMuted: neutral[600],
  fgSubtle: neutral[500],
  fgDisabled: neutral[400],
  hover: neutral[100],
  accent: brand[500],
  accentHover: '#36bab0', // accent-on 6% 叠加
  accentOn: neutral[950],
  accentText: brand[700],
  accentSoft: '#e7f8f7', // color-mix(brand-500 12%, surface)
  accentBorder: '#bcebe8', // #39c5bb57 合成到白色
  focusRing: brand[600],
  neutralSolid: neutral[900],
  neutralSolidOn: neutral[0],
};

export const dark: HinaRoles = {
  canvas: neutral[950],
  surface: neutral[1000],
  subtle: '#141414',
  inset: neutral[800],
  border: '#1a1a1a', // #ffffff1a 合成到黑色
  borderStrong: '#2e2e2e', // #ffffff2e 合成到黑色
  fgMax: neutral[0],
  fg: neutral[150],
  fgMuted: neutral[400],
  fgSubtle: neutral[500],
  fgDisabled: neutral[600],
  hover: '#141414',
  accent: brand[400],
  accentHover: '#3fc1b7', // accent-on 9% 叠加
  accentOn: neutral[1000],
  accentText: brand[400],
  accentSoft: '#113330', // color-mix(brand-400 24%, surface)
  accentBorder: '#15403c', // #45d4c94d 合成到黑色
  focusRing: brand[400],
  neutralSolid: neutral[150],
  neutralSolidOn: neutral[950],
};
