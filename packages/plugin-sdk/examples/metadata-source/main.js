export function apply(ctx) {
  ctx.config.define({
    title: 'VNDB 镜像设置',
    fields: [
      { key: 'baseUrl', type: 'string', label: '镜像站 API 地址', default: 'https://api.vndb.org/kana' },
      { key: 'apiToken', type: 'secret', label: 'API Token（可选）', default: '' },
    ],
  });

  ctx.metadata.registerSource({
    id: 'vndb-mirror',
    name: { 'zh-CN': 'VNDB 镜像', 'en-US': 'VNDB Mirror' },
    link: (sourceId) => `https://vndb.org/${sourceId}`,
    normalizeId: (input) => (/^v?\d+$/.test(input.trim()) ? 'v' + input.trim().replace(/^v/i, '') : null),
    rateLimit: { perSecond: 2 },
    async fetch(sourceId) {
      const cfg = ctx.config.get();
      const base = String(cfg.baseUrl || 'https://api.vndb.org/kana').replace(/\/$/, '');
      const resp = await fetch(`${base}/vn`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          filters: ['id', '=', sourceId],
          fields: 'id, title, alttitle, released, rating, description, image.url, developers.name',
        }),
      });
      const data = await resp.json();
      const vn = data.results && data.results[0];
      if (!vn) throw new Error(`未找到 ${sourceId}`);
      return {
        game: {
          name: vn.alttitle || vn.title,
          source_id: vn.id,
          company: (vn.developers && vn.developers[0] && vn.developers[0].name) || '',
          summary: vn.description || '',
          rating: vn.rating ? Number((vn.rating / 10).toFixed(2)) : 0,
          release_date: vn.released || '',
          cover_url: (vn.image && vn.image.url) || '',
        },
        tags: [],
      };
    },
    async search(name, { limit }) {
      const cfg = ctx.config.get();
      const base = String(cfg.baseUrl || 'https://api.vndb.org/kana').replace(/\/$/, '');
      const resp = await fetch(`${base}/vn`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          filters: ['search', '=', name],
          fields: 'id, title, alttitle, released, rating, description, image.url, developers.name',
          results: limit || 5,
        }),
      });
      const data = await resp.json();
      return (data.results || []).map((vn) => ({
        game: {
          name: vn.alttitle || vn.title,
          source_id: vn.id,
          company: (vn.developers && vn.developers[0] && vn.developers[0].name) || '',
          summary: vn.description || '',
          rating: vn.rating ? Number((vn.rating / 10).toFixed(2)) : 0,
          release_date: vn.released || '',
          cover_url: (vn.image && vn.image.url) || '',
        },
        tags: [],
      }));
    },
  });
}
