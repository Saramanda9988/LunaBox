import { useState, Column, Row, Tabs, Markdown, Badge, Button, Collapsible } from 'lunabox';

export function apply(ctx) {
  function SiteDetailTab({ game }) {
    const [tab, setTab] = useState('intro');
    const name = (game && game.name) || '未知游戏';

    return Column({ gap: 'md', children: [
      Tabs({
        k: 'site-tabs',
        value: tab,
        items: [
          { key: 'intro', label: '站点图文' },
          { key: 'patch', label: '汉化与补丁' },
        ],
        onChange: setTab,
      }),
      tab === 'intro'
        ? Markdown({
            k: 'md-intro',
            content: `# ${name}\n\n> 该内容由站点详情插件动态扩展渲染。\n\n- 支持标题、列表、引用、代码块与链接点击\n- 可调用 \`ctx.html.parse\` 或 \`fetch\` 拉取外部站点详情`,
            onLinkPress: (url) => ctx.shell.openURL(url),
          })
        : Collapsible({
            k: 'patch-box',
            title: '可用补丁说明',
            defaultOpen: true,
            children: [
              Markdown({ content: '- 可通过 `ctx.downloads.requestInstall` 一键下载补丁或本体' }),
            ],
          }),
    ]});
  }

  ctx.ui.slot('game.detail.tab', {
    id: 'site-detail',
    title: '站点详情',
    render: SiteDetailTab,
  });

  ctx.ui.slot('game.detail.header', {
    id: 'site-badge',
    render: ({ game }) => Row({ gap: 'sm', align: 'center', children: [
      Badge({ label: '已连接扩展站点', tone: 'brand', icon: 'web' }),
      Button({
        k: 'open-site',
        label: '在浏览器查看',
        variant: 'ghost',
        size: 'sm',
        icon: 'open-in-new',
        onPress: () => ctx.shell.openURL('https://www.kungal.com/'),
      }),
    ]}),
  });
}
