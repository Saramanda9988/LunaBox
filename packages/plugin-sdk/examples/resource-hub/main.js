import { useState, Column, Row, Input, Button, Grid, Card, Text, Badge, Empty } from 'lunabox';

export function apply(ctx) {
  function ResourceHubPage() {
    const [keyword, setKeyword] = useState('');
    const [items, setItems] = useState([
      { id: 'demo-1', name: '示例资源包 A', size: '1.2 GB', url: 'https://example.com/demo-a.zip', source: 'vndb', sourceId: 'v17' },
      { id: 'demo-2', name: '示例资源包 B', size: '2.4 GB', url: 'https://example.com/demo-b.7z', source: 'bangumi', sourceId: '100' },
    ]);

    const filtered = keyword.trim()
      ? items.filter((it) => it.name.toLowerCase().includes(keyword.trim().toLowerCase()))
      : items;

    return Column({ gap: 'lg', children: [
      Row({ gap: 'sm', align: 'center', children: [
        Input({ k: 'hub-q', value: keyword, placeholder: '搜索资源站作品...', onChange: setKeyword }),
        Button({ k: 'hub-refresh', label: '刷新', icon: 'refresh', onPress: () => ctx.ui.toast('列表已刷新', 'info') }),
      ]}),
      filtered.length === 0
        ? Empty({ title: '未找到匹配资源', description: '请更换关键词重试' })
        : Grid({ minColWidth: 260, gap: 'md', children: filtered.map((item) =>
            Card({ k: item.id, title: item.name, hoverable: true, children: [
              Row({ gap: 'sm', align: 'center', children: [
                Badge({ label: item.size, tone: 'brand' }),
                Text({ text: `${item.source}:${item.sourceId}`, size: 'xs', tone: 'muted' }),
              ]}),
              Button({
                k: 'dl-' + item.id,
                label: '下载并导入',
                icon: 'download',
                variant: 'primary',
                size: 'sm',
                onPress: async () => {
                  await ctx.downloads.requestInstall({
                    name: item.name,
                    downloadUrl: item.url,
                    source: item.source,
                    sourceId: item.sourceId,
                  });
                },
              }),
            ]})
          )}),
    ]});
  }

  ctx.ui.slot('sidebar.page', {
    id: 'hub',
    title: '资源广场',
    icon: 'web',
    render: ResourceHubPage,
  });

  ctx.ui.slot('library.add_menu', {
    id: 'from-hub',
    title: '从资源广场下载导入',
    icon: 'download',
    onPress: () => ctx.ui.navigate('/plugin/example-resource-hub/hub'),
  });
}
