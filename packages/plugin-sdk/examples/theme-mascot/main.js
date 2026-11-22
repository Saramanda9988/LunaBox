import { defineThemePreset, Box, Text } from 'lunabox';

const sakuraPreset = defineThemePreset({
  light: {
    appBg: '#fff7fb',
    sidebarBg: '#fff0f6',
    cardBg: '#ffffff',
    cardBorder: '#fbcfe8',
    primaryBtn: '#db2777',
    primaryBtnHover: '#be185d',
    navActiveBg: '#fce7f3',
    navActive: '#9d174d',
  },
  dark: {
    appBg: '#18111b',
    sidebarBg: '#221728',
    cardBg: '#261a2d',
    cardBorder: '#4c2c57',
    primaryBtn: '#ec4899',
    primaryBtnHover: '#db2777',
    navActiveBg: '#3b2147',
    navActive: '#fbcfe8',
  },
});

export function apply(ctx) {
  ctx.config.define({
    title: '樱花主题设置',
    fields: [
      { key: 'watermarkOpacity', type: 'number', label: '水印不透明度（%）', default: 25, min: 5, max: 100, step: 5 },
    ],
  });

  ctx.theme.extend(sakuraPreset);

  ctx.theme.registerDecoration({
    id: 'sakura-watermark',
    layer: 'foreground',
    anchor: 'bottom-right',
    offset: { x: 20, y: 12 },
    size: { width: 160, height: 48 },
    opacity: 0.35,
    pointerEvents: 'none',
    routes: ['/', '/library'],
    render: () => Box({ padding: 'sm', children: [
      Text({ text: 'Sakura Theme', size: 'sm', weight: 'semibold', tone: 'brand' }),
    ]}),
  });
}
